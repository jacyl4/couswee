package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"couswee/internal/accounts"
	"couswee/internal/usage"
)

func testApp(t *testing.T, initial []accounts.Account) *Server {
	t.Helper()
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace(initial); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	service := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	return New(service, Config{StaticDir: t.TempDir()})
}

func doReq(t *testing.T, srv *Server, method, path string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := srv.App().Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	return resp
}

func writeTestAuthFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"tokens":{"access_token":"test-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultAddrAllowsLANAccess(t *testing.T) {
	if DefaultAddr != "0.0.0.0:2199" {
		t.Fatalf("DefaultAddr = %q, want 0.0.0.0:2199", DefaultAddr)
	}
}

func TestGetAccounts(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}})
	resp := doReq(t, srv, http.MethodGet, "/api/accounts", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostAccountCreatesAccount(t *testing.T) {
	srv := testApp(t, nil)
	resp := doReq(t, srv, http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"nickname":"Dev1","auth_path":"~/auth.json"}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	resp = doReq(t, srv, http.MethodGet, "/api/accounts", nil)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"nickname":"Dev1"`)) {
		t.Fatalf("body = %s", body)
	}
}

func TestPostAccountRefreshesUsage(t *testing.T) {
	home := t.TempDir()
	writeTestAuthFile(t, filepath.Join(home, "auth.json"))
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	var collected []string
	usageService := usage.NewService(usage.DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (usage.UsageRecord, error) {
		collected = append(collected, account.ProfileName)
		return usage.UsageRecord{
			Account:         account.ProfileName,
			Remaining5h:     63,
			RemainingWeekly: 77,
			HasWeeklyWindow: true,
			Availability:    "available",
			Unit:            usage.UnitPercent,
			Source:          usage.SourceAPI,
		}, nil
	}), accountService.Accounts)
	usageService.SetAccountSink(accountService.ReplaceUsage)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})

	resp := doReq(t, srv, http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"nickname":"Dev1","auth_path":"~/auth.json"}`))
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	if len(collected) != 1 || collected[0] != "Dev1" {
		t.Fatalf("collected = %#v, want only Dev1", collected)
	}
	var got accounts.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Usage5h != 0 || got.UsageWeekly != 77 || !got.HasWeeklyWindow || got.Availability != "available" || got.UsageSource != usage.SourceAPI {
		t.Fatalf("created account usage = %#v", got)
	}
	records := usageService.Records()
	if len(records) != 1 || records[0].Account != "Dev1" {
		t.Fatalf("records = %#v", records)
	}
}

func TestPostAccountDuplicate(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1", AuthPath: "~/auth.json"}})
	resp := doReq(t, srv, http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"nickname":"Dev1","auth_path":"~/other.json"}`))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestDeleteAccounts(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1", ProfileName: "dev-1"}, {Nickname: "Dev2", ProfileName: "dev-2"}})
	resp := doReq(t, srv, http.MethodDelete, "/api/accounts", bytes.NewBufferString(`{"profile_names":["dev-1"]}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	resp = doReq(t, srv, http.MethodGet, "/api/accounts", nil)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte(`"nickname":"Dev1"`)) || !bytes.Contains(body, []byte(`"nickname":"Dev2"`)) {
		t.Fatalf("body = %s", body)
	}
}

func TestDeleteAccountsPrunesUsageCache(t *testing.T) {
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]accounts.Account{{ID: "acc-1", Nickname: "Dev1", ProfileName: "dev-1"}, {ID: "acc-2", Nickname: "Dev2", ProfileName: "dev-2"}}); err != nil {
		t.Fatal(err)
	}
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	usageService := usage.NewService(usage.DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (usage.UsageRecord, error) {
		return usage.UsageRecord{Account: account.ProfileName, Remaining5h: 70, RemainingWeekly: 80, Unit: usage.UnitPercent, Source: usage.SourceAPI}, nil
	}), accountService.Accounts)
	usageService.Refresh(context.Background())
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})

	resp := doReq(t, srv, http.MethodDelete, "/api/accounts", bytes.NewBufferString(`{"profile_names":["dev-1"]}`))
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("delete status = %d body=%s", resp.StatusCode, body)
	}
	resp = doReq(t, srv, http.MethodGet, "/api/codex/usage", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("usage status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if bytes.Contains(body, []byte(`"account":"dev-1"`)) || !bytes.Contains(body, []byte(`"account":"dev-2"`)) {
		t.Fatalf("usage body = %s", body)
	}
}

