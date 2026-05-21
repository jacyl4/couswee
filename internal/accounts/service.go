package accounts

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAccountNotFound  = errors.New("account not found")
	ErrNoActiveAccount  = errors.New("no active account")
	ErrDuplicateAccount = errors.New("duplicate account")
	ErrInvalidAccount   = errors.New("invalid account")
)

type UsageRefresher interface {
	Refresh([]Account) ([]Account, bool, error)
}

type NoopUsageRefresher struct{}

func (NoopUsageRefresher) Refresh(accounts []Account) ([]Account, bool, error) {
	return accounts, false, nil
}

type LoginStart struct {
	VerificationURL string
	DeviceCode      string
	UserCode        string
	ExpiresAt       time.Time
	AuthPath        string
	Done            <-chan error
	Cancel          context.CancelFunc
}

type LoginRunner interface {
	Start(ctx context.Context, sessionID string) (LoginStart, error)
}

type StaticLoginRunner struct{}

func (StaticLoginRunner) Start(ctx context.Context, sessionID string) (LoginStart, error) {
	done := make(chan error)
	return LoginStart{
		VerificationURL: "https://auth.openai.com/codex/device",
		UserCode:        strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:13], "-", "")),
		DeviceCode:      uuid.NewString(),
		ExpiresAt:       time.Now().UTC().Add(15 * time.Minute),
		Done:            done,
		Cancel:          func() {},
	}, nil
}

type CodexLoginRunner struct {
	Home string
}

