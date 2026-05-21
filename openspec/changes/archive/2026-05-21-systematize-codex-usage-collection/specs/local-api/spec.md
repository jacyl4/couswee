## MODIFIED Requirements

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
