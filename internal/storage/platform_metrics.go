package storage

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/jackc/pgx/v5"
)

func normalizePlatform(targetType string) string {
	t := strings.ToLower(strings.TrimSpace(targetType))
	if t == "web" {
		return "frontend"
	}
	if t == "" {
		return "docker"
	}
	return t
}

func (p *Postgres) InsertPlatformExperiment(exp *models.Experiment) error {
	if exp == nil {
		return nil
	}

	platform := normalizePlatform(exp.TargetType)
	targetsJSON, _ := json.Marshal(exp.Targets)
	observedJSON, _ := json.Marshal(exp.ObservedEndpoints)

	switch platform {
	case "android":
		_, err := p.Pool.Exec(context.Background(), `
			INSERT INTO android_experiments (
				experiment_id, fault_type, state, phase,
				package_name, activity, apk_path, targets,
				created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (experiment_id) DO UPDATE
			SET fault_type = EXCLUDED.fault_type,
				state = EXCLUDED.state,
				phase = EXCLUDED.phase,
				package_name = EXCLUDED.package_name,
				activity = EXCLUDED.activity,
				apk_path = EXCLUDED.apk_path,
				targets = EXCLUDED.targets,
				updated_at = EXCLUDED.updated_at
		`,
			exp.ID,
			exp.FaultType,
			exp.State,
			exp.Phase,
			exp.Package,
			exp.Activity,
			exp.APKPath,
			string(targetsJSON),
			exp.CreatedAt,
			exp.UpdatedAt,
		)
		return err
	case "frontend":
		targetURLsJSON, _ := json.Marshal([]string{})
		baseURL := ""
		metricsEndpoint := ""
		if exp.FrontendRun != nil {
			targetURLsJSON, _ = json.Marshal(exp.FrontendRun.TargetURLs)
			baseURL = exp.FrontendRun.BaseURL
			metricsEndpoint = exp.FrontendRun.MetricsEndpoint
		}

		_, err := p.Pool.Exec(context.Background(), `
			INSERT INTO frontend_experiments (
				experiment_id, fault_type, state, phase,
				base_url, metrics_endpoint, target_urls, targets,
				created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (experiment_id) DO UPDATE
			SET fault_type = EXCLUDED.fault_type,
				state = EXCLUDED.state,
				phase = EXCLUDED.phase,
				base_url = EXCLUDED.base_url,
				metrics_endpoint = EXCLUDED.metrics_endpoint,
				target_urls = EXCLUDED.target_urls,
				targets = EXCLUDED.targets,
				updated_at = EXCLUDED.updated_at
		`,
			exp.ID,
			exp.FaultType,
			exp.State,
			exp.Phase,
			baseURL,
			metricsEndpoint,
			string(targetURLsJSON),
			string(targetsJSON),
			exp.CreatedAt,
			exp.UpdatedAt,
		)
		return err
	default:
		_, err := p.Pool.Exec(context.Background(), `
			INSERT INTO backend_experiments (
				experiment_id, fault_type, state, phase,
				targets, observed_endpoints, expected_service_down,
				created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (experiment_id) DO UPDATE
			SET fault_type = EXCLUDED.fault_type,
				state = EXCLUDED.state,
				phase = EXCLUDED.phase,
				targets = EXCLUDED.targets,
				observed_endpoints = EXCLUDED.observed_endpoints,
				expected_service_down = EXCLUDED.expected_service_down,
				updated_at = EXCLUDED.updated_at
		`,
			exp.ID,
			exp.FaultType,
			exp.State,
			exp.Phase,
			string(targetsJSON),
			string(observedJSON),
			exp.ExpectedServiceDown,
			exp.CreatedAt,
			exp.UpdatedAt,
		)
		return err
	}
}

func (p *Postgres) InsertPlatformRawMetrics(targetType string, samples []models.MetricSample, expID string) error {
	if len(samples) == 0 {
		return nil
	}

	platform := normalizePlatform(targetType)
	batch := &pgx.Batch{}

	for _, s := range samples {
		switch platform {
		case "android":
			batch.Queue(`
				INSERT INTO android_metrics_raw (
					experiment_id, endpoint, timestamp,
					app_state, crash, anr, crash_reason,
					memory_mb, frame_drops, latency_ms, status, intensity
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			`,
				expID,
				s.Endpoint,
				s.Timestamp,
				s.AppState,
				s.Crash,
				s.ANR,
				s.CrashReason,
				s.MemoryMB,
				s.FrameDrops,
				s.LatencyMs,
				s.Status,
				s.Intensity,
			)
		default:
			batch.Queue(`
				INSERT INTO backend_metrics_raw (
					experiment_id, endpoint, timestamp,
					latency_ms, status, intensity, container_cpu, container_memory
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			`,
				expID,
				s.Endpoint,
				s.Timestamp,
				s.LatencyMs,
				s.Status,
				s.Intensity,
				s.ContainerCPU,
				s.ContainerMemoryMB,
			)
		}
	}

	br := p.Pool.SendBatch(context.Background(), batch)
	return br.Close()
}

