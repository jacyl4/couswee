## ADDED Requirements

### Requirement: Persist configured accounts
The system SHALL persist configured Codex account records in a local JSON registry at `~/.couswee/accounts.json`.

#### Scenario: Load account registry on startup
- **WHEN** the couswee service starts and the registry file exists
- **THEN** the system SHALL load account records from `~/.couswee/accounts.json` before serving account data

#### Scenario: Missing registry starts empty
- **WHEN** the couswee service starts and the registry file does not exist
- **THEN** the system SHALL start with an empty account list without failing startup

### Requirement: Account record fields
Each account record SHALL include a nickname, auth file path, subscription date, recent 5-hour usage value, weekly usage value, and active-account marker.

#### Scenario: Account data is returned consistently
- **WHEN** clients request the account list
- **THEN** each account object SHALL expose `nickname`, `auth_path`, `subscription`, `5h_usage`, `weekly_usage`, and `active` fields

### Requirement: Stable account identity
The system SHALL use `nickname` as the user-facing account selector and SHALL reject switch requests for nicknames that are not present in the registry.

#### Scenario: Unknown nickname is selected
- **WHEN** a client requests an operation for a nickname that is not in the registry
- **THEN** the system SHALL return a not-found error and SHALL NOT modify active account state
