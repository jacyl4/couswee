# codex-usage-dashboard Specification

## Purpose
TBD - created by archiving change improve-codex-usage-monitor. Update Purpose after archive.
## Requirements
### Requirement: Integrate usage into account list
The dashboard SHALL integrate Codex usage into the existing account list rather than rendering a separate usage-only panel.

#### Scenario: Account list receives usage data
- **WHEN** the dashboard receives accounts and usage records
- **THEN** it SHALL render usage values in the corresponding account row

### Requirement: Display percentage-only usage
The account list SHALL display 5-hour and weekly remaining traffic as percentages only, matching the Codex CLI status line.

#### Scenario: Usage percentages are displayed
- **WHEN** a usage record has 5-hour and weekly values
- **THEN** the account list SHALL show those remaining traffic values with percent signs and SHALL NOT show token or USD totals in the account row

### Requirement: Display reset time in account list
The account list SHALL include separate 5-hour and weekly reset times for each account when usage data provides them.

#### Scenario: Reset time is available
- **WHEN** a usage record has `reset_time`
- **THEN** the corresponding account row SHALL display the formatted 5-hour reset time and weekly reset time in separate cells

### Requirement: Poll usage automatically
The dashboard SHALL poll `/api/codex/usage` automatically as a cache-read operation and update the existing account list without a full page reload. The dashboard SHALL NOT implement its own backend write-refresh decision path.

#### Scenario: Poll interval elapses
- **WHEN** the frontend poll interval elapses
- **THEN** the dashboard SHALL fetch updated cached usage records and refresh account-list usage fields without a full page reload

#### Scenario: Switch completes
- **WHEN** the frontend receives a successful account switch response
- **THEN** the dashboard SHALL refresh account and usage views so the newly active account can show updated remaining traffic produced by the backend refresh manager

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

### Requirement: Match couswee visual style
The integrated usage fields SHALL use the existing dark evergreen/everforest visual style and remain readable in the current couswee account list.

#### Scenario: Account list is rendered
- **WHEN** usage fields are visible
- **THEN** percentage badges, reset labels, and error states SHALL align with the existing dark green couswee theme

### Requirement: Render usage with SvelteKit state
The dashboard SHALL render usage data through SvelteKit component state and SHALL NOT require Astro components or the prior ApexCharts integration.

#### Scenario: Usage data changes
- **WHEN** `/api/codex/usage` returns updated records
- **THEN** the SvelteKit dashboard SHALL merge those records into account rows and update the comparison bars without a full page reload

