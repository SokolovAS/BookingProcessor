#!/usr/bin/env bash
set -euo pipefail

IMAGE="dn010590sas/bookingprocessor:latest"
RELEASE_NAME="bookingprocessor"
NAMESPACE="default"
CHART_PATH="./helm/bookingprocessor"

if [ "${1:-}" != "-d" ]; then
  echo "➡️ Building Docker image: $IMAGE"
  docker build -t "$IMAGE" .

  echo "➡️ Pushing Docker image to registry"
  docker push "$IMAGE"
else
  echo "➡️ Deploy-only mode (-d flag detected): Skipping build and push steps"
fi

echo "➡️ Deploying Helm chart: $RELEASE_NAME"
helm upgrade --install \
  --namespace "$NAMESPACE" \
  --create-namespace \
  "$RELEASE_NAME" \
  "$CHART_PATH" \
  --set image.repository="dn010590sas/bookingprocessor" \
  --set image.tag="latest"

# Force a complete rollout so that all pods are refreshed with the new image
echo "➡️ Forcing rollout restart of deployment: $RELEASE_NAME"
kubectl rollout restart deployment/"$RELEASE_NAME" --namespace "$NAMESPACE"

echo "➡️ Waiting for rollout to finish…"
kubectl rollout status deployment/"$RELEASE_NAME" --namespace "$NAMESPACE"

echo "✅ Deployment complete!"