var (
	loginURLPattern  = regexp.MustCompile(`https?://[^\s]+`)
	loginCodePattern = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{5}\b`)
	ansiPattern      = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

func (r CodexLoginRunner) Start(ctx context.Context, sessionID string) (LoginStart, error) {
	sessionHome := filepath.Join(r.Home, ".couswee", "login-sessions", sessionID, "home")
	if err := os.MkdirAll(sessionHome, 0o700); err != nil {
		return LoginStart{}, fmt.Errorf("create login home: %w", err)
	}
	if err := os.Chmod(sessionHome, 0o700); err != nil {
		return LoginStart{}, fmt.Errorf("secure login home: %w", err)
	}
	if err := os.Chmod(filepath.Dir(sessionHome), 0o700); err != nil {
		return LoginStart{}, fmt.Errorf("secure login session directory: %w", err)
	}

	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, "codex", "login", "--device-auth")
	cmd.Env = append(os.Environ(), "HOME="+sessionHome)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return LoginStart{}, fmt.Errorf("open codex login stdout: %w", err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		cancel()
		return LoginStart{}, fmt.Errorf("start codex login: %w", err)
	}
	cancelProcess := func() {
		cancel()
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		close(done)
	}()

	start := LoginStart{
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		AuthPath:  CodexAuthPath(sessionHome),
		Done:      done,
		Cancel:    cancelProcess,
	}
	scanner := bufio.NewScanner(stdout)
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ready := make(chan LoginStart, 1)
	parseErr := make(chan error, 1)
	go func() {
		sentReady := false
		for scanner.Scan() {
			line := ansiPattern.ReplaceAllString(scanner.Text(), "")
			if start.VerificationURL == "" {
				if match := loginURLPattern.FindString(line); match != "" {
					start.VerificationURL = strings.TrimRight(match, ".),")
				}
			}
			if start.UserCode == "" {
				if match := loginCodePattern.FindString(line); match != "" {
					start.UserCode = match
				}
			}
			if !sentReady && start.VerificationURL != "" && start.UserCode != "" {
				ready <- start
				sentReady = true
			}
		}
		if err := scanner.Err(); err != nil {
			parseErr <- err
			return
		}
		if !sentReady {
			parseErr <- errors.New("codex login did not emit a verification URL and user code")
		}
	}()

	select {
	case result := <-ready:
		return result, nil
	case err := <-parseErr:
		cancelProcess()
		return LoginStart{}, fmt.Errorf("read codex login output: %w", err)
	case err := <-done:
		cancelProcess()
		return LoginStart{}, fmt.Errorf("codex login exited before authorization started: %w", err)
	case <-deadline.C:
		cancelProcess()
		return LoginStart{}, errors.New("timed out waiting for codex login authorization details")
	}
}

type Service struct {
	store        AccountStore
	home         string
	authDestPath string
	refresher    UsageRefresher
	loginRunner  LoginRunner
	loginCancels map[string]context.CancelFunc
	loginMu      sync.Mutex
}

func NewService(store AccountStore, home string, refresher UsageRefresher) *Service {
	if refresher == nil {
		refresher = NoopUsageRefresher{}
	}
	return &Service{
		store:        store,
		home:         home,
		authDestPath: CodexAuthPath(home),
		refresher:    refresher,
		loginRunner:  StaticLoginRunner{},
		loginCancels: make(map[string]context.CancelFunc),
	}
}

func (s *Service) UseCodexLoginRunner() {
	s.loginRunner = CodexLoginRunner{Home: s.home}
}

func (s *Service) Accounts() []Account {
	_ = s.SyncActiveFromAuthFile()
	return s.store.Accounts()
}

func (s *Service) CurrentAuthPath() string { return s.authDestPath }

func (s *Service) Current() (Account, error) {
	_ = s.SyncActiveFromAuthFile()
	for _, account := range s.store.Accounts() {
		if account.Active {
			return account, nil
		}
	}
	return Account{}, ErrNoActiveAccount
}

func (s *Service) Add(account Account) (Account, error) {
	if account.Nickname == "" || account.AuthPath == "" {
		return Account{}, ErrInvalidAccount
	}
	account = normalizeAccount(account)
	err := s.store.Mutate(func(current []Account) ([]Account, error) {
		for _, existing := range current {
			if existing.Nickname == account.Nickname {
				return current, ErrDuplicateAccount
			}
		}
		if account.Active {
			for i := range current {
				current[i].Active = false
			}
		}
		return append(current, account), nil
	})
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Service) Delete(nicknames []string) (int, error) {
	return s.DeleteSelectors(nicknames)
}

func (s *Service) DeleteSelectors(selectors []string) (int, error) {
	if len(selectors) == 0 {
		return 0, ErrAccountNotFound
	}
	targets := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		if selector != "" {
			targets[selector] = struct{}{}
		}
	}
	if len(targets) == 0 {
		return 0, ErrAccountNotFound
	}
	deleted := 0
	var removed []Account
	err := s.store.Mutate(func(current []Account) ([]Account, error) {
		next := current[:0]
		for _, account := range current {
			if _, ok := targets[account.Nickname]; ok {
				deleted++
				removed = append(removed, account)
				continue
			}
			if _, ok := targets[account.ID]; ok {
				deleted++
				removed = append(removed, account)
				continue
			}
			next = append(next, account)
		}
		if deleted == 0 {
			return current, ErrAccountNotFound
		}
		return append([]Account(nil), next...), nil
	})
	if err != nil {
		return 0, err
	}
	profileService := NewProfileService(s.home)
	for _, account := range removed {
		if isManagedAuthPath(s.home, account.AuthPath) {
			_ = profileService.RemoveManagedProfile(account.ProfileName)
		}
	}
	return deleted, nil
}

func (s *Service) Switch(nickname string) (Account, error) {
	return s.SwitchSelector(nickname)
}

func (s *Service) SwitchSelector(selector string) (Account, error) {
	if selector == "" {
		return Account{}, ErrAccountNotFound
	}

	accounts := s.store.Accounts()
	targetIndex := -1
	for i, account := range accounts {
		if account.Nickname == selector || account.ID == selector || account.ProfileName == selector {
			targetIndex = i
			break
		}
	}
	if targetIndex == -1 {
		return Account{}, ErrAccountNotFound
	}

	sourcePath := ExpandUserPath(accounts[targetIndex].AuthPath, s.home)
	if err := copyFile(sourcePath, s.authDestPath); err != nil {
		return Account{}, err
	}

	var selected Account
	switchedAt := nowRFC3339()
	for i := range accounts {
		accounts[i].Active = i == targetIndex
		if i == targetIndex {
			accounts[i].LastUsedAt = switchedAt
			accounts[i].UpdatedAt = switchedAt
			selected = accounts[i]
		}
	}
	if err := s.store.Replace(accounts); err != nil {
		return Account{}, err
	}
	return selected, nil
}

func (s *Service) SyncActiveFromAuthFile() error {
	current, err := os.ReadFile(s.authDestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	currentAccountID := authAccountID(current)
	accounts := s.store.Accounts()
	matched := -1
	for i, account := range accounts {
		candidatePath := ExpandUserPath(account.AuthPath, s.home)
		candidate, err := os.ReadFile(candidatePath)
		if err != nil {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(current), bytes.TrimSpace(candidate)) ||
			(currentAccountID != "" && currentAccountID == authAccountID(candidate)) {
			matched = i
			break
		}
	}
	if matched == -1 {
		return nil
	}
	changed := false
	observedAt := nowRFC3339()
	for i := range accounts {
		active := i == matched
		if accounts[i].Active != active {
			accounts[i].Active = active
			changed = true
		}
		if active && accounts[i].LastUsedAt == "" {
			accounts[i].LastUsedAt = observedAt
			accounts[i].UpdatedAt = observedAt
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.store.Replace(accounts)
}

func authAccountID(data []byte) string {
	var raw struct {
		Tokens struct {
			AccountID string `json:"account_id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return strings.TrimSpace(raw.Tokens.AccountID)
}

