## MODIFIED Requirements

### Requirement: Show stale and error states inline
The account list SHALL clearly show when an account usage record is stale, failed, or currently refreshing while retaining the last known percentage values.

#### Scenario: Stale record received
- **WHEN** a usage record has `stale` set to true
- **THEN** the corresponding account row SHALL show stale status and retain the last known percentage values

#### Scenario: Error text received
- **WHEN** a usage record includes an error message
- **THEN** the corresponding account row SHALL display a concise inline error indicator

#### Scenario: Account refresh is running
- **WHEN** a single-account usage refresh is in progress
- **THEN** the corresponding account row SHALL indicate that usage is refreshing without clearing existing remaining values

### Requirement: Poll usage automatically
The dashboard SHALL poll `/api/codex/usage` automatically and update the existing account list without a full page reload.

#### Scenario: Poll interval elapses
- **WHEN** the frontend poll interval elapses
- **THEN** the dashboard SHALL fetch updated usage records and refresh account-list usage fields without a full page reload

#### Scenario: Switch completes
- **WHEN** the frontend receives a successful account switch response
- **THEN** the dashboard SHALL refresh account and usage data so the newly active account can show updated remaining traffic promptly
