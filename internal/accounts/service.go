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
	"reflect"
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

func loginEnv(base []string, home, codexHome string) []string {
	env := make([]string, 0, len(base)+2)
	for _, value := range base {
		if strings.HasPrefix(value, "HOME=") || strings.HasPrefix(value, "CODEX_HOME=") {
			continue
		}
		env = append(env, value)
	}
	return append(env, "HOME="+home, "CODEX_HOME="+codexHome)
}

func mergeLoginOutput(start *LoginStart, line string) {
	line = strings.TrimSpace(ansiPattern.ReplaceAllString(line, ""))
	if line == "" {
		return
	}
	if start.VerificationURL == "" {
		if match := loginURLPattern.FindString(line); match != "" {
			start.VerificationURL = strings.TrimRight(match, ".),]")
		}
	}
	if start.UserCode == "" {
		if match := loginCodeFromText(line); match != "" {
			start.UserCode = match
		}
	}
	if start.UserCode == "" && start.VerificationURL != "" {
		if code := loginCodeFromURL(start.VerificationURL); code != "" {
			start.UserCode = code
		}
	}
}

func loginCodeFromText(line string) string {
	if match := loginCodePattern.FindString(line); match != "" {
		return match
	}
	fields := strings.Fields(line)
	for _, field := range fields {
		candidate := strings.Trim(field, " .,;:()[]{}<>")
		if looksLikeLoginCode(candidate) {
			return candidate
		}
	}
	return ""
}

func loginCodeFromURL(rawURL string) string {
	for _, marker := range []string{"user_code=", "userCode=", "code="} {
		idx := strings.Index(rawURL, marker)
		if idx == -1 {
			continue
		}
		candidate := rawURL[idx+len(marker):]
		if cut := strings.IndexAny(candidate, "&#? "); cut >= 0 {
			candidate = candidate[:cut]
		}
		candidate = strings.TrimSpace(candidate)
		if looksLikeLoginCode(candidate) {
			return candidate
		}
	}
	return ""
}

func looksLikeLoginCode(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 6 || len(value) > 32 {
		return false
	}
	hasAlphaNum := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasAlphaNum = true
		case r >= '0' && r <= '9':
			hasAlphaNum = true
		case r == '-':
		default:
			return false
		}
	}
	return hasAlphaNum && strings.Contains(value, "-")
}

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
	codexHome := IsolatedCodexHomePath(sessionHome)
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		cancel()
		return LoginStart{}, fmt.Errorf("create codex home: %w", err)
	}
	if err := os.Chmod(codexHome, 0o700); err != nil {
		cancel()
		return LoginStart{}, fmt.Errorf("secure codex home: %w", err)
	}
	cmd := exec.CommandContext(cmdCtx, "codex", "login", "--device-auth")
	cmd.Env = loginEnv(os.Environ(), sessionHome, codexHome)
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
		AuthPath:  filepath.Join(codexHome, "auth.json"),
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
			line := scanner.Text()
			mergeLoginOutput(&start, line)
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
	return s.withAuthState(s.store.Accounts())
}

func (s *Service) CurrentAuthPath() string { return s.authDestPath }

func (s *Service) Current() (Account, error) {
	_ = s.SyncActiveFromAuthFile()
	for _, account := range s.withAuthState(s.store.Accounts()) {
		if account.Active {
			return account, nil
		}
	}
	return Account{}, ErrNoActiveAccount
}

func (s *Service) withAuthState(accounts []Account) []Account {
	now := time.Now()
	next := append([]Account(nil), accounts...)
	for i := range next {
		next[i] = enrichAuthState(next[i], s.home, now)
	}
	return next
}

