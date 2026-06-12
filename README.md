# Docker + Kubernetes with Go — Learning Notes

---

## Project Structure

```
books-api/
├── main.go
├── go.mod
├── Dockerfile
└── k8s/
    ├── deployment.yaml
    ├── service.yaml
    ├── configmap.yaml
    ├── ingress.yaml
    └── hpa.yaml
```

---

## Part 1 — Docker

### What is Docker?

Docker packages your app with everything it needs to run into a single portable unit called an **image**. You can run that image on any machine and it behaves identically.

| Term | What it means |
|------|--------------|
| Image | A read-only snapshot of your app + its environment |
| Container | A running instance of an image |
| Dockerfile | A recipe that describes how to build an image |
| Layer | Each instruction in a Dockerfile creates a cached layer |

---

### The Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM alpine:3.19

RUN adduser -D -u 1001 appuser
USER appuser

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
```

**Line by line:**

- `FROM golang:1.22-alpine AS builder` — start from an official Go image, name this stage `builder`
- `WORKDIR /app` — all following commands run inside this directory
- `COPY go.mod ./` then `RUN go mod download` — copy the module file *before* source code so Docker caches the dependency download layer separately. If your code changes but `go.mod` doesn't, this layer is reused on the next build.
- `COPY . .` — copy your source code
- `CGO_ENABLED=0 GOOS=linux go build -o server .` — compile a fully static binary for Linux
- `FROM alpine:3.19` — **start a brand new image**. This is a multi-stage build. The Go toolchain from stage 1 is thrown away.
- `adduser` / `USER appuser` — don't run as root inside the container (security best practice)
- `COPY --from=builder /app/server .` — copy only the compiled binary from the builder stage
- `EXPOSE 8080` — documents the port (informational only, doesn't open anything)
- `CMD ["./server"]` — the command that runs when the container starts. Array form ensures signals like `SIGTERM` are received directly by your process.

**Result:** a ~12 MB image instead of ~300 MB, with no Go toolchain or source code inside.

---

### Build the Image

```bash
docker build -t books-api:latest .
```

- `-t books-api:latest` — name and tag the image
- `.` — the build context (Docker sends this directory to the build engine)

On the second build after a code change, notice most steps say `CACHED` — only layers after the change re-run.

---

### Run the Container

```bash
docker run -p 8080:8080 --rm --name my-api books-api:latest
```

- `-p 8080:8080` — map port 8080 on your machine to port 8080 inside the container (format: `host:container`)
- `--rm` — delete the container automatically when it stops
- `--name my-api` — give it a readable name

**Useful commands:**

```bash
docker ps                      # see running containers
docker logs my-api             # see logs
docker exec -it my-api sh      # open a shell inside the container
docker images                  # list all images
```

---

## Part 2 — Kubernetes

### What is Kubernetes?

Docker runs one container on one machine. Kubernetes orchestrates containers across many machines.

| Term | What it means |
|------|--------------|
| Pod | Smallest unit in K8s — one or more containers running together |
| Deployment | Manages a set of identical pods, handles restarts and rollouts |
| Service | A stable network endpoint that load-balances traffic across pods |
| Node | A machine (VM or physical) in the cluster |
| Cluster | The whole thing: a control plane + nodes |

---

### Start a Local Cluster

```bash
minikube start
kubectl get nodes        # verify it's ready
```

minikube runs a full Kubernetes cluster in a VM on your machine. `kubectl` is the CLI that talks to it.

---

### Load Your Image into minikube

minikube has its own Docker environment, separate from your machine's. You need to load your image into it.

```bash
minikube image load books-api:latest
minikube image ls | grep books-api    # verify
```

In production you'd push to a registry (Docker Hub, AWS ECR) and K8s would pull from there.

---

### The Deployment Manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: books-api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: books-api
  template:
    metadata:
      labels:
        app: books-api
    spec:
      containers:
        - name: books-api
          image: books-api:latest
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
```

**Field by field:**

- `kind: Deployment` — the type of K8s resource
- `replicas: 2` — run 2 copies of the pod at all times
- `selector.matchLabels` — how the Deployment identifies which pods it owns; must match `template.metadata.labels`
- `template` — the blueprint every pod is built from
- `imagePullPolicy: Never` — use the locally loaded image, don't try to pull from a registry
- `containerPort: 8080` — documents the port (informational only)

---

### The Service Manifest

```yaml
apiVersion: v1
kind: Service
metadata:
  name: books-api-service
spec:
  selector:
    app: books-api
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
  type: NodePort
```

**Field by field:**

- `selector: app: books-api` — routes traffic to any pod with this label
- `port: 80` — the port the Service listens on inside the cluster
- `targetPort: 8080` — the port on the pod to forward traffic to
- `type: NodePort` — makes the Service reachable from outside the cluster

**Why do you need a Service?** Pods are ephemeral — they get a new IP every time they restart. A Service gives you a stable address that always points to healthy pods and load-balances across them.

---

### Deploy

```bash
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

```bash
kubectl get deployments      # check deployment status
kubectl get pods             # check individual pods
kubectl get services         # check the service
```

Access your app:
```bash
minikube service books-api-service --url
# → gives you a URL like http://127.0.0.1:54321
```

---

### Scaling

```bash
# Scale up
kubectl scale deployment books-api --replicas=5

