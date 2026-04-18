"""
failsafe/infer.py
─────────────────
Load saved models and predict risk on new experiment data.

Usage:
    # Predict on a single new experiment dict (from GetMetrics output)
    python -m failsafe.infer --experiment '{"fault_intensity": 0.7, ...}'

    # Predict on a CSV of new experiments
    python -m failsafe.infer --data path/to/new_experiments.csv

    # Forecast the next N experiments (no new data needed)
    python -m failsafe.infer --forecast 3

What this script does:
    1. Load LightGBM booster + SARIMA meta from models/
    2. Build features for the new experiment(s)
       - Uses recent history stored in sarima_meta so lags are correct
    3. Run SARIMA + LightGBM → weighted ensemble → risk score
    4. Classify into risk tier (LOW / MEDIUM / HIGH / CRITICAL)
    5. Print + return the result dict

What this script does NOT do:
    - Model training          →  see train.py
    - SHAP explanation        →  see explain.py
"""

import warnings
warnings.filterwarnings("ignore")

import argparse
import json
import pickle
import numpy as np
import pandas as pd

from statsmodels.tsa.statespace.sarimax import SARIMAX
import lightgbm as lgb

from .config import (
    LGB_MODEL_PATH, SARIMA_META_PATH,
    TARGET, DATE_COL, ID_COL,
    SARIMA_ORDER, SARIMA_SEASONAL_ORDER, SARIMA_FIT_KWARGS,
    RAW_FEATURE_COLS,
    risk_tier,
)
from .features import build_features, load_feature_list
from .db_utils import load_experiment_features


# ─────────────────────────────────────────────────────────────────────────────
# Model loading (cached at module level so repeated calls are fast)
# ─────────────────────────────────────────────────────────────────────────────

_lgb_model   = None
_sarima_meta = None
_feature_cols = None


def _load_models():
    global _lgb_model, _sarima_meta, _feature_cols

    if _lgb_model is None:
        if not LGB_MODEL_PATH.exists():
            raise FileNotFoundError(
                f"LightGBM model not found at {LGB_MODEL_PATH}. Run train.py first."
            )
        booster = lgb.Booster(model_file=str(LGB_MODEL_PATH))
        _lgb_model = booster
        print(f"[infer] Loaded LightGBM from {LGB_MODEL_PATH}")

    if _sarima_meta is None:
        if not SARIMA_META_PATH.exists():
            raise FileNotFoundError(
                f"SARIMA meta not found at {SARIMA_META_PATH}. Run train.py first."
            )
        with open(SARIMA_META_PATH, "rb") as f:
            _sarima_meta = pickle.load(f)
        print(f"[infer] Loaded SARIMA meta from {SARIMA_META_PATH}")

    if _feature_cols is None:
        _feature_cols = load_feature_list()

    return _lgb_model, _sarima_meta, _feature_cols


# ─────────────────────────────────────────────────────────────────────────────
# SARIMA one-step forecast
# ─────────────────────────────────────────────────────────────────────────────

def _sarima_forecast(history: list[float], d: int, steps: int = 1) -> list[float]:
    """
    Forecast `steps` values from the given stability_score history.
    Returns a list of length `steps`.
    """
    order = (SARIMA_ORDER[0], d, SARIMA_ORDER[2])
    model = SARIMAX(
        history,
        order=order,
        seasonal_order=SARIMA_SEASONAL_ORDER,
        enforce_stationarity=False,
        enforce_invertibility=False,
        initialization="approximate_diffuse",
    )
    fit = model.fit(**SARIMA_FIT_KWARGS)
    return fit.forecast(steps).tolist()


# ─────────────────────────────────────────────────────────────────────────────
# Core predict function
# ─────────────────────────────────────────────────────────────────────────────

