# Monitoring Tutorial

A sample project showing how to deploy a backend application on Kubernetes with a full observability stack, managed via ArgoCD.

## Tech Stack

| Component | Role |
|---|---|
| **Kubernetes** | Container orchestration |
| **ArgoCD** | GitOps – declarative deployment from this repository |
| **Prometheus** | Metrics collection and storage |
| **Grafana** | Visualization of metrics, logs, and traces |
| **Loki** | Log aggregation and storage |
| **OpenTelemetry** | Metrics, logs, and traces collection (OTLP) |

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                │
│                                                     │
│  ┌──────────┐    ┌──────────────────────────────┐  │
│  │  ArgoCD  │───▶│         Namespaces            │  │
│  └──────────┘    │                              │  │
│                  │  ┌──────────┐  ┌──────────┐  │  │
│                  │  │ backend  │  │monitoring│  │  │
│                  │  │          │  │          │  │  │
│                  │  │ REST API │  │Prometheus│  │  │
│                  │  │  (Go /   │  │  Loki    │  │  │
│                  │  │ Node.js) │  │  Grafana │  │  │
│                  │  └────┬─────┘  └────▲─────┘  │  │
│                  │       │  OTel       │         │  │
│                  │       └─────────────┘         │  │
│                  └──────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

## Repository Structure

```
monitoring-tutorial/
├── README.md
├── backend/                  # Backend source code
│   ├── Dockerfile
│   └── ...
├── k8s/                      # Kubernetes manifests
│   ├── backend/
│   ├── monitoring/
│   │   ├── prometheus/
│   │   ├── loki/
│   │   ├── grafana/
│   │   └── opentelemetry/
│   └── argocd/               # ApplicationSet and App definitions
├── kind/                     # Local cluster setup
│   ├── cluster.yaml          # Kind cluster configuration
│   └── cluster.sh            # Helper script (up / down / status)
└── dashboards/               # Grafana dashboards (JSON)
```

## Roadmap

- [ ] **Step 1** – Create the Backend (REST API with OpenTelemetry instrumentation)
- [ ] **Step 2** – Containerize with Docker
- [ ] **Step 3** – Kubernetes manifests for the Backend
- [ ] **Step 4** – Deploy ArgoCD on the cluster
- [ ] **Step 5** – Deploy Prometheus
- [ ] **Step 6** – Deploy Loki
- [ ] **Step 7** – Deploy OpenTelemetry Collector
- [ ] **Step 8** – Deploy Grafana with pre-configured datasources and dashboards
- [ ] **Step 9** – ArgoCD Applications for GitOps management of the full stack

## Prerequisites

- `kubectl` configured with access to a Kubernetes cluster (e.g. `kind`, `minikube`, or a cloud cluster)
- `helm` v3+
- `argocd` CLI
- `docker` for building images

## Local Cluster (Kind)

```bash
# Create the cluster
./kind/cluster.sh up

# Check status
./kind/cluster.sh status

# Destroy the cluster
./kind/cluster.sh down
```

Exposed ports on `localhost`:

| Port | Service |
|---|---|
| `8080` | ArgoCD UI |
| `3000` | Grafana |
| `9090` | Prometheus |
| `8800` | Backend API |

## ArgoCD Installation

After the cluster is up, install ArgoCD into its own namespace:

```bash
kubectl create namespace argocd
kubectl apply -n argocd \
  --server-side \
  --force-conflicts \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

> **Note:** `--server-side` is required because the ArgoCD CRDs exceed the 262144-byte annotation limit of standard client-side `kubectl apply`.

Wait for all pods to be ready:

```bash
kubectl wait --for=condition=Available deployment --all -n argocd --timeout=120s
```

Retrieve the initial admin password:

```bash
kubectl get secret argocd-initial-admin-secret -n argocd \
  -o jsonpath="{.data.password}" | base64 -d && echo
```

The ArgoCD UI will be available at [http://localhost:8080](http://localhost:8080) (username: `admin`).

## Bootstrap (App of Apps)

The project uses the **App of Apps** pattern. A single Application (`bootstrap`) points to `charts/gitops`, which in turn contains one ArgoCD `Application` per component.

```
argocd/bootstrap.yaml
        │
        ▼
charts/gitops/          ← Helm chart (this repo)
  templates/
    app-backend.yaml    → namespace: backend   (charts/apps)
    app-monitoring.yaml → namespace: monitoring (charts/monitoring)
```

Before applying, set the correct `repoURL` in both files:

```bash
# one-time manual apply – ArgoCD takes over from here
kubectl apply -f argocd/bootstrap.yaml
```

After this, every push to the repository is automatically reconciled by ArgoCD.

## Getting Started

The step-by-step guide is developed incrementally — each step is documented in the corresponding section of the repository as it is completed.

---

> This repository is for educational purposes. Manifests are designed for clarity, not for production use.
