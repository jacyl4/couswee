## MODIFIED Requirements

### Requirement: Manual refresh endpoint is optional
If implemented, a manual refresh trigger SHALL NOT block or destabilize `GET /api/codex/usage`, and it SHALL support both full refresh and single-account refresh semantics.

#### Scenario: Manual refresh is not implemented
- **WHEN** only periodic refresh exists
- **THEN** the usage endpoint SHALL still return cached records and the change SHALL remain valid

#### Scenario: Manual full refresh is requested
- **WHEN** a client requests a manual full usage refresh endpoint
- **THEN** the system SHALL trigger refresh for configured accounts without changing the `GET /api/codex/usage` response contract

#### Scenario: Manual single-account refresh is requested
- **WHEN** a client requests a manual usage refresh for one account
- **THEN** the system SHALL refresh only that account and preserve other accounts' cached records

### Requirement: Expose explicit remaining fields
The usage endpoint SHALL expose explicit `5h_remaining` and `weekly_remaining` fields for UI display, while preserving legacy fields for compatibility and marking the response with `usage_basis: remaining`.

#### Scenario: Remaining fields are serialized
- **WHEN** a usage record is returned
- **THEN** `5h_remaining` and `weekly_remaining` SHALL contain the remaining traffic percentages shown in the account list

#### Scenario: Legacy fields are serialized
- **WHEN** a usage record is returned with legacy `5h_usage` and `weekly_usage`
- **THEN** those fields SHALL contain the same remaining traffic percentages and the record SHALL include `usage_basis: remaining`

## ADDED Requirements

### Requirement: Avoid exposing auth material
Usage APIs SHALL NOT return auth file contents, access tokens, Authorization headers, or token-derived secrets.

#### Scenario: Usage record is returned
- **WHEN** `GET /api/codex/usage` returns records
- **THEN** each record SHALL contain usage and refresh metadata only, without auth token values

#### Scenario: Usage refresh fails
- **WHEN** a usage refresh error is exposed through an API response
- **THEN** the error text SHALL NOT contain access tokens or Authorization header values
