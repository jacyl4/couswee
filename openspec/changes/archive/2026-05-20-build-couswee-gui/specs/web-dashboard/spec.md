## ADDED Requirements

### Requirement: Display account list
The system SHALL provide a web dashboard that lists configured accounts with nickname, subscription date, 5-hour usage, weekly usage, active marker, and a switch control.

#### Scenario: Dashboard opens with accounts
- **WHEN** a user opens the couswee dashboard
- **THEN** the dashboard SHALL display one row per configured account with the required account fields and controls

### Requirement: Highlight active account
The dashboard SHALL visually distinguish the active account using the evergreen/everforest dark green theme.

#### Scenario: Active account is present
- **WHEN** the dashboard renders an account marked active
- **THEN** that account row SHALL be highlighted and its active status SHALL be clear to the user

### Requirement: Visualize usage timeline
The dashboard SHALL include a Gantt-style or bar-based usage visualization for comparing accounts over a 24-hour or 7-day window.

#### Scenario: Usage visualization renders
- **WHEN** account usage data is available
- **THEN** the dashboard SHALL render a usage chart with accounts on one axis and usage/time representation on the other

### Requirement: Refresh account data from API
The dashboard SHALL periodically fetch account data from the backend API and update the visible account list and usage visualization.

#### Scenario: Backend data changes
- **WHEN** refreshed API data differs from the currently displayed account data
- **THEN** the dashboard SHALL update the account list and visualization without requiring a full page reload

### Requirement: Use SvelteKit frontend stack
The dashboard SHALL be implemented as a SvelteKit application and SHALL NOT depend on Astro runtime, Astro components, or Astro configuration.

#### Scenario: Frontend stack is inspected
- **WHEN** a developer inspects frontend package scripts and source files
- **THEN** they SHALL find SvelteKit routes/components and SHALL NOT find active Astro config or `.astro` component source in the application
