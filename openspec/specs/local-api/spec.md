# local-api Specification

## Purpose
TBD - created by archiving change build-couswee-gui. Update Purpose after archive.
## Requirements
### Requirement: Serve local HTTP API
The system SHALL serve a local GoFiber HTTP API on `127.0.0.1:2199` by default.

#### Scenario: Service starts
- **WHEN** couswee starts with the default configuration
- **THEN** the HTTP server SHALL listen on `127.0.0.1:2199`

### Requirement: List accounts endpoint
The system SHALL provide `GET /api/accounts` returning all configured accounts as JSON.

#### Scenario: Accounts requested
- **WHEN** a client sends `GET /api/accounts`
- **THEN** the system SHALL return a JSON array of account records

### Requirement: Current account endpoint
The system SHALL provide `GET /api/current` returning the active account as JSON when one exists.

#### Scenario: Current account requested
- **WHEN** a client sends `GET /api/current` and an active account exists
- **THEN** the system SHALL return the active account as JSON

### Requirement: Switch account endpoint
The system SHALL provide `POST /api/switch` accepting a JSON body with `nickname` and performing an account switch, then prioritizing usage refresh for the newly active account.

#### Scenario: Switch request submitted
- **WHEN** a client sends `POST /api/switch` with a valid `nickname`
- **THEN** the system SHALL switch to the requested account and return the selected account as JSON

#### Scenario: Switch refreshes usage
- **WHEN** a client successfully switches accounts through `POST /api/switch`
- **THEN** the system SHALL trigger an immediate Codex usage refresh for the newly active account before returning or immediately after returning the selected account response

#### Scenario: Other accounts are not blocking switch
- **WHEN** other configured accounts are slow or failing during usage collection
- **THEN** those accounts SHALL NOT prevent the switch API from completing after the newly active account has been handled

### Requirement: Serve frontend assets
The system SHALL serve the built SvelteKit static frontend at `/` and static asset paths from the same local service.

#### Scenario: Browser requests root page
- **WHEN** a browser sends `GET /`
- **THEN** the system SHALL return the built dashboard frontend

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