func (p *Postgres) InsertFrontendMetricsBatch(samples []models.FrontendMetrics) error {
	if len(samples) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, s := range samples {
		apiCallsJSON, _ := json.Marshal(s.APICalls)
		batch.Queue(`
			INSERT INTO frontend_metrics_raw (
				experiment_id, phase, page,
				lcp, cls, inp,
				long_tasks, errors, unhandled_rejections,
				api_calls, event_timestamp
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11)
		`,
			s.ExperimentID,
			s.Phase,
			s.Page,
			s.Metrics.LCP,
			s.Metrics.CLS,
			s.Metrics.INP,
			s.Metrics.LongTasks,
			s.Metrics.Errors,
			s.Metrics.UnhandledRejections,
			string(apiCallsJSON),
			s.TimeStamp,
		)
	}

	br := p.Pool.SendBatch(context.Background(), batch)
	return br.Close()
}

func (p *Postgres) InsertPlatformStatusMetrics(exp *models.Experiment, data map[string]interface{}) error {
	if exp == nil || data == nil {
		return nil
	}

	payloadJSON, _ := json.Marshal(data)
	failsafeScore := 0.0
	if fsIdx, ok := data["failsafe_index"].(map[string]interface{}); ok {
		failsafeScore = toFloat(fsIdx["score"])
	}

	platform := normalizePlatform(exp.TargetType)
	switch platform {
	case "android":
		health := toMap(data["health"])
		recovery := toMap(data["recovery"])
		validation := toMap(data["validation"])
		summary := toMap(data["summary"])
		_, err := p.Pool.Exec(context.Background(), `
			INSERT INTO android_status_metrics (
				experiment_id, state, phase,
				health_status, severity,
				recovered, recovery_time_ms,
				validation_passed, summary_result,
				failsafe_score, status_payload, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,NOW())
			ON CONFLICT (experiment_id) DO UPDATE
			SET state = EXCLUDED.state,
				phase = EXCLUDED.phase,
				health_status = EXCLUDED.health_status,
				severity = EXCLUDED.severity,
				recovered = EXCLUDED.recovered,
				recovery_time_ms = EXCLUDED.recovery_time_ms,
				validation_passed = EXCLUDED.validation_passed,
				summary_result = EXCLUDED.summary_result,
				failsafe_score = EXCLUDED.failsafe_score,
				status_payload = EXCLUDED.status_payload,
				updated_at = NOW()
		`,
			exp.ID,
			exp.State,
			exp.Phase,
			toString(health["status"]),
			toString(health["severity"]),
			toBool(recovery["recovered"]),
			toInt64(recovery["recovery_time_ms"]),
			toNullableBool(validation["passed"]),
			toString(summary["result"]),
			failsafeScore,
			string(payloadJSON),
		)
		return err
	case "frontend":
		frontendScore := toMap(data["frontend_score"])
		_, err := p.Pool.Exec(context.Background(), `
			INSERT INTO frontend_status_metrics (
				experiment_id, state, phase,
				frontend_score, frontend_status,
				failsafe_score, status_payload, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,NOW())
			ON CONFLICT (experiment_id) DO UPDATE
			SET state = EXCLUDED.state,
				phase = EXCLUDED.phase,
				frontend_score = EXCLUDED.frontend_score,
				frontend_status = EXCLUDED.frontend_status,
				failsafe_score = EXCLUDED.failsafe_score,
				status_payload = EXCLUDED.status_payload,
				updated_at = NOW()
		`,
			exp.ID,
			exp.State,
			exp.Phase,
			toFloat(frontendScore["score"]),
			toString(frontendScore["status"]),
			failsafeScore,
			string(payloadJSON),
		)
		return err
	default:
		_, err := p.Pool.Exec(context.Background(), `
			INSERT INTO backend_status_metrics (
				experiment_id, state, phase,
				blast_radius, cascade_depth, system_severity,
				failsafe_score, status_payload, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,NOW())
			ON CONFLICT (experiment_id) DO UPDATE
			SET state = EXCLUDED.state,
				phase = EXCLUDED.phase,
				blast_radius = EXCLUDED.blast_radius,
				cascade_depth = EXCLUDED.cascade_depth,
				system_severity = EXCLUDED.system_severity,
				failsafe_score = EXCLUDED.failsafe_score,
				status_payload = EXCLUDED.status_payload,
				updated_at = NOW()
		`,
			exp.ID,
			exp.State,
			exp.Phase,
			toFloat(data["blast_radius_percent"]),
			toInt(data["cascade_depth"]),
			toString(data["system_severity"]),
			failsafeScore,
			string(payloadJSON),
		)
		return err
	}
}
