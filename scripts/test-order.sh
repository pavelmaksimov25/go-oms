#!/usr/bin/env bash
set -euo pipefail

ORDER_HOST="${ORDER_HOST:-localhost:50051}"
MAX_WAIT="${MAX_WAIT:-30}"

wait_for_status() {
    local order_id="$1"
    local want="$2"
    local deadline=$((SECONDS + MAX_WAIT))
    while [ $SECONDS -lt $deadline ]; do
        local response
        response=$(grpcurl -plaintext -d "{\"order_id\": \"${order_id}\"}" "${ORDER_HOST}" order.v1.OrderService/GetOrder 2>/dev/null)
        if echo "${response}" | grep -q "${want}"; then
            echo "${response}"
            return 0
        fi
        sleep 1
    done
    echo "Timeout after ${MAX_WAIT}s waiting for ${want}"
    echo "${response}"
    return 1
}

run_case() {
    local label="$1"
    local amount="$2"
    local item_id="$3"
    local want_status="$4"

    echo "=== ${label} (amount=${amount}, should ${want_status#ORDER_STATUS_}) ==="
    local response
    response=$(grpcurl -plaintext -d "{
        \"items\": [{\"item_id\": \"${item_id}\", \"quantity\": 1}],
        \"total_amount\": ${amount}
    }" "${ORDER_HOST}" order.v1.OrderService/CreateOrder)
    echo "CreateOrder response:"
    echo "${response}"

    local order_id
    order_id=$(echo "${response}" | grep -oE '"orderId": "[^"]*"' | cut -d'"' -f4)
    [ -z "${order_id}" ] && order_id=$(echo "${response}" | grep -oE '"order_id": "[^"]*"' | cut -d'"' -f4)

    echo ""
    echo "Order ID: ${order_id}"
    echo "Polling for ${want_status} (max ${MAX_WAIT}s)..."
    wait_for_status "${order_id}" "${want_status}"
    echo ""
}

run_case "Happy Path Test" 100  item-1 "ORDER_STATUS_CONFIRMED"
run_case "Compensation Path Test" 5000 item-2 "ORDER_STATUS_REJECTED"
