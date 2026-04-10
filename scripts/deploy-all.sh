#!/usr/bin/env bash
#
# Multi-node deployment script — deploys to all AI Proxy nodes sequentially.
#
# Usage:
#   ADMIN_KEY=xxx bash scripts/deploy-all.sh                  # Full deploy all nodes
#   ADMIN_KEY=xxx bash scripts/deploy-all.sh --no-pull        # Skip git pull
#   bash scripts/deploy-all.sh --build-only                   # Build only
#   ADMIN_KEY=xxx bash scripts/deploy-all.sh --rollback       # Emergency rollback
#
# Environment:
#   ADMIN_KEY       — Required for smoke tests
#   DEPLOY_ARGS     — Extra args passed to deploy.sh (e.g., "--no-pull")
#   SSH_KEY         — SSH private key path (default: /home/ppuser/.ssh/id_ed25519)
#   REPO_PATH       — Repo path on remote (default: /data/aiproxy)

set -euo pipefail

# ── Helpers ──────────────────────────────────────────────────
pass() { printf "\033[32m✓\033[0m %s\n" "$*"; }
fail() { printf "\033[31m✗\033[0m %s\n" "$*"; exit 1; }
info() { printf "\033[36m→\033[0m %s\n" "$*"; }
warn() { printf "\033[33m!\033[0m %s\n" "$*"; }

send_feishu() {
  local msg="$1"
  if [[ -n "${FEISHU_WEBHOOK}" ]]; then
    curl -sf -X POST "${FEISHU_WEBHOOK}" \
      -H 'Content-Type: application/json' \
      -d "{\"msg_type\":\"text\",\"content\":{\"text\":\"${msg}\"}}" \
      >/dev/null 2>&1 || true
  fi
}

# ── Configuration ────────────────────────────────────────────
ADMIN_KEY="${ADMIN_KEY:-}"
DEPLOY_ARGS="${DEPLOY_ARGS:-$*}"
SSH_KEY="${SSH_KEY:-/home/ppuser/.ssh/id_ed25519}"
REPO_PATH="${REPO_PATH:-/data/aiproxy}"
FEISHU_WEBHOOK="${FEISHU_WEBHOOK:-}"

# Node list: type|user@host|external_url (external_url optional, for ALB smoke test)
NODES=(
  "domestic|ppuser@1.13.81.31|"
  "overseas|ppuser@52.35.158.131|https://apiproxy.pplabs.tech"
)

# ── Pre-flight ───────────────────────────────────────────────
if [[ -z "${ADMIN_KEY}" && "${DEPLOY_ARGS}" != *"--build-only"* ]]; then
  warn "ADMIN_KEY not set — smoke tests will be skipped on all nodes"
fi

echo ""
info "=========================================="
info "AI Proxy Multi-Node Deployment"
info "=========================================="
info "Nodes: ${#NODES[@]}"
info "Args:  ${DEPLOY_ARGS:-<none>}"
echo ""

DEPLOY_START=$(date +%s)
FAILED=()
SUCCEEDED=()

# ── Deploy each node ─────────────────────────────────────────
for entry in "${NODES[@]}"; do
  # Parse: type|user@host|external_url
  IFS='|' read -r NODE_TYPE HOST EXTERNAL_URL <<< "${entry}"

  echo ""
  info "------------------------------------------"
  info "Deploying ${NODE_TYPE} node (${HOST})..."
  info "------------------------------------------"

  if ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no "${HOST}" \
    "cd ${REPO_PATH} && \
     sudo GIT_SSH_COMMAND='ssh -i ${SSH_KEY} -o StrictHostKeyChecking=no' \
     ADMIN_KEY='${ADMIN_KEY}' \
     NODE_TYPE='${NODE_TYPE}' \
     bash scripts/deploy.sh ${DEPLOY_ARGS}"; then
    pass "${NODE_TYPE} node deployed successfully"
    SUCCEEDED+=("${NODE_TYPE}:${HOST}")

    # External ALB smoke test (verifies traffic through load balancer)
    if [[ -n "${EXTERNAL_URL}" ]]; then
      info "Running external smoke test: ${EXTERNAL_URL}/v1/models ..."
      sleep 5  # Allow ALB health check to pick up the new target
      HTTP_CODE=$(curl -sf -o /dev/null -w "%{http_code}" --max-time 15 \
        -H "Authorization: Bearer test" "${EXTERNAL_URL}/v1/models" 2>/dev/null || echo "000")
      if [[ "${HTTP_CODE}" == "401" || "${HTTP_CODE}" == "200" ]]; then
        pass "External ALB smoke test passed (HTTP ${HTTP_CODE})"
      else
        warn "External ALB smoke test returned HTTP ${HTTP_CODE} — check ALB/Nginx"
      fi
    fi
  else
    warn "${NODE_TYPE} node deployment FAILED"
    FAILED+=("${NODE_TYPE}:${HOST}")
  fi
done

# ── Summary ──────────────────────────────────────────────────
DEPLOY_END=$(date +%s)
DEPLOY_DURATION=$((DEPLOY_END - DEPLOY_START))

echo ""
info "=========================================="

if [[ ${#SUCCEEDED[@]} -gt 0 ]]; then
  pass "Succeeded: ${SUCCEEDED[*]}"
fi

if [[ ${#FAILED[@]} -gt 0 ]]; then
  send_feishu "[AI Proxy 部署失败] 失败节点: ${FAILED[*]} | 成功节点: ${SUCCEEDED[*]:-无} | 耗时: ${DEPLOY_DURATION}s"
  fail "Failed: ${FAILED[*]}"
  echo ""
  warn "Manual rollback commands:"
  for entry in "${FAILED[@]}"; do
    NODE_TYPE="${entry%%:*}"
    HOST="${entry#*:}"
    warn "  ssh ${HOST} 'cd ${REPO_PATH} && ADMIN_KEY=xxx NODE_TYPE=${NODE_TYPE} sudo bash scripts/deploy.sh --rollback'"
  done
  echo ""
  info "Total time: ${DEPLOY_DURATION}s"
  exit 1
fi

GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
send_feishu "[AI Proxy 部署成功] 全部 ${#SUCCEEDED[@]} 个节点部署完成 | 版本: ${GIT_SHA} | 耗时: ${DEPLOY_DURATION}s"
pass "All nodes deployed successfully (${DEPLOY_DURATION}s)"
info "=========================================="
echo ""
