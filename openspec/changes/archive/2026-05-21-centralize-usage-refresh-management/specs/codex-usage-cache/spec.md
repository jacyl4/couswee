## MODIFIED Requirements

### Requirement: Cache usage records in memory
The system SHALL maintain a thread-safe in-memory cache of the latest usage record for each configured account, while treating SQLite as the only durable usage cache. Cache mutation SHALL happen through the unified usage refresh management path.

#### Scenario: Usage API reads cache
- **WHEN** a client requests usage data
- **THEN** the system SHALL return the latest cached usage records without blocking on a live collection attempt

#### Scenario: Cache is updated
- **WHEN** a refresh cycle produces newer usage records through the unified usage refresh manager
- **THEN** the system SHALL atomically replace the affected cached records

#### Scenario: Service restarts
- **WHEN** couswee starts after a prior successful usage collection
- **THEN** the system SHALL be able to restore last known usage values from SQLite without reading a `~/.cache` usage file

### Requirement: Refresh usage periodically
The system SHALL refresh usage data automatically through the unified usage refresh manager using a configurable interval between 1 and 5 minutes, defaulting to 5 minutes.

#### Scenario: Default refresh interval is active
- **WHEN** the service starts without a custom usage refresh interval
- **THEN** the system SHALL refresh usage data approximately every 5 minutes through the unified usage refresh manager

#### Scenario: Custom interval is below minimum
- **WHEN** configuration requests a refresh interval below 1 minute
- **THEN** the system SHALL clamp the effective interval to 1 minute

#### Scenario: Custom interval is above maximum
- **WHEN** configuration requests a refresh interval above 5 minutes
- **THEN** the system SHALL clamp the effective interval to 5 minutes

### Requirement: Persist latest successful percentages and reset times
The system SHALL persist the latest successful 5-hour and weekly remaining percentages, reset times, and refresh metadata into each account's SQLite record only from the unified usage refresh management path.

#### Scenario: Usage refresh succeeds
- **WHEN** a refresh cycle produces a successful usage record for an account through the unified refresh manager
- **THEN** the system SHALL write that account's 5-hour and weekly remaining percentages plus 5-hour and weekly reset times to SQLite for subsequent account-list loads

#### Scenario: Registry fallback is read
- **WHEN** a refresh cycle uses the account registry itself as the fallback usage source
- **THEN** the system SHALL NOT write that fallback record back to SQLite as a fresh usage collection result

#### Scenario: Usage refresh fails
- **WHEN** a refresh cycle produces only an error or stale record for an account
- **THEN** the system SHALL NOT overwrite the account's persisted SQLite usage percentages or reset times with zero, unknown, or error defaults