func (s *Service) Add(account Account) (Account, error) {
	if account.Nickname == "" || account.AuthPath == "" {
		return Account{}, ErrInvalidAccount
	}
	account = normalizeAccount(account)
	err := s.store.Mutate(func(current []Account) ([]Account, error) {
		for _, existing := range current {
			if existing.ProfileName == account.ProfileName {
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
			if _, ok := targets[account.ID]; ok {
				deleted++
				removed = append(removed, account)
				continue
			}
			if _, ok := targets[account.ProfileName]; ok {
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

	_ = s.SyncActiveFromAuthFile()
	accounts := s.store.Accounts()
	targetIndex := -1
	for i, account := range accounts {
		if account.ProfileName == selector || account.ID == selector {
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
	var matchedCandidate []byte
	for i, account := range accounts {
		candidatePath := ExpandUserPath(account.AuthPath, s.home)
		candidate, err := os.ReadFile(candidatePath)
		if err != nil {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(current), bytes.TrimSpace(candidate)) ||
			(currentAccountID != "" && currentAccountID == authAccountID(candidate)) {
			matched = i
			matchedCandidate = candidate
			break
		}
	}
	if matched == -1 {
		return nil
	}
	if isManagedAuthPath(s.home, accounts[matched].AuthPath) && !bytes.Equal(bytes.TrimSpace(current), bytes.TrimSpace(matchedCandidate)) {
		if err := copyFile(s.authDestPath, ExpandUserPath(accounts[matched].AuthPath, s.home)); err != nil {
			return err
		}
	}
	if err := s.syncManagedAuthConfigFields(current, accounts); err != nil {
		return err
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

func (s *Service) syncManagedAuthConfigFields(activeAuth []byte, accounts []Account) error {
	for _, account := range accounts {
		if !isManagedAuthPath(s.home, account.AuthPath) {
			continue
		}
		path := ExpandUserPath(account.AuthPath, s.home)
		current, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		next, changed, err := mergeCodexAuthConfigFields(activeAuth, current)
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		if err := writeFileAtomic(path, next); err != nil {
			return err
		}
	}
	return nil
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
	usageByProfile := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		usageByProfile[account.ProfileName] = account
	}
	return s.store.Mutate(func(current []Account) ([]Account, error) {
		changed := false
		for i := range current {
			next, ok := usageByProfile[current[i].ProfileName]
			if !ok {
				continue
			}
			if current[i].Usage5h != next.Usage5h ||
				current[i].UsageWeekly != next.UsageWeekly ||
				current[i].ResetTime5h != next.ResetTime5h ||
				current[i].ResetTimeWeekly != next.ResetTimeWeekly ||
				current[i].UsageSource != next.UsageSource ||
				current[i].UsageLastRefresh != next.UsageLastRefresh ||
				current[i].UsageStale != next.UsageStale ||
				current[i].UsageError != next.UsageError ||
				current[i].HasWeeklyWindow != next.HasWeeklyWindow ||
				current[i].Availability != next.Availability ||
				current[i].PlanType != next.PlanType ||
				!reflect.DeepEqual(current[i].RateLimitAllowed, next.RateLimitAllowed) ||
				current[i].RateLimitReachedType != next.RateLimitReachedType ||
				!reflect.DeepEqual(current[i].CreditsAvailable, next.CreditsAvailable) ||
				!reflect.DeepEqual(current[i].CreditsUnlimited, next.CreditsUnlimited) ||
				!reflect.DeepEqual(current[i].CreditsBalance, next.CreditsBalance) ||
				!reflect.DeepEqual(current[i].CreditsApproxLocalMessages, next.CreditsApproxLocalMessages) ||
				!reflect.DeepEqual(current[i].CreditsApproxCloudMessages, next.CreditsApproxCloudMessages) ||
				!reflect.DeepEqual(current[i].CreditsOverageLimitReached, next.CreditsOverageLimitReached) ||
				!reflect.DeepEqual(current[i].SpendControlReached, next.SpendControlReached) {
				current[i].Usage5h = next.Usage5h
				current[i].UsageWeekly = next.UsageWeekly
				current[i].ResetTime5h = next.ResetTime5h
				current[i].ResetTimeWeekly = next.ResetTimeWeekly
				current[i].UsageSource = next.UsageSource
				current[i].UsageLastRefresh = next.UsageLastRefresh
				current[i].UsageStale = next.UsageStale
				current[i].UsageError = next.UsageError
				current[i].HasWeeklyWindow = next.HasWeeklyWindow
				current[i].Availability = next.Availability
				current[i].PlanType = next.PlanType
				current[i].RateLimitAllowed = next.RateLimitAllowed
				current[i].RateLimitReachedType = next.RateLimitReachedType
				current[i].CreditsAvailable = next.CreditsAvailable
				current[i].CreditsUnlimited = next.CreditsUnlimited
				current[i].CreditsBalance = next.CreditsBalance
				current[i].CreditsApproxLocalMessages = next.CreditsApproxLocalMessages
				current[i].CreditsApproxCloudMessages = next.CreditsApproxCloudMessages
				current[i].CreditsOverageLimitReached = next.CreditsOverageLimitReached
				current[i].SpendControlReached = next.SpendControlReached
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
			if current[i].ID == selector || current[i].ProfileName == selector {
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
