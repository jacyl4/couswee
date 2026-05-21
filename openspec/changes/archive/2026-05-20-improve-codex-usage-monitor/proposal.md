## Why

couswee currently has account switching and cached/manual usage fields, but it does not yet collect or display live Codex usage in a way users can trust for account rotation decisions. This change turns usage monitoring into a first-class capability by collecting recent and weekly usage, exposing reset timing, and refreshing the dashboard automatically.

## What Changes

- Add live Codex usage collection for configured accounts, including recent 5-hour usage, weekly usage, and reset time.
- Add a backend usage collection layer that can use an OpenAI/Codex usage API strategy first and fall back to local `openusage.sh`/`abtop`-style parsing when API collection fails.
- Add an in-memory thread-safe usage cache refreshed every 1 to 5 minutes.
- Add `GET /api/codex/usage` for frontend consumption.
- Integrate usage percentages and reset time directly into the existing account list; do not create a separate usage-only panel.
- Preserve the existing local-only couswee deployment model on port `2199`.
- No breaking changes to account switching or the existing `/api/accounts` contract.

## Capabilities

### New Capabilities
- `codex-usage-collection`: Collect live Codex usage and reset timing for each configured account from API or local fallback sources.
- `codex-usage-cache`: Maintain a thread-safe refreshed cache of account usage data for fast API responses and failure tolerance.
- `codex-usage-api`: Expose live usage records through a local REST API endpoint for the frontend.
- `codex-usage-dashboard`: Display live usage, reset times, and usage capacity states in the couswee dashboard with automatic refresh.

### Modified Capabilities

None. The existing project specs have not been archived into `openspec/specs/` yet, so this change introduces focused usage-monitor capabilities rather than modifying archived requirements.

## Impact

- Backend account and usage services gain a collector interface, API collector strategy, local fallback collector strategy, cache, and refresh loop.
- API surface gains `GET /api/codex/usage`.
- Frontend account list gains percentage-only 5h/weekly usage display, reset time, stale/error status, and polling refresh.
- Configuration may gain refresh interval and usage-unit selection controls, defaulting to safe local values.
- Tests must cover successful collection, API fallback, cache refresh, API response shape, and frontend build behavior.


### Decision: Resolve active account usage against the live auth file
When an account is active, collectors use the live `~/.codex/auth.json` path rather than a stale backup path, and the backend reconciles active-account markers against the current auth file when possible.

Rationale: usage collection for the currently active account must reflect the actual credentials Codex is using now.


### Revision: Codex auth + usage endpoint source
The live collector now explicitly follows the abtop-style approach: read each account's local Codex auth file, use `tokens.access_token` as a Bearer token for a configured usage/rate-limit endpoint, parse percentage-based 5h/weekly usage and reset timestamps, and fall back to the local abtop cache before registry values. No model inference request is used for usage measurement.


### Revision: CLI-accurate remaining percentages and split resets
The dashboard now treats Codex rate-limit percentages as CLI-style remaining capacity (`69% left`), adds separate 5h and weekly reset-time fields, and uses the latest Codex CLI session `payload.rate_limits` event before stale abtop cache fallback.


### Revision: Remaining traffic naming
The account list and API now use explicit remaining-traffic semantics. New response fields `5h_remaining` and `weekly_remaining` carry the values shown in the UI, while legacy `5h_usage` and `weekly_usage` remain for compatibility and are marked by `usage_basis: remaining`.

### Revision: SvelteKit dashboard implementation
The frontend usage display now targets SvelteKit rather than Astro. Usage records remain exposed by `GET /api/codex/usage`, but the dashboard merge/poll/render logic must be implemented as Svelte stateful UI and native CSS usage bars, not as carried-over Astro components or an Astro chart integration.
