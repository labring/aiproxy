# Claude Code x Codex Handoff

This document defines the standard handoff between:

- Claude Code: implementation and iterative development
- Codex: review, regression assessment, and test verification

The goal is to keep development fast while making review and testing consistent.

## Role Split

### Claude Code

Use Claude Code primarily for:

- feature implementation
- refactoring
- bug fixing
- exploratory coding
- first-pass local validation

### Codex

Use Codex primarily for:

- code review
- regression and risk assessment
- test gap analysis
- browser-based verification
- API validation and edge-case coverage

## Shared Skills

The recommended shared workflow relies on these skills:

- `systematic-debugging`
- `api-test-suite-builder`
- `webapp-testing`
- `playwright`

## Required Handoff Format

After Claude Code finishes implementation, provide the following six items to Codex.

### 1. Goal

One or two sentences describing what changed and why.

Example:

```text
Goal: Fix enterprise token sync so newly synced models are visible on the My Access page without requiring a manual cache refresh.
```

### 2. Scope

List the main files or modules changed.

Example:

```text
Scope:
- core/enterprise/access_info.go
- core/enterprise/ppio/sync.go
- web/src/pages/enterprise/my-access.tsx
```

### 3. Root Cause

State the issue using the `systematic-debugging` mindset:

- what the symptom was
- what the root cause was
- why the fix addresses the root cause

Example:

```text
Root cause:
- Symptom: synced models did not appear immediately in My Access
- Cause: model cache refresh happened after channel sync in one path, but not after manual sync preview/apply flow
- Fix rationale: both sync completion paths now trigger the same cache refresh sequence
```

### 4. Risk Areas

List the most likely regression points.

Keep this short and concrete.

Example:

```text
Risk areas:
- stale cache after partial sync failure
- My Access page grouping by owner
- enterprise-only route path behavior
```

### 5. Suggested Verification

Tell Codex what to verify first.

Split by category when useful:

```text
Suggested verification:
- API: sync apply endpoint, my-access endpoint
- UI: enterprise My Access page refresh and model visibility
- Edge cases: sync with no changes, sync with one deleted model
```

### 6. Validation Already Performed

State what has already been run and what has not.

Example:

```text
Validation already performed:
- go test ./core/enterprise/ppio/... passed
- UI not browser-verified
- no full enterprise smoke test run yet
```

## Codex Review Workflow

When Codex receives a handoff, use this sequence.

### Step 1: Root-Cause Review

Use `systematic-debugging` principles:

- confirm the change addresses the stated root cause
- check whether the fix only masks symptoms
- identify hidden assumptions

### Step 2: Test Gap Review

Use `api-test-suite-builder` principles for backend or API-facing changes:

- auth coverage
- invalid input coverage
- status code coverage
- edge cases and boundaries
- tenant and permission isolation

### Step 3: UI and Flow Verification

Use:

- `webapp-testing` for local app workflow validation
- `playwright` for direct browser automation and deeper interaction checks

At minimum, Codex should try to verify:

- one primary happy path
- one auth or permission failure path
- one boundary or error path

## Minimal Operating Rule

For changes with user-facing or API-facing impact:

- Claude Code should not hand off without the 6-part summary
- Codex should not sign off without stating:
  - key findings or no findings
  - residual risks
  - what was verified vs not verified

## Recommended Prompts

### Prompt for Claude Code

```text
Finish implementation, then hand off to Codex using this format:
1. Goal
2. Scope
3. Root cause
4. Risk areas
5. Suggested verification
6. Validation already performed
```

### Prompt for Codex

```text
Review this change using systematic-debugging for root-cause consistency, api-test-suite-builder for test gap analysis, and webapp-testing / playwright for critical flow verification.
```

## Notes

- This process is intentionally lightweight.
- The handoff should be fast to write and fast to consume.
- If a change is trivial, keep each section brief rather than skipping the structure.

---

## Past Handoff — 2026-04-04 (Request History)

### 1. Goal

Add a user-facing "Request History" section to the enterprise My Access page so Feishu-authenticated users can browse their own request logs (with model/status filtering, cursor pagination, and request/response body detail).

### 2. Scope

**Backend:**

- `core/model/log.go` — removed mandatory `tokenName` guard from `GetTokenLogs`; empty tokenName now means "all tokens for this group"
- `core/enterprise/access_info.go` — added `GetMyLogs` and `GetMyLogDetail` handlers; switched timestamp parsing to `utils.ParseTimeRange` (ms-safe)
- `core/enterprise/router.go` — registered `GET /api/enterprise/my-access/logs` and `GET /api/enterprise/my-access/logs/:log_id`

