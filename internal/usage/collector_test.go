package usage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"couswee/internal/accounts"
)

type collectorFunc func(context.Context, accounts.Account) (UsageRecord, error)

func (f collectorFunc) Collect(ctx context.Context, account accounts.Account) (UsageRecord, error) {
	return f(ctx, account)
}

func TestAPICollectorSuccess(t *testing.T) {
	authPath := writeTestAuth(t, "test-access-token", "acct_123")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("account"); got != "Dev1" {
			t.Fatalf("account query = %q", got)
		}
		if got := req.URL.Query().Get("account_id"); got != "acct_123" {
			t.Fatalf("account_id query = %q", got)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Fatalf("authorization header = %q", got)
		}
		if got := req.Header.Get("ChatGPT-Account-Id"); got != "acct_123" {
			t.Fatalf("ChatGPT-Account-Id header = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"account":"Dev1","5h_usage":12,"weekly_usage":34,"reset_time":"2026-05-14T00:00:00+08:00"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	record, err := (APICollector{URL: "https://usage.example.test", Unit: UnitTokens, Client: client, Now: func() time.Time { return time.Unix(10, 0).UTC() }}).Collect(context.Background(), accounts.Account{Nickname: "Dev1", AuthPath: authPath})
	if err != nil {
		t.Fatal(err)
	}
	if record.Source != SourceAPI || record.Unit != UnitTokens || record.Usage5h != 12 || record.UsageWeekly != 34 || record.LastRefresh.IsZero() {
		t.Fatalf("unexpected record %#v", record)
	}
}

func TestReadCodexAuth(t *testing.T) {
	authPath := writeTestAuth(t, "token-abc", "acct_456")
	auth, err := ReadCodexAuth(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if auth.AccessToken != "token-abc" || auth.AccountID != "acct_456" {
		t.Fatalf("unexpected auth %#v", auth)
	}
}

func TestReadCodexAuthMetadata(t *testing.T) {
	exp := time.Unix(1783440824, 0).UTC()
	authPath := filepath.Join(t.TempDir(), "auth.json")
	body := `{"last_refresh":"2026-07-07T10:00:00Z","tokens":{"access_token":"` + testJWTExp(t, exp) + `","refresh_token":"refresh-abc","account_id":"acct_456"}}`
	if err := os.WriteFile(authPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	auth, err := ReadCodexAuth(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if auth.RefreshToken != "refresh-abc" || auth.AccountID != "acct_456" || !auth.AccessTokenExpiresAt.Equal(exp) || auth.LastRefresh.IsZero() {
		t.Fatalf("unexpected auth metadata %#v", auth)
	}
}

func TestReadCodexAuthExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	authDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatal(err)
	}
	authPath := filepath.Join(authDir, "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"tokens":{"access_token":"home-token","account_id":"acct_home"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	auth, err := ReadCodexAuth("~/.codex/auth.json")
	if err != nil {
		t.Fatal(err)
	}
	if auth.AccessToken != "home-token" || auth.AccountID != "acct_home" {
		t.Fatalf("unexpected auth %#v", auth)
	}
}

type fakeAuthRefresher struct {
	calls  int
	update func(string) error
	err    error
}

func (f *fakeAuthRefresher) RefreshCodexAuth(_ context.Context, authPath string) error {
	f.calls++
	if f.update != nil {
		if err := f.update(authPath); err != nil {
			return err
		}
	}
	return f.err
}

func TestAPICollectorRefreshesExpiredAuthBeforeRequest(t *testing.T) {
	now := time.Unix(1783400000, 0).UTC()
	expired := testJWTExp(t, now.Add(-time.Hour))
	fresh := testJWTExp(t, now.Add(time.Hour))
	authPath := writeTestAuth(t, expired, "acct_123")
	refresher := &fakeAuthRefresher{update: func(path string) error {
		if path != authPath {
			t.Fatalf("refresh path = %q, want %q", path, authPath)
		}
		return os.WriteFile(path, []byte(`{"tokens":{"access_token":"`+fresh+`","account_id":"acct_123"}}`), 0o600)
	}}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer "+fresh {
			t.Fatalf("authorization header = %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"rate_limit":{"primary_window":{"used_percent":12},"secondary_window":{"used_percent":34}}}`)), Header: make(http.Header)}, nil
	})}
	_, err := (APICollector{URL: "https://usage.example.test", Client: client, Now: func() time.Time { return now }, AuthRefresher: refresher}).Collect(context.Background(), accounts.Account{Nickname: "Dev1", AuthPath: authPath})
	if err != nil {
		t.Fatal(err)
	}
	if refresher.calls != 1 {
		t.Fatalf("refresh calls = %d, want 1", refresher.calls)
	}
}

func TestAPICollectorRefreshesAndRetriesOn401(t *testing.T) {
	now := time.Unix(1783400000, 0).UTC()
	oldToken := testJWTExp(t, now.Add(time.Hour))
	newToken := testJWTExp(t, now.Add(2*time.Hour))
	authPath := writeTestAuth(t, oldToken, "acct_123")
	refresher := &fakeAuthRefresher{update: func(path string) error {
		return os.WriteFile(path, []byte(`{"tokens":{"access_token":"`+newToken+`","account_id":"acct_123"}}`), 0o600)
	}}
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			if got := req.Header.Get("Authorization"); got != "Bearer "+oldToken {
				t.Fatalf("first authorization header = %q", got)
			}
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"token_expired","message":"expired"}}`)), Header: make(http.Header)}, nil
		case 2:
			if got := req.Header.Get("Authorization"); got != "Bearer "+newToken {
				t.Fatalf("second authorization header = %q", got)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"rate_limit":{"primary_window":{"used_percent":22},"secondary_window":{"used_percent":42}}}`)), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request %d", requests)
			return nil, nil
		}
	})}
	record, err := (APICollector{URL: "https://usage.example.test", Client: client, Now: func() time.Time { return now }, AuthRefresher: refresher}).Collect(context.Background(), accounts.Account{Nickname: "Dev1", AuthPath: authPath})
	if err != nil {
		t.Fatal(err)
	}
	if refresher.calls != 1 || requests != 2 || record.Remaining5h != 78 || record.RemainingWeekly != 58 {
		t.Fatalf("calls=%d requests=%d record=%#v", refresher.calls, requests, record)
	}
}

