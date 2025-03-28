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



