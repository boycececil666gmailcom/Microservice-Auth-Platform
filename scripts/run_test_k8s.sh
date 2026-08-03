#!/usr/bin/env bash

# Master orchestrator script for local Kubernetes E2E test execution.
# Executes modular build, deploy, and combined flush & pytest scripts.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT_FORWARD_PID=""

cleanup() {
    if [ -n "$PORT_FORWARD_PID" ]; then
        echo "Stopping Port-Forward tunnel (PID: $PORT_FORWARD_PID)..."
        kill "$PORT_FORWARD_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "========================================================"
echo "Checking Port 8000 Availability"
echo "========================================================"
if (echo > /dev/tcp/127.0.0.1/8000) >/dev/null 2>&1; then
    echo "[ERROR] Port 8000 is already in use by another process!"
    echo "Please manually stop whatever is occupying port 8000 before running this script."
    exit 1
fi
echo "[OK] Port 8000 is free."
echo

# 1. Build local Docker images
bash "$SCRIPT_DIR/01_build_images.sh"

# 2. Deploy infrastructure to Kubernetes via Terraform
bash "$SCRIPT_DIR/02_deploy_tf.sh"

# 3. Tunnel Gateway service and run combined flush & pytest execution
echo "========================================================"
echo "Starting Port-Forward Tunnel"
echo "========================================================"
echo "Tunneling Kubernetes Gateway to localhost:8000 in background..."
kubectl port-forward -n url-shortener svc/gateway 8000:8000 >/dev/null 2>&1 &
PORT_FORWARD_PID=$!
sleep 3

# 4. Execute data flush & pytest suite
bash "$SCRIPT_DIR/03_run_tests.sh"
TEST_EXIT_CODE=$?

exit $TEST_EXIT_CODE
