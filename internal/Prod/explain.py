"""
failsafe/explain.py
────────────────────
SHAP-based explainability for the LightGBM component of the ensemble.

Usage:
    # Global feature importance over the full dataset
    python -m failsafe.explain --mode global --data path/to/experiments.csv

    # Explain a single experiment (local explanation)
    python -m failsafe.explain --mode local --experiment '{"fault_intensity": 0.7, ...}'

    # Explain a specific experiment by index from the dataset
    python -m failsafe.explain --mode local --data path/to/experiments.csv --idx 42

What this script does:
    1. Load the trained LightGBM model
    2. Build a SHAP TreeExplainer on it
    3. Compute SHAP values for the requested data
    4. For GLOBAL mode:
         - Mean |SHAP| per feature (importance ranking)
         - Save ranked importance to outputs/shap_global.csv
    5. For LOCAL mode (single experiment):
         - Show per-feature SHAP contribution for that prediction
         - Show which features pushed risk UP and which pushed it DOWN
         - Save to outputs/shap_local_exp{idx}.json

What this script does NOT do:
    - Model training   →  see train.py
    - Live inference   →  see infer.py
"""

import warnings
warnings.filterwarnings("ignore")

import argparse
import json
import pickle
import numpy as np
import pandas as pd
import shap

import lightgbm as lgb

from .config import (
    LGB_MODEL_PATH, SARIMA_META_PATH, OUTPUTS_DIR,
    TARGET, DATE_COL, ID_COL,
    risk_tier,
)
from .features import build_features, load_feature_list


# ─────────────────────────────────────────────────────────────────────────────
# Load model + build explainer
# ─────────────────────────────────────────────────────────────────────────────

def _load_explainer():
    if not LGB_MODEL_PATH.exists():
        raise FileNotFoundError(
            f"LightGBM model not found at {LGB_MODEL_PATH}. Run train.py first."
        )
    booster     = lgb.Booster(model_file=str(LGB_MODEL_PATH))
    explainer   = shap.TreeExplainer(booster)
    feature_cols = load_feature_list()
    return booster, explainer, feature_cols


def _prepare_features(data_path=None, experiment_dict=None) -> pd.DataFrame:
    """Load and feature-engineer data from either a CSV path or a raw dict."""
    if data_path:
        df = pd.read_csv(data_path, parse_dates=[DATE_COL])
    elif experiment_dict:
        df = pd.DataFrame([experiment_dict])
        if DATE_COL not in df.columns:
            df[DATE_COL] = pd.Timestamp.now()
    else:
        raise ValueError("Provide either data_path or experiment_dict.")

    if TARGET not in df.columns:
        df[TARGET] = 0.0

    return build_features(df)


# ─────────────────────────────────────────────────────────────────────────────
# Global explanation
# ─────────────────────────────────────────────────────────────────────────────

def explain_global(data_path: str) -> pd.DataFrame:
    """
    Compute mean |SHAP| for every feature across the full dataset.
    Returns a DataFrame sorted by importance (most → least).
    """
    booster, explainer, feature_cols = _load_explainer()
    df      = _prepare_features(data_path=data_path)
    X       = df[feature_cols]

    print(f"[explain] Computing SHAP values for {len(X)} experiments …")
    shap_values = explainer.shap_values(X)   # shape: (n_rows, n_features)

    importance = pd.DataFrame({
        "feature":       feature_cols,
        "mean_abs_shap": np.abs(shap_values).mean(axis=0),
        "mean_shap":     shap_values.mean(axis=0),        # sign = direction of effect
    }).sort_values("mean_abs_shap", ascending=False).reset_index(drop=True)

    importance["rank"] = importance.index + 1

    out_path = OUTPUTS_DIR / "shap_global.csv"
    importance.to_csv(out_path, index=False)

    print(f"\n{'Rank':<6} {'Feature':<35} {'Mean |SHAP|':>12}  {'Direction'}")
    print("-" * 65)
    for _, row in importance.head(15).iterrows():
        direction = "▲ raises risk" if row["mean_shap"] > 0 else "▼ lowers risk"
        print(f"  {int(row['rank']):<4} {row['feature']:<35} {row['mean_abs_shap']:>10.4f}  {direction}")

    print(f"\n✅ Full importance saved → {out_path}")
    return importance


# ─────────────────────────────────────────────────────────────────────────────
# Local explanation (single experiment)
# ─────────────────────────────────────────────────────────────────────────────

