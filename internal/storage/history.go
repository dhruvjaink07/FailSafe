package storage

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (p *Postgres) GetExperimentHistoryByAPIKey(apiKeyID string, limit int, offset int) ([]map[string]interface{}, error) {
	// If apiKeyID is empty, return history for all API keys (no filtering).
	filterByKey := strings.TrimSpace(apiKeyID) != ""
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var rows pgx.Rows
	var err error
	if filterByKey {
		rows, err = p.Pool.Query(context.Background(), `
			SELECT
				e.id::text,
				COALESCE(e.fault_type, ''),
				COALESCE(e.state, ''),
				COALESCE(e.phase, ''),
				e.created_at,
				e.updated_at,
				CASE
					WHEN be.experiment_id IS NOT NULL THEN 'backend'
					WHEN ae.experiment_id IS NOT NULL THEN 'android'
					WHEN fe.experiment_id IS NOT NULL THEN 'frontend'
					ELSE 'backend'
				END AS target_type
			FROM experiments e
			LEFT JOIN backend_experiments be ON be.experiment_id = e.id
			LEFT JOIN android_experiments ae ON ae.experiment_id = e.id
			LEFT JOIN frontend_experiments fe ON fe.experiment_id = e.id
			WHERE e.api_key_id = $1::uuid
			ORDER BY e.created_at DESC
			LIMIT $2 OFFSET $3
		`, apiKeyID, limit, offset)
	} else {
		rows, err = p.Pool.Query(context.Background(), `
			SELECT
				e.id::text,
				COALESCE(e.fault_type, ''),
				COALESCE(e.state, ''),
				COALESCE(e.phase, ''),
				e.created_at,
				e.updated_at,
				CASE
					WHEN be.experiment_id IS NOT NULL THEN 'backend'
					WHEN ae.experiment_id IS NOT NULL THEN 'android'
					WHEN fe.experiment_id IS NOT NULL THEN 'frontend'
					ELSE 'backend'
				END AS target_type
			FROM experiments e
			LEFT JOIN backend_experiments be ON be.experiment_id = e.id
			LEFT JOIN android_experiments ae ON ae.experiment_id = e.id
			LEFT JOIN frontend_experiments fe ON fe.experiment_id = e.id
			ORDER BY e.created_at DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id string
		var faultType string
		var state string
		var phase string
		var createdAt time.Time
		var updatedAt time.Time
		var targetType string

		if err := rows.Scan(&id, &faultType, &state, &phase, &createdAt, &updatedAt, &targetType); err != nil {
			return nil, err
		}

		item, err := p.buildHistoryItem(id, faultType, state, phase, createdAt, updatedAt, targetType)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (p *Postgres) GetExperimentHistoryCountByAPIKey(apiKeyID string) (int, error) {
	var count int
	if strings.TrimSpace(apiKeyID) == "" {
		err := p.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM experiments`).Scan(&count)
		if err != nil {
			return 0, err
		}
		return count, nil
	}
	err := p.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM experiments WHERE api_key_id = $1::uuid`, apiKeyID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (p *Postgres) GetExperimentHistoryDetailByAPIKey(apiKeyID string, experimentID string) (map[string]interface{}, error) {
	if strings.TrimSpace(experimentID) == "" {
		return nil, nil
	}
	filterByKey := strings.TrimSpace(apiKeyID) != ""

	var id string
	var faultType string
	var state string
	var phase string
	var createdAt time.Time
	var updatedAt time.Time
	var targetType string
	var err error

	if filterByKey {
		err = p.Pool.QueryRow(context.Background(), `
			SELECT
				e.id::text,
				COALESCE(e.fault_type, ''),
				COALESCE(e.state, ''),
				COALESCE(e.phase, ''),
				e.created_at,
				e.updated_at,
				CASE
					WHEN be.experiment_id IS NOT NULL THEN 'backend'
					WHEN ae.experiment_id IS NOT NULL THEN 'android'
					WHEN fe.experiment_id IS NOT NULL THEN 'frontend'
					ELSE 'backend'
				END AS target_type
			FROM experiments e
			LEFT JOIN backend_experiments be ON be.experiment_id = e.id
			LEFT JOIN android_experiments ae ON ae.experiment_id = e.id
			LEFT JOIN frontend_experiments fe ON fe.experiment_id = e.id
			WHERE e.id = $1::uuid AND e.api_key_id = $2::uuid
			LIMIT 1
		`, experimentID, apiKeyID).Scan(&id, &faultType, &state, &phase, &createdAt, &updatedAt, &targetType)
	} else {
		err = p.Pool.QueryRow(context.Background(), `
			SELECT
				e.id::text,
				COALESCE(e.fault_type, ''),
				COALESCE(e.state, ''),
				COALESCE(e.phase, ''),
				e.created_at,
				e.updated_at,
				CASE
					WHEN be.experiment_id IS NOT NULL THEN 'backend'
					WHEN ae.experiment_id IS NOT NULL THEN 'android'
					WHEN fe.experiment_id IS NOT NULL THEN 'frontend'
					ELSE 'backend'
				END AS target_type
			FROM experiments e
			LEFT JOIN backend_experiments be ON be.experiment_id = e.id
			LEFT JOIN android_experiments ae ON ae.experiment_id = e.id
			LEFT JOIN frontend_experiments fe ON fe.experiment_id = e.id
			WHERE e.id = $1::uuid
			LIMIT 1
		`, experimentID).Scan(&id, &faultType, &state, &phase, &createdAt, &updatedAt, &targetType)
	}
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}

	return p.buildHistoryItem(id, faultType, state, phase, createdAt, updatedAt, targetType)
}

func (p *Postgres) buildHistoryItem(id, faultType, state, phase string, createdAt, updatedAt time.Time, targetType string) (map[string]interface{}, error) {
	statusPayload, err := p.GetPlatformStatusPayload(targetType, id)
	if err != nil {
		return nil, err
	}

	aggregatedMetrics, err := p.getAggregatedMetricsForHistory(id)
	if err != nil {
		return nil, err
	}

	rawMetrics, err := p.getRawMetricsForHistory(id, targetType)
	if err != nil {
		return nil, err
	}

	summary, err := p.getSummaryForHistory(id, targetType)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"experiment": map[string]interface{}{
			"id":          id,
			"fault_type":  faultType,
			"target_type": normalizePlatform(targetType),
			"state":       state,
			"phase":       phase,
			"created_at":  createdAt,
			"updated_at":  updatedAt,
		},
		"metrics": map[string]interface{}{
			"status_payload": statusPayload,
			"aggregated":     aggregatedMetrics,
			"raw":            rawMetrics,
		},
		"summary": summary,
	}, nil
}

func (p *Postgres) getAggregatedMetricsForHistory(experimentID string) ([]map[string]interface{}, error) {
	rows, err := p.Pool.Query(context.Background(), `
		SELECT
			COALESCE(endpoint, ''),
			COALESCE(requests_total, 0),
			COALESCE(p50_ms, 0),
			COALESCE(p95_ms, 0),
			COALESCE(p99_ms, 0),
			COALESCE(avg_ms, 0),
			COALESCE(stddev_ms, 0),
			COALESCE(jitter_ms, 0),
			COALESCE(error_rate, 0),
			COALESCE(max_failure_streak, 0),
			COALESCE(latency_ratio, 0),
			COALESCE(error_delta, 0),
			COALESCE(stability_score, 0),
			COALESCE(impact_order, 0),
			COALESCE(degraded, false),
			COALESCE(avg_cpu, 0),
			COALESCE(max_cpu, 0),
			COALESCE(avg_memory, 0),
			COALESCE(max_memory, 0)
		FROM metrics_aggregated
		WHERE experiment_id = $1::uuid
		ORDER BY impact_order ASC, id ASC
	`, experimentID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var endpoint string
		var requestsTotal int
		var p50 float64
		var p95 float64
		var p99 float64
		var avg float64
		var stddev float64
		var jitter float64
		var errorRate float64
		var maxFailureStreak int
		var latencyRatio float64
		var errorDelta float64
		var stabilityScore float64
		var impactOrder int
		var degraded bool
		var avgCPU float64
		var maxCPU float64
		var avgMemory float64
		var maxMemory float64

		if err := rows.Scan(
			&endpoint,
			&requestsTotal,
			&p50,
			&p95,
			&p99,
			&avg,
			&stddev,
			&jitter,
			&errorRate,
			&maxFailureStreak,
			&latencyRatio,
			&errorDelta,
			&stabilityScore,
			&impactOrder,
			&degraded,
			&avgCPU,
			&maxCPU,
			&avgMemory,
			&maxMemory,
		); err != nil {
			return nil, err
		}

		out = append(out, map[string]interface{}{
			"endpoint":           endpoint,
			"requests_total":     requestsTotal,
			"p50_ms":             p50,
			"p95_ms":             p95,
			"p99_ms":             p99,
			"avg_ms":             avg,
			"stddev_ms":          stddev,
			"jitter_ms":          jitter,
			"error_rate":         errorRate,
			"max_failure_streak": maxFailureStreak,
			"latency_ratio":      latencyRatio,
			"error_delta":        errorDelta,
			"stability_score":    stabilityScore,
			"impact_order":       impactOrder,
			"degraded":           degraded,
			"avg_cpu":            avgCPU,
			"max_cpu":            maxCPU,
			"avg_memory":         avgMemory,
			"max_memory":         maxMemory,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (p *Postgres) getRawMetricsForHistory(experimentID, targetType string) (interface{}, error) {
	switch normalizePlatform(targetType) {
	case "android":
		return p.getAndroidRawMetricsForHistory(experimentID)
	case "frontend":
		return p.GetFrontendMetricsRaw(experimentID)
	default:
		return p.getBackendRawMetricsForHistory(experimentID)
	}
}

func (p *Postgres) getBackendRawMetricsForHistory(experimentID string) ([]map[string]interface{}, error) {
	rows, err := p.Pool.Query(context.Background(), `
		SELECT
			COALESCE(endpoint, ''),
			timestamp,
			COALESCE(latency_ms, 0),
			COALESCE(status, 0),
			COALESCE(intensity, 0),
			COALESCE(container_cpu, 0),
			COALESCE(container_memory, 0)
		FROM backend_metrics_raw
		WHERE experiment_id = $1::uuid
		ORDER BY id ASC
	`, experimentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var endpoint string
		var ts time.Time
		var latencyMs int64
		var status int
		var intensity int
		var cpu float64
		var memory float64

		if err := rows.Scan(&endpoint, &ts, &latencyMs, &status, &intensity, &cpu, &memory); err != nil {
			return nil, err
		}

		out = append(out, map[string]interface{}{
			"endpoint":         endpoint,
			"timestamp":        ts,
			"latency_ms":       latencyMs,
			"status":           status,
			"intensity":        intensity,
			"container_cpu":    cpu,
			"container_memory": memory,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (p *Postgres) getAndroidRawMetricsForHistory(experimentID string) ([]map[string]interface{}, error) {
	rows, err := p.Pool.Query(context.Background(), `
		SELECT
			COALESCE(endpoint, ''),
			timestamp,
			COALESCE(app_state, ''),
			COALESCE(crash, false),
			COALESCE(anr, false),
			COALESCE(crash_reason, ''),
			COALESCE(memory_mb, 0),
			COALESCE(frame_drops, 0),
			COALESCE(latency_ms, 0),
			COALESCE(status, 0),
			COALESCE(intensity, 0)
		FROM android_metrics_raw
		WHERE experiment_id = $1::uuid
		ORDER BY id ASC
	`, experimentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var endpoint string
		var ts time.Time
		var appState string
		var crash bool
		var anr bool
		var crashReason string
		var memoryMB float64
		var frameDrops int
		var latencyMs int64
		var status int
		var intensity int

		if err := rows.Scan(
			&endpoint,
			&ts,
			&appState,
			&crash,
			&anr,
			&crashReason,
			&memoryMB,
			&frameDrops,
			&latencyMs,
			&status,
			&intensity,
		); err != nil {
			return nil, err
		}

		out = append(out, map[string]interface{}{
			"endpoint":     endpoint,
			"timestamp":    ts,
			"app_state":    appState,
			"crash":        crash,
			"anr":          anr,
			"crash_reason": crashReason,
			"memory_mb":    memoryMB,
			"frame_drops":  frameDrops,
			"latency_ms":   latencyMs,
			"status":       status,
			"intensity":    intensity,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (p *Postgres) getSummaryForHistory(experimentID, targetType string) (map[string]interface{}, error) {
	targetType = normalizePlatform(targetType)
	if targetType == "android" {
		return p.getAndroidSummaryForHistory(experimentID)
	}

	var blastRadius float64
	var cascadeDepth int
	var systemSeverity string
	var totalRequests int
	err := p.Pool.QueryRow(context.Background(), `
		SELECT
			COALESCE(blast_radius, 0),
			COALESCE(cascade_depth, 0),
			COALESCE(system_severity, ''),
			COALESCE(total_requests, 0)
		FROM experiment_summary
		WHERE experiment_id = $1::uuid
	`, experimentID).Scan(&blastRadius, &cascadeDepth, &systemSeverity, &totalRequests)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}

	return map[string]interface{}{
		"blast_radius":    blastRadius,
		"cascade_depth":   cascadeDepth,
		"system_severity": systemSeverity,
		"total_requests":  totalRequests,
	}, nil
}

func (p *Postgres) getAndroidSummaryForHistory(experimentID string) (map[string]interface{}, error) {
	var targetPackage string
	var scenario string
	var failureType string
	var healthStatus string
	var severity string
	var crashReason string
	var recovered bool
	var autoRecovered bool
	var stableRecovered bool
	var manualIntervention bool
	var running bool
	var recoveryTimeMs int64
	var summaryResult string
	var summaryReason string
	var summarySuggestion string

	err := p.Pool.QueryRow(context.Background(), `
		SELECT
			COALESCE(target_package, ''),
			COALESCE(scenario, ''),
			COALESCE(failure_type, ''),
			COALESCE(health_status, ''),
			COALESCE(severity, ''),
			COALESCE(crash_reason, ''),
			COALESCE(recovered, false),
			COALESCE(auto_recovered, false),
			COALESCE(stable_recovered, false),
			COALESCE(manual_intervention_required, false),
			COALESCE(running, false),
			COALESCE(recovery_time_ms, 0),
			COALESCE(summary_result, ''),
			COALESCE(summary_reason, ''),
			COALESCE(summary_suggestion, '')
		FROM android_experiment_summary
		WHERE experiment_id = $1::uuid
	`, experimentID).Scan(
		&targetPackage,
		&scenario,
		&failureType,
		&healthStatus,
		&severity,
		&crashReason,
		&recovered,
		&autoRecovered,
		&stableRecovered,
		&manualIntervention,
		&running,
		&recoveryTimeMs,
		&summaryResult,
		&summaryReason,
		&summarySuggestion,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}

	return map[string]interface{}{
		"target_package":               targetPackage,
		"scenario":                     scenario,
		"failure_type":                 failureType,
		"health_status":                healthStatus,
		"severity":                     severity,
		"crash_reason":                 crashReason,
		"recovered":                    recovered,
		"auto_recovered":               autoRecovered,
		"stable_recovered":             stableRecovered,
		"manual_intervention_required": manualIntervention,
		"running":                      running,
		"recovery_time_ms":             recoveryTimeMs,
		"summary_result":               summaryResult,
		"summary_reason":               summaryReason,
		"summary_suggestion":           summarySuggestion,
	}, nil
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, pgx.ErrNoRows)
}
