import requests

services = {
    "api-gateway": {
        "base": "http://localhost:8080",
        "endpoints": [
            "/health",
            "/api/users/1",
            "/api/orders/100"
        ]
    },
    "user-service": {
        "base": "http://localhost:8081",
        "endpoints": [
            "/health",
            "/users/1"
        ]
    },
    "order-service": {
        "base": "http://localhost:8082",
        "endpoints": [
            "/health",
            "/orders/100"
        ]
    },
    "payment-service": {
        "base": "http://localhost:8083",
        "endpoints": [
            "/health",
            "/payments/100"
        ]
    },
    "inventory-service": {
        "base": "http://localhost:8084",
        "endpoints": [
            "/health",
            "/inventory/A1"
        ]
    },
    "shipping-service": {
        "base": "http://localhost:8085",
        "endpoints": [
            "/health",
            "/shipping/100"
        ]
    },
    "notification-service": {
        "base": "http://localhost:8086",
        "endpoints": [
            "/health",
            "/notifications/100"
        ]
    },
    "recommendation-service": {
        "base": "http://localhost:8087",
        "endpoints": [
            "/health",
            "/recommendations/1"
        ]
    }
}

for svc, info in services.items():
    print(f"\nTesting {svc} ({info['base']})")
    for ep in info["endpoints"]:
        url = info["base"] + ep
        try:
            resp = requests.get(url, timeout=3)
            print(f"  {ep}: {resp.status_code} {resp.json() if resp.headers.get('content-type','').startswith('application/json') else resp.text}")
        except Exception as e:
            print(f"  {ep}: ERROR {e}")
