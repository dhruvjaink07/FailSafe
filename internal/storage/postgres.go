package storage

import (
	"context"
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

	return &Postgres{Pool: pool}, nil
}

func (p *Postgres) InsertExperiment(exp *models.Experiment) error {

	query := `
	INSERT INTO experiments (
		id, fault_type, state, phase,
		created_at, updated_at,
		max_intensity, breaking_intensity, max_stable_intensity,
		baseline, dependency_graph, target_endpoint_map
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12
	)`

	_, err := p.Pool.Exec(context.Background(), query,
		exp.ID,
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

	return err
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
	defer br.Close()

	return nil
}

func (p *Postgres) InsertAggregatedMetrics(
	expID string,
	data map[string]interface{},
) error {

	endpoints := data["endpoints"].(map[string]interface{})

	batch := &pgx.Batch{}

	for endpoint, raw := range endpoints {

		ep := raw.(map[string]interface{})

		lat := ep["latency"].(map[string]interface{})
		errs := ep["errors"].(map[string]interface{})
		derived := ep["derived"].(map[string]interface{})
		container := ep["container"].(map[string]interface{})

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
	defer br.Close()

	return nil
}

func (p *Postgres) InsertExperimentSummary(
	expID string,
	data map[string]interface{},
) error {

	_, err := p.Pool.Exec(context.Background(), `
		INSERT INTO experiment_summary (
			experiment_id,
			blast_radius,
			cascade_depth,
			system_severity,
			total_requests
		) VALUES ($1,$2,$3,$4,$5)
	`,
		expID,
		data["blast_radius_percent"],
		data["cascade_depth"],
		data["system_severity"],
		data["total_requests"],
	)

	return err
}
