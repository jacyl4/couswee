## ADDED Requirements

### Requirement: Codex login APIs
The system SHALL provide local-only APIs for starting and observing Codex account login flows.

#### Scenario: Codex login starts
- **WHEN** a client sends `POST /api/codex/login/start`
- **THEN** the system SHALL create a login session and return verification URL, device/user code, expiration, and session id

#### Scenario: Legacy login start aliases
- **WHEN** a client sends `POST /api/codex/login/oauth/start` or `POST /api/codex/login/device/start`
- **THEN** the system SHALL route the request to the same Codex device-code login behavior

#### Scenario: Login status requested
- **WHEN** a client sends `GET /api/codex/login/:session_id`
- **THEN** the system SHALL return the current login session status without exposing token values

### Requirement: SQLite account management APIs
The system SHALL provide local-only APIs for creating, editing, deleting, listing, and switching SQLite-backed accounts.

#### Scenario: Account is edited
- **WHEN** a client sends `PATCH /api/accounts/:id` with editable display metadata
- **THEN** the system SHALL update SQLite metadata and return the updated account

#### Scenario: Accounts are deleted
- **WHEN** a client sends `DELETE /api/accounts` with account ids or nicknames
- **THEN** the system SHALL delete matching SQLite account records and apply safe auth/profile cleanup rules

#### Scenario: Account switch remains compatible
- **WHEN** a client sends existing `POST /api/switch` with a nickname
- **THEN** the system SHALL continue to support the compatibility switch path using the SQLite account store
