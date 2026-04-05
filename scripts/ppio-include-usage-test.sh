#!/usr/bin/env bash
# Test how PPIO endpoints behave when stream_options.include_usage is injected.
# Directly hits PPIO API (bypasses aiproxy) to isolate PPIO's own behavior.
#
# Usage:
#   PPIO_KEY=sk-... bash scripts/ppio-include-usage-test.sh
#   PPIO_KEY=sk-... CHAT_BASE=https://api.ppinfra.com/v3/openai bash scripts/ppio-include-usage-test.sh

set -uo pipefail

CHAT_BASE="${CHAT_BASE:-https://api.ppinfra.com/v3/openai}"
RESP_BASE="${RESP_BASE:-https://api.ppinfra.com/openai/v1}"
WSEARCH_BASE="${WSEARCH_BASE:-https://api.ppinfra.com/v3}"

PPIO_KEY="${PPIO_KEY:-}"
if [[ -z "$PPIO_KEY" ]]; then
  echo "ERROR: PPIO_KEY env var required (get from https://ppinfra.com console)"
  exit 2
fi

PASS=0; FAIL=0; SKIP=0

pass() { PASS=$((PASS+1)); printf "\033[32mPASS\033[0m  %-55s %s\n" "$1" "$2"; }
fail() { FAIL=$((FAIL+1)); printf "\033[31mFAIL\033[0m  %-55s %s\n" "$1" "${2:-}"; }
skip() { SKIP=$((SKIP+1)); printf "\033[33mSKIP\033[0m  %-55s %s\n" "$1" "$2"; }
info() { printf "\n\033[36m=== %s ===\033[0m\n" "$1"; }

# Check if a streaming response contains a usage chunk (last data chunk with "usage" key)
has_usage_in_stream() {
  local file="$1"
  grep -q '"usage"' "$file"
}

# Extract usage from last data chunk of a streaming response
show_usage_from_stream() {
  local file="$1"
  grep '"usage"' "$file" | tail -1 | python3 -c "
import sys, json
for line in sys.stdin:
    line = line.strip()
    if line.startswith('data:'):
        line = line[5:].strip()
    try:
        d = json.loads(line)
        u = d.get('usage') or d.get('usageMetadata')
        if u:
            print('  usage:', json.dumps(u))
    except:
        pass
" 2>/dev/null || true
}

# ============================================================
info "A. Chat/Completions streaming — with stream_options.include_usage"
# ============================================================
# This is what aiproxy currently injects. Does PPIO honor it and return usage in stream?

tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$CHAT_BASE/chat/completions" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek/deepseek-v3",
    "messages": [{"role":"user","content":"Say hi in 3 words"}],
    "max_tokens": 20,
    "stream": true,
    "stream_options": {"include_usage": true}
  }' \
  --max-time 30 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  if has_usage_in_stream "$tmp"; then
    pass "chat/completions stream + include_usage" "HTTP 200, usage present in stream ✓"
    show_usage_from_stream "$tmp"
  else
    fail "chat/completions stream + include_usage" "HTTP 200 but NO usage in stream — passthrough won't get token data"
    echo "  last 3 data lines:"
    grep '^data:' "$tmp" | tail -3 | sed 's/^/    /'
  fi
else
  fail "chat/completions stream + include_usage" "HTTP $code — $(head -c 200 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "B. Chat/Completions streaming — WITHOUT stream_options (baseline)"
# ============================================================
# Confirm that without the flag, usage is absent from stream

tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$CHAT_BASE/chat/completions" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek/deepseek-v3",
    "messages": [{"role":"user","content":"Say hi in 3 words"}],
    "max_tokens": 20,
    "stream": true
  }' \
  --max-time 30 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  if has_usage_in_stream "$tmp"; then
    pass "chat/completions stream (no flag)" "HTTP 200, usage present even without flag (PPIO default)"
    show_usage_from_stream "$tmp"
  else
    pass "chat/completions stream (no flag)" "HTTP 200, no usage in stream (expected — flag needed)"
  fi
else
  fail "chat/completions stream (no flag)" "HTTP $code — $(head -c 200 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "C. Chat/Completions NON-streaming — with stream_options.include_usage"
# ============================================================
# Does injecting stream_options into a non-streaming request cause an error?

tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$CHAT_BASE/chat/completions" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek/deepseek-v3",
    "messages": [{"role":"user","content":"Say hi in 3 words"}],
    "max_tokens": 20,
    "stream": false,
    "stream_options": {"include_usage": true}
  }' \
  --max-time 30 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  if python3 -c "import sys,json,pathlib; d=json.loads(pathlib.Path('$tmp').read_text()); assert d.get('choices')" 2>/dev/null; then
    pass "chat/completions non-stream + include_usage" "HTTP 200, request accepted (field ignored safely)"
  else
    fail "chat/completions non-stream + include_usage" "HTTP 200 but no choices — $(head -c 200 "$tmp")"
  fi
