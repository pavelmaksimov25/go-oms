#!/usr/bin/env bash
set -euo pipefail

ORDER_HOST="${ORDER_HOST:-localhost:50051}"

echo "=== Happy Path Test (amount=100, should CONFIRM) ==="

RESPONSE=$(grpcurl -plaintext -d '{
    "items": [{"item_id": "item-1", "quantity": 1}],
    "total_amount": 100
}' "${ORDER_HOST}" order.v1.OrderService/CreateOrder)

echo "CreateOrder response:"
echo "${RESPONSE}"

ORDER_ID=$(echo "${RESPONSE}" | grep -o '"orderId": "[^"]*"' | cut -d'"' -f4)
if [ -z "${ORDER_ID}" ]; then
    ORDER_ID=$(echo "${RESPONSE}" | grep -o '"order_id": "[^"]*"' | cut -d'"' -f4)
fi

echo ""
echo "Order ID: ${ORDER_ID}"
echo "Waiting 3 seconds for saga to complete..."
sleep 3

echo ""
echo "GetOrder response:"
grpcurl -plaintext -d "{\"order_id\": \"${ORDER_ID}\"}" "${ORDER_HOST}" order.v1.OrderService/GetOrder

echo ""
echo "=== Compensation Path Test (amount=5000, should REJECT) ==="

RESPONSE2=$(grpcurl -plaintext -d '{
    "items": [{"item_id": "item-2", "quantity": 1}],
    "total_amount": 5000
}' "${ORDER_HOST}" order.v1.OrderService/CreateOrder)

echo "CreateOrder response:"
echo "${RESPONSE2}"

ORDER_ID2=$(echo "${RESPONSE2}" | grep -o '"orderId": "[^"]*"' | cut -d'"' -f4)
if [ -z "${ORDER_ID2}" ]; then
    ORDER_ID2=$(echo "${RESPONSE2}" | grep -o '"order_id": "[^"]*"' | cut -d'"' -f4)
fi

echo ""
echo "Order ID: ${ORDER_ID2}"
echo "Waiting 3 seconds for saga compensation..."
sleep 3

echo ""
echo "GetOrder response (should be REJECTED):"
grpcurl -plaintext -d "{\"order_id\": \"${ORDER_ID2}\"}" "${ORDER_HOST}" order.v1.OrderService/GetOrder
