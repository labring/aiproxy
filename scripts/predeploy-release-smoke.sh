#!/usr/bin/env bash
#
# Pre-deploy smoke suite for the 2026-04-04 ~ 2026-04-05 release train.
# Focus:
# - PPIO multimodal passthrough
# - autodiscover / sync changes
# - Responses compatibility
# - enterprise request history / scope isolation regressions
#
# Usage:
#   bash scripts/predeploy-release-smoke.sh
#
# Optional env:
#   RUN_GO_TESTS=1              Default: 1
#   RUN_HTTP_SMOKE=0            Default: 0
#   RUN_TIMEOUT_SMOKE=0         Default: 0
#   BASE_URL=http://127.0.0.1:3000
#   PUBLIC_URL=https://apiproxy.example.com
#   ADMIN_KEY=...
#   API_KEY=...
#   TIMEOUT_MODE=remote         local | remote
#   TIMEOUT_WAIT_SECONDS=180

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

RUN_GO_TESTS="${RUN_GO_TESTS:-1}"
RUN_HTTP_SMOKE="${RUN_HTTP_SMOKE:-0}"
RUN_TIMEOUT_SMOKE="${RUN_TIMEOUT_SMOKE:-0}"

BASE_URL="${BASE_URL:-http://127.0.0.1:3000}"
PUBLIC_URL="${PUBLIC_URL:-${BASE_URL}}"
TIMEOUT_MODE="${TIMEOUT_MODE:-remote}"
TIMEOUT_WAIT_SECONDS="${TIMEOUT_WAIT_SECONDS:-180}"

PASS=0
FAIL=0
SKIP=0

pass() { PASS=$((PASS + 1)); printf "\033[32mPASS\033[0m: %s\n" "$*"; }
fail() { FAIL=$((FAIL + 1)); printf "\033[31mFAIL\033[0m: %s\n" "$*"; }
skip() { SKIP=$((SKIP + 1)); printf "\033[33mSKIP\033[0m: %s\n" "$*"; }
info() { printf "\033[36mINFO\033[0m: %s\n" "$*"; }

run_check() {
  local label="$1"
  shift

  info "$label"
  if "$@"; then
    pass "$label"
  else
    fail "$label"
  fi
}

run_go_pkg() {
  local label="$1"
  shift
  run_check "$label" go test "$@"
}

echo ""
info "================================================"
info "Pre-deploy smoke suite starting"
info "root=${ROOT_DIR}"
info "RUN_GO_TESTS=${RUN_GO_TESTS} RUN_HTTP_SMOKE=${RUN_HTTP_SMOKE} RUN_TIMEOUT_SMOKE=${RUN_TIMEOUT_SMOKE}"
info "================================================"
echo ""

cd "${ROOT_DIR}"

if [[ "${RUN_GO_TESTS}" == "1" ]]; then
  info "=== Go regression suite ==="

  run_go_pkg "relay passthrough adaptor tests" ./core/relay/adaptor/passthrough
  run_go_pkg "openai adaptor conversion tests" ./core/relay/adaptor/openai
  run_go_pkg "controller tests" ./core/controller/...
  run_go_pkg "model tests" ./core/model
  run_go_pkg "enterprise root tests (enterprise tag)" -tags enterprise ./core/enterprise
  run_go_pkg "enterprise PPIO sync tests (enterprise tag)" -tags enterprise ./core/enterprise/ppio
  run_go_pkg "enterprise Novita sync tests (enterprise tag)" -tags enterprise ./core/enterprise/novita
  run_go_pkg "enterprise shared synccommon tests (enterprise tag)" -tags enterprise ./core/enterprise/synccommon
else
  skip "Go regression suite disabled (RUN_GO_TESTS=0)"
fi

if [[ "${RUN_HTTP_SMOKE}" == "1" ]]; then
  info "=== HTTP smoke suite ==="
  if [[ -z "${ADMIN_KEY:-}" ]]; then
    fail "HTTP smoke suite requires ADMIN_KEY"
  else
    if ADMIN_KEY="${ADMIN_KEY}" API_KEY="${API_KEY:-}" PUBLIC_URL="${PUBLIC_URL}" \
      bash scripts/smoke-test.sh "${BASE_URL}"; then
      pass "generic deployment smoke script"
    else
      fail "generic deployment smoke script"
    fi
  fi
else
  skip "HTTP smoke suite disabled (RUN_HTTP_SMOKE=0)"
fi

if [[ "${RUN_TIMEOUT_SMOKE}" == "1" ]]; then
  info "=== Timeout smoke suite ==="
  if [[ -z "${API_KEY:-}" ]]; then
    fail "timeout smoke suite requires API_KEY"
  else
    if API_KEY="${API_KEY}" bash scripts/test-timeout-config.sh "${TIMEOUT_MODE}" "${TIMEOUT_WAIT_SECONDS}"; then
      pass "timeout configuration smoke"
    else
      fail "timeout configuration smoke"
    fi
  fi
else
  skip "Timeout smoke suite disabled (RUN_TIMEOUT_SMOKE=0)"
fi

echo ""
info "================================================"
info "Results: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped"
info "================================================"

if [[ ${FAIL} -gt 0 ]]; then
  exit 1
fi

exit 0
