import os
from flask import Flask, jsonify

app = Flask(__name__)
PORT = int(os.environ.get('PORT', 8086))

NOTIFICATIONS = {
    "100": {"order_id": "100", "type": "email", "status": "sent"},
    "101": {"order_id": "101", "type": "sms", "status": "pending"},
    "102": {"order_id": "102", "type": "email", "status": "sent"},
    "103": {"order_id": "103", "type": "push", "status": "failed"},
    "104": {"order_id": "104", "type": "email", "status": "sent"},
    "105": {"order_id": "105", "type": "sms", "status": "delivered"},
    "106": {"order_id": "106", "type": "push", "status": "pending"},
    "107": {"order_id": "107", "type": "email", "status": "bounced"},
    "108": {"order_id": "108", "type": "email", "status": "sent"},
    "109": {"order_id": "109", "type": "sms", "status": "pending"}
}

@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "notification-service"})

@app.route('/notifications/<order_id>', methods=['GET'])
def get_notification(order_id):
    notification = NOTIFICATIONS.get(order_id, {"order_id": order_id, "status": "no_notification_found"})
    return jsonify(notification)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=PORT)