def explain_local(data_path: str = None,
                  experiment_dict: dict = None,
                  idx: int = 0) -> dict:
    """
    Explain a single experiment's predicted risk score.

    Shows:
      - The ensemble risk prediction
      - SHAP base value (average model prediction)
      - Top features that RAISED the score
      - Top features that LOWERED the score

    Returns a dict suitable for passing to the Groq report generator.
    """
    booster, explainer, feature_cols = _load_explainer()

    # Load SARIMA meta for ensemble weights
    with open(SARIMA_META_PATH, "rb") as f:
        sarima_meta = pickle.load(f)
    W_LGB = sarima_meta["W_LGB"]

    df = _prepare_features(data_path=data_path, experiment_dict=experiment_dict)
    X  = df[feature_cols]

    if idx >= len(X):
        raise IndexError(f"idx={idx} out of range for dataset of {len(X)} rows.")

    row_X      = X.iloc[[idx]]
    row_df     = df.iloc[idx]

    shap_values = explainer.shap_values(X)        # all rows
    row_shap    = shap_values[idx]                 # 1D array: shap per feature
    base_value  = float(explainer.expected_value)  # model's average prediction

    lgb_pred    = float(booster.predict(row_X)[0])
    # Note: we report LGB prediction here since SHAP explains only LGB.
    # The ensemble risk would add SARIMA contribution on top.

    # Build per-feature contribution table
    contributions = pd.DataFrame({
        "feature":     feature_cols,
        "value":       row_X.values[0],
        "shap":        row_shap,
        "abs_shap":    np.abs(row_shap),
    }).sort_values("abs_shap", ascending=False).reset_index(drop=True)

    top_risk_drivers = contributions[contributions["shap"] > 0].head(5)
    top_risk_reducers = contributions[contributions["shap"] < 0].head(5)

    # ── Print ─────────────────────────────────────────────────────────────────
    exp_id = row_df.get(ID_COL, idx) if hasattr(row_df, "get") else idx
    print(f"\n{'='*60}")
    print(f"Local SHAP explanation — Experiment {exp_id}")
    print(f"{'='*60}")
    print(f"  LightGBM prediction : {lgb_pred:.2f}")
    print(f"  SHAP base value     : {base_value:.2f}")
    print(f"  Risk tier           : {risk_tier(lgb_pred)}")
    print(f"  (SHAP explains LGB component; ensemble also adds SARIMA trend)")

    print(f"\n  ▲ Top features RAISING risk:")
    for _, r in top_risk_drivers.iterrows():
        print(f"    {r['feature']:<35} val={r['value']:>8.3f}  SHAP=+{r['shap']:.4f}")

    print(f"\n  ▼ Top features LOWERING risk:")
    for _, r in top_risk_reducers.iterrows():
        print(f"    {r['feature']:<35} val={r['value']:>8.3f}  SHAP={r['shap']:.4f}")

    # ── Build output dict ─────────────────────────────────────────────────────
    result = {
        "experiment_id":  int(exp_id) if hasattr(exp_id, "item") else exp_id,
        "lgb_prediction": round(lgb_pred, 2),
        "shap_base_value": round(base_value, 2),
        "risk_tier":      risk_tier(lgb_pred),
        "top_risk_drivers": [
            {
                "feature": r["feature"],
                "value":   round(float(r["value"]), 4),
                "shap":    round(float(r["shap"]), 4),
            }
            for _, r in top_risk_drivers.iterrows()
        ],
        "top_risk_reducers": [
            {
                "feature": r["feature"],
                "value":   round(float(r["value"]), 4),
                "shap":    round(float(r["shap"]), 4),
            }
            for _, r in top_risk_reducers.iterrows()
        ],
        "all_contributions": [
            {
                "feature": r["feature"],
                "value":   round(float(r["value"]), 4),
                "shap":    round(float(r["shap"]), 4),
            }
            for _, r in contributions.iterrows()
        ],
    }

    out_path = OUTPUTS_DIR / f"shap_local_exp{exp_id}.json"
    with open(out_path, "w") as f:
        json.dump(result, f, indent=2)
    print(f"\n✅ Local SHAP explanation saved → {out_path}")

    return result


# ─────────────────────────────────────────────────────────────────────────────
if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Failsafe SHAP explainer")
    parser.add_argument("--mode", choices=["global", "local"], required=True)
    parser.add_argument("--data", type=str, default=None,
                        help="Path to experiments CSV")
    parser.add_argument("--experiment", type=str, default=None,
                        help="JSON string of a single experiment (local mode)")
    parser.add_argument("--idx", type=int, default=0,
                        help="Row index to explain (local mode, default=0)")
    args = parser.parse_args()

    if args.mode == "global":
        if not args.data:
            parser.error("--mode global requires --data")
        explain_global(data_path=args.data)

    elif args.mode == "local":
        if args.experiment:
            explain_local(experiment_dict=json.loads(args.experiment), idx=0)
        elif args.data:
            explain_local(data_path=args.data, idx=args.idx)
        else:
            parser.error("--mode local requires --data or --experiment")
