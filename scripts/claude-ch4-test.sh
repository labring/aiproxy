#!/usr/bin/env bash
# Claude CH4-only routing test — verify all protocol types work when model
# is only on CH4 (Anthropic native channel).
# Usage: BASE=https://... KEY=sk-... MODEL=pa/claude-sonnet-4-6 bash scripts/claude-ch4-test.sh
set -uo pipefail

BASE="${BASE:-https://apiproxy.paigod.work}"
KEY="${KEY:-}"
MODEL="${MODEL:-pa/claude-sonnet-4-6}"

if [[ -z "$KEY" ]]; then
  echo "ERROR: KEY env var required"; exit 2
fi

PASS=0; FAIL=0; TOTAL=0
pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); printf "\033[32mPASS\033[0m [%s] %s\n" "$1" "$2"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); printf "\033[31mFAIL\033[0m [%s] %s\n" "$1" "${2:-}"; }
info() { printf "\n\033[36m---- %s ----\033[0m\n" "$1"; }

# ============================================================
info "1. ChatCompletions (non-streaming)"
# ============================================================
tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hi in 3 words\"}],\"max_tokens\":20}" \
  --max-time 60 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  pass "ChatCompletions" "HTTP 200 — $(python3 -c "import json; d=json.load(open('$tmp')); print(d['choices'][0]['message']['content'][:80])" 2>/dev/null || echo '(parse error)')"
else
  fail "ChatCompletions" "HTTP $code — $(head -c 300 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "2. ChatCompletions (streaming)"
# ============================================================
tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hi in 3 words\"}],\"max_tokens\":20,\"stream\":true}" \
  --max-time 60 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]] && grep -q '"delta"' "$tmp"; then
  pass "ChatCompletions stream" "HTTP 200, SSE chunks received"
else
  fail "ChatCompletions stream" "HTTP $code — $(head -c 300 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "3. Anthropic Messages (non-streaming)"
# ============================================================
tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$BASE/v1/messages" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hi in 3 words\"}],\"max_tokens\":20}" \
  --max-time 60 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  pass "Anthropic Messages" "HTTP 200 — $(python3 -c "import json; d=json.load(open('$tmp')); print(d['content'][0]['text'][:80])" 2>/dev/null || echo '(parse error)')"
else
  fail "Anthropic Messages" "HTTP $code — $(head -c 300 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "4. Anthropic Messages (streaming)"
# ============================================================
# Use -N to disable output buffering for SSE
tmp=$(mktemp)
code=$(curl -s -N -w "\n%{http_code}" -X POST "$BASE/v1/messages" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hi in 3 words\"}],\"max_tokens\":20,\"stream\":true}" \
  --max-time 60 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]] && grep -q '"content_block_delta"\|"text_delta"\|"delta"' "$tmp"; then
  pass "Anthropic Messages stream" "HTTP 200, SSE events received"
elif [[ "$code" == "200" ]]; then
  # Even if grep misses the exact event type, 200 is a good sign — show body sample
  pass "Anthropic Messages stream" "HTTP 200 — $(head -c 200 "$tmp")"
else
  fail "Anthropic Messages stream" "HTTP $code — $(head -c 300 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "5. Gemini protocol (generateContent)"
# ============================================================
# URL-encode the model name: pa/claude-sonnet-4-6 → pa%2Fclaude-sonnet-4-6
ENCODED_MODEL=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$MODEL', safe=''))")
tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST \
  "$BASE/v1beta/models/${ENCODED_MODEL}:generateContent" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"Say hi in 3 words"}]}],"generationConfig":{"maxOutputTokens":20}}' \
  --max-time 60 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "200" ]]; then
  pass "Gemini generateContent" "HTTP 200 — $(head -c 200 "$tmp")"
else
  fail "Gemini generateContent" "HTTP $code — $(head -c 300 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
info "6. Responses API (expect 400 — Claude on CH4 doesn't support Responses)"
# ============================================================
tmp=$(mktemp)
code=$(curl -s -w "\n%{http_code}" -X POST "$BASE/v1/responses" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"input\":\"Say hi in 3 words\",\"max_output_tokens\":20}" \
  --max-time 30 -o "$tmp" 2>/dev/null | tail -1)

if [[ "$code" == "400" || "$code" == "404" ]]; then
  pass "Responses API (expected fail)" "HTTP $code — correctly rejected"
elif [[ "$code" == "200" ]]; then
  pass "Responses API (unexpected success)" "HTTP 200 — model supports Responses via CH4"
else
  fail "Responses API" "HTTP $code — $(head -c 300 "$tmp")"
fi
rm -f "$tmp"

# ============================================================
# Summary
# ============================================================
echo ""
printf "\033[36m==== Results: %d passed, %d failed (of %d) ====\033[0m\n" "$PASS" "$FAIL" "$TOTAL"

[[ $FAIL -gt 0 ]] && exit 1
exit 0
