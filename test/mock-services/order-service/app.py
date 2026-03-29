import os
import requests
from flask import Flask, jsonify

app = Flask(__name__)
PORT = int(os.environ.get('PORT', 8082))
PAYMENT_SERVICE_URL = os.environ.get('PAYMENT_SERVICE_URL', 'http://payment-service:8083')
INVENTORY_SERVICE_URL = os.environ.get('INVENTORY_SERVICE_URL', 'http://inventory-service:8084')
SHIPPING_SERVICE_URL = os.environ.get('SHIPPING_SERVICE_URL', 'http://shipping-service:8085')

ORDERS = {
    "100": {"id": "100", "user_id": "1", "item_id": "A1", "quantity": 2, "amount": 50.0},
    "101": {"id": "101", "user_id": "2", "item_id": "B2", "quantity": 1, "amount": 100.0},
    "102": {"id": "102", "user_id": "3", "item_id": "C1", "quantity": 5, "amount": 25.0},
    "103": {"id": "103", "user_id": "4", "item_id": "A2", "quantity": 1, "amount": 750.0},
    "104": {"id": "104", "user_id": "5", "item_id": "D1", "quantity": 10, "amount": 10.0},
    "105": {"id": "105", "user_id": "6", "item_id": "A1", "quantity": 1, "amount": 25.0},
    "106": {"id": "106", "user_id": "7", "item_id": "B1", "quantity": 3, "amount": 150.0},
    "107": {"id": "107", "user_id": "8", "item_id": "C2", "quantity": 2, "amount": 200.0},
    "108": {"id": "108", "user_id": "9", "item_id": "C3", "quantity": 4, "amount": 40.0},
    "109": {"id": "109", "user_id": "10", "item_id": "A1", "quantity": 1, "amount": 25.0}
}


@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "order-service"})

@app.route('/orders/<order_id>', methods=['GET'])
def get_order(order_id):
    order = ORDERS.get(order_id)
    if not order:
        return jsonify({"error": "Order not found"}), 404
        
    inventory_status = "unknown"
    try:
        inv_response = requests.get(f"{INVENTORY_SERVICE_URL}/inventory/{order['item_id']}", timeout=5)
        if inv_response.status_code == 200:
            inventory_status = inv_response.json().get('status', 'unknown')
    except Exception as e:
        inventory_status = f"error: {str(e)}"
        
    payment_status = "unknown"
    try:
        pay_response = requests.get(f"{PAYMENT_SERVICE_URL}/payments/{order_id}", timeout=5)
        if pay_response.status_code == 200:
            payment_status = pay_response.json().get('status', 'unknown')
    except Exception as e:
        payment_status = f"error: {str(e)}"

    shipping_info = "unknown"
    try:
        ship_response = requests.get(f"{SHIPPING_SERVICE_URL}/shipping/{order_id}", timeout=5)
        if ship_response.status_code == 200:
            shipping_info = ship_response.json()
    except Exception as e:
        shipping_info = f"error: {str(e)}"

    return jsonify({
        "order": order,
        "inventory_status": inventory_status,
        "payment_status": payment_status,
        "shipping_data": shipping_info
    })

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=PORT)