func (s *Service) RefreshUsage() error {
	return s.store.Mutate(func(current []Account) ([]Account, error) {
		next, changed, err := s.refresher.Refresh(append([]Account(nil), current...))
		if err != nil {
			return current, err
		}
		if !changed {
			return current, nil
		}
		return next, nil
	})
}

func (s *Service) ReplaceUsage(accounts []Account) error {
	usageByNickname := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		usageByNickname[account.Nickname] = account
	}
	return s.store.Mutate(func(current []Account) ([]Account, error) {
		changed := false
		for i := range current {
			next, ok := usageByNickname[current[i].Nickname]
			if !ok {
				continue
			}
			if current[i].Usage5h != next.Usage5h || current[i].UsageWeekly != next.UsageWeekly || current[i].ResetTime5h != next.ResetTime5h || current[i].ResetTimeWeekly != next.ResetTimeWeekly {
				current[i].Usage5h = next.Usage5h
				current[i].UsageWeekly = next.UsageWeekly
				current[i].ResetTime5h = next.ResetTime5h
				current[i].ResetTimeWeekly = next.ResetTimeWeekly
				current[i].UpdatedAt = nowRFC3339()
				changed = true
			}
		}
		if !changed {
			return current, nil
		}
		return current, nil
	})
}

func (s *Service) StartUsageRefresh(interval time.Duration, stop <-chan struct{}) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = s.RefreshUsage()
			case <-stop:
				return
			}
		}
	}()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open auth source: %w", err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("create auth directory: %w", err)
	}

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary auth file: %w", err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("copy auth file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temporary auth file: %w", closeErr)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace active auth file: %w", err)
	}
	return nil
}

func (s *Service) UpdateAccount(selector string, patch Account) (Account, error) {
	if updater, ok := s.store.(interface {
		UpdateAccount(string, Account) (Account, error)
	}); ok {
		return updater.UpdateAccount(selector, patch)
	}
	var updated Account
	err := s.store.Mutate(func(current []Account) ([]Account, error) {
		for i := range current {
			if current[i].ID == selector || current[i].Nickname == selector {
				if patch.Nickname != "" {
					current[i].Nickname = patch.Nickname
				}
				current[i].Subscription = patch.Subscription
				updated = current[i]
				return current, nil
			}
		}
		return current, ErrAccountNotFound
	})
	return updated, err
}

