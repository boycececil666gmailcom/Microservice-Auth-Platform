#!/usr/bin/env bash
set -e

# Resolve paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/.."
TF_DIR="$ROOT_DIR/infra_tf"

get_pod_name() {
    local selector=$1
    kubectl get pods -n url-shortener -l "$selector" -o jsonpath="{.items[0].metadata.name}" 2>/dev/null
}

echo "========================================================"
echo "2. Deploying Infrastructure via Terraform"
echo "========================================================"

echo "Installing Kubernetes Operators (Postgres and Redis)..."
kubectl apply --server-side -k "github.com/zalando/postgres-operator/manifests?ref=v1.15.1"
kubectl apply --server-side -f https://raw.githubusercontent.com/spotahome/redis-operator/v1.2.4/manifests/databases.spotahome.com_redisfailovers.yaml
kubectl apply -f https://raw.githubusercontent.com/spotahome/redis-operator/v1.2.4/example/operator/all-redis-operator-resources.yaml

echo "Waiting for Custom Resource Definitions to be established..."
kubectl wait --for=condition=established crd/postgresqls.acid.zalan.do --timeout=60s
kubectl wait --for=condition=established crd/redisfailovers.databases.spotahome.com --timeout=60s

echo "Initializing Terraform in infra_tf/..."
cd "$TF_DIR"
terraform init

echo "Applying Terraform configuration..."
terraform apply -auto-approve

echo "Waiting for Database and Cache Master pods to be created..."
while [ -z "$(get_pod_name "application=spilo,cluster-name=shortener-db,spilo-role=master")" ]; do sleep 2; done
while [ -z "$(get_pod_name "application=spilo,cluster-name=auth-db,spilo-role=master")" ]; do sleep 2; done
while [ -z "$(get_pod_name "redisfailovers-role=master,redisfailovers.databases.spotahome.com/name=shortener-redis")" ]; do sleep 2; done
while [ -z "$(get_pod_name "redisfailovers-role=master,redisfailovers.databases.spotahome.com/name=auth-redis")" ]; do sleep 2; done

echo "Waiting for Database and Cache Master nodes to be ready..."
kubectl wait -n url-shortener --for=condition=Ready pod -l "application=spilo,cluster-name=shortener-db,spilo-role=master" --timeout=300s
kubectl wait -n url-shortener --for=condition=Ready pod -l "application=spilo,cluster-name=auth-db,spilo-role=master" --timeout=300s
kubectl wait -n url-shortener --for=condition=Ready pod -l "redisfailovers-role=master,redisfailovers.databases.spotahome.com/name=shortener-redis" --timeout=300s
kubectl wait -n url-shortener --for=condition=Ready pod -l "redisfailovers-role=master,redisfailovers.databases.spotahome.com/name=auth-redis" --timeout=300s

echo "Waiting for Auth and Shortener applications to be ready..."
kubectl wait -n url-shortener --for=condition=available deployment/auth --timeout=120s
kubectl wait -n url-shortener --for=condition=available deployment/shortener --timeout=120s

echo "[OK] Terraform infrastructure deployment complete."
echo
