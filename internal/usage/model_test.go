package usage

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestUsageRecordJSONFields(t *testing.T) {
	record := UsageRecord{Account: "Dev1", Usage5h: 10, UsageWeekly: 20, ResetTime: "2026-05-14T00:00:00+08:00", ResetTime5h: "2026-05-14T00:00:00+08:00", ResetTimeWeekly: "2026-05-17T00:00:00+08:00", UsageBasis: "remaining", Unit: UnitTokens, Source: SourceAPI, LastRefresh: time.Unix(1, 0).UTC(), Stale: true, Error: "boom"}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"account", "5h_usage", "weekly_usage", "5h_remaining", "weekly_remaining", "reset_time", "5h_reset_time", "weekly_reset_time", "usage_basis", "unit", "source", "last_refresh", "stale", "error"} {
		if _, ok := got[field]; !ok {
			t.Fatalf("missing JSON field %s in %s", field, data)
		}
	}
}

func TestClampRefreshInterval(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"zero defaults", 0, DefaultRefreshInterval},
		{"below min", 30 * time.Second, MinRefreshInterval},
		{"inside", 3 * time.Minute, 3 * time.Minute},
		{"above max", 10 * time.Minute, MaxRefreshInterval},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClampRefreshInterval(tc.in); got != tc.want {
				t.Fatalf("ClampRefreshInterval(%s) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestConfigFromEnvUsesDefaultCodexUsageAPI(t *testing.T) {
	t.Setenv("COUSWEE_USAGE_API_URL", "")
	t.Setenv("COUSWEE_USAGE_API_ENABLED", "")

	cfg := ConfigFromEnv()
	if !cfg.APIEnabled {
		t.Fatal("APIEnabled = false, want true")
	}
	if cfg.APIURL != DefaultCodexUsageAPIURL {
		t.Fatalf("APIURL = %q, want %q", cfg.APIURL, DefaultCodexUsageAPIURL)
	}
}

func TestConfigFromEnvCanDisableDefaultAPI(t *testing.T) {
	t.Setenv("COUSWEE_USAGE_API_URL", "")
	t.Setenv("COUSWEE_USAGE_API_ENABLED", "false")

	cfg := ConfigFromEnv()
	if cfg.APIEnabled {
		t.Fatal("APIEnabled = true, want false")
	}
	if cfg.APIURL != DefaultCodexUsageAPIURL {
		t.Fatalf("APIURL = %q, want default endpoint retained for diagnostics", cfg.APIURL)
	}
}

func TestConfigFromEnvOverridesDefaultAPIURL(t *testing.T) {
	t.Setenv("COUSWEE_USAGE_API_URL", "https://usage.example.test")
	t.Setenv("COUSWEE_USAGE_API_ENABLED", "")
	defer os.Unsetenv("COUSWEE_USAGE_API_URL")

	cfg := ConfigFromEnv()
	if !cfg.APIEnabled || cfg.APIURL != "https://usage.example.test" {
		t.Fatalf("unexpected API config: enabled=%v url=%q", cfg.APIEnabled, cfg.APIURL)
	}
}
