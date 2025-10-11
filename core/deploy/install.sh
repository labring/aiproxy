#!/bin/bash
timestamp() {
  date +"%Y-%m-%d %T"
}
print() {
  flag=$(timestamp)
  echo -e "\033[1;32m\033[1m INFO [$flag] >> $* \033[0m"
}
warn() {
  flag=$(timestamp)
  echo -e "\033[33m WARN [$flag] >> $* \033[0m"
}
info() {
  flag=$(timestamp)
  echo -e "\033[36m INFO [$flag] >> $* \033[0m"
}
wait_for_secret() {
  local secret_name=$1
  local namespace=${2:-hubble-service}

  info "Checking if secret $secret_name exists..."

  while ! kubectl get secret "$secret_name" -n "$namespace" > /dev/null 2>&1; do
    warn "Secret $secret_name does not exist, retrying in 5 seconds..."
    sleep 5
  done

  info "Secret $secret_name exists, proceeding with the next steps."
}

#===========================================================================
HELM_OPTS=${HELM_OPTS:-""}

NODE_COUNT=$(kubectl get nodes --no-headers | wc -l)
REPLICA_OPTIONS=""
if [ "$NODE_COUNT" -eq 1 ]; then
  REPLICA_OPTIONS="--set pgsql.replicas=1 --set pgsqlLog.replicas=1 --set redis.replicas=1 --set redis.sentinelReplicas=0 --set replicas=1 "
  HELM_OPTS="${HELM_OPTS} ${REPLICA_OPTIONS}"
fi

helm upgrade -i aiproxy-database -n aiproxy-system charts/aiproxy-database  ${HELM_OPTS} --wait

wait_for_secret "aiproxy-conn-credential" "aiproxy-system"
wait_for_secret "aiproxy-log-conn-credential" "aiproxy-system"
wait_for_secret "aiproxy-redis-conn-credential" "aiproxy-system"

AIPROXY_USER=$(kubectl get secret -n aiproxy-system aiproxy-conn-credential -ojsonpath="{.data.username}" | base64 -d)
AIPROXY_PASSWORD=$(kubectl get secret -n aiproxy-system aiproxy-conn-credential -ojsonpath="{.data.password}" | base64 -d)
AIPROXY_PORT=$(kubectl get secret -n aiproxy-system aiproxy-conn-credential -ojsonpath="{.data.port}" | base64 -d)
AIPROXY_HOST=$(kubectl get secret -n aiproxy-system aiproxy-conn-credential -ojsonpath="{.data.host}" | base64 -d).aiproxy-system.svc
AIPROXY_URI="postgres://${AIPROXY_USER}:${AIPROXY_PASSWORD}@${AIPROXY_HOST}:${AIPROXY_PORT}/postgres?sslmode=disable"

LOG_USER=$(kubectl get secret -n aiproxy-system aiproxy-log-conn-credential -ojsonpath="{.data.username}" | base64 -d)
LOG_PASSWORD=$(kubectl get secret -n aiproxy-system aiproxy-log-conn-credential -ojsonpath="{.data.password}" | base64 -d)
LOG_PORT=$(kubectl get secret -n aiproxy-system aiproxy-log-conn-credential -ojsonpath="{.data.port}" | base64 -d)
LOG_HOST=$(kubectl get secret -n aiproxy-system aiproxy-log-conn-credential -ojsonpath="{.data.host}" | base64 -d).aiproxy-system.svc
LOG_URI="postgres://${LOG_USER}:${LOG_PASSWORD}@${LOG_HOST}:${LOG_PORT}/postgres?sslmode=disable"

REDIS_USER=$(kubectl get secret -n aiproxy-system aiproxy-redis-conn-credential -ojsonpath="{.data.username}" | base64 -d)
REDIS_PASSWORD=$(kubectl get secret -n aiproxy-system aiproxy-redis-conn-credential -ojsonpath="{.data.password}" | base64 -d)
REDIS_PORT=$(kubectl get secret -n aiproxy-system aiproxy-redis-conn-credential -ojsonpath="{.data.port}" | base64 -d)
REDIS_HOST=$(kubectl get secret -n aiproxy-system aiproxy-redis-conn-credential -ojsonpath="{.data.host}" | base64 -d).aiproxy-system.svc
REDIS_URI="redis://${REDIS_USER}:${REDIS_PASSWORD}@${REDIS_HOST}:${REDIS_PORT}"

varJwtInternal=$(kubectl get configmap sealos-config -n sealos-system -o jsonpath='{.data.jwtInternal}')
kubectl delete configmap aiproxy-env -n aiproxy-system --ignore-not-found
kubectl delete ingress -n aiproxy-system aiproxy --ignore-not-found
kubectl delete deployment -n aiproxy-system aiproxy --ignore-not-found
kubectl delete service -n aiproxy-system aiproxy --ignore-not-found

adminKey=$(kubectl get configmap aiproxy-env -n aiproxy-system -o jsonpath='{.data.ADMIN_KEY}' )
if [ -z "$MINIO_CONSOLE_USER" ] || [ -z "$MINIO_CONSOLE_PASSWORD" ]; then
  print "adminKey is empty, generating new credentials."
  adminKey=$(openssl rand -hex 64 | head -c 32)
fi
SEALOS_CLOUD_DOMAIN=$(kubectl get configmap sealos-config -n sealos-system -o jsonpath='{.data.cloudDomain}')
SEALOS_CLOUD_PORT=$(kubectl get configmap sealos-config -n sealos-system -o jsonpath='{.data.cloudPort}')
helm upgrade -i aiproxy -n aiproxy-system charts/aiproxy  ${HELM_OPTS} --set aiproxy.SQL_DSN=${AIPROXY_URI} --set aiproxy.LOG_SQL_DSN=${LOG_URI}  --set aiproxy.REDIS=${REDIS_URI} \
  --set aiproxy.SEALOS_JWT_KEY=${varJwtInternal}  --set aiproxy.ADMIN_KEY=${adminKey} --set cloudDomain=${SEALOS_CLOUD_DOMAIN} --set cloudPort=${SEALOS_CLOUD_PORT}
