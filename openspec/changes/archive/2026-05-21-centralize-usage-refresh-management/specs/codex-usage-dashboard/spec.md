## MODIFIED Requirements

### Requirement: Poll usage automatically
The dashboard SHALL poll `/api/codex/usage` automatically as a cache-read operation and update the existing account list without a full page reload. The dashboard SHALL NOT implement its own backend write-refresh decision path.

#### Scenario: Poll interval elapses
- **WHEN** the frontend poll interval elapses
- **THEN** the dashboard SHALL fetch updated cached usage records and refresh account-list usage fields without a full page reload

#### Scenario: Switch completes
- **WHEN** the frontend receives a successful account switch response
- **THEN** the dashboard SHALL refresh account and usage views so the newly active account can show updated remaining traffic produced by the backend refresh manager

