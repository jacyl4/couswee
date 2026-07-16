package accounts

// Account is the JSON contract shared by the store, API, and dashboard.
// Legacy fields are preserved for compatibility with existing frontends and
// usage collectors; SQLite-backed fields provide stable identity and profile
// metadata for the login workflow.
type Account struct {
	ID                         string  `json:"id"`
	Nickname                   string  `json:"nickname"`
	ProfileName                string  `json:"profile_name"`
	AuthPath                   string  `json:"auth_path"`
	LoginMethod                string  `json:"login_method"`
	Status                     string  `json:"status"`
	Subscription               string  `json:"subscription"`
	Usage5h                    int     `json:"5h_usage"`
	UsageWeekly                int     `json:"weekly_usage"`
	ResetTime5h                string  `json:"5h_reset_time,omitempty"`
	ResetTimeWeekly            string  `json:"weekly_reset_time,omitempty"`
	UsageSource                string  `json:"usage_source,omitempty"`
	UsageLastRefresh           string  `json:"usage_last_refresh,omitempty"`
	UsageStale                 bool    `json:"usage_stale,omitempty"`
	UsageError                 string  `json:"usage_error,omitempty"`
	HasWeeklyWindow            bool    `json:"has_weekly_window"`
	Availability               string  `json:"availability,omitempty"`
	PlanType                   string  `json:"plan_type,omitempty"`
	RateLimitAllowed           *bool   `json:"rate_limit_allowed,omitempty"`
	RateLimitReachedType       string  `json:"rate_limit_reached_type,omitempty"`
	CreditsAvailable           *bool   `json:"credits_available,omitempty"`
	CreditsUnlimited           *bool   `json:"credits_unlimited,omitempty"`
	CreditsBalance             *string `json:"credits_balance,omitempty"`
	CreditsApproxLocalMessages *int    `json:"credits_approx_local_messages,omitempty"`
	CreditsApproxCloudMessages *int    `json:"credits_approx_cloud_messages,omitempty"`
	CreditsOverageLimitReached *bool   `json:"credits_overage_limit_reached,omitempty"`
	SpendControlReached        *bool   `json:"spend_control_reached,omitempty"`
	AuthStatus                 string  `json:"auth_status,omitempty"`
	AuthExpired                bool    `json:"auth_expired,omitempty"`
	AuthExpiresAt              string  `json:"auth_expires_at,omitempty"`
	AuthLastRefresh            string  `json:"auth_last_refresh,omitempty"`
	AuthError                  string  `json:"auth_error,omitempty"`
	Active                     bool    `json:"active"`
	LastUsedAt                 string  `json:"last_used_at,omitempty"`
	CreatedAt                  string  `json:"created_at,omitempty"`
	UpdatedAt                  string  `json:"updated_at,omitempty"`
}

const (
	LoginMethodImported = "imported"
	LoginMethodDevice   = "device"

	AccountStatusReady        = "ready"
	AccountStatusActive       = "active"
	AccountStatusLoginPending = "login_pending"
	AccountStatusLoginFailed  = "login_failed"
	AccountStatusDisabled     = "disabled"
)
