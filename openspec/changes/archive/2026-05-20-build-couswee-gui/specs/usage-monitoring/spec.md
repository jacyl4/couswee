## ADDED Requirements

### Requirement: Expose usage metrics
The system SHALL expose recent 5-hour usage, weekly usage, and subscription date for each configured account.

#### Scenario: Account list includes usage
- **WHEN** clients request the account list
- **THEN** the response SHALL include 5-hour usage, weekly usage, and subscription information for each account

### Requirement: Refresh usage data periodically
The system SHALL refresh usage-related account data on a periodic timer while the service is running.

#### Scenario: Refresh interval elapses
- **WHEN** the configured refresh interval elapses
- **THEN** the system SHALL update cached usage data for configured accounts without requiring a service restart

### Requirement: Restore cached usage after restart
The system SHALL preserve usage-related account values in the local account registry so the dashboard can show data after restart.

#### Scenario: Service restarts after cached values exist
- **WHEN** the service starts after prior usage values were saved
- **THEN** the system SHALL load and expose the saved usage values from the registry