func TestGetCurrentNotFound(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}})
	resp := doReq(t, srv, http.MethodGet, "/api/current", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostSwitchRequiresProfileOrID(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}})
	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostSwitchUnknown(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}})
	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{"profile_name":"missing"}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostSwitchRejectsNicknameOnly(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1", ProfileName: "dev-1"}})
	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{"nickname":"Dev1"}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostSwitchRefreshesUsage(t *testing.T) {
	home := t.TempDir()
	src1 := filepath.Join(home, "auth1.json")
	src2 := filepath.Join(home, "auth2.json")
	writeTestAuthFile(t, src1)
	writeTestAuthFile(t, src2)
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]accounts.Account{{Nickname: "Dev1", AuthPath: src1, Active: true}, {Nickname: "Dev2", AuthPath: src2}}); err != nil {
		t.Fatal(err)
	}
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	collects := 0
	usageService := usage.NewService(usage.DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (usage.UsageRecord, error) {
		collects++
		return usage.UsageRecord{
			Account:         account.ProfileName,
			Remaining5h:     71,
			RemainingWeekly: 82,
			Unit:            usage.UnitPercent,
			Source:          usage.SourceAPI,
		}, nil
	}), accountService.Accounts)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})

	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{"profile_name":"Dev2"}`))
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	if collects != 1 {
		t.Fatalf("usage refresh collected %d accounts, want 1", collects)
	}
	records := usageService.Records()
	if len(records) != 1 || records[0].Account != "Dev2" {
		t.Fatalf("records = %#v", records)
	}
}

func TestPostSwitchRefreshesActiveWithoutBlockingOnOtherAccounts(t *testing.T) {
	home := t.TempDir()
	src1 := filepath.Join(home, "auth1.json")
	src2 := filepath.Join(home, "auth2.json")
	writeTestAuthFile(t, src1)
	writeTestAuthFile(t, src2)
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]accounts.Account{{Nickname: "Dev1", AuthPath: src1, Active: true}, {Nickname: "Dev2", AuthPath: src2}}); err != nil {
		t.Fatal(err)
	}
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	usageService := usage.NewService(usage.DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (usage.UsageRecord, error) {
		if account.ProfileName != "Dev2" {
			t.Fatalf("unexpected collection for %s", account.ProfileName)
		}
		return usage.UsageRecord{Account: account.ProfileName, Remaining5h: 91, RemainingWeekly: 92, Source: usage.SourceAPI}, nil
	}), accountService.Accounts)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})

	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{"profile_name":"Dev2"}`))
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
}

