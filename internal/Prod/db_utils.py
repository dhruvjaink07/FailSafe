"""
db_utils.py
───────────
Database utility for loading experiment features for ML inference.

Extracts features from backend_status_metrics.status_payload (JSONB) since
metrics_aggregated is populated at runtime by the Go backend and may be empty.
Falls back to metrics_aggregated JOIN if that table has data.
"""

import os
import json
import pandas as pd
import psycopg2
from psycopg2.extras import RealDictCursor


# ── Connection ────────────────────────────────────────────────────────────────

def get_db_connection():
    """Establish a connection to the PostgreSQL database using env vars."""
    db_url = os.getenv("DB_URL")
    if db_url:
        return psycopg2.connect(db_url, cursor_factory=RealDictCursor)

    return psycopg2.connect(
        host=os.getenv("DB_HOST", "localhost"),
        port=os.getenv("DB_PORT", "5432"),
        user=os.getenv("DB_USER", "failsafe"),
        password=os.getenv("DB_PASSWORD", "failsafe"),
        dbname=os.getenv("DB_NAME", "failsafe"),
        cursor_factory=RealDictCursor,
    )


# ── Helpers ───────────────────────────────────────────────────────────────────

def _safe(val, default=0.0):
    """Return val if not None, else default."""
    return val if val is not None else default


def _extract_from_payload(row: dict) -> dict:
    """
    Parse a backend_status_metrics row and extract RAW_FEATURE_COLS values
    from the nested status_payload JSON.
    """
    payload = row.get("status_payload") or {}
    if isinstance(payload, str):
        payload = json.loads(payload)

    endpoints: dict = payload.get("endpoints", {}) or {}

    # Aggregate across endpoints
    stability_scores, latency_ratios, error_deltas = [], [], []
    cpu_avgs, cpu_maxes, mem_avgs, mem_maxes = [], [], [], []
    impact_orders = []
    degraded_count = 0

    for ep_data in endpoints.values():
        derived  = ep_data.get("derived", {}) or {}
        latency  = ep_data.get("latency", {}) or {}
        container = ep_data.get("container", {}) or {}

        stability_scores.append(_safe(ep_data.get("stability_score")))
        latency_ratios.append(_safe(derived.get("latency_ratio"), 1.0))
        error_deltas.append(_safe(derived.get("error_delta")))
        cpu_avgs.append(_safe(container.get("avg_cpu_percent")))
        cpu_maxes.append(_safe(container.get("max_cpu_percent")))
        mem_avgs.append(_safe(container.get("avg_memory_mb")))
        mem_maxes.append(_safe(container.get("max_memory_mb")))

        io = ep_data.get("impact_order", 0) or 0
        if io > 0:
            impact_orders.append(io)

        if ep_data.get("degraded"):
            degraded_count += 1

    n = len(endpoints) or 1

    def avg(lst): return sum(lst) / len(lst) if lst else 0.0
    def mx(lst):  return max(lst) if lst else 0.0

    # Resilience threshold block
    rt = payload.get("resilience_threshold", {}) or {}

    feat = {
        "experiment_id":      row["experiment_id"],
        "date":               row["updated_at"],

        # Intensity
        "fault_intensity":        _safe(rt.get("breaking_intensity") or
                                        payload.get("breaking_intensity")),
        "max_stable_intensity":   _safe(rt.get("max_stable_intensity") or
                                        payload.get("max_stable_intensity")),
        "breaking_intensity":     _safe(rt.get("breaking_intensity") or
                                        payload.get("breaking_intensity")),

        # Blast / cascade
        "blast_radius_percent":   _safe(payload.get("blast_radius_percent"),
                                        _safe(row.get("blast_radius"))),
        "cascade_depth":          _safe(payload.get("cascade_depth"),
                                        _safe(row.get("cascade_depth"))),
        "system_severity":        payload.get("system_severity") or
                                  row.get("system_severity") or "isolated",
        "total_requests":         _safe(payload.get("total_requests")),
        "total_endpoints":        _safe(payload.get("total_endpoints"), n),

        # Per-endpoint aggregates
        "stability_score":        avg(stability_scores),
        "latency_ratio":          avg(latency_ratios),
        "error_delta":            avg(error_deltas),
        "impact_order":           min(impact_orders) if impact_orders else 0,
        "avg_cpu_percent":        avg(cpu_avgs),
        "max_cpu_percent":        mx(cpu_maxes),
        "avg_memory_mb":          avg(mem_avgs),
        "max_memory_mb":          mx(mem_maxes),
        "degraded_fraction":      degraded_count / n,
    }
    return feat


# ── Primary loader: from backend_status_metrics JSON ─────────────────────────

