#!/usr/bin/env bash
set -e

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."

get_pod_name() {
    local selector=$1
    kubectl get pods -n url-shortener -l "$selector" -o jsonpath="{.items[0].metadata.name}" 2>/dev/null
}

echo "========================================================"
echo "3. Flushing Databases and Executing Test Suite"
echo "========================================================"

echo "Flushing Shortener PostgreSQL..."
SHORTENER_DB_POD=$(get_pod_name "application=spilo,cluster-name=shortener-db,spilo-role=master")
kubectl exec -n url-shortener "$SHORTENER_DB_POD" -- psql -U postgres -c "CREATE DATABASE urlshortener;" 2>/dev/null || true
kubectl exec -n url-shortener "$SHORTENER_DB_POD" -- psql -U postgres -d urlshortener -c "TRUNCATE TABLE urls RESTART IDENTITY CASCADE;"

echo "Flushing Auth PostgreSQL..."
AUTH_DB_POD=$(get_pod_name "application=spilo,cluster-name=auth-db,spilo-role=master")
kubectl exec -n url-shortener "$AUTH_DB_POD" -- psql -U postgres -c "CREATE DATABASE auth;" 2>/dev/null || true
kubectl exec -n url-shortener "$AUTH_DB_POD" -- psql -U postgres -d auth -c "TRUNCATE TABLE users RESTART IDENTITY CASCADE;"

echo "Flushing Shortener Redis..."
SHORTENER_REDIS_POD=$(get_pod_name "redisfailovers-role=master,redisfailovers.databases.spotahome.com/name=shortener-redis")
kubectl exec -n url-shortener "$SHORTENER_REDIS_POD" -- redis-cli FLUSHALL

echo "Flushing Auth Redis..."
AUTH_REDIS_POD=$(get_pod_name "redisfailovers-role=master,redisfailovers.databases.spotahome.com/name=auth-redis")
kubectl exec -n url-shortener "$AUTH_REDIS_POD" -- redis-cli FLUSHALL

echo "[OK] Databases and caches flushed."
echo

echo "========================================================"
echo "4. Running Full Pytest Suite (Unit, Integration & E2E)"
echo "========================================================"

python -m pytest "$ROOT_DIR/tests/" -v
