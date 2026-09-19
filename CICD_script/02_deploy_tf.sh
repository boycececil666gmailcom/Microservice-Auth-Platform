#!/usr/bin/env bash
set -euo pipefail

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="$SCRIPT_DIR/../infra_tf"

echo "========================================================"
echo "2. Deploying AWS Lambda Infrastructure via Terraform"
echo "========================================================"

cd "$TF_DIR"
terraform init
terraform apply -auto-approve

echo
echo "Deployment Outputs:"
terraform output

echo
echo "[OK] AWS Lambda infrastructure deployment complete."
