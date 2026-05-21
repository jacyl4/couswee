## MODIFIED Requirements

### Requirement: Cache usage records in memory
The system SHALL maintain a thread-safe in-memory cache of the latest usage record for each configured account, while treating SQLite as the only durable usage cache.

#### Scenario: Usage API reads cache
- **WHEN** a client requests usage data
- **THEN** the system SHALL return the latest cached usage records without blocking on a live collection attempt

#### Scenario: Cache is updated
- **WHEN** a refresh cycle produces newer usage records
- **THEN** the system SHALL atomically replace the affected cached records

#### Scenario: Service restarts
- **WHEN** couswee starts after a prior successful usage collection
- **THEN** the system SHALL be able to restore last known usage values from SQLite without reading a `~/.cache` usage file

### Requirement: Persist latest successful percentages and reset times
The system SHALL persist the latest successful 5-hour and weekly remaining percentages, reset times, and refresh metadata into each account's SQLite record.

#### Scenario: Usage refresh succeeds
- **WHEN** a refresh cycle produces a successful usage record for an account
- **THEN** the system SHALL write that account's 5-hour and weekly remaining percentages plus 5-hour and weekly reset times to SQLite for subsequent account-list loads

#### Scenario: Registry fallback is read
- **WHEN** a refresh cycle uses the account registry itself as the fallback usage source
- **THEN** the system SHALL NOT write that fallback record back to SQLite as a fresh usage collection result

#### Scenario: Usage refresh fails
- **WHEN** a refresh cycle produces only an error or stale record for an account
- **THEN** the system SHALL NOT overwrite the account's persisted SQLite usage percentages or reset times with zero, unknown, or error defaults

## ADDED Requirements

### Requirement: Avoid project-external usage cache files
The system SHALL NOT create or depend on project-external usage cache files for durable couswee state.

#### Scenario: Default configuration is loaded
- **WHEN** couswee starts without explicit usage cache configuration
- **THEN** the system SHALL NOT default to `~/.cache/abtop/codex-rate-limits.json` or `~/.cache/couswee/codex-rate-limits.json`

#### Scenario: Usage refresh completes
- **WHEN** usage refresh succeeds for one or more accounts
- **THEN** the system SHALL persist durable results to SQLite and SHALL NOT write a separate usage cache file under `~/.cache`
