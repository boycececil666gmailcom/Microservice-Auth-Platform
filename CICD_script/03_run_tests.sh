#!/usr/bin/env bash
set -euo pipefail

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."
NAMESPACE="${NAMESPACE:-url-shortener}"

get_pod_name() {
    local selector=$1
    kubectl get pods -n "$NAMESPACE" -l "$selector" -o jsonpath="{.items[0].metadata.name}" 2>/dev/null
}

echo "========================================================"
echo "3. Flushing Databases and Executing Test Suite"
echo "========================================================"

echo "Flushing Shortener PostgreSQL..."
SHORTENER_DB_POD=$(get_pod_name "app=shortener-db")
kubectl exec -n "$NAMESPACE" "$SHORTENER_DB_POD" -- psql -U postgres -c "CREATE DATABASE urlshortener;" 2>/dev/null || true
kubectl exec -n "$NAMESPACE" "$SHORTENER_DB_POD" -- psql -U postgres -d urlshortener -c "TRUNCATE TABLE urls RESTART IDENTITY CASCADE;"

echo "Flushing Shortener Redis..."
SHORTENER_REDIS_POD=$(get_pod_name "app=shortener-redis")
kubectl exec -n "$NAMESPACE" "$SHORTENER_REDIS_POD" -- redis-cli FLUSHALL

echo "[OK] Databases and caches flushed."
echo

PORT_FORWARD_PID=""
cleanup() {
    if [[ -n "$PORT_FORWARD_PID" ]]; then
        kill "$PORT_FORWARD_PID" 2>/dev/null || true
        wait "$PORT_FORWARD_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

if [[ -z "${GATEWAY_URL:-}" ]]; then
    export GATEWAY_URL="http://localhost:8000"
    kubectl port-forward -n "$NAMESPACE" service/gateway 8000:8000 >/tmp/microservice-auth-port-forward.log 2>&1 &
    PORT_FORWARD_PID=$!
    for _ in {1..30}; do
        if curl --fail --silent "$GATEWAY_URL/health" >/dev/null; then
            break
        fi
        sleep 1
    done
    curl --fail --silent "$GATEWAY_URL/health" >/dev/null
fi

echo "========================================================"
echo "4. Running Full Go Test Suite (Unit & E2E)"
echo "========================================================"

cd "$ROOT_DIR"
go test ./...
go test -tags=e2e ./tests/e2e/ -v
