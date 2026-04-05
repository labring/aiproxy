#!/usr/bin/env bash
# Test whether PPIO returns usage data across all model/endpoint combinations.
# Directly hits PPIO API to measure "streaming without usage" frequency.
#
# Usage: PPIO_KEY=sk-... bash scripts/ppio-usage-coverage-test.sh

set -uo pipefail

CHAT_BASE="${CHAT_BASE:-https://api.ppinfra.com/v3/openai}"
RESP_BASE="${RESP_BASE:-https://api.ppinfra.com/openai/v1}"
PPIO_KEY="${PPIO_KEY:-}"

if [[ -z "$PPIO_KEY" ]]; then
  echo "ERROR: PPIO_KEY required"; exit 2
fi

PASS=0; FAIL=0

pass() { PASS=$((PASS+1)); printf "\033[32mPASS\033[0m  %-60s %s\n" "$1" "$2"; }
fail() { FAIL=$((FAIL+1)); printf "\033[31mFAIL\033[0m  %-60s %s\n" "$1" "${2:-}"; }
info() { printf "\n\033[36m=== %s ===\033[0m\n" "$1"; }

has_usage() { grep -q '"usage"' "$1"; }
show_usage() {
  grep '"usage"' "$1" | tail -1 | python3 -c "
import sys,json
for line in sys.stdin:
  line=line.strip()
  if line.startswith('data:'): line=line[5:].strip()
  try:
    d=json.loads(line)
    u=d.get('usage')
    if u: print('  →',json.dumps(u))
  except: pass
" 2>/dev/null || true
}
show_non_stream_usage() {
  python3 -c "
import sys,json,pathlib
d=json.loads(pathlib.Path('$1').read_text())
u=d.get('usage')
if u: print('  →',json.dumps(u))
else: print('  → usage: MISSING')
" 2>/dev/null || true
}

do_stream() {
  local label="$1" model="$2" extra="${3:-}"
  local tmp; tmp=$(mktemp)
  local body="{\"model\":\"$model\",\"messages\":[{\"role\":\"user\",\"content\":\"Hi\"}],\"max_tokens\":15,\"stream\":true $extra}"
  local code
  code=$(curl -s -w "\n%{http_code}" -X POST "$CHAT_BASE/chat/completions" \
    -H "Authorization: Bearer $PPIO_KEY" -H "Content-Type: application/json" \
    -d "$body" --max-time 45 -o "$tmp" 2>/dev/null | tail -1)
  if [[ "$code" == "200" ]]; then
    if has_usage "$tmp"; then
      pass "$label" "usage ✓"
      show_usage "$tmp"
    else
      fail "$label" "HTTP 200 but NO usage in stream"
      echo "  last 2 chunks:"; grep '^data:' "$tmp" | tail -2 | sed 's/^/    /'
    fi
  else
    fail "$label" "HTTP $code — $(head -c 150 "$tmp")"
  fi
  rm -f "$tmp"
}

do_nonstream() {
  local label="$1" model="$2"
  local tmp; tmp=$(mktemp)
  local body="{\"model\":\"$model\",\"messages\":[{\"role\":\"user\",\"content\":\"Hi\"}],\"max_tokens\":15}"
  local code
  code=$(curl -s -w "\n%{http_code}" -X POST "$CHAT_BASE/chat/completions" \
    -H "Authorization: Bearer $PPIO_KEY" -H "Content-Type: application/json" \
    -d "$body" --max-time 45 -o "$tmp" 2>/dev/null | tail -1)
  if [[ "$code" == "200" ]]; then
    pass "$label" "non-stream"
    show_non_stream_usage "$tmp"
  else
    fail "$label" "HTTP $code — $(head -c 150 "$tmp")"
  fi
  rm -f "$tmp"
}

do_embed() {
  local label="$1" model="$2"
  local tmp; tmp=$(mktemp)
  local code
  code=$(curl -s -w "\n%{http_code}" -X POST "$CHAT_BASE/embeddings" \
    -H "Authorization: Bearer $PPIO_KEY" -H "Content-Type: application/json" \
    -d "{\"model\":\"$model\",\"input\":\"hello world\"}" \
    --max-time 30 -o "$tmp" 2>/dev/null | tail -1)
  if [[ "$code" == "200" ]]; then
    show_non_stream_usage "$tmp"
    pass "$label" "embed"
  else
    fail "$label" "HTTP $code — $(head -c 150 "$tmp")"
  fi
  rm -f "$tmp"
}

