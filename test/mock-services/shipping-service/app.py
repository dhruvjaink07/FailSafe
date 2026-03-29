import os
import requests
from flask import Flask, jsonify

app = Flask(__name__)
PORT = int(os.environ.get('PORT', 8085))
NOTIFICATION_SERVICE_URL = os.environ.get('NOTIFICATION_SERVICE_URL', 'http://notification-service:8086')

SHIPPING_INFO = {
    "100": {"order_id": "100", "provider": "FedEx", "status": "shipped", "tracking": "FDX12345"},
    "101": {"order_id": "101", "provider": "UPS", "status": "processing", "tracking": "UPS98765"},
    "102": {"order_id": "102", "provider": "USPS", "status": "delivered", "tracking": "USP11223"},
    "103": {"order_id": "103", "provider": "DHL", "status": "processing", "tracking": "DHL8899"},
    "104": {"order_id": "104", "provider": "FedEx", "status": "shipped", "tracking": "FDX54321"},
    "105": {"order_id": "105", "provider": "UPS", "status": "shipped", "tracking": "UPS1122"},
    "106": {"order_id": "106", "provider": "USPS", "status": "out_for_delivery", "tracking": "USP5544"},
    "107": {"order_id": "107", "provider": "Unknown", "status": "cancelled", "tracking": None},
    "108": {"order_id": "108", "provider": "FedEx", "status": "delivered", "tracking": "FDX9988"},
    "109": {"order_id": "109", "provider": "DHL", "status": "in_transit", "tracking": "DHL7766"}
}

@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "shipping-service"})

@app.route('/shipping/<order_id>', methods=['GET'])
def get_shipping(order_id):
    info = SHIPPING_INFO.get(order_id, {"order_id": order_id, "provider": "unknown", "status": "not_shipped"})
    
    # Call notification service
    notification_status = "unknown"
    try:
        notif_resp = requests.get(f"{NOTIFICATION_SERVICE_URL}/notifications/{order_id}", timeout=5)
        if notif_resp.status_code == 200:
            notification_status = notif_resp.json().get('status', 'sent')
    except Exception as e:
        notification_status = f"error: {str(e)}"

    return jsonify({"shipping": info, "notification": notification_status})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=PORT)
