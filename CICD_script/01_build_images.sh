#!/usr/bin/env bash
set -euo pipefail

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."
BIN_DIR="$ROOT_DIR/bin"

mkdir -p "$BIN_DIR"

archive_bootstrap() {
    local zip_target=$1
    if command -v zip >/dev/null 2>&1; then
        zip -q -j "$zip_target" bootstrap
    else
        python3 -m zipfile -c "$zip_target" bootstrap
    fi
}

echo -e "\n\033[1;96m========================================================\033[0m"
echo -e "\033[1;92m>>> [1/2] [$(basename "$0")] Building Shortener Lambda Package (Linux ARM64)\033[0m"
echo -e "\033[1;96m========================================================\033[0m\n"

cd "$ROOT_DIR"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bootstrap ./cmd/shortener
archive_bootstrap "$BIN_DIR/shortener.zip"
rm -f bootstrap
echo "[OK] Shortener package generated: bin/shortener.zip"

echo -e "\n\033[1;96m========================================================\033[0m"
echo -e "\033[1;92m>>> [2/2] [$(basename "$0")] Building Analytics Lambda Package (Linux ARM64)\033[0m"
echo -e "\033[1;96m========================================================\033[0m\n"

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bootstrap ./cmd/analytics
archive_bootstrap "$BIN_DIR/analytics.zip"
rm -f bootstrap
echo "[OK] Analytics package generated: bin/analytics.zip"

echo -e "\n\033[1;92m[OK] All Lambda packages built successfully.\033[0m\n"
