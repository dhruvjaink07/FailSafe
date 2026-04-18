"""
failsafe/train.py
─────────────────
Train the SARIMA + LightGBM ensemble and save all model artifacts.

Usage:
    python -m failsafe.train
    python -m failsafe.train --data path/to/experiments.csv

What this script does:
    1. Load + feature-engineer the historical experiment dataset
    2. Temporal train/test split (last 20% = held-out test)
    3. Fit SARIMA(1,d,1) on stability_score series
    4. Fit LightGBM with TimeSeriesSplit cross-validation
    5. Search for optimal ensemble weights (minimize test MAE)
    6. Evaluate on held-out test set + print tier accuracy
    7. Save models, weights, and feature list to models/

What this script does NOT do:
    - SHAP explanation  →  see explain.py
    - Runtime inference →  see infer.py
"""

import warnings
warnings.filterwarnings("ignore")

import argparse
import json
import pickle
import numpy as np
import pandas as pd

from statsmodels.tsa.statespace.sarimax import SARIMAX
from statsmodels.tsa.stattools import adfuller
import lightgbm as lgb
from sklearn.model_selection import TimeSeriesSplit
from sklearn.metrics import mean_absolute_error, mean_squared_error

from .config import (
    DATA_PATH, LGB_MODEL_PATH, SARIMA_META_PATH, OUTPUTS_DIR,
    TARGET, DATE_COL, ID_COL,
    SARIMA_ORDER, SARIMA_SEASONAL_ORDER, SARIMA_FIT_KWARGS,
    LGB_PARAMS, LGB_CV_SPLITS, LGB_EARLY_STOP, TRAIN_TEST_SPLIT,
    risk_tier,
)
from .features import (
    build_features, get_feature_columns, save_feature_list,
)


# ─────────────────────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────────────────────

def _fit_sarima(series: np.ndarray, d: int):
    """Fit a single SARIMA model on the given series."""
    order = (SARIMA_ORDER[0], d, SARIMA_ORDER[2])
    model = SARIMAX(
        series,
        order=order,
        seasonal_order=SARIMA_SEASONAL_ORDER,
        enforce_stationarity=False,
        enforce_invertibility=False,
        initialization="approximate_diffuse",
    )
    return model.fit(**SARIMA_FIT_KWARGS)


def _rolling_sarima_forecast(train_series: np.ndarray,
                              test_series: np.ndarray,
                              d: int) -> np.ndarray:
    """
    One-step-ahead rolling forecast over the test set.
    After each step, the true value is appended to history
    so the model always has the freshest context.
    """
    history = list(train_series)
    preds   = []

    for true_val in test_series:
        fit = _fit_sarima(np.array(history), d)
        fc  = float(fit.forecast(1)[0])
        preds.append(fc)
        history.append(true_val)   # use truth, not prediction (teacher forcing)

    return np.array(preds)


# ─────────────────────────────────────────────────────────────────────────────
# Main training routine
# ─────────────────────────────────────────────────────────────────────────────

