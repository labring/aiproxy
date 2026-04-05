#!/usr/bin/env bash
#
# One-click full build script.
#
# Performs: frontend build → copy dist → swagger gen → go build -tags enterprise
# Then verifies the binary is complete (enterprise symbols + embedded frontend).
#
# Usage:
#   bash scripts/build.sh                         # default: output to core/aiproxy
#   bash scripts/build.sh -o mybin                # custom output path
#   SKIP_FRONTEND=1 bash scripts/build.sh         # skip frontend rebuild
#
# Environment:
#   SKIP_FRONTEND=1   — skip frontend build (use existing core/public/dist/)
#   SKIP_SWAGGER=1    — skip swagger regeneration
#   GOOS / GOARCH     — cross-compile (e.g., GOOS=linux GOARCH=amd64)

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORE_DIR="${ROOT_DIR}/core"
WEB_DIR="${ROOT_DIR}/web"
DIST_DIR="${CORE_DIR}/public/dist"
OUTPUT="${CORE_DIR}/aiproxy"

SKIP_FRONTEND="${SKIP_FRONTEND:-0}"
SKIP_SWAGGER="${SKIP_SWAGGER:-0}"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--output)
      OUTPUT="$2"
      shift 2
      ;;
    --skip-frontend)
      SKIP_FRONTEND=1
      shift
      ;;
    --skip-swagger)
      SKIP_SWAGGER=1
      shift
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

pass() { printf "\033[32m✓\033[0m %s\n" "$*"; }
fail() { printf "\033[31m✗\033[0m %s\n" "$*"; exit 1; }
info() { printf "\033[36m→\033[0m %s\n" "$*"; }
warn() { printf "\033[33m!\033[0m %s\n" "$*"; }

echo ""
info "=========================================="
info "Full build starting (frontend + swagger + enterprise)"
info "=========================================="
echo ""

# ─── Step 1: Frontend build ───────────────────────────────────────────
if [[ "${SKIP_FRONTEND}" == "1" ]]; then
  warn "Skipping frontend build (SKIP_FRONTEND=1)"
  if [[ ! -f "${DIST_DIR}/index.html" ]]; then
    fail "core/public/dist/index.html not found — cannot skip frontend build"
  fi
else
  info "Step 1/4: Building frontend..."
  cd "${WEB_DIR}"

  if ! command -v pnpm >/dev/null 2>&1; then
    info "pnpm not found, installing via npm..."
    npm install -g pnpm
  fi

  pnpm install --frozen-lockfile 2>/dev/null || pnpm install
  pnpm run build

  # Copy to embed directory
  info "Copying dist to core/public/dist/..."
  rm -rf "${DIST_DIR:?}/assets" "${DIST_DIR}/index.html" 2>/dev/null || true
  cp -r "${WEB_DIR}/dist/"* "${DIST_DIR}/"

  pass "Frontend built and copied"
fi

# Verify frontend artifacts
if [[ ! -f "${DIST_DIR}/index.html" ]]; then
  fail "core/public/dist/index.html missing after frontend build"
fi

JS_COUNT=$(find "${DIST_DIR}/assets" -name '*.js' 2>/dev/null | wc -l | tr -d ' ')
CSS_COUNT=$(find "${DIST_DIR}/assets" -name '*.css' 2>/dev/null | wc -l | tr -d ' ')
if [[ "${JS_COUNT}" -eq 0 ]]; then
  fail "No .js files in core/public/dist/assets/"
fi
pass "Frontend artifacts verified (${JS_COUNT} JS, ${CSS_COUNT} CSS)"

# ─── Step 2: Swagger generation ──────────────────────────────────────
if [[ "${SKIP_SWAGGER}" == "1" ]]; then
  warn "Skipping swagger generation (SKIP_SWAGGER=1)"
else
  info "Step 2/4: Generating Swagger docs..."
  cd "${CORE_DIR}"
  bash scripts/swag.sh
  pass "Swagger docs generated"
fi

# ─── Step 3: Go build with enterprise tag ────────────────────────────
info "Step 3/4: Building Go binary (tags=enterprise)..."
cd "${CORE_DIR}"

LDFLAGS="-s -w"
go build -tags enterprise -trimpath -ldflags "${LDFLAGS}" -o "${OUTPUT}"
pass "Binary built: ${OUTPUT}"

# ─── Step 4: Post-build verification ─────────────────────────────────
info "Step 4/4: Verifying build completeness..."

VERIFY_PASS=0
VERIFY_FAIL=0

verify() {
  local desc="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    pass "${desc}"
    VERIFY_PASS=$((VERIFY_PASS + 1))
  else
    printf "\033[31m✗\033[0m %s\n" "${desc}"
    VERIFY_FAIL=$((VERIFY_FAIL + 1))
  fi
}

# 4a. Binary exists and is executable
verify "Binary exists and is non-empty" test -s "${OUTPUT}"

# 4b. Enterprise module is linked
verify "Enterprise module included (enterprise.Initialize)" \
  go tool nm "${OUTPUT}" 2>/dev/null | grep -q 'enterprise'

# 4c. Key enterprise packages are present
verify "Enterprise analytics package linked" \
  go tool nm "${OUTPUT}" 2>/dev/null | grep -q 'enterprise/analytics'

verify "Enterprise feishu package linked" \
  go tool nm "${OUTPUT}" 2>/dev/null | grep -q 'enterprise/feishu'

verify "Enterprise quota package linked" \
  go tool nm "${OUTPUT}" 2>/dev/null | grep -q 'enterprise/quota'

# 4d. Frontend is embedded (check via strings for index.html marker)
verify "Frontend index.html embedded in binary" \
  strings "${OUTPUT}" | grep -q '</html>'

# 4e. Swagger docs embedded
verify "Swagger info embedded in binary" \
  strings "${OUTPUT}" | grep -q 'swagger'

echo ""
info "=========================================="
if [[ ${VERIFY_FAIL} -gt 0 ]]; then
  fail "Build verification: ${VERIFY_PASS} passed, ${VERIFY_FAIL} FAILED"
else
  pass "Build verification: all ${VERIFY_PASS} checks passed"
fi
info "Output: ${OUTPUT}"
info "=========================================="
echo ""
