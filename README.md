# Monitoring Tutorial

A sample project showing how to deploy a NestJS backend on Kubernetes with a full observability stack (metrics, logs, traces), managed via ArgoCD using the App of Apps pattern.

## Tech Stack

| Component | Role |
|---|---|
| **Kubernetes (kind)** | Container orchestration (local cluster) |
| **ArgoCD** | GitOps – declarative deployment from this repository |
| **kube-prometheus-stack** | Prometheus + Grafana + AlertManager |
| **Loki** | Log aggregation |
| **Promtail** | Log shipping (DaemonSet → Loki) |
| **Tempo** | Distributed tracing backend |
| **OpenTelemetry Operator** | Manages OTel collectors via CRDs |
| **OpenTelemetry Collector** | Receives OTLP and forwards to Tempo |
| **cert-manager** | TLS certificate management (required by OTel webhooks) |
| **CloudNative PG (CNPG)** | PostgreSQL operator |
| **NestJS** | Backend REST API, instrumented with OpenTelemetry |

## Architecture

```
                        ┌─────────────────────────────────────────────────┐
                        │              Kubernetes Cluster                  │
                        │                                                  │
  git push ────────────▶│  ArgoCD                                         │
                        │    └─ bootstrap (App of Apps)                   │
                        │         └─ charts/gitops                        │
                        │               ├─ cert-manager                   │
                        │               ├─ cloudnative-pg + pg-cluster    │
                        │               ├─ kube-prometheus-stack          │
                        │               ├─ loki                           │
                        │               ├─ promtail                       │
                        │               ├─ tempo                          │
                        │               ├─ opentelemetry-operator         │
                        │               └─ backend (charts/apps)          │
                        │                                                  │
                        │  ┌─────────────┐   OTLP    ┌──────────────────┐ │
                        │  │   NestJS    │──────────▶│  OTel Collector  │ │
                        │  │   backend   │           │  (traces → Tempo)│ │
                        │  └─────────────┘           └──────────────────┘ │
                        │                                                  │
                        │  ┌──────────┐  ┌──────┐  ┌───────┐  ┌───────┐  │
                        │  │Prometheus│  │ Loki │  │ Tempo │  │Grafana│  │
                        │  └──────────┘  └──────┘  └───────┘  └───────┘  │
                        └─────────────────────────────────────────────────┘
```

## Repository Structure

```
monitoring-tutorial/
├── README.md
├── apps/
│   └── nest-be-example/          # NestJS backend source code + Dockerfile
├── argocd/
│   └── bootstrap.yaml            # Root Application (apply once manually)
├── charts/
│   ├── apps/                     # Helm chart for the NestJS backend
│   └── gitops/                   # App of Apps chart
│       └── templates/
│           ├── apps.yaml         # ArgoCD Application → charts/apps
│           ├── monitoring/       # Loki, Tempo, Promtail, OTel, Prometheus, datasources
│           ├── postgres/         # CNPG operator + Cluster + Secrets
│           └── utils/            # cert-manager
└── kind/
    ├── cluster.yaml              # Kind cluster configuration
    └── cluster.sh                # Helper script (up / down / status)
```

## Prerequisites

- [`kubectl`](https://kubernetes.io/docs/tasks/tools/)
- [`helm`](https://helm.sh/docs/intro/install/) v3+
- [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [`argocd` CLI](https://argo-cd.readthedocs.io/en/stable/cli_installation/)
- [`docker`](https://docs.docker.com/engine/install/)

## OS-level Configuration

Before starting the cluster, apply these kernel parameters to avoid errors in `kind`, `promtail`, and other file-watcher-heavy workloads:

```bash
sudo sysctl -w fs.inotify.max_user_watches=524288
sudo sysctl -w fs.inotify.max_user_instances=512
```

To persist across reboots:

```bash
echo "fs.inotify.max_user_watches=524288" | sudo tee -a /etc/sysctl.d/99-kind.conf
echo "fs.inotify.max_user_instances=512"  | sudo tee -a /etc/sysctl.d/99-kind.conf
sudo sysctl -p /etc/sysctl.d/99-kind.conf
```

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

The ArgoCD UI is available at [http://localhost:8080](http://localhost:8080) (username: `admin`).

## Bootstrap (App of Apps)

The project uses the **App of Apps** pattern. A single Application (`bootstrap`) points to `charts/gitops`, which contains one ArgoCD `Application` per component.

```
argocd/bootstrap.yaml
        │
        ▼
charts/gitops/               ← this repo, managed by ArgoCD
  templates/
    apps.yaml                → ns: default    (charts/apps – NestJS backend)
    monitoring/              → ns: monitoring  (Prometheus, Loki, Tempo, OTel…)
    postgres/                → ns: default    (CNPG operator + cluster)
    utils/cert-manager.yaml  → ns: cert-manager
```

Apply once to bootstrap everything:

```bash
kubectl apply -f argocd/bootstrap.yaml
```

After this, every push to the repository is automatically reconciled by ArgoCD.

## Ingress

The backend is exposed via **ingress-nginx**, deployed as an ArgoCD Application in `charts/gitops/templates/utils/ingress-nginx.yaml`.

The controller Service uses NodePort `30800`, which the kind cluster maps to `localhost:8800` (see `kind/cluster.yaml`).

```
localhost:8800  →  kind NodePort 30800  →  ingress-nginx  →  Service/nest-be-example:80  →  Pod:3000
```

Once the bootstrap is applied and ArgoCD has synced, the API is reachable at:

```bash
curl http://localhost:8800/
```

No `/etc/hosts` changes are needed.

## Building and Loading the Backend Image

When working locally with `kind`, there is no need to push the image to a public registry. Build and load it directly into the cluster nodes:

```bash
# Build the image
docker build -t nest-be-example:latest apps/nest-be-example/

# Load it into the kind cluster (skips any registry)
kind load docker-image nest-be-example:latest --name monitoring-tutorial
```

The `charts/apps/values.yaml` must reference this local image:

```yaml
image:
  repository: nest-be-example
  tag: latest
  pullPolicy: IfNotPresent   # never pull – use the locally loaded image
```

> **Tip:** every time you rebuild the image, re-run `kind load docker-image` and then restart the backend pods (or trigger an ArgoCD hard-refresh) to pick up the new image.

---

> This repository is for educational purposes. Manifests are designed for clarity over production hardening.