def predict(new_experiments: pd.DataFrame,
            sarima_weight: float = None) -> pd.DataFrame:
    """
    Predict risk for one or more new experiments.

    Parameters
    ----------
    sarima_weight : float, optional
        Override the trained ensemble weight for SARIMA (0.0–1.0).
        LightGBM weight = 1 - sarima_weight.
        If None, uses the weight saved during training.
    """
    lgb_model, sarima_meta, feature_cols = _load_models()

    d = sarima_meta["d"]

    # ── Ensemble weights ──────────────────────────────────────────────────────
    if sarima_weight is not None:
        sarima_weight = float(np.clip(sarima_weight, 0.0, 1.0))
        W_SARIMA = sarima_weight
        W_LGB    = 1.0 - sarima_weight
        print(f"[infer] Ensemble weights overridden → SARIMA={W_SARIMA:.2f}  LGB={W_LGB:.2f}")
    else:
        W_SARIMA = sarima_meta["W_SARIMA"]
        W_LGB    = sarima_meta["W_LGB"]
        print(f"[infer] Ensemble weights (trained)  → SARIMA={W_SARIMA:.2f}  LGB={W_LGB:.2f}")

    history = list(sarima_meta["history_stability"])

    # ── Context Prepending ────────────────────────────────────────────────────
    # We need at least 3 historical rows to compute lags.
    # We fetch the latest 10 experiments from the DB as context.
    try:
        from .config import ID_COL, DATE_COL, TARGET
        db_history = load_experiment_features(limit=10, desc=True)
        
        # Filter out experiments that are already in our 'new' set
        if ID_COL in new_experiments.columns and ID_COL in db_history.columns:
            new_ids = set(new_experiments[ID_COL].astype(str).unique())
            db_history = db_history[~db_history[ID_COL].astype(str).isin(new_ids)]
        
        # If we have new data, only take history that happened BEFORE it
        if not new_experiments.empty and DATE_COL in new_experiments.columns:
            min_new_date = new_experiments[DATE_COL].min()
            db_history = db_history[db_history[DATE_COL] < min_new_date]

        # Combine history (context) + new data
        # We use context to calculate lags, but only return predictions for new data
        n_context = len(db_history)
        df_in = pd.concat([db_history, new_experiments], axis=0).sort_values(DATE_COL).reset_index(drop=True)
    except Exception as e:
        print(f"[infer] Warning: Could not load historical context from DB ({e})")
        n_context = 0
        df_in = new_experiments.copy().sort_values(DATE_COL).reset_index(drop=True)

    if TARGET not in df_in.columns:
        df_in[TARGET] = 0.0

    # ── Build features ────────────────────────────────────────────────────────
    df_feat_all = build_features(df_in)

    # Filter back to only the rows we actually want to predict (the 'new' ones)
    # Since build_features drops the first 3 rows of the ENTIRE set, 
    # we take the tail matching our original input size.
    n_new = len(new_experiments)
    df_feat = df_feat_all.tail(n_new).copy()

    if len(df_feat) == 0:
        print("[infer] Warning: All rows dropped. Need more historical experiments in DB.")
        return pd.DataFrame()

    # Align to the exact feature columns seen during training
    for col in feature_cols:
        if col not in df_feat.columns:
            df_feat[col] = 0.0

    X_new = df_feat[feature_cols]

    # ── LightGBM inference ────────────────────────────────────────────────────
    lgb_preds = lgb_model.predict(X_new)

    # ── SARIMA inference ──────────────────────────────────────────────────────
    sarima_risks = []
    # Note: For SARIMA, we use the full history stored in meta + any new points
    for i in range(len(df_feat)):
        fc_stability = _sarima_forecast(history, d, steps=1)[0]
        sarima_risk  = float(np.clip(100 - fc_stability, 0, 100))
        sarima_risks.append(sarima_risk)
        if "stability_score" in df_feat.columns:
            history.append(float(df_feat["stability_score"].iloc[i]))
        else:
            history.append(fc_stability)

    sarima_risks = np.array(sarima_risks)

    # ── Ensemble ──────────────────────────────────────────────────────────────
    ensemble_preds = np.clip(W_SARIMA * sarima_risks + W_LGB * lgb_preds, 0, 100)

    # ── Assemble output ───────────────────────────────────────────────────────
    result = df_feat[[DATE_COL] + ([ID_COL] if ID_COL in df_feat.columns else [])].copy()
    result["sarima_risk"]    = sarima_risks.round(2)
    result["lgb_risk"]       = lgb_preds.round(2)
    result["ensemble_risk"]  = ensemble_preds.round(2)
    result["risk_tier"]      = pd.Series(ensemble_preds).apply(risk_tier).values

    return result


def forecast(steps: int = 3, sarima_weight: float = None) -> list[dict]:
    """
    Forecast risk for the next `steps` experiments with no new data.

    Parameters
    ----------
    sarima_weight : float, optional
        Override the trained SARIMA ensemble weight (0.0–1.0).
    """
    lgb_model, sarima_meta, feature_cols = _load_models()

    d       = sarima_meta["d"]
    history = list(sarima_meta["history_stability"])

    if sarima_weight is not None:
        W_SARIMA = float(np.clip(sarima_weight, 0.0, 1.0))
        W_LGB    = 1.0 - W_SARIMA
        print(f"[infer] Ensemble weights overridden → SARIMA={W_SARIMA:.2f}  LGB={W_LGB:.2f}")
    else:
        W_SARIMA = sarima_meta["W_SARIMA"]
        W_LGB    = sarima_meta["W_LGB"]
        print(f"[infer] Ensemble weights (trained)  → SARIMA={W_SARIMA:.2f}  LGB={W_LGB:.2f}")

    # LightGBM can't see the future, so we reuse the last known feature vector
    # This is an approximation — real-world systems would extrapolate features
    # (e.g., use the scheduled fault intensity for the next experiment)
    last_feature_path = LGB_MODEL_PATH.parent / "last_feature_vector.json"
    if last_feature_path.exists():
        with open(last_feature_path) as f:
            last_vec = pd.DataFrame([json.load(f)])[feature_cols]
    else:
        # Fallback: zero vector (will give a rough estimate)
        print("[infer] Warning: last_feature_vector.json not found. "
              "LightGBM forecast will use a zero feature vector.")
        last_vec = pd.DataFrame([{c: 0.0 for c in feature_cols}])

    results = []
    for step in range(1, steps + 1):
        fc_stability = _sarima_forecast(history, d, steps=step)[-1]
        sar_risk     = float(np.clip(100 - fc_stability, 0, 100))
        lgb_risk     = float(lgb_model.predict(last_vec)[0])
        ens_risk     = float(np.clip(W_SARIMA * sar_risk + W_LGB * lgb_risk, 0, 100))

        results.append({
            "step":          step,
            "sarima_risk":   round(sar_risk, 2),
            "lgb_risk":      round(lgb_risk, 2),
            "ensemble_risk": round(ens_risk, 2),
            "risk_tier":     risk_tier(ens_risk),
        })

    return results


