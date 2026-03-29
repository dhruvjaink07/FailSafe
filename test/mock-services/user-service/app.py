import os
import requests
from flask import Flask, jsonify

app = Flask(__name__)
PORT = int(os.environ.get('PORT', 8081))
RECOMMENDATION_SERVICE_URL = os.environ.get('RECOMMENDATION_SERVICE_URL', 'http://recommendation-service:8087')

USERS = {
    "1": {"id": "1", "name": "Alice Wonderland", "email": "alice@example.com", "tier": "premium"},
    "2": {"id": "2", "name": "Bob Builder", "email": "bob@example.com", "tier": "standard"},
    "3": {"id": "3", "name": "Charlie Chaplin", "email": "charlie@example.com", "tier": "standard"},
    "4": {"id": "4", "name": "Diana Prince", "email": "diana@example.com", "tier": "premium"},
    "5": {"id": "5", "name": "Evan Hansen", "email": "evan@example.com", "tier": "basic"},
    "6": {"id": "6", "name": "Fiona Gallagher", "email": "fiona@example.com", "tier": "standard"},
    "7": {"id": "7", "name": "George Constanza", "email": "george@example.com", "tier": "basic"},
    "8": {"id": "8", "name": "Hannah Abbott", "email": "hannah@example.com", "tier": "premium"},
    "9": {"id": "9", "name": "Ian Malcolm", "email": "ian@example.com", "tier": "standard"},
    "10": {"id": "10", "name": "Jane Eyre", "email": "jane@example.com", "tier": "premium"}
}

@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "user-service"})

@app.route('/users/<user_id>', methods=['GET'])
def get_user(user_id):
    user = USERS.get(user_id)
    if not user:
        return jsonify({"error": "User not found"}), 404
        
    recommendations = {}
    try:
        rec_resp = requests.get(f"{RECOMMENDATION_SERVICE_URL}/recommendations/{user_id}", timeout=5)
        if rec_resp.status_code == 200:
            recommendations = rec_resp.json()
    except Exception as e:
        recommendations = {"error": str(e)}

    return jsonify({
        "user": user,
        "recommendations_data": recommendations
    })

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=PORT)
