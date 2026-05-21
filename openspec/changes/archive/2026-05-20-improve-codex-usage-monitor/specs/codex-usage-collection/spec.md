## ADDED Requirements

### Requirement: Collect per-account Codex usage
The system SHALL collect live usage data for each configured couswee Codex account, including 5-hour remaining percentage, weekly remaining percentage, and separate reset times for both windows.

#### Scenario: Usage collection runs for configured accounts
- **WHEN** the usage collector is triggered and configured accounts exist
- **THEN** the system SHALL attempt to collect a usage record for each account with account identifier, 5-hour remaining percentage, weekly remaining percentage, 5-hour reset time, and weekly reset time

#### Scenario: No accounts are configured
- **WHEN** the usage collector is triggered and no accounts are configured
- **THEN** the system SHALL return an empty usage result without treating it as an error

### Requirement: Prefer API-based collection
The system SHALL try an API-based usage collector before local fallback collectors when credentials and configuration allow API collection.

#### Scenario: API collection succeeds
- **WHEN** API usage collection succeeds for an account
- **THEN** the system SHALL use the API result as that account's live usage record

#### Scenario: API collection fails
- **WHEN** API usage collection fails for an account and a local fallback collector is available
- **THEN** the system SHALL attempt fallback collection before reporting the account as failed

### Requirement: Support local fallback collection
The system SHALL support a local fallback collector compatible with `openusage.sh`/`abtop`-style command output and local abtop cache parsing.

#### Scenario: Fallback collection succeeds
- **WHEN** API collection fails and fallback output can be parsed
- **THEN** the system SHALL use the fallback result as the account's usage record and mark the source as fallback

#### Scenario: All collectors fail
- **WHEN** API and fallback collection both fail for an account
- **THEN** the system SHALL preserve the last cached usage value when available and expose an error status for that account

### Requirement: Normalize usage percentage units
The system SHALL normalize Codex quota values to percentages by default and SHALL label the unit in API responses.

#### Scenario: Default unit is used
- **WHEN** no explicit usage unit is configured
- **THEN** usage records SHALL report percentage values and set `unit` to `percent`


### Requirement: Resolve active account credentials
The system SHALL collect usage for the active account using the live `~/.codex/auth.json` credentials when that account is marked active.

#### Scenario: Active account usage is collected
- **WHEN** a configured account is active and usage collection runs
- **THEN** the collector SHALL use the live active auth file path for that account instead of relying only on a potentially stale backup path

#### Scenario: Active marker differs from live auth file
- **WHEN** the live `~/.codex/auth.json` matches one configured account backup file
- **THEN** the system SHALL associate active usage and active display state with that matching account


### Requirement: Use Codex auth token for endpoint collection
The API collector SHALL read the relevant Codex auth JSON file, extract `tokens.access_token`, and call the configured usage/rate-limit endpoint with a Bearer token instead of invoking a model request.

#### Scenario: Active auth token is available
- **WHEN** usage collection runs for an account whose auth file contains `tokens.access_token`
- **THEN** the API collector SHALL send `Authorization: Bearer <access token>` to the configured endpoint and SHALL NOT log the token value

#### Scenario: Auth token is missing
- **WHEN** the auth file is missing or lacks `tokens.access_token`
- **THEN** API collection SHALL fail for that account and local fallback collection SHALL be attempted

### Requirement: Parse Codex rate-limit percentages
The collector SHALL parse rate-limit payloads containing 5-hour and weekly percentage values and reset timestamps, and SHALL convert Codex used percentages into CLI-style remaining percentages.

#### Scenario: Rate-limit payload is received
- **WHEN** a payload contains `five_hour.used_percentage`, `seven_day.used_percentage`, `resets_at`, and `updated_at`
- **THEN** couswee SHALL normalize those values into `5h_usage`, `weekly_usage`, `reset_time`, `unit: percent`, source metadata, and last-refresh metadata

### Requirement: Use abtop cache before registry fallback
When live endpoint collection and command fallback are unavailable, the system SHALL try the local abtop Codex rate-limit cache before registry-backed manual values.

#### Scenario: abtop cache exists
- **WHEN** `~/.cache/abtop/codex-rate-limits.json` contains parseable Codex rate-limit data
- **THEN** couswee SHALL use that data for account-list 5h/weekly percentages before using registry values


### Requirement: Match CLI remaining percentages
The collector SHALL display Codex rate-limit percentages using the same remaining-capacity semantics as the Codex CLI status line.

#### Scenario: Codex reports used percentages
- **WHEN** a Codex rate-limit payload reports primary `used_percent: 31` and secondary `used_percent: 89`
- **THEN** couswee SHALL normalize them as `5h_usage: 69`, `weekly_usage: 11`, and `usage_basis: remaining`

### Requirement: Prefer latest session rate-limit event
The local fallback pipeline SHALL prefer the latest Codex CLI session `payload.rate_limits` event over the abtop cache file.

#### Scenario: Session event and abtop cache disagree
- **WHEN** the latest session event reports newer remaining values than `~/.cache/abtop/codex-rate-limits.json`
- **THEN** couswee SHALL use the session event values and mark the source as `codex-session`
