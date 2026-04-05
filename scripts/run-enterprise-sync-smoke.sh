#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$ROOT_DIR/scripts"
SMOKE_SCRIPT="$SCRIPT_DIR/enterprise-sync-smoke.mjs"

BASE_URL="${AIPROXY_WEB_BASE_URL:-http://localhost:5173}"
TOKEN="${AIPROXY_TEST_TOKEN:-${1:-}}"
CHROME_PATH="${AIPROXY_CHROME_PATH:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
PW_ENV_DIR="${AIPROXY_SMOKE_PW_DIR:-/tmp/aiproxy-playwright-core}"
OUTPUT_DIR="${AIPROXY_SMOKE_OUTPUT_DIR:-$ROOT_DIR/output/playwright}"

if [[ -z "$TOKEN" ]]; then
  cat <<EOF
Usage:
  AIPROXY_TEST_TOKEN=<token> $0
  $0 <token>

Optional env:
  AIPROXY_WEB_BASE_URL       Default: http://localhost:5173
  AIPROXY_CHROME_PATH        Default: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome
  AIPROXY_SMOKE_PW_DIR       Default: /tmp/aiproxy-playwright-core
  AIPROXY_SMOKE_OUTPUT_DIR   Default: $ROOT_DIR/output/playwright
EOF
  exit 1
fi

if [[ ! -x "$CHROME_PATH" ]]; then
  echo "Chrome executable not found: $CHROME_PATH" >&2
  exit 1
fi

mkdir -p "$PW_ENV_DIR" "$OUTPUT_DIR"

if [[ ! -f "$PW_ENV_DIR/package.json" ]]; then
  (
    cd "$PW_ENV_DIR"
    npm init -y >/dev/null 2>&1
  )
fi

if [[ ! -d "$PW_ENV_DIR/node_modules/playwright-core" ]]; then
  echo "Installing playwright-core into $PW_ENV_DIR ..."
  (
    cd "$PW_ENV_DIR"
    npm install playwright-core --silent
  )
fi

PLAYWRIGHT_CORE_MODULE_DIR="$PW_ENV_DIR/node_modules" \
AIPROXY_WEB_BASE_URL="$BASE_URL" \
AIPROXY_TEST_TOKEN="$TOKEN" \
AIPROXY_CHROME_PATH="$CHROME_PATH" \
AIPROXY_SMOKE_OUTPUT_DIR="$OUTPUT_DIR" \
node "$SMOKE_SCRIPT"
