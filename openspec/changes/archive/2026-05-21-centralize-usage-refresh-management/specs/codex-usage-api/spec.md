## MODIFIED Requirements

### Requirement: Expose Codex usage endpoint
The system SHALL provide `GET /api/codex/usage` returning cached usage records for configured accounts as JSON, without using the GET request itself as a live collection trigger.

#### Scenario: Usage endpoint requested
- **WHEN** a client sends `GET /api/codex/usage`
- **THEN** the system SHALL return a JSON array of cached usage records
- **AND** the system SHALL NOT implicitly trigger a live usage refresh for that request

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