func TestGetCodexUsageEmpty(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}})
	resp := doReq(t, srv, http.MethodGet, "/api/codex/usage", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestGetVersion(t *testing.T) {
	srv := testApp(t, nil)
	resp := doReq(t, srv, http.MethodGet, "/api/version", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["version"] != "dev" || got["commit"] != "none" || got["build_time"] != "unknown" {
		t.Fatalf("version response = %#v", got)
	}
}

func TestStaticFallbackServesFrontendRoutes(t *testing.T) {
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "fallback.html"), []byte("<html>fallback</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(service, Config{StaticDir: staticDir})

	resp := doReq(t, srv, http.MethodGet, "/settings/profile", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("fallback")) {
		t.Fatalf("body = %s", body)
	}
}

func TestStaticFallbackDoesNotCatchAPI404(t *testing.T) {
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "fallback.html"), []byte("<html>fallback</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(service, Config{StaticDir: staticDir})

	resp := doReq(t, srv, http.MethodGet, "/api/missing", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("fallback")) {
		t.Fatalf("body = %s", body)
	}
}

func TestEmbeddedStaticServesFrontendWhenDirectoryMissing(t *testing.T) {
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	srv := New(service, Config{
		StaticDir: filepath.Join(t.TempDir(), "missing"),
		StaticFS: fstest.MapFS{
			"index.html":    {Data: []byte("<html>embedded index</html>")},
			"fallback.html": {Data: []byte("<html>embedded fallback</html>")},
			"_app/env.js":   {Data: []byte("export {};")},
		},
	})

	resp := doReq(t, srv, http.MethodGet, "/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("embedded index")) {
		t.Fatalf("body = %s", body)
	}

	resp = doReq(t, srv, http.MethodGet, "/_app/env.js", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("asset status = %d", resp.StatusCode)
	}
}

func TestEmbeddedStaticFallbackDoesNotCatchAPI404(t *testing.T) {
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	srv := New(service, Config{
		StaticDir: filepath.Join(t.TempDir(), "missing"),
		StaticFS: fstest.MapFS{
			"fallback.html": {Data: []byte("<html>embedded fallback</html>")},
		},
	})

	resp := doReq(t, srv, http.MethodGet, "/api/missing", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("embedded fallback")) {
		t.Fatalf("body = %s", body)
	}
}

func TestGetCodexUsageRecords(t *testing.T) {
	home := t.TempDir()
	writeTestAuthFile(t, filepath.Join(home, "auth.json"))
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]accounts.Account{{Nickname: "Dev1", AuthPath: "~/auth.json", Usage5h: 1, UsageWeekly: 2}}); err != nil {
		t.Fatal(err)
	}
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	creditsAvailable := true
	usageService := usage.NewService(usage.DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (usage.UsageRecord, error) {
		return usage.UsageRecord{Account: account.ProfileName, RemainingWeekly: 42, HasWeeklyWindow: true, Availability: "credit_available", PlanType: "plus", CreditsAvailable: &creditsAvailable, Unit: usage.UnitPercent, Source: usage.SourceAPI}, nil
	}), accountService.Accounts)
	usageService.Refresh(nil)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})
	resp := doReq(t, srv, http.MethodGet, "/api/codex/usage", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var records []usage.UsageRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Usage5h != 0 || records[0].RemainingWeekly != 42 || !records[0].HasWeeklyWindow || records[0].Availability != "credit_available" || records[0].CreditsAvailable == nil || !*records[0].CreditsAvailable {
		t.Fatalf("records = %#v", records)
	}
}

func TestGetCodexUsageStaleRecord(t *testing.T) {
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]accounts.Account{{Nickname: "Dev1"}}); err != nil {
		t.Fatal(err)
	}
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	svc := usage.NewService(usage.DefaultConfig(), collectorFunc(func(context.Context, accounts.Account) (usage.UsageRecord, error) {
		return usage.UsageRecord{}, errors.New("boom")
	}), accountService.Accounts)
	svc.Refresh(context.Background())
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: svc})
	resp := doReq(t, srv, http.MethodGet, "/api/codex/usage", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestGetCodexUsageReturnsBlockedState(t *testing.T) {
	home := t.TempDir()
	writeTestAuthFile(t, filepath.Join(home, "auth.json"))
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]accounts.Account{{Nickname: "Dev1", AuthPath: "~/auth.json"}}); err != nil {
		t.Fatal(err)
	}
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	blocked := true
	usageService := usage.NewService(usage.DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (usage.UsageRecord, error) {
		return usage.UsageRecord{Account: account.ProfileName, RemainingWeekly: 85, HasWeeklyWindow: true, Availability: "blocked", SpendControlReached: &blocked, Source: usage.SourceAPI}, nil
	}), accountService.Accounts)
	usageService.Refresh(nil)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})
	resp := doReq(t, srv, http.MethodGet, "/api/codex/usage", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var records []usage.UsageRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Availability != "blocked" || records[0].SpendControlReached == nil || !*records[0].SpendControlReached {
		t.Fatalf("records = %#v", records)
	}
}

