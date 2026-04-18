"""
failsafe/features.py
────────────────────
Feature engineering pipeline.
Called by both train.py (on the full historical dataset)
and infer.py (on a single new experiment row).

The same function must be used in both places — if the transformations
diverge, the model gets stale features at inference time and predictions
break silently.
"""

import json
import numpy as np
import pandas as pd

from .config import (
    DATE_COL, ID_COL, TARGET,
    LAG_SOURCE_COLS, LAG_STEPS, ROLL_WINDOWS,
    FEATURE_LIST_PATH,
)


# ─────────────────────────────────────────────────────────────────────────────
# Public API
# ─────────────────────────────────────────────────────────────────────────────

def build_features(df: pd.DataFrame) -> pd.DataFrame:
    """
    Add lag, rolling, and derived features to a raw experiment DataFrame.

    Input:  DataFrame sorted by date, containing at minimum RAW_FEATURE_COLS.
    Output: Same DataFrame with extra columns appended.
            Rows where any lag is NaN are DROPPED (first LAG_STEPS[-1] rows).

    Both train.py and infer.py call this function.  infer.py passes a
    DataFrame that includes recent history so lags can be computed correctly
    even for a single new row.
    """
    df = df.copy().sort_values(DATE_COL).reset_index(drop=True)

    # ── Encode system_severity → numeric (LightGBM requires int/float) ────────
    SEVERITY_MAP = {"isolated": 0, "partial": 1, "propagated": 2, "systemic": 3}
    if "system_severity" in df.columns:
        df["system_severity"] = (
            df["system_severity"]
            .map(SEVERITY_MAP)
            .fillna(0)
            .astype(int)
        )

    # ── Lag + rolling features ────────────────────────────────────────────────
    for col in LAG_SOURCE_COLS:
        for lag in LAG_STEPS:
            df[f"{col}_lag{lag}"] = df[col].shift(lag)
        for window in ROLL_WINDOWS:
            df[f"{col}_roll{window}"] = df[col].shift(1).rolling(window).mean()

    # ── Derived features ──────────────────────────────────────────────────────
    df["intensity_above_stable"] = (
        df["fault_intensity"] - df["max_stable_intensity"]
    ).clip(lower=0)

    df["days_since_start"] = (
        df[DATE_COL] - df[DATE_COL].min()
    ).dt.days

    df["experiment_index"] = df.index

    # ── Drop NaN rows (from lag warm-up period) ───────────────────────────────
    df = df.dropna().reset_index(drop=True)

    return df


def get_feature_columns(df: pd.DataFrame) -> list[str]:
    """
    Return the ordered list of LightGBM input columns.
    Excludes target, date, and id — everything else is a feature.
    """
    exclude = {TARGET, DATE_COL, ID_COL}
    return [c for c in df.columns if c not in exclude]


def save_feature_list(feature_cols: list[str]) -> None:
    """Persist the feature column order so inference loads the same list."""
    with open(FEATURE_LIST_PATH, "w") as f:
        json.dump(feature_cols, f, indent=2)
    print(f"[features] Saved {len(feature_cols)} feature names → {FEATURE_LIST_PATH}")


def load_feature_list() -> list[str]:
    """Load the feature list saved during training."""
    if not FEATURE_LIST_PATH.exists():
        raise FileNotFoundError(
            f"Feature list not found at {FEATURE_LIST_PATH}. "
            "Run train.py first."
        )
    with open(FEATURE_LIST_PATH) as f:
        cols = json.load(f)
    return cols
