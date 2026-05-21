## 1. Usage Model and Configuration

- [x] 1.1 Define a `UsageRecord` model with `account`, `5h_usage`, `weekly_usage`, `reset_time`, `unit`, `source`, `last_refresh`, `stale`, and `error` JSON fields.
- [x] 1.2 Add usage configuration for refresh interval, unit selection, API collector enablement, and optional fallback command/path settings.
- [x] 1.3 Implement refresh interval clamping so configured values below 1 minute become 1 minute and values above 5 minutes become 5 minutes.
- [x] 1.4 Add tests for usage model serialization and refresh interval clamping.

## 2. Collector Pipeline

- [x] 2.1 Create a collector interface that accepts configured accounts and returns normalized usage records.
- [x] 2.2 Implement an API collector path for retrieving recent 5-hour usage, weekly usage, and reset time when API credentials/configuration are available.
- [x] 2.3 Implement a local fallback collector path that can parse `openusage.sh` or `abtop`-style structured output or local cache data.
- [x] 2.4 Implement collector orchestration that tries API collection first and falls back to local parsing per account when API collection fails.
- [x] 2.5 Add fixture-based tests for successful API collection, API failure with fallback success, and all-collectors-failed behavior.

## 3. Usage Cache and Refresh Service

- [x] 3.1 Implement a thread-safe in-memory cache that stores the latest usage record per account and returns snapshots to callers.
- [x] 3.2 Implement asynchronous periodic refresh using `time.Ticker` and the clamped refresh interval.
- [x] 3.3 Preserve stale cached values and attach error metadata when refresh fails after a prior success.
- [x] 3.4 Ensure refresh failures for one account do not prevent other accounts from updating.
- [x] 3.5 Add tests for cache snapshot isolation, successful refresh replacement, stale preservation, and partial failure handling.

## 4. Local API Integration

- [x] 4.1 Wire the usage cache/refresh service into couswee startup without changing existing account switching behavior.
- [x] 4.2 Add `GET /api/codex/usage` returning cached usage records as a JSON array.
- [x] 4.3 Ensure the endpoint returns quickly from cache and does not run live collection inside the request handler.
- [x] 4.4 Add handler tests for empty usage, successful usage records, stale records, and response field shape.
- [x] 4.5 Keep the default local service address on `127.0.0.1:2199`.

## 5. Dashboard Usage Display

- [x] 5.1 Add usage rendering to the SvelteKit dashboard that fetches `/api/codex/usage`.
- [x] 5.2 Render a usage table with account, 5-hour usage, weekly usage, reset time, unit, source, and refresh status.
- [x] 5.3 Add progress/Gantt-style bars for 5-hour and weekly usage with green, yellow, and red state colors.
- [x] 5.4 Poll usage data automatically at an interval compatible with the backend refresh cadence and update without full page reload.
- [x] 5.5 Display stale and error indicators while retaining last known usage values.
- [x] 5.6 Match the existing dark evergreen/everforest couswee visual style.

## 6. Documentation and Operations

- [x] 6.1 Document the usage monitor configuration, including refresh interval, unit selection, and fallback command/path options.
- [x] 6.2 Document the `GET /api/codex/usage` response shape and example payload.
- [x] 6.3 Document how to validate local `openusage.sh` or `abtop` fallback integration before enabling it.
- [x] 6.4 Add troubleshooting notes for API failures, stale data, and missing usage records.

## 7. Verification

- [x] 7.1 Run Go formatting and backend unit tests.
- [x] 7.2 Run the frontend build and fix any SvelteKit/TypeScript/bundling errors.
- [x] 7.3 Run a local smoke test for `/api/codex/usage` with fixture or sample account data.
- [x] 7.4 Re-run `openspec validate improve-codex-usage-monitor --strict` and resolve any validation issues.


## 8. Integrated Account List Correction

- [x] 8.1 Update docs/specs so usage appears only in the existing account list, not a separate usage panel.
- [x] 8.2 Resolve active-account usage collection against the live `~/.codex/auth.json` file.
- [x] 8.3 Merge usage records into account-list rows and display 5h/weekly usage as percentages only.
- [x] 8.4 Add reset time, stale, and error indicators inline in the account list.
- [x] 8.5 Remove the separate Codex realtime usage panel from the frontend.
- [x] 8.6 Re-run Go tests, frontend build, smoke test, and OpenSpec validation.

## 9. Codex Auth + Usage Endpoint Collection

- [x] 9.1 Update proposal/design/specs to require the abtop-style Codex usage path: read local Codex auth, call a usage/rate-limit endpoint, and show percentage-only data.
- [x] 9.2 Implement safe `~/.codex/auth.json` access-token parsing without logging or exposing token values.
- [x] 9.3 Update API collection to send the Codex access token as a Bearer token and include account context in the request.
- [x] 9.4 Parse OpenAI/Codex rate-limit payloads with `five_hour.used_percentage`, `seven_day.used_percentage`, reset timestamps, and `updated_at` metadata.
- [x] 9.5 Add an abtop cache fallback for `~/.cache/abtop/codex-rate-limits.json` before falling back to registry values.
- [x] 9.6 Document endpoint/auth/cache configuration and re-run Go tests, frontend build, smoke test, and OpenSpec validation.

## 10. CLI-Accurate Remaining Percent and Split Reset Times

- [x] 10.1 Update proposal/design/specs to distinguish CLI-style remaining percentage from used percentage.
- [x] 10.2 Add separate 5h and weekly reset-time fields to the usage API contract.
- [x] 10.3 Parse latest Codex CLI session `payload.rate_limits` events before stale abtop cache fallback.
- [x] 10.4 Convert Codex `used_percent` / `used_percentage` values into remaining percentages matching the CLI status line.
- [x] 10.5 Update account-list UI to show 5h remaining, weekly remaining, 5h reset, and weekly reset as separate columns.
- [x] 10.6 Re-run Go tests, frontend build, smoke test, and OpenSpec validation.


## 11. Remaining Traffic Naming

- [x] 11.1 Rename user-facing 5h/weekly labels to remaining traffic.
- [x] 11.2 Add explicit `5h_remaining` and `weekly_remaining` API fields while keeping legacy usage fields compatible.
- [x] 11.3 Re-run validation after remaining-traffic naming changes.

## 12. SvelteKit Usage Dashboard Replacement

- [x] 12.1 Update usage-dashboard docs/specs so the frontend target is SvelteKit and not Astro.
- [x] 12.2 Reimplement account/usage merge, polling, stale/error display, reset columns, and remaining-traffic bars in SvelteKit state.
- [x] 12.3 Remove the Astro-era chart dependency from the usage display.
- [x] 12.4 Re-run frontend build, Go tests, smoke checks, and OpenSpec validation.
