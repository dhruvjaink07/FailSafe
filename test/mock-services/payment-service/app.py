import os
from flask import Flask, jsonify

app = Flask(__name__)
PORT = int(os.environ.get('PORT', 8083))

PAYMENTS = {
    "100": {"order_id": "100", "status": "paid", "method": "credit_card"},
    "101": {"order_id": "101", "status": "pending", "method": "paypal"},
    "102": {"order_id": "102", "status": "paid", "method": "debit_card"},
    "103": {"order_id": "103", "status": "failed", "method": "credit_card"},
    "104": {"order_id": "104", "status": "paid", "method": "bank_transfer"},
    "105": {"order_id": "105", "status": "paid", "method": "credit_card"},
    "106": {"order_id": "106", "status": "pending", "method": "paypal"},
    "107": {"order_id": "107", "status": "refunded", "method": "credit_card"},
    "108": {"order_id": "108", "status": "paid", "method": "debit_card"},
    "109": {"order_id": "109", "status": "processing", "method": "crypto"}
}

@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "payment-service"})

@app.route('/payments/<order_id>', methods=['GET'])
def get_payment(order_id):
    payment = PAYMENTS.get(order_id, {"order_id": order_id, "status": "not_found", "method": "unknown"})
    return jsonify(payment)

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=PORT)