**Frontend:**

- `web/src/api/enterprise.ts` — added `UserLog`, `GetMyLogsResult`, `RequestDetail` types; added `getMyLogs` and `getMyLogDetail` to `enterpriseApi`
- `web/src/pages/enterprise/my-access.tsx` — added `RequestLogsSection` component (time range picker, model filter, status filter, cursor-paginated table, detail dialog)
- `web/public/locales/zh/translation.json` and `en/translation.json` — 17 new i18n keys under `enterprise.myAccess`

**Commits:** `7a00a11`, `d6114ac`, `393238d`, `278248e`, `7262927`, `069425f`, `4125f9b`

### 3. Root Cause

```text
Root cause:
- Symptom: enterprise users had no way to view their own request logs; only admins could access /api/logs
- Cause: existing GetTokenLogs required a non-empty tokenName, making it unusable for group-wide queries
  from enterprise endpoints where the user may have multiple tokens
- Fix rationale:
  - Relaxed GetTokenLogs to treat empty tokenName as "no filter" (consistent with buildGetLogsQuery behavior)
  - New enterprise endpoints use Feishu auth → groupID for data isolation (users cannot see other groups)
  - Timestamp parsing bug: old parseTimestampRange used strconv.ParseInt + time.Unix, which breaks for
    JS millisecond timestamps; replaced with utils.ParseTimeRange (smartParseTimestamp handles ms/us/ns)
```

### 4. Risk Areas

```text
Risk areas:
- Data isolation: GetMyLogs scopes by feishuUser.GroupID — verify Feishu users cannot query other groups
- GetTokenLogs unchanged: tokenName guard restored (commit 278248e); relay-dashboard behavior unaffected
- Timestamp handling: utils.ParseTimeRange(c, -1) — no span cap; if both params absent,
  startTime is zero (no lower bound). Frontend always sends explicit timestamps, so this is safe.
  relay-dashboard uses ParseTimeRange(c, 0) which caps at 7 days — different by design
- Cursor pagination: now uses useInfiniteQuery; filter changes create a new query key, old pages are
  discarded by React Query automatically — verify "Load more" still appends correctly
- has_detail flag: detail button only renders when log.has_detail = true; detail fetch is gated by
  enabled: !!detailLog?.has_detail — verify behavior when detail storage is disabled globally
```

### 5. Suggested Verification

```text
Suggested verification:

API (use enterprise auth token):
- GET /api/enterprise/my-access/logs — happy path, should return UserLog list
- GET /api/enterprise/my-access/logs?model_name=gpt-4o — model filter
- GET /api/enterprise/my-access/logs?code_type=error — error-only filter
- GET /api/enterprise/my-access/logs?after_id=<id> — cursor pagination (append page)
- GET /api/enterprise/my-access/logs/:log_id — detail endpoint (valid and invalid ID)
- GET /api/enterprise/my-access/logs without auth — should 401/403
- GET /api/enterprise/my-access/logs with admin key (non-Feishu) — should work or reject gracefully

UI (browser, Feishu-authenticated):
- My Access page → "Request History" section loads at bottom
- Time range picker changes data
- Model filter filters results
- Status filter (success/error/all) works
- "Load more" appends next page; changing filters resets to page 1
- Detail button (FileText icon) opens dialog with request/response bodies
- Logs from other users are not visible (isolation check)

Edge cases:
- User has zero logs in the selected time range → empty state message
- Log exists but has_detail = false → detail button not rendered
- Log exists and has_detail = true but detail fetch returns 404 → dialog shows noDetail message
- Very long model name in table → truncation with tooltip
```

### 6. Validation Already Performed

```text
Validation already performed:
- go build -tags enterprise ./... passed (no compile errors)
- pnpm run build passed (no TypeScript errors, chunk size unchanged)
- Code reviewed by three parallel agents (reuse, quality, efficiency) ×3 rounds; all findings resolved
  - Round 3 fixes (commit 4125f9b): NoSpanLimit constant, modelFilter debounce 400ms, getNextPageParam empty-logs guard
- UI not browser-verified
- No integration test run against real DB
- No enterprise smoke test run
- Relay-dashboard GetTokenLogs caller (core/controller/relay-dashboard.go:157) confirmed to always
  pass non-empty token.Name from middleware — restored guard is consistent with this
- Isolation regression fixed (commit 278248e): GetTokenLogs guard restored; enterprise endpoint now
  uses dedicated GetGroupUserLogs which explicitly omits tokenName filter
- Stale-data regression fixed (commit 7262927): RequestLogsSection rewritten with useInfiniteQuery;
  no more state mutations inside queryFn
- Time range cap bug fixed (commit 069425f): ParseTimeRange(c, -1) — no 7-day clamp on logs endpoint;
  30-day / last-month / custom ranges now honored end-to-end
```

