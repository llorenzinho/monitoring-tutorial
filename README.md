# Monitoring Tutorial

A sample project showing how to deploy a NestJS backend on Kubernetes with a full observability stack (metrics, logs, traces), managed via ArgoCD using the App of Apps pattern.

## Tech Stack

| Component | Role |
|---|---|
| **Kubernetes (kind)** | Container orchestration (local cluster) |
| **ArgoCD** | GitOps – declarative deployment from this repository |
| **ingress-nginx** | Ingress controller (NodePort → localhost) |
| **cert-manager** | TLS certificate management (required by OTel webhooks) |
| **kube-prometheus-stack** | Prometheus + Grafana + AlertManager |
| **Loki** | Log aggregation |
| **Promtail** | Log shipping (DaemonSet → Loki) |
| **Tempo** | Distributed tracing backend |
| **OpenTelemetry Operator** | Manages OTel collectors via CRDs |
| **OpenTelemetry Collector** | Receives OTLP and forwards to Tempo |
| **CloudNative PG (CNPG)** | PostgreSQL operator |
| **NestJS** | Backend REST API, instrumented with OpenTelemetry |

## Architecture

```
                        ┌──────────────────────────────────────────────────────┐
                        │                  Kubernetes Cluster                  │
                        │                                                      │
  git push ────────────▶│  ArgoCD                                             │
                        │    └─ bootstrap (App of Apps)                       │
                        │         └─ charts/gitops                            │
                        │               ├─ ingress-nginx    (wave 1)          │
                        │               ├─ cert-manager     (wave 1)          │
                        │               ├─ otel-operator    (wave 2)          │
                        │               ├─ cloudnative-pg   (wave 2)          │
                        │               ├─ kube-prom-stack  (wave 3)          │
                        │               ├─ loki / tempo / promtail (wave 3)   │
                        │               ├─ pg-cluster / otel-collector (wave 4)│
                        │               └─ backend          (wave 4)          │
                        │                                                      │
  curl trace-app:8800 ─▶│  ingress-nginx ──▶ NestJS backend ──OTLP──▶ OTel   │
                        │                         │                   Collector│
                        │                         │ SQL          (traces)  │   │
                        │                         ▼                       ▼   │
                        │                    PostgreSQL               Tempo    │
                        │                                                      │
                        │  ┌──────────┐  ┌──────┐  ┌───────┐  ┌───────┐     │
                        │  │Prometheus│  │ Loki │  │ Tempo │  │Grafana│     │
                        │  └──────────┘  └──────┘  └───────┘  └───────┘     │
                        └──────────────────────────────────────────────────────┘
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
│           ├── apps.yaml         # ArgoCD Application → charts/apps (backend)
│           ├── monitoring/       # Loki, Tempo, Promtail, OTel operator+collector, Prometheus, datasources
│           ├── postgres/         # CNPG operator + Cluster + Secrets
│           └── utils/            # cert-manager, ingress-nginx
├── kind/
│   ├── cluster.yaml              # Kind cluster configuration
│   └── cluster.sh                # Helper script (up / down / status)
└── scripts/
    └── traffic-gen.go            # HTTP traffic generator (Go, stdlib only)
```

## Prerequisites

