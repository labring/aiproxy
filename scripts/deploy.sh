#!/usr/bin/env bash
#
# Production deployment script — the ONLY way to deploy to production.
#
# Default mode: zero-downtime deployment via Docker + Nginx upstream switching.
# Legacy mode:  direct container restart (5-10s downtime, no Nginx switching).
#
# Zero-downtime deploy (default):
#   ADMIN_KEY=xxx bash scripts/deploy.sh                    # Full deploy
#   ADMIN_KEY=xxx bash scripts/deploy.sh --no-pull          # Skip git pull
#   bash scripts/deploy.sh --build-only                     # Build image only
#   ADMIN_KEY=xxx bash scripts/deploy.sh --rollback         # Emergency rollback
#
# Legacy deploy (direct container restart, requires --legacy flag):
#   ADMIN_KEY=xxx bash scripts/deploy.sh --legacy           # Full legacy deploy
#   ADMIN_KEY=xxx bash scripts/deploy.sh --legacy --no-pull
#   ADMIN_KEY=xxx bash scripts/deploy.sh --legacy --restart-only
#
# Prerequisites:
#   - Docker + Docker Compose installed
#   - ADMIN_KEY env var set (for smoke tests)
#   - (Zero-downtime only) Nginx with deploy/nginx/aiproxy-upstream.conf installed
#
# Environment:
#   ADMIN_KEY         — Required for smoke tests
#   API_KEY           — Optional: enables /v1/ endpoint smoke tests
#   COMPOSE_PROFILES  — Docker Compose profiles (if any)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${ROOT_DIR}"

# ── Check for legacy mode ─────────────────────────────────────
LEGACY=0
PASSTHROUGH_ARGS=()

for arg in "$@"; do
  if [[ "${arg}" == "--legacy" ]]; then
    LEGACY=1
  else
    PASSTHROUGH_ARGS+=("${arg}")
  fi
done

# ── Zero-downtime mode (default) ──────────────────────────────
if [[ "${LEGACY}" == "0" ]]; then
  exec bash "${SCRIPT_DIR}/zero-downtime-deploy.sh" "${PASSTHROUGH_ARGS[@]+"${PASSTHROUGH_ARGS[@]}"}"
fi

# ══════════════════════════════════════════════════════════════
# LEGACY MODE — direct container restart (original behavior)
# ══════════════════════════════════════════════════════════════

# ── Flags ──────────────────────────────────────────────────────
BUILD_ONLY=0
RESTART_ONLY=0
NO_PULL=0

set -- "${PASSTHROUGH_ARGS[@]+"${PASSTHROUGH_ARGS[@]}"}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build-only)   BUILD_ONLY=1;   shift ;;
    --restart-only) RESTART_ONLY=1; shift ;;
    --no-pull)      NO_PULL=1;      shift ;;
    -h|--help)
      head -30 "$0" | tail -28
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# ── Helpers ────────────────────────────────────────────────────
pass() { printf "\033[32m✓\033[0m %s\n" "$*"; }
fail() { printf "\033[31m✗\033[0m %s\n" "$*"; exit 1; }
info() { printf "\033[36m→\033[0m %s\n" "$*"; }
warn() { printf "\033[33m!\033[0m %s\n" "$*"; }

COMPOSE_CMD="docker compose -f docker-compose.yaml -f docker-compose.prod.yaml"

# ── Pre-flight checks ─────────────────────────────────────────
if ! command -v docker >/dev/null 2>&1; then
  fail "docker not found — install Docker first"
fi

if ! docker compose version >/dev/null 2>&1; then
  fail "docker compose not found — install Docker Compose V2"
fi

echo ""
info "=========================================="
info "AI Proxy Production Deployment (Legacy Mode)"
info "=========================================="
warn "Using legacy mode — 5-10s service interruption expected."
warn "For zero-downtime: run without --legacy flag."
echo ""

DEPLOY_START=$(date +%s)

# ── Step 1: Pull latest code ───────────────────────────────────
if [[ "${RESTART_ONLY}" == "0" && "${NO_PULL}" == "0" ]]; then
  info "Step 1/4: Pulling latest code..."

  CURRENT_BRANCH=$(git branch --show-current)
  info "Branch: ${CURRENT_BRANCH}"

  # Check for uncommitted changes
  if ! git diff --quiet || ! git diff --cached --quiet; then
    warn "Uncommitted changes detected — stashing..."
    git stash push -m "deploy-auto-stash-$(date +%Y%m%d-%H%M%S)"
    STASHED=1
  else
    STASHED=0
  fi

  # When running under sudo, SSH agent forwarding is lost.
  if git pull --rebase origin "${CURRENT_BRANCH}"; then
    pass "Code updated"
  else
    warn "git pull failed — retrying with explicit SSH key..."
    SSH_KEY=""
    for key in /home/*/.ssh/id_ed25519 /home/*/.ssh/id_rsa /root/.ssh/id_ed25519; do
      if [[ -f "${key}" ]]; then
        SSH_KEY="${key}"
        break
      fi
    done
    if [[ -z "${SSH_KEY}" ]]; then
      fail "git pull failed and no SSH key found for retry. Use --no-pull and pull manually."
    fi
    GIT_SSH_COMMAND="ssh -i ${SSH_KEY} -o StrictHostKeyChecking=no" git pull --rebase origin "${CURRENT_BRANCH}"
    pass "Code updated (via explicit SSH key: ${SSH_KEY})"
  fi

  if [[ "${STASHED}" == "1" ]]; then
    warn "Restoring stashed changes..."
    git stash pop || warn "Stash pop failed — resolve manually after deploy"
  fi