func (s *Service) StartLogin() (LoginSession, error) {
	store, ok := s.store.(interface {
		CreateLoginSession(LoginSession) (LoginSession, error)
		LoginSession(string) (LoginSession, error)
		UpdateLoginSession(LoginSession) (LoginSession, error)
	})
	if !ok {
		return LoginSession{}, ErrInvalidAccount
	}
	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	start, err := s.loginRunner.Start(ctx, id)
	if err != nil {
		cancel()
		return LoginSession{}, err
	}
	s.loginMu.Lock()
	s.loginCancels[id] = start.Cancel
	s.loginMu.Unlock()

	status := LoginStatusPending
	if start.UserCode != "" {
		status = LoginStatusWaitingUser
	}
	session, err := store.CreateLoginSession(LoginSession{
		ID:              id,
		Method:          LoginMethodDevice,
		Status:          status,
		VerificationURL: start.VerificationURL,
		DeviceCode:      start.DeviceCode,
		UserCode:        start.UserCode,
		ExpiresAt:       start.ExpiresAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		start.Cancel()
		return LoginSession{}, err
	}
	if start.Done != nil && start.AuthPath != "" {
		go s.finishCodexLogin(id, start.AuthPath, start.Done)
	}
	return session, nil
}

func (s *Service) finishCodexLogin(sessionID, authPath string, done <-chan error) {
	err := <-done
	s.loginMu.Lock()
	delete(s.loginCancels, sessionID)
	s.loginMu.Unlock()

	store, ok := s.store.(interface {
		LoginSession(string) (LoginSession, error)
		UpdateLoginSession(LoginSession) (LoginSession, error)
	})
	if !ok {
		return
	}
	session, sessionErr := store.LoginSession(sessionID)
	if sessionErr != nil || session.Status == LoginStatusCancelled || session.Status == LoginStatusExpired {
		return
	}
	if err != nil {
		session.Status = LoginStatusFailed
		session.Error = err.Error()
		_, _ = store.UpdateLoginSession(session)
		return
	}
	authJSON, err := os.ReadFile(authPath)
	if err != nil {
		session.Status = LoginStatusFailed
		session.Error = fmt.Sprintf("read codex auth after login: %v", err)
		_, _ = store.UpdateLoginSession(session)
		return
	}
	if _, _, err := s.completeLogin(session, "", string(authJSON)); err != nil {
		return
	}
}

func (s *Service) LoginSession(id string) (LoginSession, error) {
	store, ok := s.store.(interface {
		LoginSession(string) (LoginSession, error)
	})
	if !ok {
		return LoginSession{}, ErrAccountNotFound
	}
	return store.LoginSession(id)
}

func (s *Service) CancelLoginSession(id string) (LoginSession, error) {
	store, ok := s.store.(interface {
		LoginSession(string) (LoginSession, error)
		UpdateLoginSession(LoginSession) (LoginSession, error)
	})
	if !ok {
		return LoginSession{}, ErrAccountNotFound
	}
	session, err := store.LoginSession(id)
	if err != nil {
		return LoginSession{}, err
	}
	if session.Status == LoginStatusSucceeded || session.Status == LoginStatusFailed || session.Status == LoginStatusExpired {
		return session, nil
	}
	s.loginMu.Lock()
	cancel := s.loginCancels[id]
	delete(s.loginCancels, id)
	s.loginMu.Unlock()
	if cancel != nil {
		cancel()
	}
	session.Status = LoginStatusCancelled
	return store.UpdateLoginSession(session)
}

func (s *Service) completeLogin(session LoginSession, nickname, authJSON string) (Account, LoginSession, error) {
	store, ok := s.store.(interface {
		UpdateLoginSession(LoginSession) (LoginSession, error)
	})
	if !ok {
		return Account{}, LoginSession{}, ErrAccountNotFound
	}
	if nickname == "" {
		nickname = "codex-" + session.ID[:8]
	}
	profileName := safeProfileName(nickname)
	path, err := NewProfileService(s.home).WriteAuth(profileName, []byte(authJSON))
	if err != nil {
		session.Status = LoginStatusFailed
		session.Error = err.Error()
		_, _ = store.UpdateLoginSession(session)
		return Account{}, session, err
	}
	account, err := s.Add(Account{Nickname: nickname, ProfileName: profileName, AuthPath: path, LoginMethod: session.Method, Status: AccountStatusReady})
	if err != nil {
		session.Status = LoginStatusFailed
		session.Error = err.Error()
		_, _ = store.UpdateLoginSession(session)
		return Account{}, session, err
	}
	session.AccountID = account.ID
	session.ProfileName = account.ProfileName
	session.Status = LoginStatusSucceeded
	session, err = store.UpdateLoginSession(session)
	return account, session, err
}

func isManagedAuthPath(home, authPath string) bool {
	if authPath == "" {
		return false
	}
	absRoot, err := filepath.Abs(ProfilesDir(home))
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(ExpandUserPath(authPath, home))
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, absRoot+string(os.PathSeparator))
}
