package accounts

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	mu sync.Mutex
	db *sql.DB
}

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) initSchema() error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			nickname TEXT NOT NULL,
			profile_name TEXT NOT NULL UNIQUE,
			auth_path TEXT NOT NULL,
			login_method TEXT NOT NULL,
			status TEXT NOT NULL,
			subscription TEXT NOT NULL DEFAULT '',
			usage5h INTEGER NOT NULL DEFAULT 0,
			usage_weekly INTEGER NOT NULL DEFAULT 0,
			reset_time5h TEXT NOT NULL DEFAULT '',
			reset_time_weekly TEXT NOT NULL DEFAULT '',
			usage_source TEXT NOT NULL DEFAULT '',
			usage_last_refresh TEXT NOT NULL DEFAULT '',
			usage_stale INTEGER NOT NULL DEFAULT 0,
			usage_error TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 0,
			last_used_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS login_sessions (
			id TEXT PRIMARY KEY,
			method TEXT NOT NULL,
			account_id TEXT NOT NULL DEFAULT '',
			profile_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			verification_url TEXT NOT NULL DEFAULT '',
			device_code TEXT NOT NULL DEFAULT '',
			user_code TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, datetime('now'))`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
	}
	if err := s.ensureColumn("accounts", "reset_time5h", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("accounts", "reset_time_weekly", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("accounts", "usage_source", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("accounts", "usage_last_refresh", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("accounts", "usage_stale", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("accounts", "usage_error", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.rebuildAccountTableIfNeeded(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) ensureColumn(table, column, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return fmt.Errorf("inspect sqlite table %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan sqlite table info: %w", err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("add sqlite column %s.%s: %w", table, column, err)
	}
	return nil
}

func (s *SQLiteStore) rebuildAccountTableIfNeeded() error {
	if !s.columnExists("accounts", "display_name") && !s.nicknameHasUniqueIndex() {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`CREATE TABLE accounts_new (
			id TEXT PRIMARY KEY,
			nickname TEXT NOT NULL,
			profile_name TEXT NOT NULL UNIQUE,
			auth_path TEXT NOT NULL,
			login_method TEXT NOT NULL,
			status TEXT NOT NULL,
			subscription TEXT NOT NULL DEFAULT '',
			usage5h INTEGER NOT NULL DEFAULT 0,
			usage_weekly INTEGER NOT NULL DEFAULT 0,
			reset_time5h TEXT NOT NULL DEFAULT '',
			reset_time_weekly TEXT NOT NULL DEFAULT '',
			usage_source TEXT NOT NULL DEFAULT '',
			usage_last_refresh TEXT NOT NULL DEFAULT '',
			usage_stale INTEGER NOT NULL DEFAULT 0,
			usage_error TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 0,
			last_used_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`INSERT INTO accounts_new(id, nickname, profile_name, auth_path, login_method, status, subscription, usage5h, usage_weekly, reset_time5h, reset_time_weekly, usage_source, usage_last_refresh, usage_stale, usage_error, active, last_used_at, created_at, updated_at)
			SELECT id, nickname, profile_name, auth_path, login_method, status, subscription, usage5h, usage_weekly, reset_time5h, reset_time_weekly, usage_source, usage_last_refresh, usage_stale, usage_error, active, last_used_at, created_at, updated_at FROM accounts`,
		`DROP TABLE accounts`,
		`ALTER TABLE accounts_new RENAME TO accounts`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("rebuild sqlite accounts table: %w", err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) nicknameHasUniqueIndex() bool {
	rows, err := s.db.Query(`PRAGMA index_list(accounts)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, origin, partial string
		var unique int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return false
		}
		if unique == 0 {
			continue
		}
		if s.indexColumns(name).equals([]string{"nickname"}) {
			return true
		}
	}
	return false
}

type indexColumns []string

func (s *SQLiteStore) indexColumns(indexName string) indexColumns {
	rows, err := s.db.Query(`PRAGMA index_info(` + indexName + `)`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var seqno, cid int
		var name string
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil
		}
		columns = append(columns, name)
	}
	return columns
}

func (columns indexColumns) equals(want []string) bool {
	if len(columns) != len(want) {
		return false
	}
	for i := range columns {
		if columns[i] != want[i] {
			return false
		}
	}
	return true
}

func (s *SQLiteStore) columnExists(table, column string) bool {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func (s *SQLiteStore) Accounts() []Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	accounts, err := s.accountsLocked()
	if err != nil {
		return []Account{}
	}
	return accounts
}

func (s *SQLiteStore) Replace(accounts []Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM accounts`); err != nil {
		return err
	}
	for _, account := range normalizeAccounts(accounts) {
		if err := insertAccountTx(tx, account); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) Mutate(fn func([]Account) ([]Account, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.accountsLocked()
	if err != nil {
		return err
	}
	next, err := fn(append([]Account(nil), current...))
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM accounts`); err != nil {
		return err
	}
	for _, account := range normalizeAccounts(next) {
		if err := insertAccountTx(tx, account); err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return ErrDuplicateAccount
			}
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) UpdateAccount(selector string, patch Account) (Account, error) {
	if selector == "" {
		return Account{}, ErrAccountNotFound
	}
	var updated Account
	err := s.Mutate(func(current []Account) ([]Account, error) {
		found := false
		for i := range current {
			if current[i].ID == selector || current[i].ProfileName == selector {
				found = true
				if patch.Nickname != "" {
					current[i].Nickname = patch.Nickname
				}
				current[i].Subscription = patch.Subscription
				if patch.Status != "" {
					current[i].Status = patch.Status
				}
				current[i].UpdatedAt = nowRFC3339()
				updated = current[i]
			}
		}
		if !found {
			return current, ErrAccountNotFound
		}
		return current, nil
	})
	return updated, err
}

func (s *SQLiteStore) accountsLocked() ([]Account, error) {
	rows, err := s.db.Query(`SELECT id, nickname, profile_name, auth_path, login_method, status, subscription, usage5h, usage_weekly, reset_time5h, reset_time_weekly, usage_source, usage_last_refresh, usage_stale, usage_error, active, last_used_at, created_at, updated_at FROM accounts ORDER BY created_at, nickname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []Account
	for rows.Next() {
		var a Account
		var active int
		var usageStale int
		if err := rows.Scan(&a.ID, &a.Nickname, &a.ProfileName, &a.AuthPath, &a.LoginMethod, &a.Status, &a.Subscription, &a.Usage5h, &a.UsageWeekly, &a.ResetTime5h, &a.ResetTimeWeekly, &a.UsageSource, &a.UsageLastRefresh, &usageStale, &a.UsageError, &active, &a.LastUsedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Active = active == 1
		a.UsageStale = usageStale == 1
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func insertAccountTx(tx *sql.Tx, account Account) error {
	account = normalizeAccount(account)
	_, err := tx.Exec(`INSERT INTO accounts(id, nickname, profile_name, auth_path, login_method, status, subscription, usage5h, usage_weekly, reset_time5h, reset_time_weekly, usage_source, usage_last_refresh, usage_stale, usage_error, active, last_used_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		account.ID, account.Nickname, account.ProfileName, account.AuthPath, account.LoginMethod, account.Status, account.Subscription, account.Usage5h, account.UsageWeekly, account.ResetTime5h, account.ResetTimeWeekly, account.UsageSource, account.UsageLastRefresh, boolInt(account.UsageStale), account.UsageError, boolInt(account.Active), account.LastUsedAt, account.CreatedAt, account.UpdatedAt)
	return err
}

func normalizeAccounts(accounts []Account) []Account {
	out := make([]Account, len(accounts))
	activeSeen := false
	for i, account := range accounts {
		account = normalizeAccount(account)
		if account.Active {
			if activeSeen {
				account.Active = false
			} else {
				activeSeen = true
			}
		}
		out[i] = account
	}
	return out
}

func normalizeAccount(account Account) Account {
	now := nowRFC3339()
	if account.ID == "" {
		account.ID = uuid.NewString()
	}
	if account.ProfileName == "" {
		account.ProfileName = safeProfileName(account.Nickname)
	}
	if account.LoginMethod == "" {
		account.LoginMethod = LoginMethodImported
	}
	if account.Status == "" {
		account.Status = AccountStatusReady
	}
	if account.Active {
		account.Status = AccountStatusActive
	} else if account.Status == AccountStatusActive {
		account.Status = AccountStatusReady
	}
	if account.CreatedAt == "" {
		account.CreatedAt = now
	}
	if account.UpdatedAt == "" {
		account.UpdatedAt = now
	}
	return account
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

type LoginSession struct {
	ID               string `json:"id"`
	Method           string `json:"method"`
	AccountID        string `json:"account_id,omitempty"`
	ProfileName      string `json:"profile_name,omitempty"`
	Status           string `json:"status"`
	AuthorizationURL string `json:"authorization_url,omitempty"`
	VerificationURL  string `json:"verification_url,omitempty"`
	DeviceCode       string `json:"device_code,omitempty"`
	UserCode         string `json:"user_code,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	Error            string `json:"error,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

const (
	LoginStatusPending     = "pending"
	LoginStatusWaitingUser = "waiting_user"
	LoginStatusSucceeded   = "succeeded"
	LoginStatusFailed      = "failed"
	LoginStatusExpired     = "expired"
	LoginStatusCancelled   = "cancelled"
)

func (s *SQLiteStore) CreateLoginSession(session LoginSession) (LoginSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session = normalizeSession(session)
	_, err := s.db.Exec(`INSERT INTO login_sessions(id, method, account_id, profile_name, status, verification_url, device_code, user_code, expires_at, error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.Method, session.AccountID, session.ProfileName, session.Status, session.VerificationURL, session.DeviceCode, session.UserCode, session.ExpiresAt, session.Error, session.CreatedAt, session.UpdatedAt)
	if err != nil {
		return LoginSession{}, err
	}
	return session, nil
}

func (s *SQLiteStore) LoginSession(id string) (LoginSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT id, method, account_id, profile_name, status, verification_url, device_code, user_code, expires_at, error, created_at, updated_at FROM login_sessions WHERE id = ?`, id)
	var session LoginSession
	err := row.Scan(&session.ID, &session.Method, &session.AccountID, &session.ProfileName, &session.Status, &session.VerificationURL, &session.DeviceCode, &session.UserCode, &session.ExpiresAt, &session.Error, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return LoginSession{}, ErrAccountNotFound
	}
	if err != nil {
		return LoginSession{}, err
	}
	session.AuthorizationURL = session.VerificationURL
	return expireIfNeeded(session), nil
}

func (s *SQLiteStore) UpdateLoginSession(session LoginSession) (LoginSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session.UpdatedAt = nowRFC3339()
	res, err := s.db.Exec(`UPDATE login_sessions SET account_id = ?, profile_name = ?, status = ?, verification_url = ?, device_code = ?, user_code = ?, expires_at = ?, error = ?, updated_at = ? WHERE id = ?`,
		session.AccountID, session.ProfileName, session.Status, session.VerificationURL, session.DeviceCode, session.UserCode, session.ExpiresAt, session.Error, session.UpdatedAt, session.ID)
	if err != nil {
		return LoginSession{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return LoginSession{}, ErrAccountNotFound
	}
	session.AuthorizationURL = session.VerificationURL
	return session, nil
}

func normalizeSession(session LoginSession) LoginSession {
	now := nowRFC3339()
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	if session.Status == "" {
		session.Status = LoginStatusPending
	}
	if session.CreatedAt == "" {
		session.CreatedAt = now
	}
	if session.UpdatedAt == "" {
		session.UpdatedAt = now
	}
	session.AuthorizationURL = session.VerificationURL
	return session
}

func expireIfNeeded(session LoginSession) LoginSession {
	if session.ExpiresAt == "" || session.Status == LoginStatusSucceeded || session.Status == LoginStatusFailed || session.Status == LoginStatusCancelled || session.Status == LoginStatusExpired {
		return session
	}
	expires, err := time.Parse(time.RFC3339, session.ExpiresAt)
	if err == nil && time.Now().UTC().After(expires) {
		session.Status = LoginStatusExpired
	}
	return session
}
