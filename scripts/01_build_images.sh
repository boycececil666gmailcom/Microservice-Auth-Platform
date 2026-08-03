#!/usr/bin/env bash
set -e

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

echo "Building Gateway Service..."
docker build -t url-shortener-gateway:latest -f "$ROOT_DIR/services/gateway/Dockerfile" "$ROOT_DIR"

echo "[OK] All Docker images built successfully."
echo