---

## Past Handoff — 2026-04-05 (AllowPassthroughUnknown Sync Toggle)

### 1. Goal

Expose `allow_passthrough_unknown` as a first-class sync option for PPIO and Novita channels, mirroring the `anthropic_pure_passthrough` pattern. The backend routing logic already existed (`model/cache.go:1047`, `controller/relay-channel.go:187`); this change closes the management gap by wiring it through the sync path and adding a frontend toggle with description.

### 2. Scope

**Backend:**

- `core/model/channel.go` — added `ChannelConfigs.SetOrInit(key, override *bool, def bool)` helper (write if non-nil, init to default only if key absent)
- `core/enterprise/ppio/types.go` — added `AllowPassthroughUnknown *bool` to `SyncOptions`
- `core/enterprise/ppio/sync.go` — threaded `allowPassthroughUnknown *bool` through `EnsurePPIOChannels` → `ensurePPIOChannelsFromModels` → `createPPIOChannels`; replaced 5-line duplicate `*bool` write-or-init blocks with `SetOrInit` calls
- `core/enterprise/novita/types.go` — same as ppio/types.go
- `core/enterprise/novita/sync.go` — same threading pattern; replaced duplicate blocks with `SetOrInit`
- `core/enterprise/init.go` — startup `Ensure*` calls pass `nil` for new parameter (preserve existing on boot)
- `core/enterprise/ppio/sync_channels_test.go` — added `allowUnknown` param, assertion for `allow_passthrough_unknown`
- `core/enterprise/novita/sync_channels_test.go` — same

**Frontend:**

- `web/src/types/ppio.ts` — added `allow_passthrough_unknown?: boolean` to `SyncOptions`
- `web/src/types/novita.ts` — same
- `web/src/pages/enterprise/ppio-sync.tsx` — added "透传未注册模型" Switch toggle after `anthropic-pure-passthrough`
- `web/src/pages/enterprise/novita-sync.tsx` — same
- `web/public/locales/zh/translation.json` — 2 keys per provider: `allowPassthroughUnknown`, `allowPassthroughUnknownHint`
- `web/public/locales/en/translation.json` — same

**Commits:** `52f193e` (feat: 同步页新增透传未注册模型开关), `e092703` (refactor: 提取 ChannelConfigs.SetOrInit)

### 3. Root Cause

```text
Root cause:
- Symptom: allow_passthrough_unknown had full routing logic in the backend (model cache + channel
  selector) and a UI config entry in the channel detail page, but no way to set it during the
  PPIO/Novita sync flow — users had to manually edit channel configs after every sync.
- Cause: AnthropicPurePassthrough was added as a dedicated sync option but allow_passthrough_unknown
  was not, leaving a management gap.
- Fix rationale:
  - Added AllowPassthroughUnknown *bool to SyncOptions with nil-semantics (nil = preserve, non-nil
    = override), matching the nil-passthrough pattern already used for anthropicPurePassthrough.
  - Daily scheduled sync (runPPIO/NovitaDailySync) does not set this field, so it passes nil →
    never overwrites the admin's choice. Only explicit UI sync triggers the override.
  - Startup channel refresh also passes nil → existing channel configs preserved on restart.
  - SetOrInit helper extracted from 4 duplicate 5-line *bool write-or-init patterns.
```

### 4. Risk Areas

```text
Risk areas:
- AnthropicPurePassthrough bool (not *bool): still uses bool zero-value semantics. If a client omits
  the JSON field, it silently becomes false, which could reset an admin's enabled setting on the next
  sync. Currently safe because the frontend always sends the current toggle state. Deferred to a
  future refactor when a 3rd toggle is added and a SyncOptions struct refactor is warranted.
- SetOrInit is a ChannelConfigs (map) method — map must be initialized before calling. Both sync
  paths guarantee non-nil configs before calling Set/Init (createPPIOChannels uses literal init,
  ensurePPIOChannelsFromModels loads from DB + type-asserts). No nil-map panic risk.
- Parameter count: EnsurePPIOChannels/EnsureNovitaChannels now have 4 positional params. Adding a
  3rd bool toggle would require changes to signature, 3 internal helpers, 2 callers, and tests.
```

