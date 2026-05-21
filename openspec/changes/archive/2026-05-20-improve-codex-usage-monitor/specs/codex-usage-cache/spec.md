## ADDED Requirements

### Requirement: Cache usage records in memory
The system SHALL maintain a thread-safe in-memory cache of the latest usage record for each configured account.

#### Scenario: Usage API reads cache
- **WHEN** a client requests usage data
- **THEN** the system SHALL return the latest cached usage records without blocking on a live collection attempt

#### Scenario: Cache is updated
- **WHEN** a refresh cycle produces newer usage records
- **THEN** the system SHALL atomically replace the affected cached records

### Requirement: Refresh usage periodically
The system SHALL refresh usage data automatically using a configurable interval between 1 and 5 minutes, defaulting to 5 minutes.

#### Scenario: Default refresh interval is active
- **WHEN** the service starts without a custom usage refresh interval
- **THEN** the system SHALL refresh usage data approximately every 5 minutes

#### Scenario: Custom interval is below minimum
- **WHEN** configuration requests a refresh interval below 1 minute
- **THEN** the system SHALL clamp the effective interval to 1 minute

#### Scenario: Custom interval is above maximum
- **WHEN** configuration requests a refresh interval above 5 minutes
- **THEN** the system SHALL clamp the effective interval to 5 minutes

### Requirement: Preserve stale data on refresh failure
The system SHALL preserve the last successful usage data when a refresh cycle fails and SHALL expose stale/error metadata to clients.

#### Scenario: Refresh fails after prior success
- **WHEN** a refresh cycle fails for an account that has cached usage
- **THEN** the system SHALL keep the prior usage values and mark the record as stale with the latest error message

### Requirement: Record refresh metadata
The system SHALL store collection source, last refresh time, stale status, and error text for each usage record.

#### Scenario: Usage record is returned
- **WHEN** cached usage is exposed through the API
- **THEN** each record SHALL include source, last refresh time, stale flag, and optional error text
