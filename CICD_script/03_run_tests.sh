#!/usr/bin/env bash
set -euo pipefail

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."
TF_DIR="$ROOT_DIR/infra_tf"

# Detect terraform binary (supports Linux native and Windows WSL)
TF_CMD="terraform"
if command -v terraform >/dev/null 2>&1; then
    TF_CMD="terraform"
elif command -v terraform.exe >/dev/null 2>&1; then
    TF_CMD="terraform.exe"
fi

echo -e "\n\033[1;96m========================================================\033[0m"
echo -e "\033[1;92m>>> [1/2] [$(basename "$0")] Running Go Unit Tests & Static Analysis\033[0m"
echo -e "\033[1;96m========================================================\033[0m\n"

cd "$ROOT_DIR"
go vet ./...
go test -race -v ./...

echo -e "\n\033[1;96m========================================================\033[0m"
echo -e "\033[1;92m>>> [2/2] [$(basename "$0")] Executing Live E2E Tests on AWS API Gateway\033[0m"
echo -e "\033[1;96m========================================================\033[0m\n"

if [[ -z "${GATEWAY_URL:-}" ]]; then
    if [[ -d "$TF_DIR" ]]; then
        ENDPOINT=$(cd "$TF_DIR" && "$TF_CMD" output -raw api_endpoint 2>/dev/null | tr -d '\r' | sed 's:/*$::' || true)
        if [[ -n "$ENDPOINT" ]]; then
            export GATEWAY_URL="$ENDPOINT"
            echo "Using API Gateway URL from Terraform: $GATEWAY_URL"
        fi
    fi
fi

if [[ -z "${GATEWAY_URL:-}" ]]; then
    echo "GATEWAY_URL not set and terraform output unavailable; defaulting to http://localhost:8000"
    export GATEWAY_URL="http://localhost:8000"
fi

go test -tags=e2e -count=1 ./tests/e2e/ -v

echo -e "\n\033[1;92m[OK] All Unit and E2E Tests passed successfully.\033[0m\n"
