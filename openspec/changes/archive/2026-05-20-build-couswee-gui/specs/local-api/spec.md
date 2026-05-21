## ADDED Requirements

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
The system SHALL provide `POST /api/switch` accepting a JSON body with `nickname` and performing an account switch.

#### Scenario: Switch request submitted
- **WHEN** a client sends `POST /api/switch` with a valid `nickname`
- **THEN** the system SHALL switch to the requested account and return the selected account as JSON

### Requirement: Serve frontend assets
The system SHALL serve the built SvelteKit static frontend at `/` and static asset paths from the same local service.

#### Scenario: Browser requests root page
- **WHEN** a browser sends `GET /`
- **THEN** the system SHALL return the built dashboard frontend