do_responses() {
  local label="$1" model="$2" stream="$3"
  local tmp; tmp=$(mktemp)
  local body="{\"model\":\"$model\",\"input\":\"Hi\",\"max_output_tokens\":15,\"stream\":$stream}"
  local code
  code=$(curl -s -w "\n%{http_code}" -X POST "$RESP_BASE/responses" \
    -H "Authorization: Bearer $PPIO_KEY" -H "Content-Type: application/json" \
    -d "$body" --max-time 45 -o "$tmp" 2>/dev/null | tail -1)
  if [[ "$code" == "200" ]]; then
    if [[ "$stream" == "true" ]]; then
      if grep -q 'response.completed\|"usage"' "$tmp"; then
        pass "$label" "responses stream, completed event ✓"
        grep '"usage"' "$tmp" | tail -1 | sed 's/^/  /'
      else
        fail "$label" "HTTP 200 but no completed/usage event"
        grep '^event:\|^data:' "$tmp" | tail -4 | sed 's/^/  /'
      fi
    else
      show_non_stream_usage "$tmp"
      pass "$label" "responses non-stream"
    fi
  else
    fail "$label" "HTTP $code — $(head -c 200 "$tmp")"
  fi
  rm -f "$tmp"
}

# ============================================================
info "1. Chat completions — streaming (no explicit stream_options)"
# ============================================================
do_stream "deepseek/deepseek-v3"          "deepseek/deepseek-v3"
do_stream "deepseek/deepseek-r1"          "deepseek/deepseek-r1"
do_stream "qwen/qwq-32b"                  "qwen/qwq-32b"
do_stream "moonshotai/kimi-k2.5"          "moonshotai/kimi-k2.5"
do_stream "zai-org/glm-5"                 "zai-org/glm-5"
do_stream "minimax/minimax-m2.5"          "minimax/minimax-m2.5"

# ============================================================
info "2. Chat completions — non-streaming (usage in response body)"
# ============================================================
do_nonstream "deepseek/deepseek-v3 (non-stream)" "deepseek/deepseek-v3"
do_nonstream "deepseek/deepseek-r1 (non-stream)" "deepseek/deepseek-r1"

# ============================================================
info "3. Embeddings — usage field presence"
# ============================================================
do_embed "baai/bge-m3 embed"              "baai/bge-m3"

# ============================================================
info "4. Responses API — streaming + non-streaming (responses-only model)"
# ============================================================
do_responses "pa/gpt-4.1 responses stream"     "pa/gpt-4.1"     "true"
do_responses "pa/gpt-4.1 responses non-stream" "pa/gpt-4.1"     "false"

# ============================================================
info "5. Model NOT in our sync list — direct passthrough test"
# ============================================================
# Test: if PPIO has a model that's not in our ModelConfig list,
# can the request go through when we bypass model validation?
# We test with a model name that PPIO definitely has but we may not have synced.
do_stream "deepseek/deepseek-v3.2-exp (may be unsynced)" "deepseek/deepseek-v3.2-exp"

# ============================================================
info "6. Anthropic endpoint — usage in response"
# ============================================================
ANTHROPIC_BASE="https://api.ppinfra.com/anthropic"
tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$ANTHROPIC_BASE/v1/messages" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-opus-4-5",
    "messages": [{"role":"user","content":"Hi"}],
    "max_tokens": 15
  }' \
  --max-time 45 -o "$tmp" 2>/dev/null | tail -1)
if [[ "$code" == "200" ]]; then
  pass "Anthropic endpoint (non-stream)" "HTTP 200"
  python3 -c "
import sys,json,pathlib
d=json.loads(pathlib.Path('$tmp').read_text())
u=d.get('usage')
if u: print('  →',json.dumps(u))
else: print('  → usage: MISSING from Anthropic response')
" 2>/dev/null || true
else
  fail "Anthropic endpoint" "HTTP $code — $(head -c 200 "$tmp")"
fi
rm -f "$tmp"

# Anthropic streaming
tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$ANTHROPIC_BASE/v1/messages" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-opus-4-5",
    "messages": [{"role":"user","content":"Hi"}],
    "max_tokens": 15,
    "stream": true
  }' \
  --max-time 45 -o "$tmp" 2>/dev/null | tail -1)
if [[ "$code" == "200" ]]; then
  if grep -q 'message_delta\|usage' "$tmp"; then
    pass "Anthropic endpoint (stream)" "stream events present"
    grep '"usage"' "$tmp" | tail -1 | sed 's/^/  /'
  else
    fail "Anthropic endpoint (stream)" "no usage in stream"
  fi
else
  fail "Anthropic endpoint (stream)" "HTTP $code — $(head -c 200 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
echo ""
printf "\033[36m====  Results: \033[32m%d passed  \033[31m%d failed  ====\033[0m\n" "$PASS" "$FAIL"
if [[ $FAIL -gt 0 ]]; then exit 1; fi
exit 0
