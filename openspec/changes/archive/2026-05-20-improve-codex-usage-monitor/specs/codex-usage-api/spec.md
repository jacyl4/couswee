## ADDED Requirements

### Requirement: Expose Codex usage endpoint
The system SHALL provide `GET /api/codex/usage` returning usage records for configured accounts as JSON.

#### Scenario: Usage endpoint requested
- **WHEN** a client sends `GET /api/codex/usage`
- **THEN** the system SHALL return a JSON array of usage records

### Requirement: Usage response fields
Each usage response record SHALL include account identifier, 5-hour remaining percentage, weekly remaining percentage, legacy reset time, separate 5-hour reset time, separate weekly reset time, usage basis, unit, source, last refresh time, stale flag, and optional error text.

#### Scenario: Usage record is serialized
- **WHEN** the usage endpoint returns a record
- **THEN** the record SHALL include `account`, `5h_usage`, `weekly_usage`, `5h_remaining`, `weekly_remaining`, `reset_time`, `5h_reset_time`, `weekly_reset_time`, `usage_basis`, `unit`, `source`, `last_refresh`, `stale`, and `error` fields

### Requirement: Endpoint remains local-only
The usage endpoint SHALL be served by the existing local couswee server and SHALL NOT require a separate public service.

#### Scenario: Service starts with default address
- **WHEN** couswee starts with default configuration
- **THEN** `GET /api/codex/usage` SHALL be available on `127.0.0.1:2199`

### Requirement: Manual refresh endpoint is optional
If implemented, a manual refresh trigger SHALL NOT block or destabilize `GET /api/codex/usage`.

#### Scenario: Manual refresh is not implemented
- **WHEN** only periodic refresh exists
- **THEN** the usage endpoint SHALL still return cached records and the change SHALL remain valid


### Requirement: Expose split reset fields
The usage endpoint SHALL expose separate reset timestamps for the 5-hour and weekly Codex limit windows.

#### Scenario: Split resets are serialized
- **WHEN** a usage record contains both reset windows
- **THEN** the response SHALL include `5h_reset_time` and `weekly_reset_time` separately


### Requirement: Expose explicit remaining fields
The usage endpoint SHALL expose explicit `5h_remaining` and `weekly_remaining` fields for UI display, while preserving legacy fields for compatibility.

#### Scenario: Remaining fields are serialized
- **WHEN** a usage record is returned
- **THEN** `5h_remaining` and `weekly_remaining` SHALL contain the remaining traffic percentages shown in the account list
