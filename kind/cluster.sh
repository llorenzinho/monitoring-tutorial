#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="monitoring-tutorial"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  echo "Usage: $0 {up|down|status}"
  echo ""
  echo "  up      Create the Kind cluster"
  echo "  down    Destroy the Kind cluster"
  echo "  status  Show cluster status"
  exit 1
}

up() {
  echo "==> Creating Kind cluster '${CLUSTER_NAME}'..."
  kind create cluster --config "${SCRIPT_DIR}/cluster.yaml"
  echo ""
  echo "==> Cluster ready. Ports exposed on localhost:"
  echo "    http://localhost:8080  → ArgoCD UI"
  echo "    http://localhost:3000  → Grafana"
  echo "    http://localhost:9090  → Prometheus"
  echo "    http://localhost:8800  → Backend API"
}

down() {
  echo "==> Destroying Kind cluster '${CLUSTER_NAME}'..."
  kind delete cluster --name "${CLUSTER_NAME}"
  echo "==> Cluster deleted."
}

status() {
  if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "==> Cluster '${CLUSTER_NAME}': RUNNING"
    echo ""
    kubectl cluster-info --context "kind-${CLUSTER_NAME}"
    echo ""
    kubectl get nodes --context "kind-${CLUSTER_NAME}"
  else
    echo "==> Cluster '${CLUSTER_NAME}': NOT FOUND"
  fi
}

case "${1:-}" in
  up)     up ;;
  down)   down ;;
  status) status ;;
  *)      usage ;;
esac