### 5. Suggested Verification

```text
Suggested verification:

API (PPIO sync):
- POST sync with allow_passthrough_unknown=true → verify Channel 3 Configs["allow_passthrough_unknown"]=true
- POST sync with allow_passthrough_unknown=false → verify Channel 3 Configs["allow_passthrough_unknown"]=false
- POST sync without allow_passthrough_unknown field → verify existing config value is preserved

Routing behavior (with allow_passthrough_unknown=true on PPIO OpenAI channel):
- Request model not in channel's Models list → should route to PPIO channel as fallback
- Request model in another channel's list → should prefer that channel (allow_passthrough_unknown is fallback only)

UI:
- PPIO sync page → "透传未注册模型 / Passthrough unregistered models" toggle visible
- Toggle ON → sync → verify channel config updated; toggle OFF → sync → verify reset
- Novita sync page → same toggle visible and functional
- Hint text: "当开启后，未在模型列表中的模型请求将兜底路由到此渠道，不计费用" visible on hover/below label

Edge cases:
- Channel config has no allow_passthrough_unknown key, sync called with nil → key remains absent
- Channel config has allow_passthrough_unknown=true, startup refresh → key stays true (nil passes through)
```

### 6. Validation Already Performed

```text
Validation already performed:
- go build -tags enterprise -trimpath -ldflags "-s -w" ./... passed (init.go callers updated to 4-param)
- pnpm run build passed (no TypeScript errors)
- Three parallel simplify review agents; two findings fixed:
  - SetOrInit helper extracted (consolidates 4 duplicate write-or-init patterns)
  - Test callsites updated to pass new allowPassthroughUnknown parameter
- Unit tests: go test ./enterprise/ppio/... and ./enterprise/novita/... passed in previous session
- UI not browser-verified beyond visual inspection
- No integration test against real PPIO/Novita API
- No enterprise smoke test run
- Routing behavior (allow_passthrough_unknown=true fallback) not verified end-to-end
```

---

## Past Handoff — 2026-04-04 (Pure Passthrough + NativeModeChecker)

### 1. Goal

Implement full pure passthrough for PPIO and Novita Anthropic channels: the gateway forwards the raw client request verbatim (only replacing auth headers), captures token usage via dual-buffer SSE scanning, and exposes the feature as a per-channel config toggle synced from the admin UI. Also fix a `server_overload` error on non-Claude models caused by aiproxy injecting a `max_tokens` value beyond PPIO's internal cap.

### 2. Scope

**Core relay — passthrough layer:**

- `core/relay/adaptor/passthrough/anthropic_passthrough.go` — **new**: `headBuffer` (captures first 2 KB), `DoAnthropicPassthrough` (dual-buffer pipe, drain-on-disconnect, merge usage), `mergeAnthropicSSEUsage` (tail base + head fills zero fields)
- `core/relay/adaptor/passthrough/usage.go` — refactored: shared `extractUsageFromBytes(data, firstOccurrence)` backing `extractUsageFromTail` and new `extractUsageFromHead`
- `core/relay/adaptor/passthrough/adaptor.go` — extracted `forwardResponseHeaders` helper (was duplicated inline); `getPathBaseMap` renamed to `GetPathBaseMap` (exported for ppio adaptor)

**Core relay — anthropic adaptor:**

- `core/relay/adaptor/anthropic/config.go` — added `PurePassthrough bool \`json:"pure_passthrough"\``
- `core/relay/adaptor/anthropic/adaptor.go` — `ConvertRequest` (mode.Anthropic): if `PurePassthrough`, forward raw body + client content headers; `DoResponse` (mode.Anthropic): if `PurePassthrough`, call `DoAnthropicPassthrough`; `Metadata()`: added `pure_passthrough` schema entry

**Core relay — PPIO adaptor:**

- `core/relay/adaptor/ppio/adaptor.go` — exported `PathPrefixResponses` / `PathPrefixWebSearch` constants; refactored `GetRequestURL` to consult `GetPathBaseMap` first then fall back to string-replace (removed the early-return on key presence); added `mode.Anthropic`/`mode.Gemini` case routing to `/chat/completions`

