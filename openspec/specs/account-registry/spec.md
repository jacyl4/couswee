# account-registry Specification

## Purpose

Persist and expose couswee Codex account metadata from the local SQLite database. SQLite is the only account registry read/write source.
## Requirements
### Requirement: Persist configured accounts
The system SHALL persist configured Codex account records in a local SQLite database managed by couswee, using Go `database/sql` with the `modernc.org/sqlite` driver.

#### Scenario: Load account registry on startup
- **WHEN** the couswee service starts and the SQLite database exists
- **THEN** the system SHALL load account records from the SQLite database before serving account data

#### Scenario: Missing database initializes schema
- **WHEN** the couswee service starts and the SQLite database does not exist
- **THEN** the system SHALL create the database schema and start with an empty account list without failing startup

### Requirement: Account record fields
Each account record SHALL include a stable id, nickname, profile name, auth file path, login method, login/status state, subscription metadata, active-account marker, and created/updated timestamps.

#### Scenario: Account data is returned consistently
- **WHEN** clients request the account list
- **THEN** each account object SHALL expose compatibility fields `nickname`, `auth_path`, `subscription`, `5h_usage`, `weekly_usage`, and `active`, and SHALL also expose SQLite-backed fields such as `id`, `profile_name`, `login_method`, and `status`

### Requirement: Stable account identity
The system SHALL use `profile_name` as the stable account identity for user-visible account recognition and account-scoped data matching. `nickname` SHALL be treated only as display metadata that helps users distinguish accounts in the UI.

#### Scenario: Unknown profile is selected
- **WHEN** a client requests an operation for a `profile_name` or id that is not in the SQLite account store
- **THEN** the system SHALL return a not-found error and SHALL NOT modify active account state

#### Scenario: Display nickname changes
- **WHEN** a user edits an account nickname
- **THEN** the system SHALL update only the display label and SHALL NOT change the account identity, usage matching key, active-account selection key, or managed profile path
