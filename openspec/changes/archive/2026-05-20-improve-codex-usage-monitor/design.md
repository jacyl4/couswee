## Context

couswee already has a local GoFiber backend, Astro dashboard, account registry, and account-switching workflow. The initial usage fields are cached/manual fields on account records, which is not sufficient for real account rotation because users need live 5-hour and weekly usage plus reset timing.

The source document `couswee_codex_usage.md` calls out two reference directions: API-based collection similar to `openusage.sh`, and local/log/cache parsing similar to `abtop`. This design keeps those as interchangeable collectors behind one backend interface so the UI and API are stable even if the collection mechanism evolves.

## Goals / Non-Goals

**Goals:**
- Collect per-account Codex 5-hour usage, weekly usage, and reset time.
- Prefer API-based collection and fall back to local parsing when API collection fails.
- Cache usage in memory with thread-safe reads for API requests.
- Refresh usage automatically every 1 to 5 minutes, defaulting to 5 minutes.
- Expose `GET /api/codex/usage` for the dashboard.
- Display usage directly in the account list as percentage-only 5h/weekly values plus reset time and clear stale/error states.
- Preserve the local-only couswee deployment model on `127.0.0.1:2199`.

**Non-Goals:**
- No WebSocket push in this change; polling is sufficient for 1 to 5 minute refresh.
- No paid-plan inference or billing reconciliation beyond values exposed by collectors.
- No public remote service or multi-user access.
- No destructive changes to account switching or `~/.codex/auth.json` handling.

## Decisions

### Decision: Introduce a collector interface
Create a backend interface that accepts configured accounts and returns normalized usage records. Implement collectors behind this interface, starting with an API collector and a local fallback collector.

Rationale: the exact reliable source of Codex usage may change. A collector boundary lets the implementation start with available scripts/log parsing while keeping the API response stable.

Alternative considered: put shell/API calls directly in HTTP handlers. Rejected because API requests should be fast and should not block on external commands or network calls.

### Decision: API collector first, local fallback second
The refresh pipeline tries the API collector first when account credentials/configuration allow it, then falls back to `openusage.sh`/`abtop`-style local output parsing.

Rationale: API data is expected to be most authoritative, while local parsing provides resilience when network/API calls fail.

Alternative considered: use only local log parsing. Rejected because it can miss server-side usage or reset information.

### Decision: Cache-first HTTP endpoint
`GET /api/codex/usage` returns cached usage records. Refresh cycles update the cache asynchronously with a goroutine and `time.Ticker`.

Rationale: the dashboard should remain responsive, and one slow account collection should not make every frontend request slow.

Alternative considered: collect live usage on every HTTP request. Rejected due to latency, rate-limit, and failure-coupling risks.

### Decision: Clamp refresh interval to 1-5 minutes
Use a configurable interval but clamp it between 1 and 5 minutes, defaulting to 5 minutes.

Rationale: refresh faster than 1 minute risks unnecessary API pressure, while slower than 5 minutes weakens the "real-time" promise in the source document.

Alternative considered: hard-code 5 minutes only. Rejected because users may want more responsive local monitoring while staying within safe bounds.

### Decision: Normalize response records
Expose one usage record shape: `account`, `5h_usage`, `weekly_usage`, `reset_time`, `unit`, `source`, `last_refresh`, `stale`, and `error`; the frontend renders `5h_usage` and `weekly_usage` as percentages only.

Rationale: frontend rendering and tests need a stable contract regardless of whether data came from API, `openusage.sh`, or `abtop`.

Alternative considered: expose source-specific raw payloads. Rejected because that would leak implementation details and complicate the UI.

### Decision: Frontend polls the backend
The SvelteKit dashboard polls `/api/codex/usage` and merges usage records into the existing account list without a full page reload.

Rationale: polling is simple, robust, and consistent with the requested 1-5 minute refresh cadence. Svelte component state and lifecycle hooks are a natural fit for this behavior.

Alternative considered: WebSocket push. Deferred as a later optimization because it adds lifecycle complexity without being required for minute-level updates.

## Risks / Trade-offs

