#!/usr/bin/env bash
set -eo pipefail

IMAGE="dn010590sas/bookingprocessor:latest"
RELEASE_NAME="bookingprocessor"
NAMESPACE="default"
CHART_PATH="./helm/bookingprocessor"

echo "➡️ Building Docker image: $IMAGE"
docker build -t "$IMAGE" .

echo "➡️ Pushing Docker image to registry"
docker push "$IMAGE"

echo "➡️ Deploying Helm chart: $RELEASE_NAME"
# --install: installs if not present, upgrades if it is
# You can override values (like image tag) via --set or --values
helm upgrade --install \
  --namespace "$NAMESPACE" \
  --create-namespace \
  "$RELEASE_NAME" \
  "$CHART_PATH" \
  --set image.repository="dn010590sas/bookingprocessor" \
  --set image.tag="latest"

echo "➡️ Waiting for rollout to finish…"
kubectl rollout status deployment/"$RELEASE_NAME" --namespace "$NAMESPACE"

echo "✅ Deployment complete!"
