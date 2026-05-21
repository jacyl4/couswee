## MODIFIED Requirements

### Requirement: Collect per-account Codex usage
The system SHALL collect live usage data for each configured couswee Codex account by using that account's own auth file when a valid access token exists, including 5-hour remaining percentage, weekly remaining percentage, and separate reset times for both windows. Collection SHALL be invoked by the unified usage refresh manager and SHALL NOT decide the refresh trigger source itself.

#### Scenario: Usage collection runs for configured accounts
- **WHEN** the unified usage refresh manager triggers collection and configured accounts exist
- **THEN** the system SHALL attempt to collect a usage record for each targeted account with account identifier, 5-hour remaining percentage, weekly remaining percentage, 5-hour reset time, and weekly reset time

#### Scenario: Account switch triggers immediate collection
- **WHEN** a configured account is switched to active state through the local switch API
- **THEN** the unified usage refresh manager SHALL immediately trigger usage collection for the newly active account so the dashboard can refresh against the newly active auth file

#### Scenario: No accounts are configured
- **WHEN** the unified usage refresh manager triggers collection and no accounts are configured
- **THEN** the system SHALL return an empty usage result without treating it as an error