def _load_from_status_metrics(limit: int = None, desc: bool = False) -> pd.DataFrame:
    order = "DESC" if desc else "ASC"
    limit_clause = f"LIMIT {int(limit)}" if limit else ""

    query = f"""
        SELECT
            bsm.experiment_id,
            bsm.blast_radius,
            bsm.cascade_depth,
            bsm.system_severity,
            bsm.status_payload,
            bsm.updated_at,
            e.max_intensity        AS max_intensity,
            e.max_stable_intensity AS max_stable_intensity,
            e.breaking_intensity   AS breaking_intensity,
            e.created_at           AS created_at
        FROM backend_status_metrics bsm
        JOIN experiments e ON bsm.experiment_id = e.id
        ORDER BY e.created_at {order}
        {limit_clause}
    """

    with get_db_connection() as conn:
        rows = []
        with conn.cursor() as cur:
            cur.execute(query)
            rows = cur.fetchall()

    if not rows:
        return pd.DataFrame()

    records = [_extract_from_payload(dict(r)) for r in rows]
    df = pd.DataFrame(records)
    df["date"] = pd.to_datetime(df["date"])
    if desc:
        df = df.sort_values("date").reset_index(drop=True)
    return df


# ── Fallback loader: from metrics_aggregated JOIN (original approach) ─────────

def _load_from_aggregated(limit: int = None, desc: bool = False) -> pd.DataFrame:
    order = "DESC" if desc else "ASC"
    limit_clause = f"LIMIT {int(limit)}" if limit else ""

    query = f"""
        SELECT
            e.id                                                AS experiment_id,
            e.max_intensity                                     AS fault_intensity,
            e.max_stable_intensity,
            e.breaking_intensity,
            s.blast_radius                                      AS blast_radius_percent,
            s.cascade_depth,
            s.system_severity,
            s.total_requests,
            AVG(m.stability_score)                              AS stability_score,
            AVG(m.latency_ratio)                                AS latency_ratio,
            AVG(m.error_delta)                                  AS error_delta,
            MIN(CASE WHEN m.impact_order > 0 THEN m.impact_order END) AS impact_order,
            AVG(m.avg_cpu)                                      AS avg_cpu_percent,
            MAX(m.max_cpu)                                      AS max_cpu_percent,
            AVG(m.avg_memory)                                   AS avg_memory_mb,
            MAX(m.max_memory)                                   AS max_memory_mb,
            COUNT(DISTINCT m.endpoint)                          AS total_endpoints,
            AVG(CASE WHEN m.degraded THEN 1.0 ELSE 0.0 END)    AS degraded_fraction,
            e.created_at                                        AS date
        FROM experiments e
        JOIN metrics_aggregated   m ON e.id = m.experiment_id
        JOIN experiment_summary   s ON e.id = s.experiment_id
        GROUP BY e.id, e.max_intensity, e.max_stable_intensity, e.breaking_intensity,
                 s.blast_radius, s.cascade_depth, s.system_severity, s.total_requests, e.created_at
        ORDER BY e.created_at {order}
        {limit_clause}
    """

    with get_db_connection() as conn:
        df = pd.read_sql(query, conn)

    if desc and not df.empty:
        df = df.sort_values("date").reset_index(drop=True)

    return df


# ── Public API ────────────────────────────────────────────────────────────────

def load_experiment_features(limit: int = None, desc: bool = False) -> pd.DataFrame:
    """
    Load experiment features for inference.

    Strategy:
      1. Try metrics_aggregated JOIN (used after live Go backend runs)
      2. Fall back to extracting from backend_status_metrics JSON payload
         (used when loading from a DB dump)
    """
    # Try aggregated first
    try:
        df = _load_from_aggregated(limit=limit, desc=desc)
        if not df.empty:
            print(f"[db_utils] Loaded {len(df)} experiments from metrics_aggregated")
            df["date"] = pd.to_datetime(df["date"])
            _fill_numeric(df)
            return df
    except Exception as e:
        print(f"[db_utils] metrics_aggregated unavailable: {e}")

    # Fall back to status_payload JSON
    try:
        df = _load_from_status_metrics(limit=limit, desc=desc)
        if not df.empty:
            print(f"[db_utils] Loaded {len(df)} experiments from backend_status_metrics payload")
            _fill_numeric(df)
            return df
    except Exception as e:
        print(f"[db_utils] backend_status_metrics unavailable: {e}")

    print("[db_utils] No experiment data found in database.")
    return pd.DataFrame()


def _fill_numeric(df: pd.DataFrame) -> None:
    """Coerce numeric columns and fill NaN with 0 in-place."""
    numeric_cols = [
        "fault_intensity", "max_stable_intensity", "breaking_intensity",
        "blast_radius_percent", "cascade_depth",
        "stability_score", "latency_ratio", "error_delta", "impact_order",
        "avg_cpu_percent", "max_cpu_percent",
        "avg_memory_mb", "max_memory_mb",
        "total_endpoints", "total_requests", "degraded_fraction",
    ]
    for col in numeric_cols:
        if col in df.columns:
            df[col] = pd.to_numeric(df[col], errors="coerce").fillna(0.0)