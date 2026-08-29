#!/usr/bin/env bash
set -e

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="$SCRIPT_DIR/../infra_tf"

echo "========================================================"
echo "2. Deploying Infrastructure via Terraform"
echo "========================================================"

cd "$TF_DIR"
terraform init
terraform apply -auto-approve

echo "Waiting for deployments to be ready..."
kubectl rollout status deployment/auth-db deployment/shortener-db deployment/auth-redis deployment/shortener-redis deployment/auth deployment/shortener deployment/gateway -n url-shortener --timeout=120s

echo "[OK] Terraform infrastructure deployment complete."
