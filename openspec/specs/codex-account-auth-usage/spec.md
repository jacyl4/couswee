# codex-account-auth-usage Specification

## Purpose
TBD - created by archiving change systematize-codex-usage-collection. Update Purpose after archive.
## Requirements
### Requirement: Query usage from each account auth file
The system SHALL query Codex usage for each configured account by reading that account's SQLite `auth_path` and using the contained `tokens.access_token` as the request credential.

#### Scenario: Account auth token is available
- **WHEN** usage collection runs for an account whose `auth_path` contains `tokens.access_token`
- **THEN** the collector SHALL call the configured or default usage/rate-limit endpoint with that token and associate the result with that account

#### Scenario: Account auth token is unavailable
- **WHEN** an account has no readable auth file or the auth JSON lacks `tokens.access_token`
- **THEN** the system SHALL keep the last SQLite usage value for that account when available and mark the live query as stale or failed

### Requirement: Protect auth token secrecy
The system SHALL NOT expose Codex access tokens through logs, API responses, UI state, error messages, or persisted usage metadata.

#### Scenario: Endpoint request fails
- **WHEN** a usage endpoint request fails for an account
- **THEN** the recorded error SHALL describe the failure without including `tokens.access_token` or Authorization header values

### Requirement: Verify account attribution
The system SHALL verify usage results against account identity when the auth file or usage endpoint provides `account_id` or equivalent identity metadata.

#### Scenario: Response account does not match auth account
- **WHEN** the usage endpoint returns account identity metadata that conflicts with the account's auth identity
- **THEN** the collector SHALL reject the result for that account and SHALL NOT write it to SQLite

#### Scenario: Response account identity is not comparable to auth account identity
- **WHEN** the ChatGPT wham usage endpoint returns a top-level `account_id` from a different identity namespace than `tokens.account_id`
- **THEN** the collector SHALL NOT treat that field as a strict mismatch and SHALL associate the result with the account whose Bearer token was used

#### Scenario: Response has no account identity metadata
- **WHEN** the usage endpoint response contains valid usage data but no account identity metadata
- **THEN** the collector MAY accept the result only for the account whose token was used and SHALL record the source as account-auth API data

### Requirement: Persist successful usage to SQLite
The system SHALL persist the latest successful per-account usage query result to SQLite as the durable usage state.

#### Scenario: Account usage query succeeds
- **WHEN** a per-account auth usage query returns valid remaining percentages and reset times
- **THEN** the system SHALL update that account's SQLite usage fields and refresh metadata

#### Scenario: Default endpoint is not overridden
- **WHEN** API collection is enabled and `COUSWEE_USAGE_API_URL` is not set
- **THEN** the collector SHALL use the built-in Codex ChatGPT usage endpoint and each account's own auth token

#### Scenario: Account usage query fails after prior success
- **WHEN** a per-account auth usage query fails and SQLite already has prior usage data
- **THEN** the system SHALL preserve the prior percentages and reset times instead of overwriting them with zero or unknown values

