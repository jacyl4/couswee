## Why

couswee will make it easier to manage multiple Codex accounts from one local desktop tool instead of manually inspecting usage data and replacing `~/.codex/auth.json`. The project is starting from a design document now, so the first change should establish the user-facing capabilities, local API contract, and implementation task plan before code is added.

## What Changes

- Add a local couswee desktop/web GUI served by a GoFiber backend on `127.0.0.1:2199`.
- Add account registry management backed by `~/.couswee/accounts.json`.
- Add account usage and subscription display for each configured Codex account.
- Add current-account detection and a guarded account-switch operation that replaces `~/.codex/auth.json` from a configured auth file.
- Add a dark evergreen/everforest themed SvelteKit frontend with account list, current-account highlight, switch controls, and usage visualization.
- Add REST APIs for listing accounts, reading the current account, and switching accounts.
- No breaking changes; this is a new project capability set.

## Capabilities

### New Capabilities
- `account-registry`: Manage configured Codex account metadata and persisted local account data.
- `account-switching`: Detect and switch the active Codex account by updating `~/.codex/auth.json` from a selected account auth file.
- `usage-monitoring`: Track, cache, and expose 5-hour usage, weekly usage, and subscription information for configured accounts.
- `web-dashboard`: Provide the local SvelteKit dashboard, account list, switch controls, and usage/Gantt-style visualization.
- `local-api`: Provide the GoFiber HTTP API and static frontend serving surface on `127.0.0.1:2199`.

### Modified Capabilities

None. There are no existing OpenSpec capabilities in this project yet.

## Impact

- New Go backend using GoFiber for API and static file serving.
- New SvelteKit frontend implemented as a fresh client application that consumes the GoFiber APIs and uses lightweight native/CSS visualization rather than carrying over the Astro component structure.
- Local filesystem state under `~/.couswee/accounts.json` and `~/.codex/auth.json`.
- Security-sensitive file-copy behavior for Codex authentication files; implementation must validate account selection and handle read/write failures safely.
- Developer commands for running the backend and building/serving the frontend.

### Revision: Replace Astro with SvelteKit
The frontend technology stack is now SvelteKit. This is not an Astro-to-Svelte file translation: the dashboard is reimplemented around SvelteKit component state, lifecycle hooks, and static adapter output while preserving the backend API contract and local GoFiber deployment model. Astro config, generated metadata, component files, and dependencies must be removed.
