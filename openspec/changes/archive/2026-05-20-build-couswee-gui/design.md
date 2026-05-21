## Context

couswee is a new local desktop/web utility for users who operate multiple Codex accounts. The current project has an initial design document but no implementation. The tool must combine a Go backend, a browser-based dashboard, local JSON persistence, usage visualization, and security-sensitive switching of `~/.codex/auth.json`.

The first implementation should be intentionally local-only: it serves on `127.0.0.1:2199`, stores account metadata under the user's home directory, and avoids external services unless later requirements add them.

## Goals / Non-Goals

**Goals:**
- Provide a GoFiber backend that owns account loading, usage cache state, active-account state, account switching, and API responses.
- Provide a SvelteKit frontend with an evergreen/everforest dark theme, account list, active-account highlighting, switch buttons, and usage visualization.
- Persist account records in `~/.couswee/accounts.json` so state survives service restarts.
- Switch accounts by safely copying a selected configured auth file to `~/.codex/auth.json`.
- Keep the first version simple enough to run locally with `go run` plus a SvelteKit static build output served by GoFiber.

**Non-Goals:**
- No cloud synchronization or remote multi-user access.
- No automatic discovery of all Codex accounts unless added by a later change.
- No background daemon installation, tray integration, or system service packaging in this change.
- No extra Rime/desktop/system package work; this project scope is couswee only.

## Decisions

### Decision: GoFiber owns local API and static serving
Use GoFiber for `GET /api/accounts`, `GET /api/current`, `POST /api/switch`, and static serving of the SvelteKit static adapter build output.

Rationale: the design document already targets Go + GoFiber, and Go gives a simple single-binary backend for local file operations. Serving the built frontend from the same process keeps local deployment simple.

Alternative considered: run Astro dev/server separately in production. Rejected because the first version should minimize runtime moving parts.

### Decision: JSON registry under `~/.couswee/accounts.json`
Persist account records in a user-local JSON file with fields matching the API model: `nickname`, `auth_path`, `subscription`, `5h_usage`, `weekly_usage`, and `active`.

Rationale: the data model is small, user-local, and explicitly described in the design document. JSON is inspectable and easy to back up.

Alternative considered: SQLite. Rejected for the initial version because it adds schema/migration overhead before the data model needs relational queries.

### Decision: Switch by validated file copy
Implement switching as a backend operation that resolves the selected account, validates that `auth_path` is readable, copies it to `~/.codex/auth.json`, and only then updates in-memory and persisted active markers.

Rationale: switching modifies an authentication file. The operation must fail safely and avoid marking an account active if the copy failed.

Alternative considered: update active marker first and copy afterward. Rejected because it can leave UI state inconsistent with the actual Codex auth file.

### Decision: SvelteKit frontend consumes JSON API
Build the UI as a SvelteKit static application that fetches from the backend APIs, keeps account/usage data in component state, and periodically refreshes account data without a full page reload.

Rationale: SvelteKit fits the desired interactive dashboard better than Astro for this project because the core screen is stateful, polling-driven, and mostly client-side after the initial shell renders. Keeping the API boundary unchanged separates UI rendering from filesystem-sensitive backend operations.

Alternative considered: translate the existing Astro components to Svelte one-for-one. Rejected because the goal is a fresh SvelteKit implementation that follows Svelte state/lifecycle patterns rather than preserving Astro-era file boundaries.

### Decision: Use native Svelte/CSS visualization before chart libraries
Implement the Gantt-style account usage view with Svelte markup and CSS bars unless requirements later demand a heavier charting library.

Rationale: the current usage comparison is percentage-based and can be rendered clearly without an additional chart dependency. This keeps the SvelteKit stack small and removes the Astro-era ApexCharts dependency.

Alternative considered: keep ApexCharts from the Astro implementation. Rejected because it is unnecessary for the current bar comparison and conflicts with the cleanup goal of removing the old frontend stack.

## Risks / Trade-offs

- [Risk] Copying auth files can corrupt or overwrite the active Codex auth file if implemented carelessly. → Mitigation: validate source readability, write/copy with error handling, and update active state only after successful copy.
- [Risk] `~` expansion differs between shell examples and Go filesystem code. → Mitigation: use `os.UserHomeDir()` and explicit path joining rather than `os.ExpandEnv("~...")`.
- [Risk] Usage metrics may not have a real Codex source in the first implementation. → Mitigation: treat registry values as cache fields initially and isolate refresh logic behind a service boundary for later improvement.
- [Risk] Local HTTP service could be exposed unintentionally if bound to all interfaces. → Mitigation: default to `127.0.0.1:2199`, not `:2199`.
- [Risk] Frontend and backend models can drift. → Mitigation: keep a single JSON account shape and test API responses against expected fields.

## Migration Plan

1. Scaffold the Go backend and account model.
2. Implement JSON registry load/save and safe home-directory path resolution.
3. Implement API endpoints and static file serving.
4. Remove Astro config, dependencies, generated metadata, and component files.
5. Scaffold SvelteKit routes/components/styles with the evergreen/everforest theme.
6. Add account list, switch action, polling, and native usage comparison rendering.
7. Add targeted tests for registry loading, switch failure behavior, and API handlers.
8. Validate by running backend tests, frontend build, and a local smoke test.

### Revision: Frontend stack replacement
The completed Astro implementation is deprecated. The current implementation pass must remove Astro entirely and introduce SvelteKit as a fresh frontend, keeping only the backend API contract, theme intent, and user-facing behavior.

Rollback is simple before release: stop the local service and remove generated build artifacts. The implementation must not delete account auth source files.

## Open Questions

- What real source should populate `5h_usage` and `weekly_usage` beyond cached/manual values?
- Should account creation/editing be included in the first UI, or should the first version read manually edited `accounts.json` only?
- Should switching create a timestamped backup of the previous `~/.codex/auth.json` before replacing it?
