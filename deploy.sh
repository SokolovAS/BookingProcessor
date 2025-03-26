#!/usr/bin/env bash
set -eo pipefail

IMAGE="dn010590sas/bookingprocessor:latest"
DEPLOYMENT="bookingprocessor"

echo "➡️ Building Docker image: $IMAGE"
docker build -t "$IMAGE" .

echo "➡️ Pushing Docker image to registry"
docker push "$IMAGE"

echo "➡️ Applying updated YAML manifests"
kubectl apply -f bookingprocessor-deployment.yaml

echo "➡️ Restarting Kubernetes Deployment: $DEPLOYMENT"
kubectl rollout restart deployment "$DEPLOYMENT"

echo "➡️ Waiting for rollout to finish…"
kubectl rollout status deployment "$DEPLOYMENT"

echo "✅ Deployment complete!"
