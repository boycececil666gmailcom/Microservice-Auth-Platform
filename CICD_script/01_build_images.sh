#!/usr/bin/env bash
set -euo pipefail

# Resolve the absolute path of the root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."

echo "========================================================"
echo "1. Building Local Docker Images"
echo "========================================================"

echo "Building Shortener Service..."
docker build -t url-shortener-shortener:latest -f "$ROOT_DIR/services/shortener/Dockerfile" "$ROOT_DIR"

echo "Building Auth Service..."
docker build -t url-shortener-auth:latest -f "$ROOT_DIR/services/auth/Dockerfile" "$ROOT_DIR"

echo "Building Analytics Service..."
docker build -t url-shortener-analytics:latest -f "$ROOT_DIR/services/analytics/Dockerfile" "$ROOT_DIR"

echo "[OK] All Go service images built successfully."
echo