- [Risk] The exact Codex usage API or local log format may differ from assumptions. → Mitigation: isolate parsing behind collector implementations and add fixture-based tests for each supported source.
- [Risk] API calls may fail due to network, credentials, or rate limits. → Mitigation: fallback to local parsing and preserve stale cached values with error metadata.
- [Risk] Shelling out to scripts can be unsafe or brittle. → Mitigation: make script paths explicit/configurable, use timeouts, avoid shell interpolation, and parse structured output where possible.
- [Risk] Refresh goroutines could race with API handlers. → Mitigation: use a mutex or atomic cache replacement and return snapshots to callers.
- [Risk] Usage units can confuse users if tokens and USD are mixed. → Mitigation: normalize to one configured unit and include `unit` on every record.
- [Risk] A large charting bundle can slow the dashboard. → Mitigation: avoid a chart dependency for the current percentage comparison and render native Svelte/CSS bars.

## Migration Plan

1. Add usage record models and collector/cache service types.
2. Implement API collector skeleton and local fallback parser with test fixtures.
3. Wire periodic refresh into service startup with interval clamping.
4. Add `GET /api/codex/usage` returning cache snapshots.
5. Extend the existing account list with percentage-only usage columns, reset time, and stale/error indicators.
6. Add tests for collectors, fallback, cache refresh, endpoint shape, and frontend build.
7. Validate with `go test ./...`, `npm run build`, and `openspec validate improve-codex-usage-monitor --strict`.

Rollback is low-risk: remove or disable the usage refresh service and endpoint. Existing account switching and account registry behavior should remain unchanged.

## Open Questions

- What exact local paths and output formats should be supported for `openusage.sh` and `abtop` on this machine?
- Which authoritative reset-time source should be used when API and local fallback disagree?
- Should the UI expose usage in tokens only for the MVP, or add a user-facing tokens/USD selector immediately?


### Decision: Resolve active account usage against the live auth file
When an account is active, collectors use the live `~/.codex/auth.json` path rather than a stale backup path, and the backend reconciles active-account markers against the current auth file when possible.

Rationale: usage collection for the currently active account must reflect the actual credentials Codex is using now.


### Decision: Read Codex auth and call usage/rate-limit endpoints
Usage collection follows the abtop-style principle requested for couswee: read the local Codex auth JSON for the account, extract `tokens.access_token`, and call a configured OpenAI/Codex usage or rate-limit endpoint with `Authorization: Bearer <access token>`. This request only asks the service for metering data; it does not invoke a model and should not consume model quota.

The collector normalizes rate-limit payloads such as `five_hour.used_percentage`, `seven_day.used_percentage`, `resets_at`, and `updated_at` into the stable `UsageRecord` contract. If live endpoint collection is unavailable, couswee reads the local abtop cache at `~/.cache/abtop/codex-rate-limits.json` before using registry percentages.

Rationale: the current active Codex account must be measured from the credentials Codex actually uses, not by synthetic model calls or stale manual fields.


### Decision: Match Codex CLI remaining-percent semantics
The account list displays Codex quota the same way the CLI status line does: as remaining percentage, not consumed percentage. For rate-limit payloads where Codex reports `used_percent` or `used_percentage`, couswee converts the value to `100 - used` before displaying it. The API records this with `usage_basis: remaining`.

### Decision: Keep 5h and weekly reset times separate
The normalized usage record now includes `5h_reset_time` and `weekly_reset_time` in addition to the legacy `reset_time` compatibility field. The frontend renders separate reset columns because the 5-hour and weekly windows reset independently.

### Decision: Prefer latest Codex CLI session rate-limit event over stale abtop cache
When a live usage endpoint is unavailable, couswee reads the latest `payload.rate_limits` event from `~/.codex/sessions/**/*.jsonl` before the abtop cache file. This matches the CLI status line more closely than a stale cache file.


### Revision: Remaining traffic naming
The account list and API now use explicit remaining-traffic semantics. New response fields `5h_remaining` and `weekly_remaining` carry the values shown in the UI, while legacy `5h_usage` and `weekly_usage` remain for compatibility and are marked by `usage_basis: remaining`.

### Revision: SvelteKit dashboard implementation
The usage dashboard is now implemented in SvelteKit. The frontend should keep `/api/accounts` and `/api/codex/usage` as separate backend fetches, merge them client-side by account nickname, and render remaining percentages, reset times, stale states, and errors in a single account list plus lightweight comparison bars.
