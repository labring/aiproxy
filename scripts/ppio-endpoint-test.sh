#!/usr/bin/env bash
# PPIO endpoint smoke test — tests different endpoint types and protocol conversions.
# Usage: BASE=https://... KEY=sk-... bash scripts/ppio-endpoint-test.sh
set -uo pipefail

BASE="${BASE:-https://apiproxy.paigod.work}"
KEY="${KEY:-}"

if [[ -z "$KEY" ]]; then
  echo "ERROR: KEY env var required"; exit 2
fi

PASS=0; FAIL=0; TOTAL=0
pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); printf "\033[32mPASS\033[0m [%s] %s\n" "$1" "$2"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); printf "\033[31mFAIL\033[0m [%s] %s\n" "$1" "${2:-}"; }
info() { printf "\033[36m---- %s ----\033[0m\n" "$1"; }

# Helper: call endpoint, check HTTP status and extract key field
test_endpoint() {
  local label="$1" url="$2" method="$3" data="$4" expect_field="$5"
  local tmp; tmp=$(mktemp)
  local code

  if [[ "$method" == "POST" ]]; then
    code=$(curl -s -w "\n%{http_code}" -X POST "$url" \
      -H "Authorization: Bearer $KEY" \
      -H "Content-Type: application/json" \
      -d "$data" --max-time 60 -o "$tmp" 2>/dev/null | tail -1)
  else
    code=$(curl -s -w "\n%{http_code}" "$url" \
      -H "Authorization: Bearer $KEY" \
      --max-time 30 -o "$tmp" 2>/dev/null | tail -1)
  fi

  if [[ "$code" != "200" ]]; then
    local errmsg
    errmsg=$(head -c 300 "$tmp" 2>/dev/null)
    fail "$label" "HTTP $code — $errmsg"
    rm -f "$tmp"
    return 1
  fi

  if [[ -n "$expect_field" ]]; then
    if python3 -c "import sys,json; d=json.load(open('$tmp')); assert $expect_field" 2>/dev/null; then
      pass "$label" "HTTP 200, field check OK"
    else
      local body; body=$(head -c 300 "$tmp")
      fail "$label" "HTTP 200 but field check failed ($expect_field) — $body"
      rm -f "$tmp"
      return 1
    fi
  else
    pass "$label" "HTTP 200"
  fi

  rm -f "$tmp"
  return 0
}

# ============================================================
info "1. ChatCompletions — PPIO native model"
# ============================================================
test_endpoint \
  "ChatCompletions / deepseek-v3.2" \
  "$BASE/v1/chat/completions" \
  POST \
  '{"model":"deepseek/deepseek-v3.2","messages":[{"role":"user","content":"Say hi in 3 words"}],"max_tokens":20}' \
  "d.get('choices') and len(d['choices'])>0"

# ============================================================
info "2. ChatCompletions — PPIO reasoning model (deepseek-r1)"
# ============================================================
test_endpoint \
  "ChatCompletions / deepseek-r1" \
  "$BASE/v1/chat/completions" \
  POST \
  '{"model":"deepseek/deepseek-r1","messages":[{"role":"user","content":"What is 2+3?"}],"max_tokens":200}' \
  "d.get('choices') and len(d['choices'])>0"

# ============================================================
info "3. ChatCompletions — responses-only model (protocol conversion)"
# ============================================================
# This tests the critical path: ChatCompletions request → internally converted
# to Responses API → PPIO adaptor must rewrite base URL to /openai/v1/responses
test_endpoint \
  "ChatCompletions→Responses / pa/gpt-5.4-pro" \
  "$BASE/v1/chat/completions" \
  POST \
  '{"model":"pa/gpt-5.4-pro","messages":[{"role":"user","content":"Say hi in 3 words"}],"max_tokens":20}' \
  "d.get('choices') and len(d['choices'])>0"

# ============================================================
info "4. Responses API — direct call with responses-only model"
# ============================================================
test_endpoint \
  "Responses API / pa/gpt-5.4-pro" \
  "$BASE/v1/responses" \
  POST \
  '{"model":"pa/gpt-5.4-pro","input":"Say hi in 3 words","max_output_tokens":20}' \
  "d.get('output') and len(d['output'])>0"

# ============================================================
info "5. Responses API — with ChatCompletions model (dual-endpoint)"
# ============================================================
# Some models support both ChatCompletions and Responses API
test_endpoint \
  "Responses API / deepseek-v3.2" \
  "$BASE/v1/responses" \
  POST \
  '{"model":"deepseek/deepseek-v3.2","input":"Say hi in 3 words","max_output_tokens":20}' \
  "d.get('output') and len(d['output'])>0"

# ============================================================
info "6. Anthropic protocol — PPIO model via Messages API"
# ============================================================
test_endpoint \
  "Anthropic Messages / deepseek-v3.2" \
  "$BASE/v1/messages" \
  POST \
  '{"model":"deepseek/deepseek-v3.2","messages":[{"role":"user","content":"Say hi in 3 words"}],"max_tokens":20}' \
  "d.get('content') and len(d['content'])>0"

# ============================================================
info "7. Anthropic protocol — responses-only model (double conversion)"
# ============================================================
# Anthropic Messages → internally converted to Responses API
# PPIO adaptor must also rewrite base URL here
test_endpoint \
  "Anthropic→Responses / pa/gpt-5.4-pro" \
  "$BASE/v1/messages" \
  POST \
  '{"model":"pa/gpt-5.4-pro","messages":[{"role":"user","content":"Say hi in 3 words"}],"max_tokens":20}' \
  "d.get('content') and len(d['content'])>0"

# ============================================================
info "8. ChatCompletions — streaming (PPIO native)"
# ============================================================
tmp_stream=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek/deepseek-v3.2","messages":[{"role":"user","content":"Say hi"}],"max_tokens":20,"stream":true}' \
  --max-time 30 -o "$tmp_stream" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]] && grep -q '"delta"' "$tmp_stream"; then
  pass "ChatCompletions stream / deepseek-v3.2" "HTTP 200, SSE chunks received"
else
  fail "ChatCompletions stream / deepseek-v3.2" "HTTP $code — $(head -c 200 "$tmp_stream")"
fi
rm -f "$tmp_stream"

# ============================================================
info "9. ChatCompletions streaming — responses-only model"
# ============================================================
tmp_stream2=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"pa/gpt-5.4-pro","messages":[{"role":"user","content":"Say hi"}],"max_tokens":20,"stream":true}' \
  --max-time 30 -o "$tmp_stream2" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]] && grep -q '"delta"' "$tmp_stream2"; then
  pass "ChatCompletions stream→Responses / pa/gpt-5.4-pro" "HTTP 200, SSE chunks received"
else
  fail "ChatCompletions stream→Responses / pa/gpt-5.4-pro" "HTTP $code — $(head -c 200 "$tmp_stream2")"
fi
rm -f "$tmp_stream2"

# ============================================================
# Summary
# ============================================================
echo ""
printf "\033[36m==== Results: %d passed, %d failed (of %d) ====\033[0m\n" "$PASS" "$FAIL" "$TOTAL"

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
exit 0
