package storage

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

func NewPostgres(connStr string) (*Postgres, error) {

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, err
	}

	p := &Postgres{Pool: pool}

	// Attempt to apply local schema file if present. This is safe to run repeatedly
	// because `schema.sql` uses CREATE TABLE IF NOT EXISTS and ALTER TABLE IF NOT EXISTS.
	if schemaBytes, err := os.ReadFile("internal/storage/schema.sql"); err == nil {
		schema := string(schemaBytes)
		// execute schema SQL; ignore individual statement errors but return on fatal Exec error
		if _, execErr := p.Pool.Exec(context.Background(), schema); execErr != nil {
			return nil, execErr
		}
	}

	return p, nil
}

func (p *Postgres) InsertExperiment(exp *models.Experiment) error {

	query := `
	INSERT INTO experiments (
		id, api_key_id, fault_type, state, phase,
		created_at, updated_at,
		max_intensity, breaking_intensity, max_stable_intensity,
		baseline, dependency_graph, target_endpoint_map
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13
	)`
	var apiKeyID any
	if strings.TrimSpace(exp.APIKeyID) != "" {
		apiKeyID = exp.APIKeyID
	}

	_, err := p.Pool.Exec(context.Background(), query,
		exp.ID,
		apiKeyID,
		exp.FaultType,
		exp.State,
		exp.Phase,
		exp.CreatedAt,
		exp.UpdatedAt,
		exp.MaxIntensity,
		exp.BreakingIntensity,
		exp.MaxStableIntensity,
		exp.Baseline,
		exp.DependencyGraph,
		exp.TargetEndpointMap,
	)
	if err != nil {
		return err
	}

	return p.InsertPlatformExperiment(exp)
}

func (p *Postgres) UpdateBaseline(exp *models.Experiment) error {

	query := `
	UPDATE experiments
	SET baseline = $1,
	    updated_at = $2
	WHERE id = $3
	`

	_, err := p.Pool.Exec(context.Background(), query,
		exp.Baseline,
		time.Now(),
		exp.ID,
	)

	return err
}

func (p *Postgres) UpdateExperimentResults(exp *models.Experiment) error {

	query := `
	UPDATE experiments
	SET breaking_intensity = $1,
	    max_stable_intensity = $2,
	    state = $3,
	    phase = $4,
	    updated_at = $5
	WHERE id = $6
	`

	_, err := p.Pool.Exec(context.Background(), query,
		exp.BreakingIntensity,
		exp.MaxStableIntensity,
		exp.State,
		exp.Phase,
		time.Now(),
		exp.ID,
	)

	return err
}

