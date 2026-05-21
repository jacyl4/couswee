## MODIFIED Requirements

### Requirement: Persist configured accounts
The system SHALL persist configured Codex account records in a local SQLite database managed by couswee, using Go `database/sql` with the `modernc.org/sqlite` driver.

#### Scenario: Load account registry on startup
- **WHEN** the couswee service starts and the SQLite database exists
- **THEN** the system SHALL load account records from the SQLite database before serving account data

#### Scenario: Missing database initializes schema
- **WHEN** the couswee service starts and the SQLite database does not exist
- **THEN** the system SHALL create the database schema and start with an empty account list without failing startup

### Requirement: Account record fields
Each account record SHALL include a stable id, nickname, display name, profile name, auth file path, login method, login/status state, subscription metadata, active-account marker, and created/updated timestamps.

#### Scenario: Account data is returned consistently
- **WHEN** clients request the account list
- **THEN** each account object SHALL expose compatibility fields `nickname`, `auth_path`, `subscription`, `5h_usage`, `weekly_usage`, and `active`, and SHALL also expose SQLite-backed fields such as `id`, `profile_name`, `display_name`, `login_method`, and `status`

### Requirement: Stable account identity
The system SHALL use a stable SQLite account id as the internal selector, while preserving `nickname` as the user-facing label and compatibility selector.

#### Scenario: Unknown nickname is selected
- **WHEN** a client requests an operation for a nickname or id that is not in the SQLite account store
- **THEN** the system SHALL return a not-found error and SHALL NOT modify active account state