- [`kubectl`](https://kubernetes.io/docs/tasks/tools/)
- [`helm`](https://helm.sh/docs/intro/install/) v3+
- [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [`argocd` CLI](https://argo-cd.readthedocs.io/en/stable/cli_installation/)
- [`docker`](https://docs.docker.com/engine/install/)
- [`go`](https://go.dev/doc/install) 1.21+ (only for the traffic generator)

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
| `8800` | Backend API (via ingress-nginx) |

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

## Building and Loading the Backend Image

When working locally with `kind`, there is no need to push the image to a public registry. Build and load it directly into the cluster nodes:

```bash
# Build the image
docker build -t nest-be-example:latest apps/nest-be-example/

# Load it into the kind cluster (skips any registry)
kind load docker-image nest-be-example:latest --name monitoring-tutorial
```

The `charts/apps/values.yaml` references this local image:

```yaml
image:
  repository: nest-be-example
  tag: latest
  pullPolicy: IfNotPresent   # never pull – use the locally loaded image
```

> **Tip:** every time you rebuild the image, re-run `kind load docker-image` and then restart the backend pods (or trigger an ArgoCD hard-refresh) to pick up the new image.

## Bootstrap (App of Apps)

The project uses the **App of Apps** pattern. A single Application (`bootstrap`) points to `charts/gitops`, which contains one ArgoCD `Application` per component.

```
argocd/bootstrap.yaml
        │
        ▼
charts/gitops/                    ← this repo, managed by ArgoCD
  templates/
    apps.yaml                     → ns: default                (NestJS backend)
    monitoring/                   → ns: monitoring / opentelemetry-operator-system
    postgres/                     → ns: cloudnative-pg-system / default
    utils/cert-manager.yaml       → ns: cert-manager
    utils/ingress-nginx.yaml      → ns: ingress-nginx
```

Apply once to bootstrap everything:

```bash
kubectl apply -f argocd/bootstrap.yaml
```

After this, every push to the repository is automatically reconciled by ArgoCD.

## Sync Waves (Installation Order)

Resources in `charts/gitops` use `argocd.argoproj.io/sync-wave` annotations so that ArgoCD installs components in dependency order. Each wave must reach `Healthy` before the next one starts.

| Wave | Components | Reason |
|---|---|---|
| **1** | `cert-manager`, `ingress-nginx` | No dependencies |
| **2** | `opentelemetry-operator`, `cloudnative-pg` | Require cert-manager webhooks |
| **3** | `kube-prometheus-stack`, `loki`, `tempo`, `promtail`, pg secrets | Require operators to be ready |
| **4** | `otel-collector` CR, `pg-cluster` CR, Grafana datasources ConfigMap, `backend` | Require wave-3 CRDs and services |

## Ingress

The backend is exposed via **ingress-nginx**, deployed as an ArgoCD Application in `charts/gitops/templates/utils/ingress-nginx.yaml`.

The controller Service uses NodePort `30800`, which the kind cluster maps to `localhost:8800` (see `kind/cluster.yaml`).

```
trace-app:8800  →  kind NodePort 30800  →  ingress-nginx  →  Service/nest-be-example:80  →  Pod:3000
```

To reach the API from a browser or `curl`, add the hostname to `/etc/hosts`:

```bash
echo "127.0.0.1 trace-app" | sudo tee -a /etc/hosts
```

```bash
curl http://trace-app:8800/
```

> **Note:** the traffic generator (`scripts/traffic-gen.go`) resolves `trace-app` to `127.0.0.1` internally via a custom dialer, so it works without the `/etc/hosts` entry.

## Generating Traffic (Grafana test data)

`scripts/traffic-gen.go` sends a continuous mix of requests (`GET /`, `GET /items`, `GET /items/:id`, `POST /items`, `DELETE /items/:id`) until stopped. No external dependencies — standard library only.

```bash
# default: trace-app:8800, one request every 300 ms
go run scripts/traffic-gen.go

# custom base URL and interval
go run scripts/traffic-gen.go -base-url http://trace-app:8800 -interval 500ms
```

`trace-app` is resolved to `127.0.0.1` internally — no `/etc/hosts` changes needed to run the script.

Stop with `Ctrl+C`. A summary (`total / success / errors`) is printed on exit, and running totals are logged every 50 requests.

---

## Troubleshooting

### OTel sidecar not injected on first boot

On the very first install, the `backend` Application (wave 4) may start before the OpenTelemetry Operator webhooks are fully ready. In that case the instrumentation sidecar is **not** injected into the pod, so traces will be missing in Tempo.

**How to detect it:** the backend pod has no `otc-container` sidecar listed in `kubectl describe pod`.

**Fix:** restart the backend Deployment after the operator is healthy:

```bash
kubectl rollout restart deployment/nest-be-example -n backend
```

ArgoCD will sync the rollout automatically. After the new pod starts you should see traces appearing in Grafana → Tempo.

---

> This repository is for educational purposes. Manifests are designed for clarity over production hardening.
