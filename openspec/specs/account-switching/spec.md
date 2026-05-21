# account-switching Specification

## Purpose
TBD - created by archiving change build-couswee-gui. Update Purpose after archive.
## Requirements
### Requirement: Detect active account
The system SHALL expose the currently active Codex account based on the SQLite active marker and the auth/profile file state managed by couswee.

#### Scenario: Active account exists
- **WHEN** one configured account is marked active in SQLite
- **THEN** the system SHALL return that account from the current-account API

#### Scenario: No active account exists
- **WHEN** no configured account is marked active
- **THEN** the system SHALL return a not-found response for the current-account API

### Requirement: Switch active account by nickname
The system SHALL switch the active Codex account when given a valid configured nickname or account id by activating that account's managed profile/auth file.

#### Scenario: Successful switch
- **WHEN** a client submits a switch request for an existing account whose managed auth file is readable
- **THEN** the system SHALL activate that account's auth/profile for Codex CLI use, mark only that account active in SQLite, and return the selected account

#### Scenario: Auth source cannot be read
- **WHEN** a client submits a switch request for an existing account whose managed auth file cannot be read
- **THEN** the system SHALL return an error and SHALL NOT mark the account active

### Requirement: Single active account
The system SHALL maintain at most one active account after every successful switch.

#### Scenario: Account is switched
- **WHEN** a switch request succeeds
- **THEN** the selected account SHALL be active and every other configured account SHALL be inactive in SQLite