func (p *Postgres) InsertMetricsBatch(samples []models.MetricSample, expID string) error {

	if len(samples) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, s := range samples {

		batch.Queue(`
			INSERT INTO metrics_raw (
				experiment_id,
				endpoint,
				timestamp,
				latency_ms,
				status,
				intensity,
				container_cpu,
				container_memory
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

	br := p.Pool.SendBatch(context.Background(), batch)
	return br.Close()
}

func (p *Postgres) InsertAggregatedMetrics(
	expID string,
	data map[string]interface{},
) error {
	endpointsRaw, ok := data["endpoints"]
	if !ok || endpointsRaw == nil {
		return p.insertAndroidAggregatedMetrics(expID, data)
	}

	endpoints, ok := endpointsRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	batch := &pgx.Batch{}

	for endpoint, raw := range endpoints {
		ep, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		lat, _ := ep["latency"].(map[string]interface{})
		errs, _ := ep["errors"].(map[string]interface{})
		derived, _ := ep["derived"].(map[string]interface{})
		container, _ := ep["container"].(map[string]interface{})

		if lat == nil || errs == nil || derived == nil || container == nil {
			continue
		}

		batch.Queue(`
			INSERT INTO metrics_aggregated (
				experiment_id,
				endpoint,
				requests_total,
				p50_ms,
				p95_ms,
				p99_ms,
				avg_ms,
				stddev_ms,
				jitter_ms,
				error_rate,
				max_failure_streak,
				latency_ratio,
				error_delta,
				stability_score,
				impact_order,
				degraded,
				avg_cpu,
				max_cpu,
				avg_memory,
				max_memory
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
				$11,$12,$13,$14,$15,$16,$17,$18,$19,$20
			)
		`,
			expID,
			endpoint,
			ep["requests_total"],
			lat["p50_ms"],
			lat["p95_ms"],
			lat["p99_ms"],
			lat["avg_ms"],
			lat["stddev_ms"],
			lat["jitter_ms"],
			errs["rate_percent"],
			errs["max_failure_streak"],
			derived["latency_ratio"],
			derived["error_delta"],
			ep["stability_score"],
			ep["impact_order"],
			ep["degraded"],
			container["avg_cpu_percent"],
			container["max_cpu_percent"],
			container["avg_memory_mb"],
			container["max_memory_mb"],
		)
	}

	br := p.Pool.SendBatch(context.Background(), batch)
	return br.Close()
}

func (p *Postgres) insertAndroidAggregatedMetrics(expID string, data map[string]interface{}) error {
	if !strings.EqualFold(toString(data["target_type"]), "android") {
		return nil
	}

	endpoint := "android"
	if timeline, ok := data["timeline"].(map[string]interface{}); ok {
		if firstImpact, ok := timeline["first_impact"].(map[string]interface{}); ok {
			for ep := range firstImpact {
				if strings.TrimSpace(ep) != "" {
					endpoint = ep
					break
				}
			}
		}
	}

	health, _ := data["health"].(map[string]interface{})
	stability, _ := data["stability"].(map[string]interface{})

	status := strings.ToLower(toString(health["status"]))
	degraded := status != "healthy"

	stabilityScore := 100.0
	switch status {
	case "down":
		stabilityScore = 20
	case "degraded":
		stabilityScore = 60
	}

	batch := &pgx.Batch{}
	batch.Queue(`
		INSERT INTO metrics_aggregated (
			experiment_id,
			endpoint,
			requests_total,
			p50_ms,
			p95_ms,
			p99_ms,
			avg_ms,
			stddev_ms,
			jitter_ms,
			error_rate,
			max_failure_streak,
			latency_ratio,
			error_delta,
			stability_score,
			impact_order,
			degraded,
			avg_cpu,
			max_cpu,
			avg_memory,
			max_memory
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
			$11,$12,$13,$14,$15,$16,$17,$18,$19,$20
		)
	`,
		expID,
		endpoint,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		toFloat(stability["crash_rate_percent"]),
		0,
		0,
		0,
		stabilityScore,
		0,
		degraded,
		0,
		0,
		0,
		0,
	)

	br := p.Pool.SendBatch(context.Background(), batch)
	return br.Close()
}

func (p *Postgres) InsertExperimentSummary(
	expID string,
	data map[string]interface{},
) error {
	blastRadius := toFloat(data["blast_radius_percent"])
	cascadeDepth := toInt(data["cascade_depth"])
	systemSeverity := toString(data["system_severity"])
	totalRequests := toInt(data["total_requests"])

	if systemSeverity == "" {
		if health, ok := data["health"].(map[string]interface{}); ok {
			systemSeverity = strings.ToLower(toString(health["severity"]))
		}
	}
	if systemSeverity == "" {
		systemSeverity = "unknown"
	}

	_, err := p.Pool.Exec(context.Background(), `
		INSERT INTO experiment_summary (
			experiment_id,
			blast_radius,
			cascade_depth,
			system_severity,
			total_requests
		) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (experiment_id) DO UPDATE
		SET blast_radius = EXCLUDED.blast_radius,
			cascade_depth = EXCLUDED.cascade_depth,
			system_severity = EXCLUDED.system_severity,
			total_requests = EXCLUDED.total_requests
	`,
		expID,
		blastRadius,
		cascadeDepth,
		systemSeverity,
		totalRequests,
	)

	return err
}

func (p *Postgres) InsertAndroidExperimentSummary(
	expID string,
	data map[string]interface{},
) error {
	if !strings.EqualFold(toString(data["target_type"]), "android") {
		return nil
	}

	health := toMap(data["health"])
	recovery := toMap(data["recovery"])
	stability := toMap(data["stability"])
	validation := toMap(data["validation"])
	summary := toMap(data["summary"])
	timeline := toMap(data["timeline"])
	firstImpact := toMap(timeline["first_impact"])
	recoveryMap := toMap(timeline["recovery"])

	targetPackage := firstNonEmptyMapKey(firstImpact)
	if targetPackage == "" {
		targetPackage = firstNonEmptyMapKey(recoveryMap)
	}
	if targetPackage == "" {
		targetPackage = "android"
	}

	firstImpactAt := toTime(firstImpact[targetPackage])
	recoveredAt := toTime(recoveryMap[targetPackage])

	validationReasonsJSON, _ := json.Marshal(toInterfaceSlice(validation["reasons"]))

	_, err := p.Pool.Exec(context.Background(), `
		INSERT INTO android_experiment_summary (
			experiment_id,
			target_package,
			scenario,
			failure_type,
			health_status,
			severity,
			crash_reason,
			recovered,
			auto_recovered,
			stable_recovered,
			manual_intervention_required,
			running,
			recovery_time_ms,
			first_impact_at,
			recovered_at,
			crash_rate_percent,
			uptime_percent,
			anr_detected,
			warning_signals,
			unexpected_restarts,
			validation_configured,
			validation_passed,
			validation_reasons,
			summary_result,
			summary_reason,
			summary_suggestion,
			updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
			$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
			$21,$22,$23,$24,$25,$26,NOW()
		)
		ON CONFLICT (experiment_id) DO UPDATE
		SET target_package = EXCLUDED.target_package,
			scenario = EXCLUDED.scenario,
			failure_type = EXCLUDED.failure_type,
			health_status = EXCLUDED.health_status,
			severity = EXCLUDED.severity,
			crash_reason = EXCLUDED.crash_reason,
			recovered = EXCLUDED.recovered,
			auto_recovered = EXCLUDED.auto_recovered,
			stable_recovered = EXCLUDED.stable_recovered,
			manual_intervention_required = EXCLUDED.manual_intervention_required,
			running = EXCLUDED.running,
			recovery_time_ms = EXCLUDED.recovery_time_ms,
			first_impact_at = EXCLUDED.first_impact_at,
			recovered_at = EXCLUDED.recovered_at,
			crash_rate_percent = EXCLUDED.crash_rate_percent,
			uptime_percent = EXCLUDED.uptime_percent,
			anr_detected = EXCLUDED.anr_detected,
			warning_signals = EXCLUDED.warning_signals,
			unexpected_restarts = EXCLUDED.unexpected_restarts,
			validation_configured = EXCLUDED.validation_configured,
			validation_passed = EXCLUDED.validation_passed,
			validation_reasons = EXCLUDED.validation_reasons,
			summary_result = EXCLUDED.summary_result,
			summary_reason = EXCLUDED.summary_reason,
			summary_suggestion = EXCLUDED.summary_suggestion,
			updated_at = NOW()
	`,
		expID,
		targetPackage,
		toString(data["scenario"]),
		toString(health["failure_type"]),
		toString(health["status"]),
		toString(health["severity"]),
		toString(health["crash_reason"]),
		toBool(recovery["recovered"]),
		toBool(recovery["auto_recovered"]),
		toBool(recovery["stable_recovered"]),
		toBool(recovery["manual_intervention_required"]),
		toBool(recovery["running"]),
		toInt64(recovery["recovery_time_ms"]),
		nullableTime(firstImpactAt),
		nullableTime(recoveredAt),
		toFloat(stability["crash_rate_percent"]),
		toFloat(stability["uptime_percent"]),
		toBool(stability["anr_detected"]),
		toInt(stability["warning_signals"]),
		toInt(stability["unexpected_restarts"]),
		toBool(validation["configured"]),
		toNullableBool(validation["passed"]),
		validationReasonsJSON,
		toString(summary["result"]),
		toString(summary["reason"]),
		toString(summary["suggestion"]),
	)

	return err
}

func (p *Postgres) InsertAndroidExperimentReport(
	expID string,
	data map[string]interface{},
) error {
	if !strings.EqualFold(toString(data["target_type"]), "android") {
		return nil
	}

	timeline := toMap(data["timeline"])
	firstImpact := toMap(timeline["first_impact"])
	recoveryMap := toMap(timeline["recovery"])

	targetPackage := firstNonEmptyMapKey(firstImpact)
	if targetPackage == "" {
		targetPackage = firstNonEmptyMapKey(recoveryMap)
	}
	if targetPackage == "" {
		targetPackage = "android"
	}

	reportJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = p.Pool.Exec(context.Background(), `
		INSERT INTO android_experiment_report (
			experiment_id,
			target_package,
			scenario,
			report,
			updated_at
		) VALUES ($1,$2,$3,$4::jsonb,NOW())
		ON CONFLICT (experiment_id) DO UPDATE
		SET target_package = EXCLUDED.target_package,
			scenario = EXCLUDED.scenario,
			report = EXCLUDED.report,
			updated_at = NOW()
	`,
		expID,
		targetPackage,
		toString(data["scenario"]),
		string(reportJSON),
	)

	return err
}

func toString(v interface{}) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func toFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int32:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func toInt(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	default:
		return 0
	}
}

func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case float64:
		return int64(t)
	case float32:
		return int64(t)
	default:
		return 0
	}
}

func toBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func toNullableBool(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	b, ok := v.(bool)
	if !ok {
		return nil
	}
	return b
}

func toMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

func toTime(v interface{}) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(t)); err == nil {
			return parsed
		}
		return time.Time{}
	default:
		return time.Time{}
	}
}

func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

func firstNonEmptyMapKey(m map[string]interface{}) string {
	for k := range m {
		if strings.TrimSpace(k) != "" {
			return k
		}
	}
	return ""
}

func toInterfaceSlice(v interface{}) []interface{} {
	switch t := v.(type) {
	case []interface{}:
		return t
	case []string:
		out := make([]interface{}, 0, len(t))
		for _, s := range t {
			out = append(out, s)
		}
		return out
	default:
		return []interface{}{}
	}
}
