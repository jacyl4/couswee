# codex-usage-collection Specification

## Purpose
TBD - created by archiving change improve-codex-usage-monitor. Update Purpose after archive.
## Requirements
### Requirement: Collect per-account Codex usage
The system SHALL collect live usage data for each configured couswee Codex account by using that account's own auth file when a valid access token exists, including 5-hour remaining percentage, weekly remaining percentage, and separate reset times for both windows.

#### Scenario: Usage collection runs for configured accounts
- **WHEN** the usage collector is triggered and configured accounts exist
- **THEN** the system SHALL attempt to collect a usage record for each account with account identifier, 5-hour remaining percentage, weekly remaining percentage, 5-hour reset time, and weekly reset time

#### Scenario: Account switch triggers immediate collection
- **WHEN** a configured account is switched to active state through the local switch API
- **THEN** the system SHALL immediately trigger usage collection for the newly active account so the dashboard can refresh against the newly active auth file

#### Scenario: No accounts are configured
- **WHEN** the usage collector is triggered and no accounts are configured
- **THEN** the system SHALL return an empty usage result without treating it as an error

### Requirement: Prefer API-based collection
The system SHALL try an account-auth API usage collector before local fallback collectors when the account has readable credentials and API collection is enabled.

#### Scenario: API collection succeeds
- **WHEN** API usage collection succeeds for an account
- **THEN** the system SHALL use the API result as that account's live usage record

#### Scenario: Default API endpoint is available
- **WHEN** `COUSWEE_USAGE_API_URL` is empty and `COUSWEE_USAGE_API_ENABLED` is not `false`
- **THEN** the system SHALL use the built-in Codex ChatGPT usage endpoint for account-auth API collection

#### Scenario: API collection fails
- **WHEN** API usage collection fails for an account and a local fallback collector is available
- **THEN** the system SHALL attempt fallback collection before reporting the account as failed

### Requirement: Support local fallback collection
The system SHALL support explicit local fallback collection through configured command output and explicitly configured active-only session rate-limit parsing, but SHALL NOT use abtop cache files as a target-system fallback.

#### Scenario: Command fallback collection succeeds
- **WHEN** API collection fails and configured command fallback output can be parsed
- **THEN** the system SHALL use the fallback result as the account's usage record and mark the source as fallback

#### Scenario: Active session fallback collection succeeds
- **WHEN** API collection fails for the active account and `COUSWEE_USAGE_SESSION_GLOB` is explicitly configured
- **AND** the latest Codex session `payload.rate_limits` can be matched to the live active auth file
- **AND** the session event is not older than the account `last_used_at`
- **THEN** the system SHALL use the session result for the active account only and mark the source as `codex-session`

#### Scenario: All collectors fail
- **WHEN** API and permitted fallback collection both fail for an account
- **THEN** the system SHALL preserve the last cached or SQLite usage value when available and expose an error status for that account

### Requirement: Normalize usage percentage units
The system SHALL normalize Codex quota values to percentages by default and SHALL label the unit in API responses.

#### Scenario: Default unit is used
- **WHEN** no explicit usage unit is configured
- **THEN** usage records SHALL report percentage values and set `unit` to `percent`

### Requirement: Resolve active account credentials
The system SHALL collect usage for the active account using the live `~/.codex/auth.json` credentials when that live auth can be matched to the active account, while continuing to use each non-active account's own `auth_path` for independent account queries.

#### Scenario: Active account usage is collected
- **WHEN** a configured account is active and usage collection runs
- **THEN** the collector SHALL use the live active auth file path for that account instead of relying only on a potentially stale backup path

#### Scenario: Active marker differs from live auth file
- **WHEN** the live `~/.codex/auth.json` matches one configured account backup file
- **THEN** the system SHALL associate active usage and active display state with that matching account

#### Scenario: Non-active account usage is collected
- **WHEN** a configured account is not active and has a readable auth file
- **THEN** the collector SHALL use that account's stored `auth_path` and SHALL NOT read global session data for that account

### Requirement: Use Codex auth token for endpoint collection
The API collector SHALL read the relevant Codex auth JSON file for each target account, extract `tokens.access_token`, and call the configured usage/rate-limit endpoint with a Bearer token instead of invoking a model request.

#### Scenario: Account auth token is available
- **WHEN** usage collection runs for an account whose auth file contains `tokens.access_token`
- **THEN** the API collector SHALL send `Authorization: Bearer <access token>` to the configured endpoint and SHALL NOT log the token value

#### Scenario: Auth token is missing
- **WHEN** the auth file is missing or lacks `tokens.access_token`
- **THEN** API collection SHALL fail for that account and permitted local fallback collection SHALL be attempted

### Requirement: Parse Codex rate-limit percentages
The collector SHALL parse rate-limit payloads containing 5-hour and weekly percentage values and reset timestamps, and SHALL convert Codex used percentages into CLI-style remaining percentages.

#### Scenario: Rate-limit payload is received
- **WHEN** a payload contains `five_hour.used_percentage`, `seven_day.used_percentage`, `resets_at`, and `updated_at`
- **THEN** couswee SHALL normalize those values into `5h_usage`, `weekly_usage`, `reset_time`, `unit: percent`, source metadata, and last-refresh metadata

### Requirement: Match CLI remaining percentages
The collector SHALL display Codex rate-limit percentages using the same remaining-capacity semantics as the Codex CLI status line.

#### Scenario: Codex reports used percentages
- **WHEN** a Codex rate-limit payload reports primary `used_percent: 31` and secondary `used_percent: 89`
- **THEN** couswee SHALL normalize them as `5h_usage: 69`, `weekly_usage: 11`, and `usage_basis: remaining`

### Requirement: Prefer latest session rate-limit event
The local fallback pipeline SHALL use the latest Codex CLI session `payload.rate_limits` event only when session fallback is explicitly configured, only for the account that matches the live active Codex auth file, only when the event is not older than that account's `last_used_at`, and SHALL NOT use session events for non-active accounts.

#### Scenario: Active session event is available
- **WHEN** session fallback is explicitly configured and the latest session event reports remaining values
- **AND** the target account auth matches the live active Codex auth file
- **AND** the session event timestamp is not older than the account `last_used_at`
- **THEN** couswee SHALL use the session event values and mark the source as `codex-session`

#### Scenario: Session event predates account switch
- **WHEN** the latest session event reports Codex rate-limit data
- **AND** the event timestamp is older than the target account `last_used_at`
- **THEN** couswee SHALL NOT apply that session data to the target account

#### Scenario: Session event has no account attribution
- **WHEN** the latest session event reports Codex rate-limit data
- **AND** the target account auth does not match the live active Codex auth file
- **THEN** couswee SHALL NOT apply that global session data to the target account

### Requirement: Parse ChatGPT wham usage payloads
The collector SHALL parse Codex ChatGPT `/backend-api/wham/usage` payloads and map their rate-limit windows to couswee remaining traffic fields.

#### Scenario: Wham usage payload is returned
- **WHEN** the endpoint response contains `rate_limit.primary_window.used_percent` and `rate_limit.secondary_window.used_percent`
- **THEN** the collector SHALL map `primary_window` to 5h remaining and `secondary_window` to weekly remaining by computing `100 - used_percent`

#### Scenario: Wham usage reports zero used percent
- **WHEN** a wham window reports `used_percent: 0` with a valid reset timestamp
- **THEN** the collector SHALL treat the remaining traffic as `100`, not as missing data

#### Scenario: Wham account id uses a different namespace
- **WHEN** a wham payload includes top-level `account_id`
- **THEN** the collector SHALL NOT reject the response solely because that value differs from local auth `tokens.account_id`

