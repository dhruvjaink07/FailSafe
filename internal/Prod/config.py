"""
failsafe/config.py
──────────────────
Single source of truth for paths, feature lists, model constants,
and any shared utility functions used across train / infer / explain.
"""

from pathlib import Path

# ── Paths ─────────────────────────────────────────────────────────────────────
ROOT_DIR        = Path(__file__).parent
MODELS_DIR      = ROOT_DIR / "models"
OUTPUTS_DIR     = ROOT_DIR / "outputs"

DATA_PATH       = ROOT_DIR / "data" / "failsafe_experiments.csv"
LGB_MODEL_PATH  = MODELS_DIR / "lgb_model.txt"
SARIMA_META_PATH= MODELS_DIR / "sarima_meta.pkl"   # weights + d value
FEATURE_LIST_PATH = MODELS_DIR / "feature_list.json"

# Create dirs on import so other modules never have to worry about it
MODELS_DIR.mkdir(parents=True, exist_ok=True)
OUTPUTS_DIR.mkdir(parents=True, exist_ok=True)
(ROOT_DIR / "data").mkdir(parents=True, exist_ok=True)

# ── Column names ──────────────────────────────────────────────────────────────
TARGET    = "risk_score"
DATE_COL  = "date"
ID_COL    = "experiment_id"

# Raw columns that come straight from the analysis engine
RAW_FEATURE_COLS = [
    "fault_intensity", "max_stable_intensity", "breaking_intensity",
    "blast_radius_percent", "cascade_depth", "system_severity",
    "latency_ratio", "error_delta", "stability_score",
    "avg_cpu_percent", "max_cpu_percent",
    "avg_memory_mb", "max_memory_mb",
    "total_endpoints", "total_requests",
    "impact_order", "degraded_fraction",
]

# Columns we build lag + rolling features from
LAG_SOURCE_COLS = [
    "stability_score", "latency_ratio", "error_delta",
    "blast_radius_percent", "cascade_depth",
]
LAG_STEPS    = [1, 2, 3]
ROLL_WINDOWS = [3, 7]

# ── SARIMA order ──────────────────────────────────────────────────────────────
# Simple (1,d,1) with no seasonality — stable on small datasets.
# Bump to (2,d,1)(1,0,1,s) once you have 200+ real experiments.
SARIMA_ORDER          = (1, None, 1)   # d filled in at runtime from ADF test
SARIMA_SEASONAL_ORDER = (0, 0, 0, 0)
SARIMA_FIT_KWARGS     = {
    "disp":   False,
    "maxiter": 200,
    "method": "powell",
}

# ── LightGBM hyperparameters ──────────────────────────────────────────────────
LGB_PARAMS = {
    "objective":         "regression",
    "metric":            "mae",
    "n_estimators":      400,
    "learning_rate":     0.05,
    "num_leaves":        31,
    "max_depth":         6,
    "min_child_samples": 10,
    "subsample":         0.8,
    "colsample_bytree":  0.8,
    "reg_alpha":         0.1,
    "reg_lambda":        0.1,
    "random_state":      42,
    "verbose":          -1,
}
LGB_CV_SPLITS       = 5
LGB_EARLY_STOP      = 50
TRAIN_TEST_SPLIT    = 0.80   # fraction used for training

# ── Risk tier thresholds ──────────────────────────────────────────────────────
RISK_TIERS = [
    (20,  "LOW"),
    (45,  "MEDIUM"),
    (70,  "HIGH"),
    (101, "CRITICAL"),
]

def risk_tier(score: float) -> str:
    """Map a numeric risk score (0–100) to a tier label."""
    for threshold, label in RISK_TIERS:
        if score < threshold:
            return label
    return "CRITICAL"
