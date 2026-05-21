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
	MinRefreshInterval      = time.Minute
	DefaultRefreshInterval  = 5 * time.Minute
	MaxRefreshInterval      = 5 * time.Minute
	DefaultCodexUsageAPIURL = "https://chatgpt.com/backend-api/wham/usage"
)

type UsageRecord struct {
	Account         string    `json:"account"`
	Usage5h         float64   `json:"5h_usage"`
	UsageWeekly     float64   `json:"weekly_usage"`
	Remaining5h     float64   `json:"5h_remaining"`
	RemainingWeekly float64   `json:"weekly_remaining"`
	ResetTime       string    `json:"reset_time"`
	ResetTime5h     string    `json:"5h_reset_time"`
	ResetTimeWeekly string    `json:"weekly_reset_time"`
	UsageBasis      string    `json:"usage_basis"`
	Unit            string    `json:"unit"`
	Source          string    `json:"source"`
	LastRefresh     time.Time `json:"last_refresh"`
	Stale           bool      `json:"stale"`
	Error           string    `json:"error"`
}

type Config struct {
	RefreshInterval time.Duration
	Unit            string
	APIEnabled      bool
	APIURL          string
	FallbackCommand string
	FallbackTimeout time.Duration
	SessionGlob     string
	ActiveAuthPath  string
}

func DefaultConfig() Config {
	return Config{
		RefreshInterval: DefaultRefreshInterval,
		Unit:            UnitPercent,
		APIEnabled:      true,
		APIURL:          DefaultCodexUsageAPIURL,
		FallbackTimeout: 20 * time.Second,
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
