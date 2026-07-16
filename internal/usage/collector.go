package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"couswee/internal/accounts"
)

var ErrNoCollector = errors.New("usage collector unavailable")

type Collector interface {
	Collect(ctx context.Context, account accounts.Account) (UsageRecord, error)
}

type AccountCollector struct {
	Unit           string
	Now            func() time.Time
	ActiveAuthPath string
}

func (c AccountCollector) Collect(_ context.Context, account accounts.Account) (UsageRecord, error) {
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	unit := c.Unit
	if unit == "" {
		unit = UnitPercent
	}
	return UsageRecord{
		Account:                    accountIdentity(account),
		Usage5h:                    float64(account.Usage5h),
		UsageWeekly:                float64(account.UsageWeekly),
		Remaining5h:                float64(account.Usage5h),
		RemainingWeekly:            float64(account.UsageWeekly),
		ResetTime:                  firstNonEmpty(account.ResetTime5h, account.ResetTimeWeekly),
		ResetTime5h:                account.ResetTime5h,
		ResetTimeWeekly:            account.ResetTimeWeekly,
		HasWeeklyWindow:            account.HasWeeklyWindow,
		Availability:               account.Availability,
		PlanType:                   account.PlanType,
		RateLimitAllowed:           account.RateLimitAllowed,
		RateLimitReachedType:       account.RateLimitReachedType,
		CreditsAvailable:           account.CreditsAvailable,
		CreditsUnlimited:           account.CreditsUnlimited,
		CreditsBalance:             account.CreditsBalance,
		CreditsApproxLocalMessages: account.CreditsApproxLocalMessages,
		CreditsApproxCloudMessages: account.CreditsApproxCloudMessages,
		CreditsOverageLimitReached: account.CreditsOverageLimitReached,
		SpendControlReached:        account.SpendControlReached,
		Unit:                       unit,
		UsageBasis:                 "remaining",
		Source:                     SourceAccount,
		LastRefresh:                now,
		Stale:                      true,
		Error:                      account.UsageError,
	}, nil
}

type APICollector struct {
	URL            string
	Unit           string
	Client         *http.Client
	Now            func() time.Time
	ActiveAuthPath string
	AuthRefresher  AuthRefresher
}

func (c APICollector) Collect(ctx context.Context, account accounts.Account) (UsageRecord, error) {
	if strings.TrimSpace(c.URL) == "" {
		return UsageRecord{}, ErrNoCollector
	}
	authPath := collectorAuthPath(account, c.ActiveAuthPath)
	auth, err := c.readFreshAuth(ctx, authPath)
	if err != nil {
		return UsageRecord{}, err
	}
	endpoint, err := c.usageEndpoint(account, authPath, auth)
	if err != nil {
		return UsageRecord{}, err
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	statusCode, body, err := c.requestUsageAPI(ctx, client, endpoint, auth)
	if err != nil {
		return UsageRecord{}, err
	}
	if statusCode == http.StatusUnauthorized && c.AuthRefresher != nil {
		if err := c.AuthRefresher.RefreshCodexAuth(ctx, authPath); err != nil {
			return UsageRecord{}, fmt.Errorf("refresh codex auth after usage api 401: %w", err)
		}
		auth, err = ReadCodexAuth(authPath)
		if err != nil {
			return UsageRecord{}, fmt.Errorf("read refreshed codex auth file: %w", err)
		}
		endpoint, err = c.usageEndpoint(account, authPath, auth)
		if err != nil {
			return UsageRecord{}, err
		}
		statusCode, body, err = c.requestUsageAPI(ctx, client, endpoint, auth)
		if err != nil {
			return UsageRecord{}, err
		}
	}
	if statusCode < 200 || statusCode >= 300 {
		return UsageRecord{}, usageAPIStatusError(statusCode, body)
	}
	record, err := ParseUsageRecord(body)
	if err != nil {
		return UsageRecord{}, fmt.Errorf("decode usage api response: %w", err)
	}
	if responseAccountID := usageResponseAccountID(body); responseAccountID != "" && auth.AccountID != "" && responseAccountID != auth.AccountID {
		return UsageRecord{}, fmt.Errorf("usage api account_id mismatch")
	}
	return normalizeRecord(record, account, SourceAPI, c.Unit, c.Now), nil
}

func (c APICollector) readFreshAuth(ctx context.Context, authPath string) (CodexAuth, error) {
	auth, err := ReadCodexAuth(authPath)
	if err != nil {
		return CodexAuth{}, err
	}
	if c.AuthRefresher == nil {
		return auth, nil
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	if !codexAuthNeedsRefresh(auth, now) {
		return auth, nil
	}
	if err := c.AuthRefresher.RefreshCodexAuth(ctx, authPath); err != nil {
		return CodexAuth{}, fmt.Errorf("refresh expired codex auth: %w", err)
	}
	auth, err = ReadCodexAuth(authPath)
	if err != nil {
		return CodexAuth{}, fmt.Errorf("read refreshed codex auth file: %w", err)
	}
	return auth, nil
}

func (c APICollector) usageEndpoint(account accounts.Account, authPath string, auth CodexAuth) (string, error) {
	endpoint, err := url.Parse(c.URL)
	if err != nil {
		return "", fmt.Errorf("parse usage api url: %w", err)
	}
	q := endpoint.Query()
	q.Set("account", accountIdentity(account))
	q.Set("auth_path", authPath)
	if auth.AccountID != "" {
		q.Set("account_id", auth.AccountID)
	}
	endpoint.RawQuery = q.Encode()
	return endpoint.String(), nil
}

func (c APICollector) requestUsageAPI(ctx context.Context, client *http.Client, endpoint string, auth CodexAuth) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	if auth.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", auth.AccountID)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request usage api: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read usage api response: %w", err)
	}
	return resp.StatusCode, body, nil
}