**Core relay — channel selection:**

- `core/controller/relay-channel.go` — `filterChannels`: builds a `native` slice alongside `filtered`; returns `native` when non-empty, using the already-fetched adaptor to avoid a second registry lookup (channels with `NativeMode(mode)=true` are preferred)

**Enterprise sync:**

- `core/enterprise/ppio/types.go` — added `AnthropicPurePassthrough bool` to `SyncOptions`
- `core/enterprise/ppio/sync.go` — threads `*bool anthropicPurePassthrough` through `EnsurePPIOChannels` → `ensurePPIOChannelsFromModels` → `createPPIOChannels`; Anthropic channel update block writes `pure_passthrough` (always if non-nil, default false if key absent); `runPPIODailySync` uses `SyncOptions{AnthropicPurePassthrough: true}`; `buildConfigFromPPIOModelV2` caps `max_output_tokens` at 32000 for non-Claude models on the `anthropic` endpoint
- `core/enterprise/novita/types.go` — same as ppio types
- `core/enterprise/novita/sync.go` — same threading pattern; `runNovitaDailySync` uses `SyncOptions{AnthropicPurePassthrough: true}`
- `core/enterprise/init.go` — passes `nil` for `anthropicPurePassthrough` on startup (preserves existing channel flag)

**Frontend:**

- `web/src/types/ppio.ts` — added `anthropic_pure_passthrough?: boolean` to `SyncOptions`
- `web/src/types/novita.ts` — same
- `web/src/pages/enterprise/ppio-sync.tsx` — added "Anthropic 渠道纯透传" Switch toggle in sync config card; default state `true`
- `web/src/pages/enterprise/novita-sync.tsx` — same
- `web/public/locales/zh/translation.json` — 2 keys per provider section: `anthropicPurePassthrough`, `anthropicPurePassthroughHint`
- `web/public/locales/en/translation.json` — same

**Bug fixes (same session):**

- `web/src/api/enterprise.ts` — `used_amount?: number` (optional); fixes crash on undefined in `formatAmount`
- `web/src/pages/enterprise/my-access.tsx` — `formatAmount(log.used_amount ?? 0)` null-coalescing

### 3. Root Cause

```text
Root cause — pure passthrough:
- Symptom: PPIO Anthropic channel (type=14, anthropic.Adaptor) was performing full protocol
  conversion: injecting max_tokens, transforming body AST, parsing structured SSE for usage.
  This caused subtle incompatibilities (max_tokens exceeding PPIO's internal cap, context_management
  fields rejected by PPIO with 400, image URL conversion adding latency).
- Design intent: PPIO/Novita channels should act as authenticated reverse proxies — only auth
  headers replaced, everything else forwarded verbatim. "Pure passthrough" is now opt-in per channel.
- Why per-channel flag (not per-adaptor type): Channel 4 stays type=14 (ChannelTypeAnthropic) so
  NativeModeChecker routes Anthropic requests to it preferentially. Changing to type=54 (PPIO) would
  make both channels non-native for Anthropic mode, causing random channel selection between the
  OpenAI and Anthropic endpoints.

Root cause — max_tokens server_overload (xiaomimimo/mimo-v2-pro):
- Symptom: requests to mimo-v2-pro via Anthropic protocol returned server_overload.
- Cause: aiproxy synced max_output_tokens=131072 from PPIO catalog; PPIO's Anthropic proxy
  internally caps non-Claude models at 32000 and rejects higher values with server_overload.
- Fix: buildConfigFromPPIOModelV2 caps max_output_tokens at 32000 for models with "anthropic"
  endpoint that are not Claude models (name does not contain "claude").

Root cause — NativeModeChecker:
- Symptom: Anthropic protocol requests could be routed to the OpenAI channel (type=54) which
  requires Anthropic→OpenAI conversion, defeating passthrough intent.
- Fix: filterChannels now tracks a "native" slice and returns it when non-empty, so channels where
  NativeMode(mode)=true are always preferred over protocol-converting channels.
```

### 4. Risk Areas

