package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

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

func TestPostAccountDuplicate(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1", AuthPath: "~/auth.json"}})
	resp := doReq(t, srv, http.MethodPost, "/api/accounts", bytes.NewBufferString(`{"nickname":"Dev1","auth_path":"~/other.json"}`))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestDeleteAccounts(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}, {Nickname: "Dev2"}})
	resp := doReq(t, srv, http.MethodDelete, "/api/accounts", bytes.NewBufferString(`{"nicknames":["Dev1"]}`))
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

func TestGetCurrentNotFound(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}})
	resp := doReq(t, srv, http.MethodGet, "/api/current", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostSwitchRequiresNickname(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}})
	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostSwitchUnknown(t *testing.T) {
	srv := testApp(t, []accounts.Account{{Nickname: "Dev1"}})
	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{"nickname":"missing"}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostSwitchRefreshesUsage(t *testing.T) {
	home := t.TempDir()
	src1 := filepath.Join(home, "auth1.json")
	src2 := filepath.Join(home, "auth2.json")
	if err := os.WriteFile(src1, []byte(`{"account":"one"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src2, []byte(`{"account":"two"}`), 0o600); err != nil {
		t.Fatal(err)
	}
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
			Account:         account.Nickname,
			Remaining5h:     71,
			RemainingWeekly: 82,
			Unit:            usage.UnitPercent,
			Source:          usage.SourceAPI,
		}, nil
	}), accountService.Accounts)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})

	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{"nickname":"Dev2"}`))
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
	if err := os.WriteFile(src1, []byte(`{"account":"one"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src2, []byte(`{"account":"two"}`), 0o600); err != nil {
		t.Fatal(err)
	}
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
		if account.Nickname != "Dev2" {
			t.Fatalf("unexpected collection for %s", account.Nickname)
		}
		return usage.UsageRecord{Account: account.Nickname, Remaining5h: 91, RemainingWeekly: 92, Source: usage.SourceAPI}, nil
	}), accountService.Accounts)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})

	resp := doReq(t, srv, http.MethodPost, "/api/switch", bytes.NewBufferString(`{"nickname":"Dev2"}`))
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

func TestGetCodexUsageRecords(t *testing.T) {
	home := t.TempDir()
	store, err := accounts.OpenSQLiteStore(accounts.DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]accounts.Account{{Nickname: "Dev1", Usage5h: 1, UsageWeekly: 2}}); err != nil {
		t.Fatal(err)
	}
	accountService := accounts.NewService(store, home, accounts.NoopUsageRefresher{})
	usageService := usage.NewService(usage.DefaultConfig(), usage.AccountCollector{Unit: usage.UnitTokens}, accountService.Accounts)
	usageService.Refresh(nil)
	srv := New(accountService, Config{StaticDir: t.TempDir(), Usage: usageService})
	resp := doReq(t, srv, http.MethodGet, "/api/codex/usage", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
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
