# Kubernetes Deployment

How to run the Books API on a local Kubernetes cluster (minikube).

---

## deployment.yaml — Step by Step

```yaml
apiVersion: apps/v1
kind: Deployment
```
`apiVersion` tells K8s which API group handles this resource. `apps/v1` is the stable version for workload resources like Deployments. `kind: Deployment` is the resource type — it manages a set of identical pods and keeps the desired number running at all times.

```yaml
metadata:
  name: books-api
```
The name K8s uses to identify this Deployment. You'll reference it in `kubectl` commands: `kubectl get deployment books-api`.

```yaml
spec:
  replicas: 2
```
Run 2 pod instances at all times. If one crashes, K8s starts a replacement automatically on a healthy node.

```yaml
  selector:
    matchLabels:
      app: books-api
```
How the Deployment finds the pods it owns. K8s uses this label selector to track which pods belong to this Deployment. Must exactly match the labels in `template.metadata.labels`.

```yaml
  template:
    metadata:
      labels:
        app: books-api
```
Every pod created by this Deployment gets this label attached. The selector above uses it to claim ownership.

```yaml
    spec:
      containers:
        - name: books-api
          image: books-api:latest
```
The container image to run inside each pod. `books-api:latest` is the tag you build locally — it must exist in the node's image cache before K8s can schedule the pod.

```yaml
          imagePullPolicy: Never
```
Skip the registry entirely and use only the locally available image. Without this, K8s would try to pull `books-api:latest` from Docker Hub and fail because it isn't published there. This is what makes `minikube image load` work.

```yaml
          ports:
            - containerPort: 8080
```
Documents the port the application listens on. This is informational only — it doesn't open or expose anything. The actual exposure is handled by a Service resource.

---

## service.yaml — Step by Step

```yaml
apiVersion: v1
kind: Service
```
`v1` is the core API group — Services predate the `apps/` split and live here. `kind: Service` is a stable network endpoint that load-balances traffic across a set of pods.

```yaml
metadata:
  name: books-api-service
```
The name used to reach this Service inside the cluster via DNS: `books-api-service.default.svc.cluster.local`.

```yaml
spec:
  selector:
    app: books-api
```
The Service finds pods by this label. Any pod with `app: books-api` receives traffic from this Service. This is how it stays connected even when pods restart and get new IPs — the label stays constant, the IP doesn't matter.

```yaml
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
```
`port: 80` is the port the Service is reachable on inside the cluster. `targetPort: 8080` is the port on each pod to forward traffic to — this must match `containerPort` in your Deployment.

```yaml
  type: NodePort
```
Makes the Service reachable from outside the cluster via a port on the minikube node. K8s automatically assigns a port in the range 30000–32767. Use `minikube service books-api-service --url` to get the exact address.

---

## Running Locally with Minikube

```bash
# 1. Start the cluster
minikube start

# 2. Build the image (from the project root)
docker build -t books-api:latest .

# 3. Load the image into minikube's node (bypasses a registry)
minikube image load books-api:latest

# 4. Apply the Deployment and Service
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# 5. Verify pods and service are running
kubectl get pods
kubectl get service books-api-service

# 6. Get the URL to hit the API
minikube service books-api-service --url
```

---

## Useful kubectl Commands

```bash
# Watch pod status in real time
kubectl get pods -w

# Stream logs from all books-api pods
kubectl logs -l app=books-api --follow

# Describe a pod (useful for debugging CrashLoopBackOff)
kubectl describe pod <pod-name>

# Delete and recreate the deployment
kubectl delete -f k8s/deployment.yaml
kubectl apply -f k8s/deployment.yaml

# Scale replicas on the fly
kubectl scale deployment books-api --replicas=3
```
