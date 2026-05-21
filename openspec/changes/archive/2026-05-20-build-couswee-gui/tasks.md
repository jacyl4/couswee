## 1. Project Scaffold

- [x] 1.1 Initialize the Go module and create the backend directory structure for command entrypoint, account service, HTTP handlers, and static serving.
- [x] 1.2 Initialize the SvelteKit frontend structure with components for account list, switch button, and usage chart.
- [x] 1.3 Add the minimal required backend and frontend dependencies: GoFiber for the API and one charting library for the dashboard.
- [x] 1.4 Add developer commands or documentation for running the backend, building the frontend, and serving the built assets.

## 2. Account Registry

- [x] 2.1 Define the account model with `nickname`, `auth_path`, `subscription`, `5h_usage`, `weekly_usage`, and `active` fields matching the API JSON contract.
- [x] 2.2 Implement user-home path resolution for `~/.couswee/accounts.json` and `~/.codex/auth.json` using `os.UserHomeDir()`.
- [x] 2.3 Implement registry load behavior that returns an empty account list when the registry file is missing.
- [x] 2.4 Implement registry save behavior that persists account records and active-state changes to `~/.couswee/accounts.json`.
- [x] 2.5 Add tests for loading existing registry data, handling a missing registry, and preserving account fields during save/load.

## 3. Account Switching

- [x] 3.1 Implement active account lookup with a not-found result when no account is active.
- [x] 3.2 Implement switch-by-nickname lookup that rejects unknown nicknames without changing state.
- [x] 3.3 Implement safe auth replacement from the selected account `auth_path` to `~/.codex/auth.json` with error handling.
- [x] 3.4 Update active markers only after the auth file copy succeeds, ensuring exactly one active account after a successful switch.
- [x] 3.5 Add tests for successful switch, unknown nickname, unreadable auth source, and single-active-account behavior.

## 4. Usage Monitoring

- [x] 4.1 Implement a usage data service boundary that exposes cached 5-hour usage, weekly usage, and subscription values from account records.
- [x] 4.2 Add a periodic refresh loop using `time.Ticker` that updates usage cache state without blocking API requests.
- [x] 4.3 Persist refreshed usage values back to the account registry when they change.
- [x] 4.4 Add tests or a fake refresh source to verify periodic refresh updates cached values predictably.

## 5. Local API and Static Serving

- [x] 5.1 Implement `GET /api/accounts` returning all configured accounts as JSON.
- [x] 5.2 Implement `GET /api/current` returning the active account or a 404 response when no active account exists.
- [x] 5.3 Implement `POST /api/switch` accepting `{ "nickname": "..." }`, returning validation errors, not-found errors, or the switched account as JSON.
- [x] 5.4 Bind the server to `127.0.0.1:2199` by default.
- [x] 5.5 Serve the built SvelteKit static frontend at `/` and static asset paths from the GoFiber process.
- [x] 5.6 Add handler tests for the account list, current account, and switch endpoints.

## 6. Dashboard UI

- [x] 6.1 Build the evergreen/everforest dark theme with the documented colors for background, panels, buttons, text, and active-row highlighting.
- [x] 6.2 Implement Svelte account-list components to render nickname, subscription, 5-hour usage, weekly usage, active marker, and switch control for each account.
- [x] 6.3 Implement the Svelte switch action to call `POST /api/switch` and refresh visible account state after success.
- [x] 6.4 Implement a Svelte/CSS usage comparison view to compare usage across accounts without retaining the Astro-era chart dependency.
- [x] 6.5 Add periodic frontend refresh of `/api/accounts` and update the list/chart without a full page reload.
- [x] 6.6 Add loading and error states for API failures and switch failures.

## 7. Verification

- [x] 7.1 Run Go formatting and backend unit tests.
- [x] 7.2 Run the frontend build and fix any TypeScript, SvelteKit, or bundling errors.
- [x] 7.3 Run a local smoke test that starts the service and verifies `/`, `/api/accounts`, `/api/current`, and `/api/switch` behavior with sample data.
- [x] 7.4 Re-run `openspec validate build-couswee-gui --strict` and resolve any OpenSpec validation issues.

## 8. SvelteKit Frontend Replacement

- [x] 8.1 Remove Astro configuration, generated metadata, `.astro` source files, and Astro/ApexCharts dependencies.
- [x] 8.2 Add SvelteKit static-adapter configuration and package scripts that build to `web/dist`.
- [x] 8.3 Reimplement the dashboard as a fresh SvelteKit route using Svelte state/lifecycle patterns against the existing backend APIs.
- [x] 8.4 Re-run frontend build, Go tests, smoke checks, and OpenSpec validation after the stack replacement.