# Watch pods come up in real time
kubectl get pods -w

# Scale down
kubectl scale deployment books-api --replicas=1
```

**Simulate a crash:**
```bash
kubectl delete pod <pod-name>
kubectl get pods -w           # watch K8s replace it automatically
```

This is the **reconciliation loop** — K8s continuously compares desired state vs actual state and corrects any difference. It's why Kubernetes is self-healing.

---

### Useful Debugging Commands

```bash
kubectl describe deployment books-api       # detailed info + events
kubectl describe pod <pod-name>             # why is a pod failing?
kubectl logs <pod-name>                     # stdout from the container
kubectl logs -f <pod-name>                  # follow logs in real time
kubectl exec -it <pod-name> -- sh           # shell inside a pod
```

---

### Clean Up

```bash
kubectl delete -f k8s/      # delete all K8s resources
minikube stop               # pause the cluster
minikube delete             # delete the cluster entirely
```

---

## Part 3 — ConfigMaps & Secrets

Instead of hardcoding config in your app, read it from environment variables and inject them via K8s.

**Read env var in Go:**
```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
```

**`k8s/configmap.yaml`:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: books-api-config
data:
  PORT: "8080"
```

**Reference it in `deployment.yaml`:**
```yaml
env:
  - name: PORT
    valueFrom:
      configMapKeyRef:
        name: books-api-config
        key: PORT
```

**Verify it's set inside the pod:**
```bash
kubectl exec -it <pod-name> -- env | grep PORT
```

**ConfigMap vs Secret:** ConfigMap is for non-sensitive config (ports, URLs). Secret is for sensitive values (passwords, API keys) — same usage, just `secretKeyRef` instead of `configMapKeyRef`.

---

## Part 4 — Liveness & Readiness Probes

Probes tell K8s how to check if your pod is healthy.

- **Readiness probe** — is this pod ready to receive traffic? K8s won't route requests until this passes.
- **Liveness probe** — is this pod still alive? If this fails, K8s kills and restarts the pod.

**Add to `deployment.yaml` under your container:**
```yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 3
  periodSeconds: 5
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
```

- `initialDelaySeconds` — wait this long after container starts before first probe (gives app time to boot)
- `periodSeconds` — how often to run the probe
- Readiness has a shorter delay so traffic routing starts quickly; liveness has a longer delay so K8s doesn't kill a slow-starting pod

---

## Part 5 — Rolling Updates

Deploy a new version with zero downtime. K8s brings up new pods first, waits for readiness probe to pass, then kills old ones.

**Add strategy to `deployment.yaml`:**
```yaml
spec:
  replicas: 2
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
```

- `maxUnavailable: 0` — never take a pod down until a new one is ready
- `maxSurge: 1` — allow 1 extra pod above replica count during rollout

**Deploy a new version:**
```bash
docker build -t books-api:v2 .
minikube image load books-api:v2
# Update image in deployment.yaml to books-api:v2
kubectl apply -f k8s/deployment.yaml
kubectl rollout status deployment books-api
```

**Roll back instantly if something goes wrong:**
```bash
kubectl rollout undo deployment books-api
```

---

## Part 6 — Ingress

Instead of a random NodePort like `:55504`, Ingress gives you clean URLs like `books.local/books`.

**Enable ingress in minikube:**
```bash
minikube addons enable ingress
```

**`k8s/ingress.yaml`:**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: books-api-ingress
spec:
  rules:
    - host: books.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: books-api-service
                port:
                  number: 80
```

**Add to `/etc/hosts`:**
```bash
echo "$(minikube ip) books.local" | sudo tee -a /etc/hosts
```

**On Mac with Docker driver, run the tunnel to make it reachable:**
```bash
minikube tunnel   # keep this running in a separate terminal
```

**Test:**
```bash
curl http://books.local/health
curl http://books.local/books
```

- `host: books.local` — match requests with this hostname
- `pathType: Prefix` — match all paths starting with `/`
- You can have multiple rules routing different hostnames/paths to different services

---

## Part 7 — Horizontal Pod Autoscaler (HPA)

Automatically scale pods up/down based on CPU usage — no manual `kubectl scale` needed.

**Enable metrics server:**
```bash
minikube addons enable metrics-server
kubectl top pods    # verify it's working
```

**`k8s/hpa.yaml`:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: books-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: books-api
  minReplicas: 2
  maxReplicas: 5
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
```

**Important:** HPA requires CPU `requests` defined in your Deployment, otherwise it shows `<unknown>`:
```yaml
resources:
  requests:
    cpu: "100m"
    memory: "64Mi"
  limits:
    cpu: "250m"
    memory: "128Mi"
```

**Load test to trigger scaling:**
```bash
# Install hey
brew install hey

# Hammer the API
hey -z 60s -c 50 http://books.local/books

# Watch HPA react in another terminal
kubectl get hpa -w
```

- `minReplicas / maxReplicas` — never go below 2 or above 5 pods
- `averageUtilization: 50` — scale up when average CPU across all pods exceeds 50%
- K8s waits ~5 minutes before scaling back down to avoid flapping

---

### Clean Up

```bash
kubectl delete -f k8s/      # delete all K8s resources
minikube stop               # pause the cluster
minikube delete             # delete the cluster entirely
```
# test
