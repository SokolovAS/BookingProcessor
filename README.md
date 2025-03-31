# BookingProcessor - Local Testing & Monitoring Guide

## 🚀 To Test Locally

### 1. Port Forward Services
Forward the PostgreSQL and BookingProcessor services to your local machine:

```bash
kubectl port-forward svc/postgres 5432:5432
```
```bash
kubectl port-forward deployment/bookingprocessor 8080:8080
```
### 2. Run Load Test
Remove any existing loadtest pod, then run a high-concurrency test using the custom hey image:
```bash
kubectl delete pod loadtest
```
```bash
kubectl run loadtest --image=dn010590sas/hey:latest \
  --restart=Never -- -n 50000 -c 500 http://bookingprocessor:80/insert
  ```
```bash
kubectl logs loadtest -f
```
## 📊 Monitoring
### Watch BookingProcessor Pods
```bash
kubectl get pods -l app=bookingprocessor -w
```
### Watch Horizontal Pod Autoscaler
```
kubectl get hpa bookingprocessor-hpa -w
```
### Patch the metrics-server deployment to add the --kubelet-insecure-tls flag. For example, run:
```
kubectl patch deployment metrics-server -n kube-system --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'

kubectl get pods -n kube-system
```
### Enable Grafana dashboard
#### Install Prometeus/Grafana stack
```
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus prometheus-community/kube-prometheus-stack --namespace default --create-namespace
```
Port forward
```
kubectl port-forward svc/prometheus-grafana --namespace default 3000:80
```
or
```
helm upgrade prometeus prometheus-community/prometheus --version 20.1.0 --install --set prometheus-pushgateway.enabled=false --set prometheus-node-exporter.hostRootFsMount.enabled=false --set server.global.scrape_interval=15s --set server.global.evaluation_interval=15s
```
```
helm install grafana grafana/grafana --namespace default --set adminPassword='YourStrongPassword' \
  --set "datasources.datasources\.yaml.apiVersion=1" \
  --set "datasources.datasources\.yaml.datasources[0].name=Prometheus" \
  --set "datasources.datasources\.yaml.datasources[0].type=prometheus" \
  --set "datasources.datasources\.yaml.datasources[0].url=http://prometeus-prometheus-server.default.svc.cluster.local" \
  --set "datasources.datasources\.yaml.datasources[0].access=proxy" \
  --set "datasources.datasources\.yaml.datasources[0].isDefault=true"
```
```
100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```