func usageAPIStatusError(statusCode int, body []byte) error {
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if code := strings.TrimSpace(payload.Error.Code); code != "" {
			return fmt.Errorf("usage api status %d: %s", statusCode, code)
		}
		if message := strings.TrimSpace(payload.Error.Message); message != "" {
			return fmt.Errorf("usage api status %d: %s", statusCode, message)
		}
	}
	return fmt.Errorf("usage api status %d", statusCode)
}

type CodexAuth struct {
	AccessToken          string
	RefreshToken         string
	AccountID            string
	LastRefresh          time.Time
	AccessTokenExpiresAt time.Time
}

func ReadCodexAuth(path string) (CodexAuth, error) {
	if strings.TrimSpace(path) == "" {
		return CodexAuth{}, fmt.Errorf("codex auth path is empty")
	}
	path = ExpandHome(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return CodexAuth{}, fmt.Errorf("read codex auth file: %w", err)
	}
	var raw struct {
		LastRefresh string `json:"last_refresh"`
		Tokens      struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CodexAuth{}, fmt.Errorf("parse codex auth file: %w", err)
	}
	auth := CodexAuth{
		AccessToken:          strings.TrimSpace(raw.Tokens.AccessToken),
		RefreshToken:         strings.TrimSpace(raw.Tokens.RefreshToken),
		AccountID:            strings.TrimSpace(raw.Tokens.AccountID),
		LastRefresh:          parseAuthTime(raw.LastRefresh),
		AccessTokenExpiresAt: jwtExpiresAt(strings.TrimSpace(raw.Tokens.AccessToken)),
	}
	if auth.AccessToken == "" {
		return CodexAuth{}, fmt.Errorf("codex auth file has no tokens.access_token")
	}
	return auth, nil
}

type CommandCollector struct {
	Command        string
	Unit           string
	Timeout        time.Duration
	Now            func() time.Time
	ActiveAuthPath string
}

