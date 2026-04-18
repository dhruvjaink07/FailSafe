"""
api.py
──────
REST API for the Failsafe ML Pipeline. Serves inference and explainability results
to the frontend and Go orchestrator.
"""

from flask import Flask, jsonify, request
from flask_cors import CORS
import pandas as pd
import json

from .infer import predict, forecast
from .explain import explain_global, explain_local
from .db_utils import load_experiment_features
from .config import DATE_COL

def load_env():
    import os
    # Try multiple possible paths for .env
    possible_paths = [
        os.path.join(os.getcwd(), ".env"),
        os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(__file__))), ".env"),
        ".env"
    ]
    
    log_file = os.path.join(os.getcwd(), "ml_api_debug.log")
    with open(log_file, "a") as log:
        log.write(f"\n--- API Startup (CWD: {os.getcwd()}) ---\n")
        
        found = False
        for env_path in possible_paths:
            if os.path.exists(env_path):
                log.write(f"Found .env at: {env_path}\n")
                with open(env_path, "r") as f:
                    for line in f:
                        line = line.strip()
                        if line and not line.startswith("#") and "=" in line:
                            k, v = line.split("=", 1)
                            key = k.strip()
                            val = v.strip()
                            if key not in os.environ:
                                os.environ[key] = val
                found = True
                break
        
        if not found:
            log.write("ERROR: No .env file found in any searched path!\n")
        
        db_url = os.environ.get("DB_URL", "NOT_SET")
        log.write(f"DB_URL is now: {db_url[:20]}...\n")

load_env()

app = Flask(__name__)
CORS(app)  # Allow frontend to call directly

@app.errorhandler(Exception)
def handle_exception(e):
    import traceback
    import os
    log_file = os.path.join(os.getcwd(), "ml_api_debug.log")
    with open(log_file, "a") as log:
        log.write(f"\n--- ERROR at {pd.Timestamp.now()} ---\n")
        log.write(traceback.format_exc())
    
    # Return JSON error
    response = {
        "error": str(e),
        "type": e.__class__.__name__
    }
    return jsonify(response), 500

@app.route("/api/health", methods=["GET"])
def health():
    return jsonify({"status": "ok", "service": "failsafe-ml", "db_connected": "DB_URL" in os.environ})


@app.route("/api/predict/latest", methods=["GET"])
def predict_latest():
    """Predict risk for the latest experiments loaded from DB."""
    limit = request.args.get("limit", default=20, type=int)
    sarima_weight = request.args.get("sarima_weight", default=None, type=float)
    
    df = load_experiment_features(limit=limit, desc=True)
    if df.empty:
        return jsonify([])
        
    if DATE_COL not in df.columns:
        df[DATE_COL] = pd.to_datetime(df["date"]) if "date" in df.columns else pd.Timestamp.now()
        
    out = predict(df, sarima_weight=sarima_weight)
    
    # Convert dates to string for JSON serialization
    if DATE_COL in out.columns:
        out[DATE_COL] = out[DATE_COL].dt.strftime("%Y-%m-%dT%H:%M:%S")
        
    return jsonify(out.to_dict(orient="records"))


@app.route("/api/predict", methods=["POST"])
def predict_experiment():
    """Predict risk for a provided experiment JSON payload."""
    data = request.json
    sarima_weight = request.args.get("sarima_weight", default=None, type=float)
    
    if not data:
        return jsonify({"error": "Missing JSON body"}), 400
        
    df = pd.DataFrame([data])
    if DATE_COL not in df.columns:
        df[DATE_COL] = pd.Timestamp.now()
        
    out = predict(df, sarima_weight=sarima_weight)
    
    if DATE_COL in out.columns:
        out[DATE_COL] = out[DATE_COL].dt.strftime("%Y-%m-%dT%H:%M:%S")
        
    return jsonify(out.to_dict(orient="records")[0])


@app.route("/api/forecast", methods=["GET"])
def get_forecast():
    """Forecast N steps into the future."""
    steps = request.args.get("steps", default=3, type=int)
    sarima_weight = request.args.get("sarima_weight", default=None, type=float)
    
    try:
        results = forecast(steps=steps, sarima_weight=sarima_weight)
        return jsonify(results)
    except Exception as e:
        # If forecast fails (e.g. not enough history), return a helpful empty response
        app.logger.error(f"Forecast failed: {e}")
        return jsonify([])


@app.route("/api/explain/global", methods=["GET"])
def get_explain_global():
    """Get global SHAP feature importance."""
    # explain.py global_explain writes to CSV, we can read it or adapt it.
    from .config import OUTPUTS_DIR
    df = load_experiment_features(limit=200) # Get a good sample for global
    if df.empty:
        return jsonify([])
        
    temp_csv = OUTPUTS_DIR / "temp_global_explain.csv"
    df.to_csv(temp_csv, index=False)
    
    try:
        explain_global(str(temp_csv))
    except Exception as e:
        app.logger.error(f"Global explanation failed: {e}")
        if temp_csv.exists():
            temp_csv.unlink()
        return jsonify([])
    
    # Cleanup temp file if desired, though leaving it is fine for cache
    if temp_csv.exists():
        temp_csv.unlink()

    csv_path = OUTPUTS_DIR / "shap_global.csv"
    if not csv_path.exists():
        return jsonify({"error": "Explanation failed"}), 500
        
    df_shap = pd.read_csv(csv_path)
    return jsonify(df_shap.to_dict(orient="records"))


@app.route("/api/explain/local", methods=["GET"])
def explain_local():
    """Get local SHAP explanation for a specific experiment ID."""
    exp_id = request.args.get("id")
    if not exp_id:
        return jsonify({"error": "Missing 'id' query parameter"}), 400
        
    from .config import OUTPUTS_DIR
    # explain_local requires either data_path or experiment_dict. If we only have exp_id, we need to load from DB and pass experiment_dict or pass the DB output to explain_local?
    # explain.py doesn't have a direct "load from DB" yet, so let's load from DB and pass to explain_local.
    df = load_experiment_features(limit=50) # Get recent history
    if df.empty:
        return jsonify({"error": "No data in DB to explain"}), 404
        
    # Find the row with the given ID
    from .config import ID_COL
    if ID_COL in df.columns:
        row = df[df[ID_COL].astype(str) == exp_id]
        if row.empty:
            return jsonify({"error": f"Experiment {exp_id} not found"}), 404
        exp_dict = row.iloc[0].to_dict()
    else:
        exp_dict = df.iloc[-1].to_dict() # Fallback to latest

    # explain_local saves to outputs/shap_local_exp{idx}.json. If we pass exp_dict, it uses idx=0 by default and saves to shap_local_exp0.json or exp_id.json if ID_COL is present.
    try:
        data = explain_local(experiment_dict=exp_dict)
        return jsonify(data)
    except Exception as e:
        return jsonify({"error": str(e)}), 500



if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=True)
