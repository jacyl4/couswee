package accounts

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newSQLiteTestService(t *testing.T, initial []Account) (*Service, *SQLiteStore, string) {
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

func TestSQLiteStoreCRUDAndSingleActive(t *testing.T) {
	service, store, _ := newSQLiteTestService(t, nil)
	first, err := service.Add(Account{Nickname: "main-dev", AuthPath: "~/main.json", Active: true})
	if err != nil {
		t.Fatalf("Add first: %v", err)
	}
	second, err := service.Add(Account{Nickname: "backup-01", AuthPath: "~/backup.json", Active: true})
	if err != nil {
		t.Fatalf("Add second: %v", err)
	}
	if first.ID == "" || second.ID == "" || second.ProfileName == "" {
		t.Fatalf("missing sqlite metadata: %#v %#v", first, second)
	}
	accounts := store.Accounts()
	activeCount := 0
	for _, account := range accounts {
		if account.Active {
			activeCount++
			if account.Nickname != "backup-01" {
				t.Fatalf("wrong active account: %#v", accounts)
			}
		}
	}
	if len(accounts) != 2 || activeCount != 1 {
		t.Fatalf("single active not enforced: %#v", accounts)
	}
	updated, err := service.UpdateAccount(second.ID, Account{Nickname: "backup-renamed", Subscription: "team"})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	if updated.Nickname != "backup-renamed" || updated.Subscription != "team" {
		t.Fatalf("updated = %#v", updated)
	}
	deleted, err := service.DeleteSelectors([]string{first.ID})
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteSelectors() = %d, %v", deleted, err)
	}
}

func TestSQLiteStoreMigratesResetTimeColumns(t *testing.T) {
	home := t.TempDir()
	dbPath := DBPath(home)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE accounts (
		id TEXT PRIMARY KEY,
		nickname TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL DEFAULT '',
		profile_name TEXT NOT NULL UNIQUE,
		auth_path TEXT NOT NULL,
		login_method TEXT NOT NULL,
		status TEXT NOT NULL,
		subscription TEXT NOT NULL DEFAULT '',
		usage5h INTEGER NOT NULL DEFAULT 0,
		usage_weekly INTEGER NOT NULL DEFAULT 0,
		active INTEGER NOT NULL DEFAULT 0,
		last_used_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Replace([]Account{{Nickname: "Dev1", AuthPath: "~/auth.json", ResetTime5h: "2026-05-20T23:00:00+08:00", ResetTimeWeekly: "2026-05-24T23:00:00+08:00"}}); err != nil {
		t.Fatal(err)
	}
	got := store.Accounts()[0]
	if got.ResetTime5h == "" || got.ResetTimeWeekly == "" {
		t.Fatalf("reset times not persisted: %#v", got)
	}
}

func TestSQLiteStorePersistsUsageMetadata(t *testing.T) {
	_, store, _ := newSQLiteTestService(t, []Account{{
		Nickname:         "Dev1",
		AuthPath:         "~/auth.json",
		UsageSource:      "api",
		UsageLastRefresh: "2026-05-21T00:00:00Z",
		UsageStale:       true,
		UsageError:       "temporary failure",
	}})
	got := store.Accounts()[0]
	if got.UsageSource != "api" || got.UsageLastRefresh == "" || !got.UsageStale || got.UsageError == "" {
		t.Fatalf("usage metadata not persisted: %#v", got)
	}
}

func TestSQLiteStoreMigratesDisplayNameColumn(t *testing.T) {
	home := t.TempDir()
	dbPath := DBPath(home)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE accounts (
		id TEXT PRIMARY KEY,
		nickname TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL DEFAULT '',
		profile_name TEXT NOT NULL UNIQUE,
		auth_path TEXT NOT NULL,
		login_method TEXT NOT NULL,
		status TEXT NOT NULL,
		subscription TEXT NOT NULL DEFAULT '',
		usage5h INTEGER NOT NULL DEFAULT 0,
		usage_weekly INTEGER NOT NULL DEFAULT 0,
		reset_time5h TEXT NOT NULL DEFAULT '',
		reset_time_weekly TEXT NOT NULL DEFAULT '',
		active INTEGER NOT NULL DEFAULT 0,
		last_used_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	INSERT INTO accounts(id, nickname, display_name, profile_name, auth_path, login_method, status, created_at, updated_at)
	VALUES ('acc-1', 'Dev1', 'Developer One', 'dev1', '~/auth.json', 'imported', 'ready', '2026-05-20T00:00:00Z', '2026-05-20T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	got := store.Accounts()[0]
	if got.Nickname != "Dev1" || got.ProfileName != "dev1" {
		t.Fatalf("account not preserved: %#v", got)
	}
	if store.columnExists("accounts", "display_name") {
		t.Fatalf("display_name column still exists")
	}
}

func TestProfileServiceWritesSecureAuth(t *testing.T) {
	home := t.TempDir()
	profile := NewProfileService(home)
	path, err := profile.WriteAuth("main-dev", []byte(`{"token":"secret"}`))
	if err != nil {
		t.Fatalf("WriteAuth() error = %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o", fileInfo.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode = %o", dirInfo.Mode().Perm())
	}
}

func TestLoginSessionStateTransitions(t *testing.T) {
	service, _, _ := newSQLiteTestService(t, nil)
	session, err := service.StartLogin()
	if err != nil {
		t.Fatalf("StartLogin() error = %v", err)
	}
	if session.Status != LoginStatusWaitingUser || session.UserCode == "" || session.DeviceCode == "" {
		t.Fatalf("session = %#v", session)
	}
	cancelled, err := service.CancelLoginSession(session.ID)
	if err != nil {
		t.Fatalf("CancelLoginSession() error = %v", err)
	}
	if cancelled.Status != LoginStatusCancelled {
		t.Fatalf("cancelled status = %s", cancelled.Status)
	}
	_, err = service.LoginSession("missing")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("LoginSession missing err = %v", err)
	}
}

func TestLoginSessionExpiresOnRead(t *testing.T) {
	_, store, _ := newSQLiteTestService(t, nil)
	session, err := store.CreateLoginSession(LoginSession{Method: LoginMethodDevice, Status: LoginStatusPending, ExpiresAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.LoginSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != LoginStatusExpired {
		t.Fatalf("status = %s", got.Status)
	}
}
