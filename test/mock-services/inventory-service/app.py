import os
from flask import Flask, jsonify

app = Flask(__name__)
PORT = int(os.environ.get('PORT', 8084))

INVENTORY = {
    "A1": {"item_id": "A1", "name": "Wireless Mouse", "stock": 100, "status": "in_stock"},
    "A2": {"item_id": "A2", "name": "Mechanical Keyboard", "stock": 50, "status": "in_stock"},
    "B1": {"item_id": "B1", "name": "USB-C Hub", "stock": 200, "status": "in_stock"},
    "B2": {"item_id": "B2", "name": "Monitor Stand", "stock": 5, "status": "low_stock"},
    "C1": {"item_id": "C1", "name": "HDMI Cable", "stock": 500, "status": "in_stock"},
    "C2": {"item_id": "C2", "name": "Webcam 1080p", "stock": 15, "status": "in_stock"},
    "C3": {"item_id": "C3", "name": "Bluetooth Speaker", "stock": 0, "status": "out_of_stock"},
    "D1": {"item_id": "D1", "name": "Ergonomic Chair", "stock": 2, "status": "low_stock"}
}

@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "inventory-service"})

@app.route('/inventory/<item_id>', methods=['GET'])
def get_inventory(item_id):
    item = INVENTORY.get(item_id, {"item_id": item_id, "stock": 0, "status": "unknown_item"})
    return jsonify(item)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=PORT)