func TestParseUsageRecord(t *testing.T) {
	record, err := ParseUsageRecord([]byte(`{"account":"Dev1","5h_usage":1,"weekly_usage":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if record.Account != "Dev1" || record.UsageWeekly != 2 {
		t.Fatalf("unexpected record %#v", record)
	}
}

func TestOrchestratorFallbackSuccess(t *testing.T) {
	primaryErr := errors.New("api failed")
	orchestrator := Orchestrator{
		Primary: collectorFunc(func(context.Context, accounts.Account) (UsageRecord, error) { return UsageRecord{}, primaryErr }),
		Fallback: collectorFunc(func(context.Context, accounts.Account) (UsageRecord, error) {
			return UsageRecord{Account: "Dev1", Usage5h: 7, Source: SourceFallback}, nil
		}),
	}
	record, err := orchestrator.Collect(context.Background(), accounts.Account{Nickname: "Dev1"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Source != SourceFallback || record.Usage5h != 7 {
		t.Fatalf("unexpected record %#v", record)
	}
}

func TestOrchestratorAllFail(t *testing.T) {
	orchestrator := Orchestrator{
		Primary: collectorFunc(func(context.Context, accounts.Account) (UsageRecord, error) {
			return UsageRecord{}, errors.New("api failed")
		}),
		Fallback: collectorFunc(func(context.Context, accounts.Account) (UsageRecord, error) {
			return UsageRecord{}, errors.New("fallback failed")
		}),
	}
	_, err := orchestrator.Collect(context.Background(), accounts.Account{Nickname: "Dev1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOrchestratorAccountFallbackPreservesLiveError(t *testing.T) {
	orchestrator := Orchestrator{
		Primary: collectorFunc(func(context.Context, accounts.Account) (UsageRecord, error) {
			return UsageRecord{}, errors.New("api failed")
		}),
		AccountFallback: AccountCollector{},
	}
	record, err := orchestrator.Collect(context.Background(), accounts.Account{Nickname: "Dev1", Usage5h: 44, UsageWeekly: 55})
	if err != nil {
		t.Fatal(err)
	}
	if record.Source != SourceAccount || !record.Stale || !strings.Contains(record.Error, "api failed") {
		t.Fatalf("record = %#v", record)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAPICollectorUsesLiveAuthForActiveAccount(t *testing.T) {
	authPath := writeTestAuth(t, "active-token", "")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("auth_path"); got != authPath {
			t.Fatalf("auth_path query = %q", got)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer active-token" {
			t.Fatalf("authorization header = %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"account":"Dev1","5h_usage":88,"weekly_usage":42}`)), Header: make(http.Header)}, nil
	})}
	_, err := (APICollector{URL: "https://usage.example.test", Client: client, ActiveAuthPath: authPath}).Collect(context.Background(), accounts.Account{Nickname: "Dev1", AuthPath: "/backup/auth.json", Active: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseRateLimitRecord(t *testing.T) {
	record, err := ParseUsageRecord([]byte(`{"source":"codex","five_hour":{"used_percentage":19,"resets_at":1778678402},"seven_day":{"used_percentage":87,"resets_at":1779000935},"updated_at":1778662940}`))
	if err != nil {
		t.Fatal(err)
	}
	if record.Usage5h != 81 || record.UsageWeekly != 13 || record.Unit != UnitPercent || record.UsageBasis != "remaining" {
		t.Fatalf("unexpected record %#v", record)
	}
	if record.ResetTime == "" || record.ResetTime5h == "" || record.ResetTimeWeekly == "" || record.LastRefresh.IsZero() {
		t.Fatalf("missing reset/refresh %#v", record)
	}
}

func TestParseWhamUsageRecord(t *testing.T) {
	record, err := ParseUsageRecord([]byte(`{
		"account_id": "acct_123",
		"rate_limit": {
			"primary_window": {
				"limit_window_seconds": 18000,
				"reset_after_seconds": 17528,
				"reset_at": 1779372190,
				"used_percent": 12
			},
			"secondary_window": {
				"limit_window_seconds": 604800,
				"reset_after_seconds": 604328,
				"reset_at": 1779958990,
				"used_percent": 2
			}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if record.Usage5h != 88 || record.UsageWeekly != 98 || record.Remaining5h != 88 || record.RemainingWeekly != 98 {
		t.Fatalf("unexpected remaining values %#v", record)
	}
	if record.ResetTime5h == "" || record.ResetTimeWeekly == "" || record.UsageBasis != "remaining" || record.Unit != UnitPercent {
		t.Fatalf("unexpected metadata %#v", record)
	}
}

func TestParseWhamUsageRecordAllowsZeroUsedPercent(t *testing.T) {
	record, err := ParseUsageRecord([]byte(`{
		"rate_limit": {
			"primary_window": {"reset_at": 1779372190, "used_percent": 0},
			"secondary_window": {"reset_at": 1779958990, "used_percent": 0}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if record.Usage5h != 100 || record.UsageWeekly != 100 {
		t.Fatalf("unexpected zero-used remaining values %#v", record)
	}
}

func writeTestAuth(t *testing.T, token, accountID string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.json")
	body := `{"tokens":{"access_token":"` + token + `","account_id":"` + accountID + `"}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func testJWTExp(t *testing.T, exp time.Time) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(map[string]int64{"exp": exp.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestSessionLogCollectorUsesLatestCodexRateLimitEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	body := strings.Join([]string{
		`{"timestamp":"2026-05-13T09:21:39.953Z","type":"event_msg","payload":{"rate_limits":{"primary":{"used_percent":31.0,"window_minutes":300,"resets_at":1778678402},"secondary":{"used_percent":89.0,"window_minutes":10080,"resets_at":1779000935}}}}`,
		`{"timestamp":"2026-05-13T09:30:43.300Z","type":"event_msg","payload":{"rate_limits":{"primary":{"used_percent":44.0,"window_minutes":300,"resets_at":1778678402},"secondary":{"used_percent":91.0,"window_minutes":10080,"resets_at":1779000935}}}}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	record, err := (SessionLogCollector{Glob: filepath.Join(dir, "*.jsonl")}).Collect(context.Background(), accounts.Account{Nickname: "Dev1", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if record.Usage5h != 56 || record.UsageWeekly != 9 || record.ResetTime5h == "" || record.ResetTimeWeekly == "" {
		t.Fatalf("unexpected record %#v", record)
	}
}

func TestAPICollectorRejectsMismatchedAccountID(t *testing.T) {
	authPath := writeTestAuth(t, "token-abc", "acct_expected")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"account_id":"acct_other","5h_usage":88,"weekly_usage":42}`)), Header: make(http.Header)}, nil
	})}
	_, err := (APICollector{URL: "https://usage.example.test", Client: client}).Collect(context.Background(), accounts.Account{Nickname: "Dev1", AuthPath: authPath})
	if err == nil || strings.Contains(err.Error(), "token-abc") {
		t.Fatalf("Collect() error = %v, want sanitized mismatch error", err)
	}
}

func TestAPICollectorAcceptsWhamAccountIDNamespace(t *testing.T) {
	authPath := writeTestAuth(t, "token-abc", "auth-account-id")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{
			"account_id": "wham-account-id",
			"rate_limit": {
				"primary_window": {"reset_at": 1779372190, "used_percent": 12},
				"secondary_window": {"reset_at": 1779958990, "used_percent": 2}
			}
		}`)), Header: make(http.Header)}, nil
	})}
	record, err := (APICollector{URL: "https://usage.example.test", Client: client}).Collect(context.Background(), accounts.Account{Nickname: "Dev1", AuthPath: authPath})
	if err != nil {
		t.Fatal(err)
	}
	if record.Source != SourceAPI || record.Usage5h != 88 || record.UsageWeekly != 98 {
		t.Fatalf("unexpected record %#v", record)
	}
}

func TestAPICollectorMissingTokenErrorIsSanitized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"tokens":{"account_id":"acct_123"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (APICollector{URL: "https://usage.example.test"}).Collect(context.Background(), accounts.Account{Nickname: "Dev1", AuthPath: path})
	if err == nil || strings.Contains(err.Error(), "access_token\":\"") {
		t.Fatalf("Collect() error = %v, want sanitized missing token error", err)
	}
}

func TestSessionLogCollectorSkipsNonActiveAuth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	body := `{"timestamp":"2026-05-13T09:30:43.300Z","type":"event_msg","payload":{"rate_limits":{"primary":{"used_percent":44.0,"window_minutes":300},"secondary":{"used_percent":91.0,"window_minutes":10080}}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	activeAuthPath := writeTestAuth(t, "active-token", "acct_active")
	inactiveAuthPath := writeTestAuth(t, "inactive-token", "acct_inactive")
	_, err := (SessionLogCollector{Glob: filepath.Join(dir, "*.jsonl"), ActiveAuthPath: activeAuthPath}).Collect(context.Background(), accounts.Account{
		Nickname: "Dev1",
		AuthPath: inactiveAuthPath,
	})
	if !errors.Is(err, ErrNoCollector) {
		t.Fatalf("Collect() error = %v, want ErrNoCollector", err)
	}
}

func TestSessionLogCollectorSkipsEventsBeforeLastSwitch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	body := `{"timestamp":"2026-05-21T08:48:49.129Z","type":"event_msg","payload":{"rate_limits":{"primary":{"used_percent":60.0,"window_minutes":300},"secondary":{"used_percent":47.0,"window_minutes":10080}}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (SessionLogCollector{Glob: filepath.Join(dir, "*.jsonl")}).Collect(context.Background(), accounts.Account{
		Nickname:   "Dev1",
		Active:     true,
		LastUsedAt: "2026-05-21T08:50:12Z",
	})
	if !errors.Is(err, ErrNoCollector) {
		t.Fatalf("Collect() error = %v, want ErrNoCollector", err)
	}
}

func TestParseCurrentWhamUsageRecordWithCredits(t *testing.T) {
	record, err := ParseUsageRecord([]byte(`{
		"user_id": "user-redacted",
		"account_id": "user-redacted",
		"plan_type": "plus",
		"rate_limit": {
			"allowed": true,
			"limit_reached": false,
			"primary_window": {"used_percent": 8, "limit_window_seconds": 18000, "reset_after_seconds": 15683, "reset_at": 1780509758},
			"secondary_window": {"used_percent": 45, "limit_window_seconds": 604800, "reset_after_seconds": 414512, "reset_at": 1780908587}
		},
		"credits": {"has_credits": false, "unlimited": false, "balance": "0"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if record.Remaining5h != 92 || record.RemainingWeekly != 55 || record.UsageBasis != "remaining" || record.Unit != UnitPercent {
		t.Fatalf("record = %#v", record)
	}
	if record.ResetTime5h == "" || record.ResetTimeWeekly == "" {
		t.Fatalf("missing reset metadata: %#v", record)
	}
}
