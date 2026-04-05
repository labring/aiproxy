#!/usr/bin/env bash
# Smoke test for AI Proxy production deployment.
# Usage: bash scripts/smoke-test.sh [local|<base_url>]
#
# Required env vars:
#   ADMIN_KEY    — Admin API authentication key
# Optional env vars:
#   API_KEY      — A valid user API key for /v1/ endpoint tests
#   PUBLIC_URL   — Public API URL (for isolation checks; defaults to same as admin URL)
set -euo pipefail

# ----- Configuration -----
TARGET="${1:-local}"

case "${TARGET}" in
  local)
    BASE_URL="http://localhost:3000"
    ;;
  *)
    BASE_URL="${TARGET}"
    ;;
esac

ADMIN_KEY="${ADMIN_KEY:-}"
API_KEY="${API_KEY:-}"
PUBLIC_URL="${PUBLIC_URL:-${BASE_URL}}"

if [[ -z "${ADMIN_KEY}" ]]; then
  echo "ERROR: ADMIN_KEY env var is required."
  exit 2
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "ERROR: curl is required."
  exit 2
fi

# ----- Helpers -----
PASS=0
FAIL=0
SKIP=0

pass()  { PASS=$((PASS + 1)); printf "\033[32mPASS\033[0m: %s\n" "$*"; }
fail()  { FAIL=$((FAIL + 1)); printf "\033[31mFAIL\033[0m: %s\n" "$*"; }
skip()  { SKIP=$((SKIP + 1)); printf "\033[33mSKIP\033[0m: %s\n" "$*"; }
info()  { printf "\033[36mINFO\033[0m: %s\n" "$*"; }

check_http() {
  local desc="$1"
  local url="$2"
  shift 2
  local expected_code="${1:-200}"
  shift || true

  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$@" "${url}" 2>/dev/null || echo "000")

  if [[ "${code}" == "${expected_code}" ]]; then
    pass "${desc} (HTTP ${code})"
  else
    fail "${desc} — expected HTTP ${expected_code}, got ${code}"
  fi
}

check_json_field() {
  local desc="$1"
  local url="$2"
  local field="$3"
  shift 3

  local body
  body=$(curl -s --max-time 10 "$@" "${url}" 2>/dev/null || echo "{}")

  if echo "${body}" | python3 -c "import sys,json; d=json.load(sys.stdin); assert ${field}" 2>/dev/null; then
    pass "${desc}"
  else
    fail "${desc} — field check failed: ${field}"
    info "Response: ${body:0:200}"
  fi
}

# ----- Tests -----
info "Smoke testing ${BASE_URL}"
echo ""

# 1. Health check
info "=== Health Check ==="
check_http "GET /api/status" "${BASE_URL}/api/status"

# 2. Model list (admin)
info "=== Admin API ==="
check_http "GET /api/models (admin)" \
  "${BASE_URL}/api/models/" \
  200 \
  -H "Authorization: Bearer ${ADMIN_KEY}"

# 3. Channel list
check_http "GET /api/channels (admin)" \
  "${BASE_URL}/api/channels/" \
  200 \
  -H "Authorization: Bearer ${ADMIN_KEY}"

# 4. Group list
check_http "GET /api/groups (admin)" \
  "${BASE_URL}/api/groups/" \
  200 \
  -H "Authorization: Bearer ${ADMIN_KEY}"

# 5. Model list via OpenAI-compatible endpoint (if API_KEY provided)
info "=== Public API ==="
if [[ -n "${API_KEY}" ]]; then
  check_http "GET /v1/models" \
    "${PUBLIC_URL}/v1/models" \
    200 \
    -H "Authorization: Bearer ${API_KEY}"

  # Chat completions — basic request
  check_http "POST /v1/chat/completions (simple)" \
    "${PUBLIC_URL}/v1/chat/completions" \
    200 \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Say hi in 3 words"}],"max_tokens":20}'
else
  skip "v1/models — no API_KEY provided"
  skip "v1/chat/completions — no API_KEY provided"
fi

# 6. Feishu OAuth login link reachability
info "=== Feishu OAuth ==="
check_http "GET /api/auth/feishu/login (redirect)" \
  "${BASE_URL}/api/auth/feishu/login" \
  302

# 7. Enterprise analytics API (admin)
info "=== Enterprise Analytics ==="
check_http "GET /api/enterprise/feishu/sync-status" \
  "${BASE_URL}/api/enterprise/feishu/sync-status" \
  200 \
  -H "Authorization: Bearer ${ADMIN_KEY}"

check_http "GET /api/enterprise/feishu/sync-history" \
  "${BASE_URL}/api/enterprise/feishu/sync-history" \
  200 \
  -H "Authorization: Bearer ${ADMIN_KEY}"

check_http "GET /api/enterprise/quota/notif-config" \
  "${BASE_URL}/api/enterprise/quota/notif-config" \
  200 \
  -H "Authorization: Bearer ${ADMIN_KEY}"

check_http "GET /api/enterprise/quota/alert-history" \
  "${BASE_URL}/api/enterprise/quota/alert-history" \
  200 \
  -H "Authorization: Bearer ${ADMIN_KEY}"

# 8. Public URL isolation — admin endpoints should be blocked
info "=== Public Path Isolation ==="
if [[ "${PUBLIC_URL}" != "${BASE_URL}" ]]; then
  check_http "Admin API blocked on public URL" \
    "${PUBLIC_URL}/api/channels/" \
    403 \
    -H "Authorization: Bearer ${ADMIN_KEY}"

  check_http "Enterprise API blocked on public URL" \
    "${PUBLIC_URL}/api/enterprise/feishu/sync-status" \
    403 \
    -H "Authorization: Bearer ${ADMIN_KEY}"
else
  skip "Public isolation — PUBLIC_URL same as BASE_URL"
fi

# 9. Frontend assets
info "=== Frontend ==="
check_http "Frontend index.html" \
  "${BASE_URL}/" \
  200

# ----- Summary -----
echo ""
info "================================================"
info "Results: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped"
info "================================================"

if [[ ${FAIL} -gt 0 ]]; then
  exit 1
fi

exit 0