# ─────────────────────────────────────────────────────────────────────────────
# CLI & Output Formatting
# ─────────────────────────────────────────────────────────────────────────────

def display_results(results, title="Inference Results"):
    """Pretty-print the results with ANSI colors."""
    if isinstance(results, pd.DataFrame):
        if results.empty:
            print(f"\n[!] No results to display for {title}.\n")
            return
        records = results.to_dict("records")
    else:
        if not results:
            print(f"\n[!] No results to display for {title}.\n")
            return
        records = results

    print(f"\n┌─ {title} " + "─" * (60 - len(title)))
    
    colors = {
        "LOW": "\033[92m",            # Green
        "MEDIUM": "\033[93m",         # Yellow
        "HIGH": "\033[91m",           # Red
        "CRITICAL": "\033[95m\033[1m" # Magenta Bold
    }
    reset = "\033[0m"
    bold = "\033[1m"
    dim = "\033[2m"
    
    for row in records:
        # Date or Step
        if "date" in row or "created_at" in row:
            date_val = str(row.get("date") or row.get("created_at", "N/A"))
            prefix = date_val[:19] # YYYY-MM-DD HH:MM:SS
        elif "step" in row:
            prefix = f"Future Step +{row['step']}"
        else:
            prefix = "New Experiment"
            
        # ID
        exp_id = ""
        from .config import ID_COL
        if ID_COL in row:
            exp_id = f" ({str(row[ID_COL])[:8]})"
            
        tier = row.get("risk_tier", "UNKNOWN")
        color = colors.get(tier, reset)
        
        sarima = row.get('sarima_risk', 0.0)
        lgb = row.get('lgb_risk', 0.0)
        ens = row.get('ensemble_risk', 0.0)
        
        print(f"│ {dim}[{prefix:<19}]{reset}{exp_id:<11} │ "
              f"SARIMA: {sarima:>5.1f}% │ LGB: {lgb:>5.1f}% │ "
              f"Risk: {color}{bold}{ens:>5.1f}% ({tier}){reset}")
              
    print("└" + "─" * 63 + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Failsafe inference")
    parser.add_argument("--experiment", type=str, default=None,
                        help="JSON string of a single experiment dict")
    parser.add_argument("--data", type=str, default=None,
                        help="Path to CSV of new experiments")
    parser.add_argument("--from-db", action="store_true", dest="from_db",
                        help="Load recent experiments from the configured database")
    parser.add_argument("--db-limit", type=int, default=None,
                        help="Limit number of experiments loaded from DB")
    parser.add_argument("--forecast", type=int, default=None,
                        help="Forecast N future experiments")
    parser.add_argument("--sarima-weight", type=float, default=None, dest="sarima_weight",
                        help="Override SARIMA ensemble weight (0.0–1.0). LGB = 1 - this value. "
                             "E.g. --sarima-weight 0.7 gives SARIMA 70%% and LGB 30%%.")
    args = parser.parse_args()

    if args.forecast:
        results = forecast(steps=args.forecast, sarima_weight=args.sarima_weight)
        display_results(results, title=f"Forecast ({args.forecast} steps)")

    elif args.experiment:
        row = json.loads(args.experiment)
        df  = pd.DataFrame([row])
        if DATE_COL not in df.columns:
            df[DATE_COL] = pd.Timestamp.now()
        out = predict(df, sarima_weight=args.sarima_weight)
        display_results(out, title="Single Experiment Inference")

    elif args.data:
        df  = pd.read_csv(args.data, parse_dates=[DATE_COL])
        out = predict(df, sarima_weight=args.sarima_weight)
        display_results(out, title=f"Batch Inference ({args.data})")

    elif args.from_db:
        df = load_experiment_features(limit=args.db_limit)
        if DATE_COL not in df.columns:
            df[DATE_COL] = pd.to_datetime(df["date"]) if "date" in df.columns else pd.Timestamp.now()
        out = predict(df, sarima_weight=args.sarima_weight)
        display_results(out, title="Database Inference")

    else:
        print("Provide --experiment, --data, or --forecast. See --help.")
