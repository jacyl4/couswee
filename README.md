# couswee

couswee is a local GUI for monitoring multiple Codex accounts and switching the active account by replacing `~/.codex/auth.json` from a configured auth file.

## Requirements

- Go available at `/usr/local/go/bin/go` or on `PATH`
- Node.js and npm

## Account database

couswee stores account metadata only in SQLite at `~/.couswee/couswee.db`. The service creates the database and schema automatically on startup.

Add accounts from the web UI with one of these flows:

- Web login
- Device-code login
- Manual auth-file import

Manual import still records metadata in SQLite; it does not create or update any old registry file.

## Development

Install frontend dependencies:

```bash
npm install
```

Build the SvelteKit frontend:

```bash
npm run build
```

Run backend tests:

```bash
/usr/local/go/bin/go test ./...
```

Run the local service:

```bash
COUSWEE_STATIC_DIR=web/dist /usr/local/go/bin/go run ./cmd/couswee
```

Open <http://127.0.0.1:2199>.

## API

- `GET /api/accounts` returns all SQLite-backed accounts.
- `POST /api/accounts` manually imports an account auth path into SQLite.
- `PATCH /api/accounts/:id` edits non-secret account metadata.
- `DELETE /api/accounts` deletes accounts by id or nickname.
- `GET /api/current` returns the active account or 404.
- `POST /api/switch` with `{ "nickname": "Dev1" }` or `{ "id": "..." }` switches the active account.
- `POST /api/codex/login/start` starts a Codex device-code login session.
- `POST /api/codex/login/oauth/start` and `POST /api/codex/login/device/start` remain compatibility aliases.
- `GET /api/codex/login/:session_id` returns login status.
- `POST /api/codex/login/:session_id/cancel` cancels a login session.

## Codex usage monitor

couswee merges live/cached usage records into the existing account list and also exposes them at:

```bash
curl http://127.0.0.1:2199/api/codex/usage
```

Example response:

```json
[
  {
    "account": "Dev1",
    "5h_usage": 65,
    "weekly_usage": 42,
    "5h_remaining": 65,
    "weekly_remaining": 42,
    "reset_time": "2026-05-14T00:00:00+08:00",
    "5h_reset_time": "2026-05-14T00:00:00+08:00",
    "weekly_reset_time": "2026-05-17T14:55:35+08:00",
    "usage_basis": "remaining",
    "unit": "percent",
    "source": "abtop-cache",
    "last_refresh": "2026-05-13T15:00:00Z",
    "stale": false,
    "error": ""
  }
]
```

Configuration is via environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `COUSWEE_USAGE_REFRESH_INTERVAL` | `5m` | Usage refresh interval. Values are clamped to 1-5 minutes. |
| `COUSWEE_USAGE_UNIT` | `percent` | Collector unit label. The account list renders 5h/weekly remaining traffic values as percentages only. |
| `COUSWEE_USAGE_API_ENABLED` | `true` | Enable API collector. Set `false` to force local fallback. |
| `COUSWEE_USAGE_API_URL` | `https://chatgpt.com/backend-api/wham/usage` | OpenAI/Codex usage or rate-limit endpoint. couswee reads the account auth JSON, sends `Authorization: Bearer <tokens.access_token>`, and appends `account`, `auth_path`, and `account_id` query parameters. |
| `COUSWEE_USAGE_SESSION_GLOB` | empty | Optional diagnostic Codex CLI session log fallback for the account matching the live active auth file. Session events older than `last_used_at` are ignored. |
| `COUSWEE_USAGE_FALLBACK_CMD` | empty | Optional local fallback command, such as an `openusage.sh`/`abtop` wrapper. couswee appends account nickname and auth path as arguments. If set, it takes precedence over cache-file fallback. |
| `COUSWEE_USAGE_FALLBACK_TIMEOUT` | `20s` | Timeout for fallback command execution. |

Usage collection follows the abtop-style principle: it reads Codex local auth (`~/.codex/auth.json` for the active account, or the configured backup auth path for inactive accounts), uses `tokens.access_token` only as a Bearer credential for the ChatGPT usage endpoint, and parses returned usage/rate-limit data. For Codex rate-limit payloads, couswee converts `used_percent` / `used_percentage` to the same **remaining traffic percentage** shown by the CLI, such as `69% left`. It does **not** call a Codex model to measure usage, so the usage query itself is not a model-token inference request.

If no live API or command fallback succeeds, couswee uses explicitly enabled session-log fallback when permitted, then percentage values already present in the SQLite account record.

### Validating a fallback command

The fallback command should print a JSON usage record/array or an abtop-style rate-limit object. Minimal usage-record example:

```json
{"account":"Dev1","5h_usage":69,"weekly_usage":11,"5h_remaining":69,"weekly_remaining":11,"5h_reset_time":"2026-05-14T21:20:02+08:00","weekly_reset_time":"2026-05-17T14:55:35+08:00","usage_basis":"remaining","unit":"percent"}
```

Test the command manually before enabling it:

```bash
/path/to/openusage-wrapper Dev1 ~/.codex/auth-dev1.json
```

Then run couswee with:

```bash
COUSWEE_USAGE_FALLBACK_CMD=/path/to/openusage-wrapper npm run go:run
```

### Troubleshooting usage data

- Empty list: add an account from the web UI, or verify `~/.couswee/couswee.db` is writable and restart couswee.
- `stale: true`: the last refresh failed; check the `error` field for the account.
- API failures: verify `COUSWEE_USAGE_API_URL`, network access, and the selected auth file contains `tokens.access_token`. couswee never prints the token value.
- CLI/session fallback: verify `COUSWEE_USAGE_SESSION_GLOB` can see `~/.codex/sessions/**/*.jsonl`, those lines contain `payload.rate_limits`, the target account auth matches the live active auth file, and the latest usable event is newer than the account `last_used_at`.
- Fallback failures: run the fallback command manually and ensure it prints valid JSON.