def train(data_path=None):
    path = data_path or DATA_PATH
    print("=" * 60)
    print("Failsafe — Training pipeline")
    print("=" * 60)

    # ── 1. Load & engineer features ───────────────────────────────────────────
    raw = pd.read_csv(path, parse_dates=[DATE_COL])
    raw = raw.sort_values(DATE_COL).reset_index(drop=True)
    print(f"Loaded {len(raw)} experiments from {path}")

    df = build_features(raw)
    print(f"After feature engineering: {len(df)} usable rows")

    feature_cols = get_feature_columns(df)
    save_feature_list(feature_cols)
    print(f"Feature columns: {len(feature_cols)}")

    # ── 2. Train / test split ─────────────────────────────────────────────────
    split_idx = int(len(df) * TRAIN_TEST_SPLIT)
    train_df  = df.iloc[:split_idx].reset_index(drop=True)
    test_df   = df.iloc[split_idx:].reset_index(drop=True)
    print(f"\nTrain: {len(train_df)} rows  |  Test: {len(test_df)} rows")

    X_train = train_df[feature_cols]
    y_train = train_df[TARGET]
    X_test  = test_df[feature_cols]
    y_test  = test_df[TARGET]

    # ── 3. SARIMA ─────────────────────────────────────────────────────────────
    print("\n[SARIMA] Fitting …")
    train_stability = train_df["stability_score"].values
    test_stability  = test_df["stability_score"].values

    adf_p = adfuller(train_stability, autolag="AIC")[1]
    d     = 0 if adf_p < 0.05 else 1
    print(f"  ADF p={adf_p:.4f}  →  d={d}")

    # Final model on full training series (saved for future inference)
    sarima_final = _fit_sarima(train_stability, d)
    print(f"  AIC: {sarima_final.aic:.2f}")

    # Evaluate via rolling forecast on test set
    sarima_stability_preds = _rolling_sarima_forecast(
        train_stability, test_stability, d
    )
    sarima_risk = np.clip(100 - sarima_stability_preds, 0, 100)
    mae_sarima  = mean_absolute_error(y_test, sarima_risk)
    print(f"  Test MAE (risk): {mae_sarima:.3f}")

    # ── 4. LightGBM ──────────────────────────────────────────────────────────
    print("\n[LightGBM] Training …")
    tscv   = TimeSeriesSplit(n_splits=LGB_CV_SPLITS)
    cv_maes = []

    for tr_idx, val_idx in tscv.split(X_train):
        m = lgb.LGBMRegressor(**LGB_PARAMS)
        m.fit(
            X_train.iloc[tr_idx], y_train.iloc[tr_idx],
            eval_set=[(X_train.iloc[val_idx], y_train.iloc[val_idx])],
            callbacks=[
                lgb.early_stopping(LGB_EARLY_STOP, verbose=False),
                lgb.log_evaluation(-1),
            ],
        )
        cv_maes.append(mean_absolute_error(y_train.iloc[val_idx], m.predict(X_train.iloc[val_idx])))

    print(f"  CV MAE folds: {[round(x, 3) for x in cv_maes]}")
    print(f"  CV MAE mean : {np.mean(cv_maes):.3f}")

    lgb_model = lgb.LGBMRegressor(**LGB_PARAMS)
    lgb_model.fit(
        X_train, y_train,
        eval_set=[(X_test, y_test)],
        callbacks=[
            lgb.early_stopping(LGB_EARLY_STOP, verbose=False),
            lgb.log_evaluation(-1),
        ],
    )
    lgb_preds = lgb_model.predict(X_test)
    mae_lgb   = mean_absolute_error(y_test, lgb_preds)
    print(f"  Test MAE: {mae_lgb:.3f}")

    # ── 5. Ensemble weight search ─────────────────────────────────────────────
    print("\n[Ensemble] Searching optimal weights …")
    best_mae, best_w = 1e9, 0.0

    for w in np.arange(0.0, 1.01, 0.05):
        ens = np.clip(w * sarima_risk + (1 - w) * lgb_preds, 0, 100)
        m   = mean_absolute_error(y_test, ens)
        if m < best_mae:
            best_mae = m
            best_w   = w

    W_SARIMA = best_w
    W_LGB    = 1 - best_w
    ensemble_preds = np.clip(W_SARIMA * sarima_risk + W_LGB * lgb_preds, 0, 100)

    mae_ens  = mean_absolute_error(y_test, ensemble_preds)
    rmse_ens = np.sqrt(mean_squared_error(y_test, ensemble_preds))

    print(f"  Weights → SARIMA={W_SARIMA:.2f}  LGB={W_LGB:.2f}")
    print(f"  MAE={mae_ens:.3f}  RMSE={rmse_ens:.3f}")
    print(f"  [SARIMA={mae_sarima:.3f}  LGB={mae_lgb:.3f}  Ensemble={mae_ens:.3f}]")

    # ── 6. Tier accuracy ──────────────────────────────────────────────────────
    true_tiers = y_test.apply(risk_tier)
    pred_tiers = pd.Series(ensemble_preds).apply(risk_tier)
    tier_acc   = (true_tiers.values == pred_tiers.values).mean() * 100
    print(f"\n[Risk Tiers] Accuracy: {tier_acc:.1f}%")

    # ── 7. Save artifacts ─────────────────────────────────────────────────────
    print("\n[Save] Writing model artifacts …")

    # LightGBM booster
    lgb_model.booster_.save_model(str(LGB_MODEL_PATH))
    print(f"  LightGBM → {LGB_MODEL_PATH}")

    # SARIMA meta (fit object + constants needed at inference time)
    sarima_meta = {
        "d":        d,
        "W_SARIMA": W_SARIMA,
        "W_LGB":    W_LGB,
        # Store the last N stability_score values so infer.py can extend history
        "history_stability": train_df["stability_score"].tolist()
                             + test_df["stability_score"].tolist(),
    }
    with open(SARIMA_META_PATH, "wb") as f:
        pickle.dump(sarima_meta, f)
    print(f"  SARIMA meta → {SARIMA_META_PATH}")

    # Training summary JSON
    summary = {
        "n_train": len(train_df),
        "n_test":  len(test_df),
        "n_features": len(feature_cols),
        "sarima_mae":    round(mae_sarima, 3),
        "lgb_mae":       round(mae_lgb, 3),
        "ensemble_mae":  round(mae_ens, 3),
        "ensemble_rmse": round(rmse_ens, 3),
        "tier_accuracy": round(tier_acc, 1),
        "W_SARIMA": round(W_SARIMA, 2),
        "W_LGB":    round(W_LGB, 2),
    }
    summary_path = OUTPUTS_DIR / "train_summary.json"
    with open(summary_path, "w") as f:
        json.dump(summary, f, indent=2)

    print(f"\n✅ Training complete.")
    print(json.dumps(summary, indent=2))
    return summary


# ─────────────────────────────────────────────────────────────────────────────
if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--data", default=None, help="Path to experiments CSV")
    args = parser.parse_args()
    train(data_path=args.data)