else
  # Some providers reject stream_options on non-streaming requests
  body=$(head -c 300 "$tmp")
  fail "chat/completions non-stream + include_usage" "HTTP $code — PPIO REJECTS this combination — $body"
fi
rm -f "$tmp"

# ============================================================
info "D. Responses API streaming — with stream_options.include_usage"
# ============================================================
# Responses API uses different event format. Does it accept this field?

tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$RESP_BASE/responses" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek/deepseek-v3",
    "input": "Say hi in 3 words",
    "max_output_tokens": 20,
    "stream": true,
    "stream_options": {"include_usage": true}
  }' \
  --max-time 30 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  if grep -q 'response.completed\|usage' "$tmp"; then
    pass "Responses API stream + include_usage" "HTTP 200, response.completed event present"
    show_usage_from_stream "$tmp"
  else
    pass "Responses API stream + include_usage" "HTTP 200 (check events manually)"
    grep '^event:\|^data:' "$tmp" | tail -5 | sed 's/^/  /'
  fi
elif [[ "$code" == "400" ]]; then
  body=$(head -c 300 "$tmp")
  fail "Responses API stream + include_usage" "HTTP 400 — PPIO REJECTS stream_options here — $body"
else
  fail "Responses API stream + include_usage" "HTTP $code — $(head -c 200 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "E. Responses API NON-streaming — with stream_options.include_usage"
# ============================================================

tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$RESP_BASE/responses" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek/deepseek-v3",
    "input": "Say hi in 3 words",
    "max_output_tokens": 20,
    "stream_options": {"include_usage": true}
  }' \
  --max-time 30 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  pass "Responses API non-stream + include_usage" "HTTP 200, field accepted"
elif [[ "$code" == "400" ]]; then
  body=$(head -c 300 "$tmp")
  fail "Responses API non-stream + include_usage" "HTTP 400 — REJECTED — $body"
else
  fail "Responses API non-stream + include_usage" "HTTP $code — $(head -c 200 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "F. Embeddings — with stream_options.include_usage (should be irrelevant)"
# ============================================================
# Embeddings don't stream; injecting this field should either be ignored or error

tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$CHAT_BASE/embeddings" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "baai/bge-m3",
    "input": "hello world",
    "stream_options": {"include_usage": true}
  }' \
  --max-time 30 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  if python3 -c "import sys,json,pathlib; d=json.loads(pathlib.Path('$tmp').read_text()); assert d.get('data')" 2>/dev/null; then
    pass "embeddings + include_usage" "HTTP 200, field ignored (safe)"
  else
    fail "embeddings + include_usage" "HTTP 200 but no data field — $(head -c 200 "$tmp")"
  fi
else
  body=$(head -c 300 "$tmp")
  fail "embeddings + include_usage" "HTTP $code — PPIO REJECTS this — $body"
fi
rm -f "$tmp"

# ============================================================
info "G. Web Search — with stream_options.include_usage"
# ============================================================

tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$WSEARCH_BASE/web-search" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "what is 2+2",
    "stream_options": {"include_usage": true}
  }' \
  --max-time 30 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  pass "web-search + include_usage" "HTTP 200, field ignored (safe)"
elif [[ "$code" == "400" ]]; then
  body=$(head -c 300 "$tmp")
  fail "web-search + include_usage" "HTTP 400 — REJECTED — $body"
else
  fail "web-search + include_usage" "HTTP $code — $(head -c 200 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "H. Reasoning model (deepseek-r1) streaming — with stream_options.include_usage"
# ============================================================
# Reasoning models emit reasoning_content; does include_usage still work?

tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$CHAT_BASE/chat/completions" \
  -H "Authorization: Bearer $PPIO_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek/deepseek-r1",
    "messages": [{"role":"user","content":"What is 1+1?"}],
    "max_tokens": 50,
    "stream": true,
    "stream_options": {"include_usage": true}
  }' \
  --max-time 45 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  if has_usage_in_stream "$tmp"; then
    pass "deepseek-r1 stream + include_usage" "HTTP 200, usage present ✓"
    show_usage_from_stream "$tmp"
  else
    fail "deepseek-r1 stream + include_usage" "HTTP 200 but no usage in stream"
  fi
else
  fail "deepseek-r1 stream + include_usage" "HTTP $code — $(head -c 200 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
# Summary
# ============================================================
echo ""
printf "\033[36m====  Results: \033[32m%d passed\033[36m  \033[31m%d failed\033[36m  \033[33m%d skipped\033[36m  ====\033[0m\n" \
  "$PASS" "$FAIL" "$SKIP"
echo ""
echo "Interpretation:"
echo "  All PASS → stream_options.include_usage is safe to inject on all endpoints"
echo "  FAIL on non-stream tests → must NOT inject when stream=false (or stream absent)"
echo "  FAIL on Responses/embeddings → need per-endpoint injection guards"
echo ""

if [[ $FAIL -gt 0 ]]; then exit 1; fi
exit 0