func (c CommandCollector) Collect(ctx context.Context, account accounts.Account) (UsageRecord, error) {
	if strings.TrimSpace(c.Command) == "" {
		return UsageRecord{}, ErrNoCollector
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	parts := strings.Fields(c.Command)
	if len(parts) == 0 {
		return UsageRecord{}, ErrNoCollector
	}
	args := append(parts[1:], accountIdentity(account), collectorAuthPath(account, c.ActiveAuthPath))
	cmd := exec.CommandContext(ctx, parts[0], args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return UsageRecord{}, fmt.Errorf("run usage fallback: %s", msg)
	}
	record, err := ParseUsageRecord(out)
	if err != nil {
		return UsageRecord{}, err
	}
	return normalizeRecord(record, account, SourceFallback, c.Unit, c.Now), nil
}

type SessionLogCollector struct {
	Glob           string
	Unit           string
	Now            func() time.Time
	ActiveAuthPath string
}

func (c SessionLogCollector) Collect(_ context.Context, account accounts.Account) (UsageRecord, error) {
	if strings.TrimSpace(c.Glob) == "" {
		return UsageRecord{}, ErrNoCollector
	}
	if !matchesActiveAuth(account, c.ActiveAuthPath) {
		return UsageRecord{}, ErrNoCollector
	}
	matches, err := expandGlob(c.Glob)
	if err != nil {
		return UsageRecord{}, fmt.Errorf("parse codex session glob: %w", err)
	}
	var latest UsageRecord
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range bytes.Split(data, []byte("\n")) {
			if !bytes.Contains(line, []byte("rate_limits")) {
				continue
			}
			record, err := parseRateLimitPayload(line)
			if err != nil {
				continue
			}
			if latest.LastRefresh.IsZero() || record.LastRefresh.After(latest.LastRefresh) {
				latest = record
			}
		}
	}
	if latest.LastRefresh.IsZero() && latest.Usage5h == 0 && latest.UsageWeekly == 0 {
		return UsageRecord{}, ErrNoCollector
	}
	if !sessionEventFreshForAccount(latest.LastRefresh, account.LastUsedAt) {
		return UsageRecord{}, ErrNoCollector
	}
	return normalizeRecord(latest, account, SourceSession, c.Unit, c.Now), nil
}

func sessionEventFreshForAccount(eventTime time.Time, lastUsedAt string) bool {
	lastUsedAt = strings.TrimSpace(lastUsedAt)
	if lastUsedAt == "" || eventTime.IsZero() {
		return true
	}
	switchedAt, err := time.Parse(time.RFC3339, lastUsedAt)
	if err != nil {
		return true
	}
	return !eventTime.Before(switchedAt)
}

