To test locally
kubectl port-forward svc/postgres 5432:5432

kubectl port-forward deployment/bookingprocessor 8080:8080

kubectl delete pod loadtest
kubectl run loadtest --image=dn010590sas/hey:latest \
--restart=Never -- -n 50000 -c 500 http://bookingprocessor:80/insert
kubectl logs loadtest -f


Monitoring
kubectl get pods -l app=bookingprocessor -w
kubectl get hpa bookingprocessor-hpa -w