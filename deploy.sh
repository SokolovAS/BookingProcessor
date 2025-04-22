#!/usr/bin/env bash
set -euo pipefail

#############################################
# Parse Command-Line Flags
#############################################
DEPLOY_ONLY=false
RESTART_POSTGRES=false

# Use getopts to parse flags. Ex: "-d -p"
while getopts ":dp" opt; do
  case $opt in
    d)
      DEPLOY_ONLY=true
      ;;
    p)
      RESTART_POSTGRES=true
      ;;
    \?)
      echo "Invalid option: -$OPTARG"
      exit 1
      ;;
  esac
done
# Shift away the flags so $1 will refer to any subsequent arguments
shift $((OPTIND - 1))

#############################################
# 1. Define Variables
#############################################
IMAGE="dn010590sas/bookingprocessor:latest"
RELEASE_NAME="bookingprocessor"
NAMESPACE="default"
CHART_PATH="./helm/bookingprocessor"

#############################################
# 2. Build/Push Docker Image (Optional)
#############################################
if [ "$DEPLOY_ONLY" = false ]; then
  echo "➡️ Building Docker image: $IMAGE"
  docker build -t "$IMAGE" .

  echo "➡️ Pushing Docker image to registry"
  docker push "$IMAGE"
else
  echo "➡️ Deploy-only mode (-d flag detected): Skipping build and push steps"
fi

#############################################
# 3. Deploy Helm Chart (BookingProcessor + Postgres)
#############################################
echo "➡️ Deploying/Upgrading Helm release: $RELEASE_NAME"
helm upgrade --install \
  --namespace "$NAMESPACE" \
  --create-namespace \
  "$RELEASE_NAME" \
  "$CHART_PATH" \
  --set image.repository="dn010590sas/bookingprocessor" \
  --set image.tag="latest"

#############################################
# 4. Rollout Restart BookingProcessor
#############################################
echo "➡️ Forcing rollout restart of deployment: $RELEASE_NAME"
kubectl rollout restart deployment/"$RELEASE_NAME" --namespace "$NAMESPACE"

echo "➡️ Waiting for BookingProcessor rollout to finish…"
kubectl rollout status deployment/"$RELEASE_NAME" --namespace "$NAMESPACE"

#############################################
# 5. Conditionally Rollout Restart PostgreSQL
#############################################
if [ "$RESTART_POSTGRES" = true ]; then
  echo "➡️ Forcing rollout restart of PostgreSQL deployment: postgres"
  helm upgrade --install citus ./helm/citus

else
  echo "➡️ Skipping PostgreSQL restart (use -p flag to enable)"
fi

#############################################
# 6. Done!
#############################################
echo "✅ Deployment complete!"
