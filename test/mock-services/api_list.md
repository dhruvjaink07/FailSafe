# Mock Microservices API Endpoints

Below is a complete list of the API endpoints available across the mock microservices. All services run locally on their respective ports.

## API Gateway (`localhost:8080`)
The main entry point for the frontend or external clients.
- `GET /health` : Health check endpoint.
- `GET /api/users/<user_id>` : Fetches user information along with recommendations.
- `GET /api/orders/<order_id>` : Fetches order details along with inventory, payment, and shipping statuses.

## User Service (`localhost:8081`)
Manages user profiles.
- `GET /health` : Health check endpoint.
- `GET /users/<user_id>` : Returns user details (ID, name, email, tier).

## Order Service (`localhost:8082`)
Manages order details and interacts with inventory, payment, and shipping.
- `GET /health` : Health check endpoint.
- `GET /orders/<order_id>` : Returns the specific order and aggregates data from other services.

## Payment Service (`localhost:8083`)
Manages payment statuses for orders.
- `GET /health` : Health check endpoint.
- `GET /payments/<order_id>` : Returns payment status and method.

## Inventory Service (`localhost:8084`)
Manages stock levels for items.
- `GET /health` : Health check endpoint.
- `GET /inventory/<item_id>` : Returns the current stock level and status of an item.

## Shipping Service (`localhost:8085`)
Handles tracking and shipping statuses for orders.
- `GET /health` : Health check endpoint.
- `GET /shipping/<order_id>` : Returns shipping provider, tracking number, and triggers a notification.

## Notification Service (`localhost:8086`)
Sends system communications (emails, SMS, push notifications).
- `GET /health` : Health check endpoint.
- `GET /notifications/<order_id>` : Returns the status of the notification sent for an order.

## Recommendation Service (`localhost:8087`)
Recommends products based on a user's ID.
- `GET /health` : Health check endpoint.
- `GET /recommendations/<user_id>` : Returns recommended item IDs and checks inventory for the top item.