else
  if [[ "${RESTART_ONLY}" == "1" ]]; then
    info "Step 1/4: Skipping pull + build (--restart-only)"
  else
    info "Step 1/4: Skipping pull (--no-pull)"
  fi
fi

# ── Step 2: Build Docker image ─────────────────────────────────
if [[ "${RESTART_ONLY}" == "0" ]]; then
  info "Step 2/4: Building Docker image..."
  info "(This builds frontend + swagger + Go binary with enterprise tag)"

  ${COMPOSE_CMD} build --no-cache aiproxy
  pass "Docker image built: aiproxy:local"

  # Verify the image contains enterprise symbols
  info "Verifying image contents..."
  VERIFY_OUTPUT=$(docker run --rm --entrypoint sh aiproxy:local -c \
    'strings /usr/local/bin/aiproxy | grep -c "enterprise" || echo 0' 2>/dev/null)
  if [[ "${VERIFY_OUTPUT}" -gt 0 ]]; then
    pass "Enterprise module verified in image (${VERIFY_OUTPUT} symbols)"
  else
    fail "Enterprise module NOT found in image — build may be broken"
  fi

  FRONTEND_CHECK=$(docker run --rm --entrypoint sh aiproxy:local -c \
    'strings /usr/local/bin/aiproxy | grep -c "</html>" || echo 0' 2>/dev/null)
  if [[ "${FRONTEND_CHECK}" -gt 0 ]]; then
    pass "Frontend embedded in image"
  else
    fail "Frontend NOT embedded in image"
  fi
else
  info "Step 2/4: Skipping build (--restart-only)"
fi

if [[ "${BUILD_ONLY}" == "1" ]]; then
  echo ""
  pass "Build complete (--build-only). Image: aiproxy:local"
  exit 0
fi

# ── Step 3: Restart service ────────────────────────────────────
info "Step 3/4: Restarting service..."

# Ensure infra (postgres, redis) is running
${COMPOSE_CMD} up -d pgsql redis
info "Waiting for database and Redis..."
sleep 3

# Restart aiproxy with new image
${COMPOSE_CMD} up -d aiproxy
pass "Service restarted"

# Wait for health check
info "Waiting for health check..."
HEALTH_TIMEOUT=60
HEALTH_ELAPSED=0
while [[ ${HEALTH_ELAPSED} -lt ${HEALTH_TIMEOUT} ]]; do
  STATUS=$(docker inspect --format='{{.State.Health.Status}}' aiproxy 2>/dev/null || echo "unknown")
  if [[ "${STATUS}" == "healthy" ]]; then
    pass "Health check passed"
    break
  fi
  sleep 2
  HEALTH_ELAPSED=$((HEALTH_ELAPSED + 2))
done

if [[ ${HEALTH_ELAPSED} -ge ${HEALTH_TIMEOUT} ]]; then
  fail "Health check timeout (${HEALTH_TIMEOUT}s) — check logs: docker logs aiproxy"
fi

# ── Step 4: Smoke test ─────────────────────────────────────────
info "Step 4/4: Running smoke tests..."

ADMIN_KEY="${ADMIN_KEY:-}"
if [[ -n "${ADMIN_KEY}" ]]; then
  if ADMIN_KEY="${ADMIN_KEY}" API_KEY="${API_KEY:-}" \
    bash scripts/smoke-test.sh "http://localhost:3000"; then
    pass "Smoke tests passed"
  else
    warn "Smoke tests had failures — check output above"
    warn "Service is running but may have issues. Check: docker logs aiproxy"
  fi
else
  warn "ADMIN_KEY not set — skipping smoke tests"
  warn "Strongly recommended: ADMIN_KEY=xxx bash scripts/deploy.sh"
fi

# ── Post-deploy cleanup check ─────────────────────────────────
if [[ -f "${SCRIPT_DIR}/post-deploy-cleanup.sh" ]]; then
  info "Running post-deploy disk cleanup check..."
  bash "${SCRIPT_DIR}/post-deploy-cleanup.sh" --auto || true
fi

# ── Summary ────────────────────────────────────────────────────
DEPLOY_END=$(date +%s)
DEPLOY_DURATION=$((DEPLOY_END - DEPLOY_START))

echo ""
info "=========================================="
pass "Legacy deployment complete (${DEPLOY_DURATION}s)"
info "=========================================="
info "Verify: curl http://localhost:3000/api/status"
info "Logs:   docker logs -f aiproxy"
info "Rollback: docker compose -f docker-compose.yaml -f docker-compose.prod.yaml down aiproxy"
info "          然后切换到之前的 git commit 重新运行 deploy.sh --legacy"
echo ""