type collectorFunc func(context.Context, accounts.Account) (usage.UsageRecord, error)

func (f collectorFunc) Collect(ctx context.Context, account accounts.Account) (usage.UsageRecord, error) {
	return f(ctx, account)
}

func testSQLiteApp(t *testing.T, initial []accounts.Account) *Server {
	t.Helper()
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace(initial); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	service := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	return New(service, Config{StaticDir: t.TempDir()})
}

func TestPatchAccountSQLite(t *testing.T) {
	srv := testSQLiteApp(t, []accounts.Account{{ID: "acc-1", Nickname: "Dev1", AuthPath: "~/auth.json"}})
	resp := doReq(t, srv, http.MethodPatch, "/api/accounts/acc-1", bytes.NewBufferString(`{"nickname":"DevRenamed","subscription":"team"}`))
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	resp = doReq(t, srv, http.MethodGet, "/api/accounts", nil)
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte(`"nickname":"DevRenamed"`)) || bytes.Contains(body, []byte(`"display_name"`)) {
		t.Fatalf("body = %s", body)
	}
}

func TestLoginAPIsSQLite(t *testing.T) {
	srv := testSQLiteApp(t, nil)
	resp := doReq(t, srv, http.MethodPost, "/api/codex/login/start", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("login start status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"method":"device"`)) || !bytes.Contains(body, []byte(`"status":"waiting_user"`)) || bytes.Contains(body, []byte("token")) {
		t.Fatalf("login body = %s", body)
	}
	resp = doReq(t, srv, http.MethodPost, "/api/codex/login/oauth/start", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("oauth compatibility start status = %d", resp.StatusCode)
	}
	body, _ = io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte(`"method":"device"`)) || bytes.Contains(body, []byte("refresh_token")) {
		t.Fatalf("oauth compatibility body = %s", body)
	}
}

func TestLoginStatusRefreshesUsageForSucceededAccount(t *testing.T) {
	home := t.TempDir()
	writeTestAuthFile(t, filepath.Join(home, "auth.json"))
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]accounts.Account{{ID: "acc-1", Nickname: "Dev1", AuthPath: "~/auth.json"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateLoginSession(accounts.LoginSession{
		ID:        "login-1",
		Method:    accounts.LoginMethodDevice,
		Status:    accounts.LoginStatusSucceeded,
		AccountID: "acc-1",
	}); err != nil {
		t.Fatal(err)
	}
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	var collected []string
	usageService := usage.NewService(usage.DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (usage.UsageRecord, error) {
		collected = append(collected, account.ID)
		return usage.UsageRecord{
			Account:         account.ProfileName,
			Remaining5h:     41,
			RemainingWeekly: 82,
			HasWeeklyWindow: true,
			Availability:    "available",
			Unit:            usage.UnitPercent,
			Source:          usage.SourceAPI,
		}, nil
	}), accountService.Accounts)
	usageService.SetAccountSink(accountService.ReplaceUsage)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})

	resp := doReq(t, srv, http.MethodGet, "/api/codex/login/login-1", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, body)
	}
	if len(collected) != 1 || collected[0] != "acc-1" {
		t.Fatalf("collected = %#v, want only acc-1", collected)
	}
	updated := accountService.Accounts()
	if len(updated) != 1 || updated[0].Usage5h != 0 || updated[0].UsageWeekly != 82 || !updated[0].HasWeeklyWindow || updated[0].Availability != "available" || updated[0].UsageSource != usage.SourceAPI {
		t.Fatalf("account usage = %#v", updated)
	}

	resp = doReq(t, srv, http.MethodGet, "/api/codex/login/login-1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d", resp.StatusCode)
	}
	if len(collected) != 1 {
		t.Fatalf("second status refreshed again: %#v", collected)
	}
}
