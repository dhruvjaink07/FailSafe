import os
import requests
from flask import Flask, jsonify

app = Flask(__name__)

USER_SERVICE_URL = os.environ.get('USER_SERVICE_URL', 'http://user-service:8081')
ORDER_SERVICE_URL = os.environ.get('ORDER_SERVICE_URL', 'http://order-service:8082')
PORT = int(os.environ.get('PORT', 8080))

@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "api-gateway"})

@app.route('/api/users/<user_id>', methods=['GET'])
def get_user(user_id):
    try:
        response = requests.get(f"{USER_SERVICE_URL}/users/{user_id}", timeout=5)
        return jsonify(response.json()), response.status_code
    except Exception as e:
        return jsonify({"error": str(e), "service": "api-gateway"}), 500

@app.route('/api/orders/<order_id>', methods=['GET'])
def get_order(order_id):
    try:
        response = requests.get(f"{ORDER_SERVICE_URL}/orders/{order_id}", timeout=5)
        return jsonify(response.json()), response.status_code
    except Exception as e:
        return jsonify({"error": str(e), "service": "api-gateway"}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=PORT)
