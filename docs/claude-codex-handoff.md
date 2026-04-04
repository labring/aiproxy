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

## Active Handoff — 2026-04-04

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

**Commits:** `7a00a11`, `d6114ac`, `393238d`, `278248e`

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
- Timestamp handling: utils.ParseTimeRange defaults to last 7 days when params absent;
  the relay-dashboard endpoint does NOT have this default — behavior is intentionally different
- Cursor pagination: afterId is managed in frontend state + useEffect; filter changes must correctly
  reset the cursor and discard stale accumulated logs
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
- Code reviewed by three parallel agents (reuse, quality, efficiency); all findings resolved
- UI not browser-verified
- No integration test run against real DB
- No enterprise smoke test run
- Relay-dashboard GetTokenLogs caller (core/controller/relay-dashboard.go:157) confirmed to always
  pass non-empty token.Name from middleware — restored guard is consistent with this
- Isolation regression fixed (commit 278248e): GetTokenLogs guard restored; enterprise endpoint now
  uses dedicated GetGroupUserLogs which explicitly omits tokenName filter
```
