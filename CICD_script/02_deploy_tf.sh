#!/usr/bin/env bash
set -euo pipefail

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."
TF_DIR="$ROOT_DIR/infra_tf"
BIN_DIR="$ROOT_DIR/bin"

# Detect terraform binary (supports Linux native and Windows WSL)
TF_CMD="terraform"
if command -v terraform >/dev/null 2>&1; then
    TF_CMD="terraform"
elif command -v terraform.exe >/dev/null 2>&1; then
    TF_CMD="terraform.exe"
fi

echo -e "\n\033[1;96m========================================================\033[0m"
echo -e "\033[1;92m>>> [1/3] [$(basename "$0")] Checking Lambda Deployment Packages\033[0m"
echo -e "\033[1;96m========================================================\033[0m\n"

if [[ ! -f "$BIN_DIR/shortener.zip" || ! -f "$BIN_DIR/analytics.zip" ]]; then
    echo "Lambda packages not found in bin/. Triggering build..."
    bash "$SCRIPT_DIR/01_build_images.sh"
else
    echo "[OK] Found bin/shortener.zip and bin/analytics.zip"
fi

echo -e "\n\033[1;96m========================================================\033[0m"
echo -e "\033[1;92m>>> [2/3] [$(basename "$0")] Initializing Terraform Provider\033[0m"
echo -e "\033[1;96m========================================================\033[0m\n"

cd "$TF_DIR"
"$TF_CMD" init -input=false

echo -e "\n\033[1;96m========================================================\033[0m"
echo -e "\033[1;92m>>> [3/3] [$(basename "$0")] Applying Infrastructure Changes\033[0m"
echo -e "\033[1;96m========================================================\033[0m\n"

"$TF_CMD" apply -auto-approve -input=false

echo -e "\n\033[1;92m>>> Deployment Outputs:\033[0m"
"$TF_CMD" output

echo -e "\n\033[1;92m[OK] AWS Lambda infrastructure deployment complete.\033[0m\n"
