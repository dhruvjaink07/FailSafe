package storage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dhruvjaink07/failsafe/internal/models"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) GetExperimentTargetType(id string) (string, error) {
	var targetType string
	err := p.Pool.QueryRow(context.Background(), `
		SELECT target_type FROM (
			SELECT experiment_id, 'docker' AS target_type FROM backend_experiments
			UNION ALL
			SELECT experiment_id, 'android' AS target_type FROM android_experiments
			UNION ALL
			SELECT experiment_id, 'frontend' AS target_type FROM frontend_experiments
		) t
		WHERE experiment_id = $1::uuid
		LIMIT 1
	`, id).Scan(&targetType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return normalizePlatform(targetType), nil
}

func (p *Postgres) GetPlatformStatusPayload(targetType, id string) (map[string]interface{}, error) {
	table := "backend_status_metrics"
	switch normalizePlatform(targetType) {
	case "android":
		table = "android_status_metrics"
	case "frontend":
		table = "frontend_status_metrics"
	}

	query := `SELECT status_payload FROM ` + table + ` WHERE experiment_id = $1::uuid`

	var payloadRaw []byte
	err := p.Pool.QueryRow(context.Background(), query, id).Scan(&payloadRaw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(payloadRaw) == 0 {
		return nil, nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (p *Postgres) GetFrontendMetricsRaw(id string) ([]models.FrontendMetrics, error) {
	rows, err := p.Pool.Query(context.Background(), `
		SELECT phase, page, lcp, cls, inp, long_tasks, errors, unhandled_rejections, api_calls, event_timestamp
		FROM frontend_metrics_raw
		WHERE experiment_id = $1::uuid
		ORDER BY id ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]models.FrontendMetrics, 0)
	for rows.Next() {
		var phase string
		var page string
		var lcp float64
		var cls float64
		var inp float64
		var longTasks int
		var errs int
		var unhandled int
		var apiCallsRaw []byte
		var ts int64

		if err := rows.Scan(
			&phase,
			&page,
			&lcp,
			&cls,
			&inp,
			&longTasks,
			&errs,
			&unhandled,
			&apiCallsRaw,
			&ts,
		); err != nil {
			return nil, err
		}

		m := models.FrontendMetrics{
			ExperimentID: id,
			Phase:        phase,
			Page:         page,
			TimeStamp:    ts,
		}
		m.Metrics.LCP = lcp
		m.Metrics.CLS = cls
		m.Metrics.INP = inp
		m.Metrics.LongTasks = longTasks
		m.Metrics.Errors = errs
		m.Metrics.UnhandledRejections = unhandled

		if len(apiCallsRaw) > 0 {
			_ = json.Unmarshal(apiCallsRaw, &m.APICalls)
		}

		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *Postgres) GetExperimentSnapshot(id string, targetType string) (*models.Experiment, error) {
	platform := normalizePlatform(targetType)
	switch platform {
	case "android":
		return p.getAndroidExperimentSnapshot(id)
	case "frontend":
		return p.getFrontendExperimentSnapshot(id)
	default:
		return p.getBackendExperimentSnapshot(id)
	}
}

func (p *Postgres) getBackendExperimentSnapshot(id string) (*models.Experiment, error) {
	var faultType, state, phase string
	var targetsRaw []byte
	var observedRaw []byte
	var createdAt, updatedAt time.Time

	err := p.Pool.QueryRow(context.Background(), `
		SELECT fault_type, state, phase, targets, observed_endpoints, created_at, updated_at
		FROM backend_experiments
		WHERE experiment_id = $1::uuid
	`, id).Scan(&faultType, &state, &phase, &targetsRaw, &observedRaw, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	exp := &models.Experiment{
		ID:              id,
		FaultType:       faultType,
		TargetType:      string(models.TargetDocker),
		State:           models.ExperimentState(strings.TrimSpace(state)),
		Phase:           models.ExperimentPhase(strings.TrimSpace(phase)),
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		TimelineHistory: make(map[int]models.IntensityTimeline),
	}
	_ = json.Unmarshal(targetsRaw, &exp.Targets)
	_ = json.Unmarshal(observedRaw, &exp.ObservedEndpoints)
	return exp, nil
}

func (p *Postgres) getAndroidExperimentSnapshot(id string) (*models.Experiment, error) {
	var faultType, state, phase string
	var pkg, activity, apkPath string
	var targetsRaw []byte
	var createdAt, updatedAt time.Time

	err := p.Pool.QueryRow(context.Background(), `
		SELECT fault_type, state, phase, package_name, activity, apk_path, targets, created_at, updated_at
		FROM android_experiments
		WHERE experiment_id = $1::uuid
	`, id).Scan(&faultType, &state, &phase, &pkg, &activity, &apkPath, &targetsRaw, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	exp := &models.Experiment{
		ID:              id,
		FaultType:       faultType,
		TargetType:      string(models.TargetAndroid),
		ObservationType: "android",
		Package:         pkg,
		Activity:        activity,
		APKPath:         apkPath,
		State:           models.ExperimentState(strings.TrimSpace(state)),
		Phase:           models.ExperimentPhase(strings.TrimSpace(phase)),
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		TimelineHistory: make(map[int]models.IntensityTimeline),
	}
	_ = json.Unmarshal(targetsRaw, &exp.Targets)
	return exp, nil
}

func (p *Postgres) getFrontendExperimentSnapshot(id string) (*models.Experiment, error) {
	var faultType, state, phase string
	var baseURL, metricsEndpoint string
	var targetURLsRaw []byte
	var targetsRaw []byte
	var createdAt, updatedAt time.Time

	err := p.Pool.QueryRow(context.Background(), `
		SELECT fault_type, state, phase, base_url, metrics_endpoint, target_urls, targets, created_at, updated_at
		FROM frontend_experiments
		WHERE experiment_id = $1::uuid
	`, id).Scan(&faultType, &state, &phase, &baseURL, &metricsEndpoint, &targetURLsRaw, &targetsRaw, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	exp := &models.Experiment{
		ID:         id,
		FaultType:  faultType,
		TargetType: string(models.TargetFrontend),
		State:      models.ExperimentState(strings.TrimSpace(state)),
		Phase:      models.ExperimentPhase(strings.TrimSpace(phase)),
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		FrontendRun: &models.FrontendRunConfig{
			BaseURL:         baseURL,
			MetricsEndpoint: metricsEndpoint,
		},
		TimelineHistory: make(map[int]models.IntensityTimeline),
	}
	_ = json.Unmarshal(targetURLsRaw, &exp.FrontendRun.TargetURLs)
	_ = json.Unmarshal(targetsRaw, &exp.Targets)
	return exp, nil
}
