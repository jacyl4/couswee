package usage

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	UnitTokens  = "tokens"
	UnitUSD     = "usd"
	UnitPercent = "percent"

	SourceAPI      = "api"
	SourceFallback = "fallback"
	SourceSession  = "codex-session"
	SourceAccount  = "account"
)

const (
	MinRefreshInterval        = time.Minute
	DefaultRefreshInterval    = 5 * time.Minute
	MaxRefreshInterval        = 5 * time.Minute
	DefaultAuthRefreshTimeout = 30 * time.Second
	DefaultCodexUsageAPIURL   = "https://chatgpt.com/backend-api/wham/usage"
)

type UsageRecord struct {
	Account                    string    `json:"account"`
	Usage5h                    float64   `json:"5h_usage"`
	UsageWeekly                float64   `json:"weekly_usage"`
	Remaining5h                float64   `json:"5h_remaining"`
	RemainingWeekly            float64   `json:"weekly_remaining"`
	ResetTime                  string    `json:"reset_time"`
	ResetTime5h                string    `json:"5h_reset_time"`
	ResetTimeWeekly            string    `json:"weekly_reset_time"`
	UsageBasis                 string    `json:"usage_basis"`
	Unit                       string    `json:"unit"`
	Source                     string    `json:"source"`
	LastRefresh                time.Time `json:"last_refresh"`
	Stale                      bool      `json:"stale"`
	Error                      string    `json:"error"`
	HasWeeklyWindow            bool      `json:"has_weekly_window"`
	Availability               string    `json:"availability"`
	PlanType                   string    `json:"plan_type,omitempty"`
	RateLimitAllowed           *bool     `json:"rate_limit_allowed,omitempty"`
	RateLimitReachedType       string    `json:"rate_limit_reached_type,omitempty"`
	CreditsAvailable           *bool     `json:"credits_available,omitempty"`
	CreditsUnlimited           *bool     `json:"credits_unlimited,omitempty"`
	CreditsBalance             *string   `json:"credits_balance,omitempty"`
	CreditsApproxLocalMessages *int      `json:"credits_approx_local_messages,omitempty"`
	CreditsApproxCloudMessages *int      `json:"credits_approx_cloud_messages,omitempty"`
	CreditsOverageLimitReached *bool     `json:"credits_overage_limit_reached,omitempty"`
	SpendControlReached        *bool     `json:"spend_control_reached,omitempty"`
}

type Config struct {
	RefreshInterval    time.Duration
	Unit               string
	APIEnabled         bool
	APIURL             string
	AuthRefreshEnabled bool
	AuthRefreshTimeout time.Duration
	FallbackCommand    string
	FallbackTimeout    time.Duration
	SessionGlob        string
	ActiveAuthPath     string
}

func DefaultConfig() Config {
	return Config{
		RefreshInterval:    DefaultRefreshInterval,
		Unit:               UnitPercent,
		APIEnabled:         true,
		APIURL:             DefaultCodexUsageAPIURL,
		AuthRefreshEnabled: true,
		AuthRefreshTimeout: DefaultAuthRefreshTimeout,
		FallbackTimeout:    20 * time.Second,
	}
}

func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if unit := strings.TrimSpace(os.Getenv("COUSWEE_USAGE_UNIT")); unit != "" {
		cfg.Unit = strings.ToLower(unit)
	}
	if interval := strings.TrimSpace(os.Getenv("COUSWEE_USAGE_REFRESH_INTERVAL")); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			cfg.RefreshInterval = d
		}
	}
	rawAPIEnabled := strings.TrimSpace(os.Getenv("COUSWEE_USAGE_API_ENABLED"))
	if apiURL := strings.TrimSpace(os.Getenv("COUSWEE_USAGE_API_URL")); apiURL != "" {
		cfg.APIURL = apiURL
	}
	if rawAPIEnabled != "" {
		if enabled, err := strconv.ParseBool(rawAPIEnabled); err == nil {
			cfg.APIEnabled = enabled
		}
	}
	if rawAuthRefreshEnabled := strings.TrimSpace(os.Getenv("COUSWEE_USAGE_AUTH_REFRESH_ENABLED")); rawAuthRefreshEnabled != "" {
		if enabled, err := strconv.ParseBool(rawAuthRefreshEnabled); err == nil {
			cfg.AuthRefreshEnabled = enabled
		}
	}
	if timeout := strings.TrimSpace(os.Getenv("COUSWEE_USAGE_AUTH_REFRESH_TIMEOUT")); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.AuthRefreshTimeout = d
		}
	}
	cfg.FallbackCommand = strings.TrimSpace(os.Getenv("COUSWEE_USAGE_FALLBACK_CMD"))
	cfg.SessionGlob = ExpandHome(os.Getenv("COUSWEE_USAGE_SESSION_GLOB"))
	if timeout := strings.TrimSpace(os.Getenv("COUSWEE_USAGE_FALLBACK_TIMEOUT")); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.FallbackTimeout = d
		}
	}
	cfg.RefreshInterval = ClampRefreshInterval(cfg.RefreshInterval)
	if cfg.Unit == "" {
		cfg.Unit = UnitPercent
	}
	if cfg.FallbackTimeout <= 0 {
		cfg.FallbackTimeout = 20 * time.Second
	}
	if cfg.AuthRefreshTimeout <= 0 {
		cfg.AuthRefreshTimeout = DefaultAuthRefreshTimeout
	}
	return cfg
}

func ClampRefreshInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return DefaultRefreshInterval
	}
	if interval < MinRefreshInterval {
		return MinRefreshInterval
	}
	if interval > MaxRefreshInterval {
		return MaxRefreshInterval
	}
	return interval
}
