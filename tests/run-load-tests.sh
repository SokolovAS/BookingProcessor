#!/usr/bin/env bash
set -euo pipefail

echo "Deleting old 'loadtest' pod if it exists..."
kubectl delete pod loadtest --ignore-not-found=true

echo "Launching new 'loadtest' pod..."
kubectl run loadtest \
  --image=dn010590sas/hey:latest \
  --restart=Never \
  -- -n 100000 \
     -c 20000 \
     -m POST \
     -H "Content-Type: application/json" \
     -d '{"example":"data"}' \
     http://bookingprocessor:80/insert


echo "Waiting up to 60 seconds for 'loadtest' pod to be ready..."
kubectl wait --for=condition=Ready pod/loadtest --timeout=60s

echo "Tailing logs for 'loadtest' pod..."
kubectl logs loadtest -f
