package accounts

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRefresher struct {
	next    []Account
	changed bool
	err     error
}

func (f fakeRefresher) Refresh([]Account) ([]Account, bool, error) {
	return f.next, f.changed, f.err
}

func newTestService(t *testing.T, initial []Account) (*Service, *SQLiteStore, string) {
	t.Helper()
	home := t.TempDir()
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace(initial); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	return NewService(store, home, NoopUsageRefresher{}), store, home
}

func TestCurrent(t *testing.T) {
	service, _, _ := newTestService(t, []Account{{Nickname: "Dev1", Active: true}})
	got, err := service.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if got.Nickname != "Dev1" {
		t.Fatalf("Current() nickname = %q", got.Nickname)
	}
}

func TestCurrentNoActive(t *testing.T) {
	service, _, _ := newTestService(t, []Account{{Nickname: "Dev1"}})
	_, err := service.Current()
	if !errors.Is(err, ErrNoActiveAccount) {
		t.Fatalf("Current() error = %v, want ErrNoActiveAccount", err)
	}
}

func TestSwitchSuccess(t *testing.T) {
	home := t.TempDir()
	src1 := filepath.Join(home, "auth1.json")
	src2 := filepath.Join(home, "auth2.json")
	if err := os.WriteFile(src1, []byte(`{"account":"one"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src2, []byte(`{"account":"two"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", AuthPath: src1, Active: true}, {Nickname: "Dev2", AuthPath: src2}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, NoopUsageRefresher{})

	selected, err := service.Switch("Dev2")
	if err != nil {
		t.Fatalf("Switch() error = %v", err)
	}
	if selected.Nickname != "Dev2" || !selected.Active {
		t.Fatalf("selected = %#v", selected)
	}
	if selected.LastUsedAt == "" {
		t.Fatalf("selected LastUsedAt is empty: %#v", selected)
	}
	content, err := os.ReadFile(CodexAuthPath(home))
	if err != nil {
		t.Fatalf("read active auth: %v", err)
	}
	if string(content) != `{"account":"two"}` {
		t.Fatalf("auth content = %s", content)
	}
	accounts := store.Accounts()
	active := ""
	for _, account := range accounts {
		if account.Active {
			active = account.Nickname
			if account.LastUsedAt == "" {
				t.Fatalf("active account LastUsedAt is empty: %#v", account)
			}
		}
	}
	if active != "Dev2" {
		t.Fatalf("active markers = %#v", accounts)
	}
}

func TestSwitchUnknownDoesNotChangeState(t *testing.T) {
	service, store, _ := newTestService(t, []Account{{Nickname: "Dev1", Active: true}})
	_, err := service.Switch("missing")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("Switch() error = %v, want ErrAccountNotFound", err)
	}
	if !store.Accounts()[0].Active {
		t.Fatalf("active state changed after unknown switch")
	}
}

func TestAddAccountPersists(t *testing.T) {
	service, store, _ := newTestService(t, []Account{{Nickname: "Dev1", AuthPath: "~/one.json", Active: true}})
	added, err := service.Add(Account{Nickname: "Dev2", AuthPath: "~/two.json", Subscription: "2026-06-01"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added.Nickname != "Dev2" {
		t.Fatalf("added = %#v", added)
	}
	got := store.Accounts()
	found := false
	for _, account := range got {
		if account.Nickname == "Dev2" {
			found = true
		}
	}
	if len(got) != 2 || !found {
		t.Fatalf("accounts = %#v", got)
	}
}

func TestAddDuplicateRejected(t *testing.T) {
	service, _, _ := newTestService(t, []Account{{Nickname: "Dev1", AuthPath: "~/one.json"}})
	_, err := service.Add(Account{Nickname: "Dev1", AuthPath: "~/other.json"})
	if !errors.Is(err, ErrDuplicateAccount) {
		t.Fatalf("Add() error = %v, want ErrDuplicateAccount", err)
	}
}

func TestAddDuplicateNicknameWithDistinctProfileAllowed(t *testing.T) {
	service, store, _ := newTestService(t, []Account{{Nickname: "Dev", ProfileName: "dev-main", AuthPath: "~/one.json"}})
	_, err := service.Add(Account{Nickname: "Dev", ProfileName: "dev-backup", AuthPath: "~/other.json"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got := store.Accounts(); len(got) != 2 {
		t.Fatalf("accounts = %#v", got)
	}
}

func TestDeleteAccountsPersists(t *testing.T) {
	service, store, _ := newTestService(t, []Account{{Nickname: "Dev1", ProfileName: "dev-1"}, {Nickname: "Dev2", ProfileName: "dev-2"}, {Nickname: "Dev3", ProfileName: "dev-3"}})
	deleted, err := service.Delete([]string{"dev-1", "dev-3"})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	got := store.Accounts()
	if len(got) != 1 || got[0].Nickname != "Dev2" {
		t.Fatalf("accounts = %#v", got)
	}
}

func TestDeleteUnknownRejected(t *testing.T) {
	service, _, _ := newTestService(t, []Account{{Nickname: "Dev1"}})
	_, err := service.Delete([]string{"missing"})
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("Delete() error = %v, want ErrAccountNotFound", err)
	}
}

func TestSwitchUnreadableSourceDoesNotChangeState(t *testing.T) {
	service, store, _ := newTestService(t, []Account{{Nickname: "Dev1", AuthPath: "/no/such/auth.json", Active: false}, {Nickname: "Dev2", AuthPath: "/no/such/auth2.json", Active: true}})
	_, err := service.Switch("Dev1")
	if err == nil {
		t.Fatalf("Switch() expected error")
	}
	accounts := store.Accounts()
	active := ""
	for _, account := range accounts {
		if account.Active {
			active = account.Nickname
		}
	}
	if active != "Dev2" {
		t.Fatalf("active markers changed after failed switch: %#v", accounts)
	}
}

func TestRefreshUsagePersistsChangedValues(t *testing.T) {
	home := t.TempDir()
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", Usage5h: 1, UsageWeekly: 2}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, fakeRefresher{changed: true, next: []Account{{Nickname: "Dev1", Usage5h: 3, UsageWeekly: 4}}})
	if err := service.RefreshUsage(); err != nil {
		t.Fatalf("RefreshUsage() error = %v", err)
	}
	if got := store.Accounts()[0]; got.Usage5h != 3 || got.UsageWeekly != 4 {
		t.Fatalf("refreshed account = %#v", got)
	}
}

func TestReplaceUsagePersistsChangedValues(t *testing.T) {
	home := t.TempDir()
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", Usage5h: 0, UsageWeekly: 0}, {Nickname: "Dev2", Usage5h: 9, UsageWeekly: 8}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, NoopUsageRefresher{})
	if err := service.ReplaceUsage([]Account{{ProfileName: "Dev1", Usage5h: 55, UsageWeekly: 66, ResetTime5h: "2026-05-20T23:00:00+08:00", ResetTimeWeekly: "2026-05-24T23:00:00+08:00", UsageSource: "api", UsageLastRefresh: "2026-05-21T00:00:00Z", UsageStale: true, UsageError: "temporary failure"}}); err != nil {
		t.Fatalf("ReplaceUsage() error = %v", err)
	}
	accounts := store.Accounts()
	if accounts[0].Nickname != "Dev1" ||
		accounts[0].Usage5h != 55 ||
		accounts[0].UsageWeekly != 66 ||
		accounts[0].ResetTime5h == "" ||
		accounts[0].ResetTimeWeekly == "" ||
		accounts[0].UsageSource != "api" ||
		accounts[0].UsageLastRefresh == "" ||
		!accounts[0].UsageStale ||
		accounts[0].UsageError == "" {
		t.Fatalf("Dev1 = %#v", accounts[0])
	}
	if accounts[1].Nickname != "Dev2" || accounts[1].Usage5h != 9 || accounts[1].UsageWeekly != 8 {
		t.Fatalf("Dev2 = %#v", accounts[1])
	}
}

func TestSyncActiveFromAuthFileMatchesBackup(t *testing.T) {
	home := t.TempDir()
	src1 := filepath.Join(home, "auth1.json")
	src2 := filepath.Join(home, "auth2.json")
	if err := os.WriteFile(src1, []byte(`{"account":"one"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src2, []byte(`{"account":"two"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CodexAuthPath(home), []byte(`{"account":"two"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", AuthPath: src1, Active: true}, {Nickname: "Dev2", AuthPath: src2}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, NoopUsageRefresher{})
	if err := service.SyncActiveFromAuthFile(); err != nil {
		t.Fatalf("SyncActiveFromAuthFile() error = %v", err)
	}
	accounts := store.Accounts()
	active := ""
	for _, account := range accounts {
		if account.Active {
			active = account.Nickname
			if account.LastUsedAt == "" {
				t.Fatalf("active account LastUsedAt is empty: %#v", account)
			}
		}
	}
	if active != "Dev2" {
		t.Fatalf("active markers = %#v", accounts)
	}
}

func TestSyncActiveFromAuthFileMatchesAccountID(t *testing.T) {
	home := t.TempDir()
	src1 := filepath.Join(home, "auth1.json")
	src2 := filepath.Join(home, "auth2.json")
	if err := os.WriteFile(src1, []byte(`{"tokens":{"access_token":"old-one","account_id":"acct_one"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src2, []byte(`{"tokens":{"access_token":"backup-two","account_id":"acct_two"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CodexAuthPath(home), []byte(`{"tokens":{"access_token":"live-two","account_id":"acct_two"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", AuthPath: src1, Active: true}, {Nickname: "Dev2", AuthPath: src2}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, NoopUsageRefresher{})
	if err := service.SyncActiveFromAuthFile(); err != nil {
		t.Fatalf("SyncActiveFromAuthFile() error = %v", err)
	}
	accounts := store.Accounts()
	active := ""
	for _, account := range accounts {
		if account.Active {
			active = account.Nickname
		}
	}
	if active != "Dev2" {
		t.Fatalf("active markers = %#v", accounts)
	}
}

func TestSyncActiveFromAuthFileWritesFreshManagedProfile(t *testing.T) {
	home := t.TempDir()
	profile := NewProfileService(home)
	managedPath, err := profile.WriteAuth("dev-two", []byte(`{"tokens":{"access_token":"backup-two","account_id":"acct_two"}}`))
	if err != nil {
		t.Fatal(err)
	}
	activeBody := `{"last_refresh":"2026-07-07T10:00:00Z","tokens":{"access_token":"live-two","account_id":"acct_two"}}`
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CodexAuthPath(home), []byte(activeBody), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", AuthPath: filepath.Join(home, "missing.json"), Active: true}, {Nickname: "Dev2", ProfileName: "dev-two", AuthPath: managedPath}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, NoopUsageRefresher{})
	if err := service.SyncActiveFromAuthFile(); err != nil {
		t.Fatalf("SyncActiveFromAuthFile() error = %v", err)
	}
	synced, err := os.ReadFile(managedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(synced) != activeBody {
		t.Fatalf("managed auth = %s, want %s", synced, activeBody)
	}
	if got := store.Accounts()[1]; !got.Active || got.LastUsedAt == "" {
		t.Fatalf("synced account = %#v", got)
	}
}

func TestSyncActiveFromAuthFileCopiesManagedConfigFieldsToIdleProfiles(t *testing.T) {
	home := t.TempDir()
	profile := NewProfileService(home)
	activePath, err := profile.WriteAuth("dev-one", []byte(`{"auth_mode":"old","tokens":{"access_token":"profile-active","account_id":"acct_active"},"last_refresh":"2026-01-01T00:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	idlePath, err := profile.WriteAuth("dev-two", []byte(`{"auth_mode":"old","tokens":{"access_token":"idle-token","account_id":"acct_idle"},"last_refresh":"2026-01-02T00:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	activeBody := `{"auth_mode":"chatgpt","OPENAI_API_KEY":null,"codex_schema":{"version":2},"tokens":{"access_token":"live-active","account_id":"acct_active"},"last_refresh":"2026-07-07T10:00:00Z"}`
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CodexAuthPath(home), []byte(activeBody), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", ProfileName: "dev-one", AuthPath: activePath, Active: true}, {Nickname: "Dev2", ProfileName: "dev-two", AuthPath: idlePath}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, NoopUsageRefresher{})
	if err := service.SyncActiveFromAuthFile(); err != nil {
		t.Fatalf("SyncActiveFromAuthFile() error = %v", err)
	}
	var idle map[string]any
	readJSONFile(t, idlePath, &idle)
	if idle["auth_mode"] != "chatgpt" {
		t.Fatalf("auth_mode = %#v", idle["auth_mode"])
	}
	if _, ok := idle["codex_schema"].(map[string]any); !ok {
		t.Fatalf("codex_schema missing from idle auth: %#v", idle)
	}
	tokens := idle["tokens"].(map[string]any)
	if tokens["access_token"] != "idle-token" || tokens["account_id"] != "acct_idle" || idle["last_refresh"] != "2026-01-02T00:00:00Z" {
		t.Fatalf("idle account-specific fields were not preserved: %#v", idle)
	}
}

func TestSyncActiveFromAuthFileDoesNotCopyNonEmptySensitiveConfigFields(t *testing.T) {
	home := t.TempDir()
	profile := NewProfileService(home)
	activePath, err := profile.WriteAuth("dev-one", []byte(`{"auth_mode":"old","tokens":{"access_token":"profile-active","account_id":"acct_active"}}`))
	if err != nil {
		t.Fatal(err)
	}
	idlePath, err := profile.WriteAuth("dev-two", []byte(`{"auth_mode":"old","OPENAI_API_KEY":{"api_key":"idle-secret"},"tokens":{"access_token":"idle-token","account_id":"acct_idle"}}`))
	if err != nil {
		t.Fatal(err)
	}
	activeBody := `{"auth_mode":"chatgpt","OPENAI_API_KEY":{"api_key":"active-secret"},"tokens":{"access_token":"live-active","account_id":"acct_active"}}`
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(CodexAuthPath(home), []byte(activeBody), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", ProfileName: "dev-one", AuthPath: activePath, Active: true}, {Nickname: "Dev2", ProfileName: "dev-two", AuthPath: idlePath}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, NoopUsageRefresher{})
	if err := service.SyncActiveFromAuthFile(); err != nil {
		t.Fatalf("SyncActiveFromAuthFile() error = %v", err)
	}
	var idle map[string]any
	readJSONFile(t, idlePath, &idle)
	apiKey := idle["OPENAI_API_KEY"].(map[string]any)
	if apiKey["api_key"] != "idle-secret" {
		t.Fatalf("sensitive config was overwritten: %#v", idle)
	}
}

func TestAccountsAnnotatesExpiredAuth(t *testing.T) {
	home := t.TempDir()
	expiredAt := time.Now().Add(-time.Hour).UTC()
	authPath := filepath.Join(home, "expired-auth.json")
	body := `{"tokens":{"access_token":"` + testJWTExp(t, expiredAt) + `","account_id":"acct_expired"},"last_refresh":"2026-07-07T10:00:00Z"}`
	if err := os.WriteFile(authPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenSQLiteStore(DBPath(home))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Expired", AuthPath: authPath}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, home, NoopUsageRefresher{})
	got := service.Accounts()
	if len(got) != 1 || got[0].AuthStatus != "expired" || !got[0].AuthExpired || got[0].AuthExpiresAt == "" || got[0].AuthLastRefresh == "" {
		t.Fatalf("accounts = %#v", got)
	}
}

func TestLoginEnvIsolatesCodexHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "session-home")
	codexHome := IsolatedCodexHomePath(home)
	env := loginEnv([]string{"HOME=/real/home", "CODEX_HOME=/real/codex", "PATH=/bin"}, home, codexHome)
	seen := map[string]string{}
	for _, value := range env {
		if strings.HasPrefix(value, "HOME=") {
			seen["HOME"] = strings.TrimPrefix(value, "HOME=")
		}
		if strings.HasPrefix(value, "CODEX_HOME=") {
			seen["CODEX_HOME"] = strings.TrimPrefix(value, "CODEX_HOME=")
		}
	}
	if seen["HOME"] != home || seen["CODEX_HOME"] != codexHome {
		t.Fatalf("env = %#v", env)
	}
}

func TestMergeLoginOutputParsesCurrentCodexDeviceFormat(t *testing.T) {
	var start LoginStart
	mergeLoginOutput(&start, "\x1b[94mhttps://auth.openai.com/codex/device\x1b[0m")
	mergeLoginOutput(&start, "   \x1b[94m2614-ERVJL\x1b[0m")
	if start.VerificationURL != "https://auth.openai.com/codex/device" || start.UserCode != "2614-ERVJL" {
		t.Fatalf("start = %#v", start)
	}
}

func TestMergeLoginOutputParsesEmbeddedUserCodeURL(t *testing.T) {
	var start LoginStart
	mergeLoginOutput(&start, "Open https://auth.openai.com/codex/device?user_code=ABCD-12345.")
	if start.VerificationURL != "https://auth.openai.com/codex/device?user_code=ABCD-12345" || start.UserCode != "ABCD-12345" {
		t.Fatalf("start = %#v", start)
	}
}

func TestCodexAuthPathRespectsCodexHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "~/custom-codex")
	want := filepath.Join(home, "custom-codex", "auth.json")
	if got := CodexAuthPath(home); got != want {
		t.Fatalf("CodexAuthPath() = %q, want %q", got, want)
	}
}

func TestIsolatedCodexHomePath(t *testing.T) {
	home := t.TempDir()
	if got, want := IsolatedCodexHomePath(home), filepath.Join(home, ".codex"); got != want {
		t.Fatalf("IsolatedCodexHomePath() = %q, want %q", got, want)
	}
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
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
