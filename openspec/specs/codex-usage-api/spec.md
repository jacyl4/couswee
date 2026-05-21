# codex-usage-api Specification

## Purpose
TBD - created by archiving change improve-codex-usage-monitor. Update Purpose after archive.
## Requirements
### Requirement: Expose Codex usage endpoint
The system SHALL provide `GET /api/codex/usage` returning cached usage records for configured accounts as JSON, without using the GET request itself as a live collection trigger.

#### Scenario: Usage endpoint requested
- **WHEN** a client sends `GET /api/codex/usage`
- **THEN** the system SHALL return a JSON array of cached usage records
- **AND** the system SHALL NOT implicitly trigger a live usage refresh for that request

### Requirement: Usage response fields
Each usage response record SHALL include account identifier, 5-hour remaining percentage, weekly remaining percentage, legacy reset time, separate 5-hour reset time, separate weekly reset time, usage basis, unit, source, last refresh time, stale flag, and optional error text.

#### Scenario: Usage record is serialized
- **WHEN** the usage endpoint returns a record
- **THEN** the record SHALL include `account`, `5h_usage`, `weekly_usage`, `5h_remaining`, `weekly_remaining`, `reset_time`, `5h_reset_time`, `weekly_reset_time`, `usage_basis`, `unit`, `source`, `last_refresh`, `stale`, and `error` fields

### Requirement: Endpoint remains on the couswee server
The usage endpoint SHALL be served by the existing couswee server and SHALL NOT require a separate public service.

#### Scenario: Service starts with default address
- **WHEN** couswee starts with default configuration
- **THEN** `GET /api/codex/usage` SHALL be available on `0.0.0.0:2199`

### Requirement: Manual refresh endpoint is optional
If implemented, a manual refresh trigger SHALL route through the unified usage refresh manager, SHALL NOT block or destabilize `GET /api/codex/usage`, and it SHALL support both full refresh and single-account refresh semantics.

#### Scenario: Manual refresh is not implemented
- **WHEN** only periodic refresh exists
- **THEN** the usage endpoint SHALL still return cached records and the change SHALL remain valid

#### Scenario: Manual full refresh is requested
- **WHEN** a client requests a manual full usage refresh endpoint
- **THEN** the system SHALL trigger refresh for configured accounts through the unified usage refresh manager without changing the `GET /api/codex/usage` response contract

#### Scenario: Manual single-account refresh is requested
- **WHEN** a client requests a manual usage refresh for one account
- **THEN** the system SHALL refresh only that account through the unified usage refresh manager and preserve other accounts' cached records

### Requirement: Expose split reset fields
The usage endpoint SHALL expose separate reset timestamps for the 5-hour and weekly Codex limit windows.

#### Scenario: Split resets are serialized
- **WHEN** a usage record contains both reset windows
- **THEN** the response SHALL include `5h_reset_time` and `weekly_reset_time` separately

### Requirement: Expose explicit remaining fields
The usage endpoint SHALL expose explicit `5h_remaining` and `weekly_remaining` fields for UI display, while preserving legacy fields for compatibility and marking the response with `usage_basis: remaining`.

#### Scenario: Remaining fields are serialized
- **WHEN** a usage record is returned
- **THEN** `5h_remaining` and `weekly_remaining` SHALL contain the remaining traffic percentages shown in the account list

#### Scenario: Legacy fields are serialized
- **WHEN** a usage record is returned with legacy `5h_usage` and `weekly_usage`
- **THEN** those fields SHALL contain the same remaining traffic percentages and the record SHALL include `usage_basis: remaining`

### Requirement: Avoid exposing auth material
Usage APIs SHALL NOT return auth file contents, access tokens, Authorization headers, or token-derived secrets.

#### Scenario: Usage record is returned
- **WHEN** `GET /api/codex/usage` returns records
- **THEN** each record SHALL contain usage and refresh metadata only, without auth token values

#### Scenario: Usage refresh fails
- **WHEN** a usage refresh error is exposed through an API response
- **THEN** the error text SHALL NOT contain access tokens or Authorization header values

### Requirement: Resolve user auth paths
The system SHALL resolve user-relative auth paths before reading Codex auth files for usage collection.

#### Scenario: Auth path uses home shorthand
- **WHEN** an account auth path starts with `~/`
- **THEN** the usage collector SHALL expand it to the current user's home directory before reading token data