func expandGlob(pattern string) ([]string, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Glob(pattern)
	}
	prefix := pattern[:strings.Index(pattern, "**")]
	root := strings.TrimRight(prefix, string(os.PathSeparator))
	if root == "" {
		root = "."
	}
	suffix := strings.TrimLeft(pattern[strings.Index(pattern, "**")+2:], string(os.PathSeparator))
	var matches []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if suffix == "" {
			matches = append(matches, path)
			return nil
		}
		matched, matchErr := filepath.Match(suffix, filepath.Base(path))
		if matchErr != nil {
			return matchErr
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func ParseUsageRecord(data []byte) (UsageRecord, error) {
	var record UsageRecord
	if err := json.Unmarshal(data, &record); err == nil && isUsageRecordPayload(record) && !hasRateLimitPayload(data) {
		return record, nil
	}
	var records []UsageRecord
	if err := json.Unmarshal(data, &records); err == nil && len(records) > 0 {
		return records[0], nil
	}
	if record, err := parseRateLimitPayload(data); err == nil {
		return record, nil
	}
	return UsageRecord{}, errors.New("parse usage output: expected usage JSON object, array, or rate-limit JSON")
}

func hasRateLimitPayload(data []byte) bool {
	var payload struct {
		RateLimit json.RawMessage `json:"rate_limit"`
	}
	return json.Unmarshal(data, &payload) == nil && len(bytes.TrimSpace(payload.RateLimit)) > 0 && string(bytes.TrimSpace(payload.RateLimit)) != "null"
}

func isUsageRecordPayload(record UsageRecord) bool {
	return record.Account != "" || record.Usage5h != 0 || record.UsageWeekly != 0 || record.ResetTime != "" ||
		record.HasWeeklyWindow || record.Availability != "" || record.PlanType != "" || record.RateLimitAllowed != nil ||
		record.RateLimitReachedType != "" || record.CreditsAvailable != nil || record.CreditsUnlimited != nil ||
		record.CreditsBalance != nil || record.CreditsApproxLocalMessages != nil || record.CreditsApproxCloudMessages != nil ||
		record.CreditsOverageLimitReached != nil || record.SpendControlReached != nil
}

func usageResponseAccountID(data []byte) string {
	var wham struct {
		RateLimit json.RawMessage `json:"rate_limit"`
	}
	if err := json.Unmarshal(data, &wham); err == nil && len(wham.RateLimit) > 0 && string(wham.RateLimit) != "null" {
		return ""
	}
	var payload struct {
		AccountID string `json:"account_id"`
		Account   struct {
			ID string `json:"id"`
		} `json:"account"`
		Tokens struct {
			AccountID string `json:"account_id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	switch {
	case strings.TrimSpace(payload.AccountID) != "":
		return strings.TrimSpace(payload.AccountID)
	case strings.TrimSpace(payload.Account.ID) != "":
		return strings.TrimSpace(payload.Account.ID)
	default:
		return strings.TrimSpace(payload.Tokens.AccountID)
	}
}

type rateLimitPayload struct {
	Source      string          `json:"source"`
	FiveHour    rateLimitWindow `json:"five_hour"`
	SevenDay    rateLimitWindow `json:"seven_day"`
	Weekly      rateLimitWindow `json:"weekly"`
	UpdatedAt   int64           `json:"updated_at"`
	Timestamp   string          `json:"timestamp"`
	RateLimits  []rateLimitItem `json:"rate_limits"`
	RateLimits2 []rateLimitItem `json:"rateLimits"`
	Payload     struct {
		RateLimits codexRateLimits `json:"rate_limits"`
	} `json:"payload"`
	CodexRateLimits      codexRateLimits `json:"rate_limits_object"`
	PlanType             string          `json:"plan_type"`
	RateLimitReachedType string          `json:"rate_limit_reached_type"`
	Credits              struct {
		HasCredits          *bool           `json:"has_credits"`
		Available           *bool           `json:"available"`
		Unlimited           *bool           `json:"unlimited"`
		Balance             json.RawMessage `json:"balance"`
		ApproxLocalMessages json.RawMessage `json:"approx_local_messages"`
		ApproxCloudMessages json.RawMessage `json:"approx_cloud_messages"`
		OverageLimitReached *bool           `json:"overage_limit_reached"`
	} `json:"credits"`
	SpendControl struct {
		Reached *bool `json:"reached"`
	} `json:"spend_control"`
	RateLimit struct {
		Allowed         *bool           `json:"allowed"`
		LimitReached    *bool           `json:"limit_reached"`
		ReachedType     string          `json:"reached_type"`
		PrimaryWindow   rateLimitWindow `json:"primary_window"`
		SecondaryWindow rateLimitWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

type codexRateLimits struct {
	Primary   rateLimitItem `json:"primary"`
	Secondary rateLimitItem `json:"secondary"`
}

type rateLimitWindow struct {
	UsedPercentage      float64 `json:"used_percentage"`
	UsedPercent         float64 `json:"used_percent"`
	RemainingPercentage float64 `json:"remaining_percentage"`
	RemainingPercent    float64 `json:"remaining_percent"`
	ResetsAt            int64   `json:"resets_at"`
	ResetAt             int64   `json:"reset_at"`
	LimitWindowSeconds  int     `json:"limit_window_seconds"`
	ResetAfterSeconds   int     `json:"reset_after_seconds"`
	usedSet             bool
	remainingSet        bool
}

type rateLimitItem struct {
	Name                string  `json:"name"`
	Type                string  `json:"type"`
	Window              string  `json:"window"`
	WindowMinutes       int     `json:"window_minutes"`
	UsedPercentage      float64 `json:"used_percentage"`
	UsedPercent         float64 `json:"used_percent"`
	RemainingPercentage float64 `json:"remaining_percentage"`
	RemainingPercent    float64 `json:"remaining_percent"`
	ResetsAt            int64   `json:"resets_at"`
	ResetAt             int64   `json:"reset_at"`
	usedSet             bool
	remainingSet        bool
}

func (w *rateLimitWindow) UnmarshalJSON(data []byte) error {
	var raw struct {
		UsedPercentage      *float64 `json:"used_percentage"`
		UsedPercent         *float64 `json:"used_percent"`
		RemainingPercentage *float64 `json:"remaining_percentage"`
		RemainingPercent    *float64 `json:"remaining_percent"`
		ResetsAt            int64    `json:"resets_at"`
		ResetAt             int64    `json:"reset_at"`
		LimitWindowSeconds  int      `json:"limit_window_seconds"`
		ResetAfterSeconds   int      `json:"reset_after_seconds"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.UsedPercentage != nil {
		w.UsedPercentage = *raw.UsedPercentage
		w.usedSet = true
	}
	if raw.UsedPercent != nil {
		w.UsedPercent = *raw.UsedPercent
		w.usedSet = true
	}
	if raw.RemainingPercentage != nil {
		w.RemainingPercentage = *raw.RemainingPercentage
		w.remainingSet = true
	}
	if raw.RemainingPercent != nil {
		w.RemainingPercent = *raw.RemainingPercent
		w.remainingSet = true
	}
	w.ResetsAt = raw.ResetsAt
	w.ResetAt = raw.ResetAt
	w.LimitWindowSeconds = raw.LimitWindowSeconds
	w.ResetAfterSeconds = raw.ResetAfterSeconds
	return nil
}

func (i *rateLimitItem) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name                string   `json:"name"`
		Type                string   `json:"type"`
		Window              string   `json:"window"`
		WindowMinutes       int      `json:"window_minutes"`
		UsedPercentage      *float64 `json:"used_percentage"`
		UsedPercent         *float64 `json:"used_percent"`
		RemainingPercentage *float64 `json:"remaining_percentage"`
		RemainingPercent    *float64 `json:"remaining_percent"`
		ResetsAt            int64    `json:"resets_at"`
		ResetAt             int64    `json:"reset_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	i.Name = raw.Name
	i.Type = raw.Type
	i.Window = raw.Window
	i.WindowMinutes = raw.WindowMinutes
	if raw.UsedPercentage != nil {
		i.UsedPercentage = *raw.UsedPercentage
		i.usedSet = true
	}
	if raw.UsedPercent != nil {
		i.UsedPercent = *raw.UsedPercent
		i.usedSet = true
	}
	if raw.RemainingPercentage != nil {
		i.RemainingPercentage = *raw.RemainingPercentage
		i.remainingSet = true
	}
	if raw.RemainingPercent != nil {
		i.RemainingPercent = *raw.RemainingPercent
		i.remainingSet = true
	}
	i.ResetsAt = raw.ResetsAt
	i.ResetAt = raw.ResetAt
	return nil
}

func parseRateLimitPayload(data []byte) (UsageRecord, error) {
	var payload rateLimitPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return UsageRecord{}, err
	}
	legacyFive := payload.FiveHour
	weekly := payload.SevenDay
	if weekly.hasNoData() {
		weekly = payload.Weekly
	}
	if !payload.RateLimit.PrimaryWindow.hasNoData() {
		weekly = payload.RateLimit.PrimaryWindow
	}
	items := append(payload.RateLimits, payload.RateLimits2...)
	if !payload.Payload.RateLimits.Primary.hasNoData() || !payload.Payload.RateLimits.Secondary.hasNoData() {
		items = append(items, payload.Payload.RateLimits.Primary, payload.Payload.RateLimits.Secondary)
	}
	for _, item := range items {
		window := strings.ToLower(strings.TrimSpace(item.Window + " " + item.Name + " " + item.Type))
		switch {
		case item.WindowMinutes == 300 || strings.Contains(window, "5h") || strings.Contains(window, "5 hour") || strings.Contains(window, "five"):
			legacyFive = rateLimitWindow{RemainingPercentage: item.remainingPercent(), ResetsAt: item.resetUnix(), remainingSet: true}
		case item.WindowMinutes == 10080 || strings.Contains(window, "7d") || strings.Contains(window, "7 day") || strings.Contains(window, "seven") || strings.Contains(window, "week") || strings.Contains(window, "primary"):
			weekly = rateLimitWindow{RemainingPercentage: item.remainingPercent(), ResetsAt: item.resetUnix(), remainingSet: true}
		}
	}
	if weekly.hasNoData() && legacyFive.hasNoData() && !payloadHasEntitlement(payload) {
		return UsageRecord{}, errors.New("rate-limit payload has no recognizable usage or entitlement data")
	}
	reachedType := strings.TrimSpace(payload.RateLimitReachedType)
	if reachedType == "" {
		reachedType = strings.TrimSpace(payload.RateLimit.ReachedType)
	}
	record := UsageRecord{
		UsageBasis:                 "remaining",
		Unit:                       UnitPercent,
		Source:                     payload.Source,
		LastRefresh:                payloadRefreshTime(payload),
		PlanType:                   strings.TrimSpace(payload.PlanType),
		RateLimitAllowed:           payload.RateLimit.Allowed,
		RateLimitReachedType:       reachedType,
		CreditsAvailable:           firstBool(payload.Credits.HasCredits, payload.Credits.Available),
		CreditsUnlimited:           payload.Credits.Unlimited,
		CreditsBalance:             creditBalance(payload.Credits.Balance),
		CreditsApproxLocalMessages: creditMessageEstimate(payload.Credits.ApproxLocalMessages),
		CreditsApproxCloudMessages: creditMessageEstimate(payload.Credits.ApproxCloudMessages),
		CreditsOverageLimitReached: payload.Credits.OverageLimitReached,
		SpendControlReached:        payload.SpendControl.Reached,
	}
	if !weekly.hasNoData() {
		record.HasWeeklyWindow = true
		record.UsageWeekly = weekly.remainingPercent()
		record.RemainingWeekly = weekly.remainingPercent()
	} else if !legacyFive.hasNoData() {
		// Legacy sources remain parseable, but must not be treated as the current quota.
		record.Usage5h = legacyFive.remainingPercent()
		record.Remaining5h = legacyFive.remainingPercent()
	}
	record.Availability = normalizeAvailability(record, payload.RateLimit.LimitReached)
	return record, nil
}

func payloadHasEntitlement(payload rateLimitPayload) bool {
	return payload.RateLimit.Allowed != nil || payload.RateLimit.LimitReached != nil ||
		strings.TrimSpace(payload.RateLimitReachedType) != "" || strings.TrimSpace(payload.RateLimit.ReachedType) != "" ||
		strings.TrimSpace(payload.PlanType) != "" || payload.Credits.HasCredits != nil || payload.Credits.Available != nil || payload.Credits.Unlimited != nil ||
		len(bytes.TrimSpace(payload.Credits.Balance)) > 0 || len(bytes.TrimSpace(payload.Credits.ApproxLocalMessages)) > 0 ||
		len(bytes.TrimSpace(payload.Credits.ApproxCloudMessages)) > 0 || payload.Credits.OverageLimitReached != nil ||
		payload.SpendControl.Reached != nil || !payload.RateLimit.SecondaryWindow.hasNoData()
}

func creditBalance(raw json.RawMessage) *string {
	value := bytes.TrimSpace(raw)
	if len(value) == 0 || bytes.Equal(value, []byte("null")) {
		return nil
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		text = strings.TrimSpace(text)
		if text != "" {
			return &text
		}
		return nil
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		text = number.String()
		return &text
	}
	return nil
}

func creditMessageEstimate(raw json.RawMessage) *int {
	value := bytes.TrimSpace(raw)
	if len(value) == 0 || bytes.Equal(value, []byte("null")) {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil {
		return nil
	}
	parsed, err := number.Int64()
	if err != nil || parsed < 0 || parsed > int64(^uint(0)>>1) {
		return nil
	}
	result := int(parsed)
	return &result
}

func normalizeAvailability(record UsageRecord, limitReached *bool) string {
	if boolIsTrue(record.SpendControlReached) || boolIsTrue(record.CreditsOverageLimitReached) {
		return "blocked"
	}
	if boolIsTrue(record.RateLimitAllowed) {
		return "available"
	}
	if boolIsTrue(record.CreditsAvailable) || boolIsTrue(record.CreditsUnlimited) {
		return "credit_available"
	}
	if boolIsTrue(limitReached) || boolIsFalse(record.RateLimitAllowed) || strings.TrimSpace(record.RateLimitReachedType) != "" {
		return "limited"
	}
	if record.HasWeeklyWindow {
		if record.RemainingWeekly <= 0 {
			return "limited"
		}
		return "available"
	}
	return "unknown"
}

func boolIsTrue(value *bool) bool { return value != nil && *value }

func boolIsFalse(value *bool) bool { return value != nil && !*value }

func firstBool(values ...*bool) *bool {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func (w rateLimitWindow) hasNoData() bool {
	return !w.usedSet && !w.remainingSet && w.resetUnix() == 0
}

func (w rateLimitWindow) remainingPercent() float64 {
	if w.remainingSet && w.RemainingPercentage != 0 {
		return w.RemainingPercentage
	}
	if w.remainingSet {
		return w.RemainingPercent
	}
	used := w.UsedPercentage
	if w.UsedPercent != 0 {
		used = w.UsedPercent
	}
	if !w.usedSet {
		return 0
	}
	return 100 - used
}

func (w rateLimitWindow) resetUnix() int64 {
	if w.ResetsAt != 0 {
		return w.ResetsAt
	}
	return w.ResetAt
}

func (i rateLimitItem) hasNoData() bool {
	return !i.usedSet && !i.remainingSet && i.resetUnix() == 0
}

func (i rateLimitItem) remainingPercent() float64 {
	if i.remainingSet && i.RemainingPercentage != 0 {
		return i.RemainingPercentage
	}
	if i.remainingSet {
		return i.RemainingPercent
	}
	used := i.UsedPercentage
	if i.UsedPercent != 0 {
		used = i.UsedPercent
	}
	if !i.usedSet {
		return 0
	}
	return 100 - used
}

func (i rateLimitItem) resetUnix() int64 {
	if i.ResetsAt != 0 {
		return i.ResetsAt
	}
	return i.ResetAt
}

func payloadRefreshTime(payload rateLimitPayload) time.Time {
	if t := unixTime(payload.UpdatedAt); !t.IsZero() {
		return t
	}
	if payload.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339Nano, payload.Timestamp); err == nil {
			return t
		}
	}
	return time.Time{}
}

func unixString(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

func nearestReset(values ...int64) string {
	var best int64
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if best == 0 || value < best {
			best = value
		}
	}
	if best == 0 {
		return ""
	}
	return time.Unix(best, 0).UTC().Format(time.RFC3339)
}

func unixTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0).UTC()
}

func normalizeRecord(record UsageRecord, account accounts.Account, source, unit string, nowFunc func() time.Time) UsageRecord {
	record.Account = accountIdentity(account)
	if record.Unit == "" {
		record.Unit = unit
	}
	if record.Unit == "" {
		record.Unit = UnitPercent
	}
	if record.RemainingWeekly == 0 && record.UsageWeekly != 0 {
		record.RemainingWeekly = record.UsageWeekly
	}
	if record.UsageWeekly == 0 && record.RemainingWeekly != 0 {
		record.UsageWeekly = record.RemainingWeekly
	}
	if record.UsageBasis == "" {
		record.UsageBasis = "remaining"
	}
	if record.Availability == "" {
		record.Availability = normalizeAvailability(record, nil)
	}
	if record.Source == "" {
		record.Source = source
	}
	if record.LastRefresh.IsZero() {
		now := time.Now()
		if nowFunc != nil {
			now = nowFunc()
		}
		record.LastRefresh = now
	}
	return record
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func accountIdentity(account accounts.Account) string {
	if strings.TrimSpace(account.ProfileName) != "" {
		return account.ProfileName
	}
	return account.Nickname
}

func collectorAuthPath(account accounts.Account, activeAuthPath string) string {
	if account.Active && strings.TrimSpace(activeAuthPath) != "" && matchesActiveAuth(account, activeAuthPath) {
		return activeAuthPath
	}
	return account.AuthPath
}

func matchesActiveAuth(account accounts.Account, activeAuthPath string) bool {
	activeAuthPath = strings.TrimSpace(activeAuthPath)
	if activeAuthPath == "" {
		return account.Active
	}
	accountAuthPath := strings.TrimSpace(account.AuthPath)
	if accountAuthPath == "" {
		return account.Active
	}
	activeAuth, activeErr := ReadCodexAuth(activeAuthPath)
	accountAuth, accountErr := ReadCodexAuth(accountAuthPath)
	if activeErr == nil && accountErr == nil && activeAuth.AccountID != "" && accountAuth.AccountID != "" {
		return activeAuth.AccountID == accountAuth.AccountID
	}
	activeData, activeReadErr := os.ReadFile(activeAuthPath)
	accountData, accountReadErr := os.ReadFile(accountAuthPath)
	if activeReadErr == nil && accountReadErr == nil {
		return bytes.Equal(bytes.TrimSpace(activeData), bytes.TrimSpace(accountData))
	}
	return account.Active
}

func ExpandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if path == "" {
				return ""
			}
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home + path[1:]
		}
	}
	return path
}

type Orchestrator struct {
	Primary         Collector
	Fallback        Collector
	Fallbacks       []Collector
	AccountFallback Collector
}

func (o Orchestrator) Collect(ctx context.Context, account accounts.Account) (UsageRecord, error) {
	var errs []error
	collectors := []Collector{o.Primary, o.Fallback}
	collectors = append(collectors, o.Fallbacks...)
	collectors = append(collectors, o.AccountFallback)
	for _, collector := range collectors {
		if collector == nil {
			continue
		}
		record, err := collector.Collect(ctx, account)
		if err == nil {
			if record.Source == SourceAccount && record.Error == "" && len(errs) > 0 {
				record.Error = errors.Join(errs...).Error()
			}
			return record, nil
		}
		if !errors.Is(err, ErrNoCollector) {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return UsageRecord{}, ErrNoCollector
	}
	return UsageRecord{}, errors.Join(errs...)
}
