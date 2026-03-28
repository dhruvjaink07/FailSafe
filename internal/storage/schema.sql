CREATE TABLE IF NOT EXISTS experiments (
    id UUID PRIMARY KEY,
    fault_type TEXT,
    state TEXT,
    phase TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    max_intensity INT,
    breaking_intensity INT,
    max_stable_intensity INT,
    baseline JSONB,
    dependency_graph JSONB,
    target_endpoint_map JSONB
);

CREATE TABLE IF NOT EXISTS metrics_raw (
    id BIGSERIAL PRIMARY KEY,
    experiment_id UUID,
    endpoint TEXT,
    timestamp TIMESTAMP,
    latency_ms BIGINT,
    status INT,
    intensity INT,
    container_cpu FLOAT,
    container_memory FLOAT
);

CREATE TABLE IF NOT EXISTS metrics_aggregated (
    id BIGSERIAL PRIMARY KEY,
    experiment_id UUID,
    endpoint TEXT,
    requests_total INT,
    p50_ms FLOAT,
    p95_ms FLOAT,
    p99_ms FLOAT,
    avg_ms FLOAT,
    stddev_ms FLOAT,
    jitter_ms FLOAT,
    error_rate FLOAT,
    max_failure_streak INT,
    latency_ratio FLOAT,
    error_delta FLOAT,
    stability_score FLOAT,
    impact_order INT,
    degraded BOOLEAN,
    avg_cpu FLOAT,
    max_cpu FLOAT,
    avg_memory FLOAT,
    max_memory FLOAT
);

CREATE TABLE IF NOT EXISTS experiment_summary (
    experiment_id UUID PRIMARY KEY,
    blast_radius FLOAT,
    cascade_depth INT,
    system_severity TEXT,
    total_requests INT
);

CREATE TABLE IF NOT EXISTS android_experiment_summary (
    experiment_id UUID PRIMARY KEY,
    target_package TEXT,
    scenario TEXT,
    failure_type TEXT,
    health_status TEXT,
    severity TEXT,
    crash_reason TEXT,
    recovered BOOLEAN,
    auto_recovered BOOLEAN,
    stable_recovered BOOLEAN,
    manual_intervention_required BOOLEAN,
    running BOOLEAN,
    recovery_time_ms BIGINT,
    first_impact_at TIMESTAMP,
    recovered_at TIMESTAMP,
    crash_rate_percent FLOAT,
    uptime_percent FLOAT,
    anr_detected BOOLEAN,
    warning_signals INT,
    unexpected_restarts INT,
    validation_configured BOOLEAN,
    validation_passed BOOLEAN,
    validation_reasons JSONB,
    summary_result TEXT,
    summary_reason TEXT,
    summary_suggestion TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS android_experiment_report (
    experiment_id UUID PRIMARY KEY,
    target_package TEXT,
    scenario TEXT,
    report JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);