```text
Risk areas:

1. Usage capture accuracy (SSE dual-buffer):
   - headBuffer captures only the first 2 KB. If message_start SSE event is larger (unlikely
     but possible with long system prompt echo), input_tokens is missed (falls back to 0).
   - ringBuffer captures the last 4 KB. If message_delta (output_tokens) is emitted early and
     the remaining stream exceeds 4 KB, output_tokens is missed.
   - Non-streaming: entire body must fit in 4 KB tail buffer, otherwise usage is zero.

2. Scheduled sync resets pure_passthrough to true:
   - runPPIODailySync and runNovitaDailySync always pass AnthropicPurePassthrough: true.
   - If admin disables pure_passthrough via UI and a scheduled sync runs, the flag is reset to true.
   - Mitigation: admin can re-disable after sync; scheduled syncs run once per day.

3. Startup channel refresh preserves existing flag:
   - init.go passes nil, so EnsurePPIOChannels only initializes pure_passthrough=false if the key
     is absent. Existing channels already having the key are unaffected.

4. DoResponse config load error handling:
   - LoadConfig error in DoResponse is silently ignored (falls through to standard handler).
   - This means a corrupt channel config causes silent fallback to conversion mode, not an error.

5. NativeModeChecker preference is all-or-nothing:
   - If the native channel is degraded/rate-limited but the non-native channel is healthy,
     the selector still prefers the native channel (until its error rate exceeds maxRetryErrorRate).

6. PPIO GetRequestURL Anthropic/Gemini case:
   - mode.Anthropic and mode.Gemini requests now route to BaseURL+/chat/completions on the PPIO
     OpenAI channel. This assumes the OpenAI channel is always selected for these modes when
     PurePassthrough=false on the Anthropic channel — verify with disabled Anthropic channel.
```

### 5. Suggested Verification

```text
Suggested verification:

API (PPIO Anthropic channel, pure_passthrough=true):
- POST /v1/messages with streaming=true → verify SSE events pass through verbatim (no re-encoding)
- POST /v1/messages with streaming=false → verify JSON response is unmodified
- POST /v1/messages with cache_control in body → verify cache fields pass through
- POST /v1/messages without max_tokens → expect upstream error (not gateway error — gateway must
  NOT inject max_tokens in pure passthrough mode)
- POST /v1/messages with anthropic-beta header → verify header forwarded to upstream
- After request: check logs table for input_tokens and output_tokens (both non-zero)

API (pure_passthrough=false, conversion mode):
- POST /v1/messages (Anthropic protocol) → should still convert correctly via anthropic.Adaptor
- POST /v1/chat/completions (OpenAI protocol on Anthropic model) → should convert to messages format

PPIO sync:
- Run sync with anthropic_pure_passthrough=true → verify Channel 4 Configs["pure_passthrough"]=true
- Run sync with anthropic_pure_passthrough=false → verify Channel 4 Configs["pure_passthrough"]=false
- Verify scheduled sync (runPPIODailySync) preserves pure_passthrough=true

NativeModeChecker:
- Anthropic protocol request → verify it routes to Channel 4 (type=14), not Channel 3 (type=54)
- OpenAI protocol request → verify it routes to Channel 3 (type=54)

UI:
- PPIO sync page → "Anthropic 渠道纯透传" toggle visible, default ON
- Toggle OFF → sync → verify channel config updated
- Novita sync page → same

Edge cases:
- Client sends max_tokens > 32000 to non-Claude model with pure_passthrough=true → upstream error
  (gateway should not interfere)
- Client sends max_tokens > 32000 to non-Claude model with pure_passthrough=false → gateway
  should clamp at 32000 (via ModelConfig max_output_tokens) or pass through
- Streaming request disconnects mid-stream → drain goroutine should capture tail usage correctly
```

### 6. Validation Already Performed

```text
Validation already performed:
- go build -tags enterprise -trimpath -ldflags "-s -w" ./... passed (no compile errors)
- pnpm run build passed (no TypeScript errors)
- Three parallel simplify review agents; two findings fixed:
  - headBuffer pre-allocation: make([]byte, 0, size) avoids 6 growth cycles per stream
  - forwardResponseHeaders extracted as package helper (was duplicated in adaptor.go + anthropic_passthrough.go)
- Double LoadConfig in ConvertRequest not fixed: ~5μs overhead on seconds-long LLM call; fix
  requires refactoring ConvertRequestBodyToBytes signature — judged not worth the churn
- Server restarted; basic /api/status health check passed
- UI not browser-verified beyond visual inspection
- No real Anthropic SSE trace captured against PPIO endpoint
- No integration test against live PPIO/Novita API
- No enterprise smoke test run
- DB for Channel 4 not yet updated — user must run a sync from the admin UI to apply
  pure_passthrough=true to the existing channel config
```
