## ADDED Requirements

### Requirement: Detect active account
The system SHALL expose the currently active Codex account based on the account registry active marker and the auth file state managed by couswee.

#### Scenario: Active account exists
- **WHEN** one configured account is marked active
- **THEN** the system SHALL return that account from the current-account API

#### Scenario: No active account exists
- **WHEN** no configured account is marked active
- **THEN** the system SHALL return a not-found response for the current-account API

### Requirement: Switch active account by nickname
The system SHALL switch the active Codex account when given a valid configured nickname by replacing `~/.codex/auth.json` with the selected account's configured auth file.

#### Scenario: Successful switch
- **WHEN** a client submits a switch request for an existing nickname whose `auth_path` is readable
- **THEN** the system SHALL copy that auth file to `~/.codex/auth.json`, mark only that account active, and return the selected account

#### Scenario: Auth source cannot be read
- **WHEN** a client submits a switch request for an existing nickname whose `auth_path` cannot be read
- **THEN** the system SHALL return an error and SHALL NOT mark the account active

### Requirement: Single active account
The system SHALL maintain at most one active account after every successful switch.

#### Scenario: Account is switched
- **WHEN** a switch request succeeds
- **THEN** the selected account SHALL be active and every other configured account SHALL be inactive
