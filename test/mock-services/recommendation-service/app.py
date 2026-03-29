import os
import requests
from flask import Flask, jsonify

app = Flask(__name__)
PORT = int(os.environ.get('PORT', 8087))
INVENTORY_SERVICE_URL = os.environ.get('INVENTORY_SERVICE_URL', 'http://inventory-service:8084')

# Mapping user_id to an array of recommended item_ids
RECOMMENDATIONS = {
    "1": ["A1", "A2", "B1"],
    "2": ["C1", "C2", "C3"],
    "3": ["A1", "B2", "D1"],
    "4": ["A2", "C1", "D1"],
    "5": ["B1", "B2", "C2"],
    "6": ["A1", "C3"],
    "7": ["C1", "D1"],
    "8": ["A2", "B1", "C2"],
    "9": ["B2", "C3", "D1"],
    "10": ["A1", "C1"]
}

@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "recommendation-service"})

@app.route('/recommendations/<user_id>', methods=['GET'])
def get_recommendations(user_id):
    item_ids = RECOMMENDATIONS.get(user_id, ["A1"])
    
    # check inventory for the first recommended item just to branch out dependency
    in_stock = "unknown"
    first_item = item_ids[0] if item_ids else "A1"
    try:
        inv_resp = requests.get(f"{INVENTORY_SERVICE_URL}/inventory/{first_item}", timeout=5)
        if inv_resp.status_code == 200:
            in_stock = inv_resp.json().get('status', 'unknown')
    except Exception as e:
        in_stock = f"error: {str(e)}"
        
    return jsonify({"user_id": user_id, "recommendations": item_ids, "top_item_stock": in_stock})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=PORT)
