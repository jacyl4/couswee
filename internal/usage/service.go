package usage

import (
	"context"
	"math"
	"sync"
	"time"

	"couswee/internal/accounts"
)

type AccountSource func() []accounts.Account
type AccountSink func([]accounts.Account) error

type Service struct {
	mu        sync.Mutex
	cache     *Cache
	collector Collector
	accounts  AccountSource
	sink      AccountSink
	interval  time.Duration
	timeout   time.Duration
	unit      string
	now       func() time.Time
}

func NewService(cfg Config, collector Collector, accountSource AccountSource) *Service {
	if collector == nil {
		collector = AccountCollector{Unit: cfg.Unit, ActiveAuthPath: cfg.ActiveAuthPath}
	}
	return &Service{
		cache:     NewCache(),
		collector: collector,
		accounts:  accountSource,
		interval:  ClampRefreshInterval(cfg.RefreshInterval),
		timeout:   cfg.FallbackTimeout,
		unit:      unitOrPercent(cfg.Unit),
		now:       time.Now,
	}
}

func (s *Service) SetAccountSink(sink AccountSink) {
	if s != nil {
		s.sink = sink
	}
}

func (s *Service) Records() []UsageRecord {
	if s == nil || s.cache == nil {
		return []UsageRecord{}
	}
	if s.accounts != nil {
		return s.cache.SnapshotAllowed(accountKeys(s.accounts()))
	}
	return s.cache.Snapshot()
}

func (s *Service) PruneCurrentAccounts() {
	if s == nil || s.cache == nil || s.accounts == nil {
		return
	}
	s.cache.Prune(accountKeys(s.accounts()))
}

func (s *Service) Refresh(ctx context.Context) {
	s.RefreshAllWithReason(ctx, RefreshReasonManual)
}

func (s *Service) RefreshAll(ctx context.Context) {
	s.RefreshAllWithReason(ctx, RefreshReasonManual)
}

func (s *Service) RefreshAllWithReason(ctx context.Context, reason RefreshReason) {
	if s == nil || s.accounts == nil || s.collector == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = ContextWithRefreshReason(ctx, reason)
	s.mu.Lock()
	defer s.mu.Unlock()

	accountsList := s.accounts()
	if len(accountsList) == 0 {
		s.cache.Replace([]UsageRecord{})
		return
	}
	records := make([]UsageRecord, 0, len(accountsList))
	for _, account := range accountsList {
		records = append(records, s.collectAccount(ctx, account))
	}
	s.cache.Replace(records)
	s.persistUsage(accountsList, records)
}

func (s *Service) RefreshAccount(ctx context.Context, selector string) bool {
	return s.RefreshAccountWithReason(ctx, selector, RefreshReasonManual)
}

func (s *Service) RefreshAccountWithReason(ctx context.Context, selector string, reason RefreshReason) bool {
	if s == nil || s.accounts == nil || s.collector == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = ContextWithRefreshReason(ctx, reason)
	s.mu.Lock()
	defer s.mu.Unlock()

	accountsList := s.accounts()
	for _, account := range accountsList {
		if account.ID != selector && accountIdentity(account) != selector {
			continue
		}
		record := s.collectAccount(ctx, account)
		s.cache.Merge([]UsageRecord{record})
		s.persistUsage(accountsList, []UsageRecord{record})
		return true
	}
	return false
}

func (s *Service) collectAccount(ctx context.Context, account accounts.Account) UsageRecord {
	if shouldSkipUsageRefresh(account) {
		return s.skippedUsageRecord(account)
	}
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	record, err := s.collector.Collect(ctx, account)
	if err != nil {
		if cached, ok := s.cache.Get(accountIdentity(account)); ok {
			cached.Stale = true
			cached.Error = err.Error()
			cached.LastRefresh = s.now()
			return cached
		}
		fallback := AccountCollector{Unit: s.unit, Now: s.now}.Collect
		record, _ := fallback(ctx, account)
		record.Stale = true
		record.Error = err.Error()
		if record.Source == "" {
			record.Source = "error"
		}
		if record.LastRefresh.IsZero() {
			record.LastRefresh = s.now()
		}
		return record
	}
	if record.Source != SourceAccount {
		record.Stale = false
		record.Error = ""
	}
	return record
}

func shouldSkipUsageRefresh(account accounts.Account) bool {
	if account.AuthExpired {
		return true
	}
	switch account.AuthStatus {
	case "expired", "missing", "invalid":
		return true
	default:
		return false
	}
}

func (s *Service) skippedUsageRecord(account accounts.Account) UsageRecord {
	lastRefresh := parseAuthTime(account.UsageLastRefresh)
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
		Unit:                       unitOrPercent(s.unit),
		UsageBasis:                 "remaining",
		Source:                     SourceAccount,
		LastRefresh:                lastRefresh,
		Stale:                      true,
	}
}

func (s *Service) Start(ctx context.Context) {
	if s == nil || s.interval <= 0 {
		return
	}
	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.RefreshAllWithReason(ctx, RefreshReasonPeriodic)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Service) persistUsage(accountsList []accounts.Account, records []UsageRecord) {
	if s == nil || s.sink == nil {
		return
	}
	byAccount := make(map[string]UsageRecord, len(records))
	for _, record := range records {
		if record.Account == "" {
			continue
		}
		byAccount[record.Account] = record
	}
	if len(byAccount) == 0 {
		return
	}

	changed := false
	next := append([]accounts.Account(nil), accountsList...)
	for i := range next {
		record, ok := byAccount[accountIdentity(next[i])]
		if !ok {
			continue
		}
		liveSuccess := !record.Stale && record.Error == "" && record.Source != "error" && record.Source != SourceAccount
		usage5h := next[i].Usage5h
		usageWeekly := next[i].UsageWeekly
		resetTime5h := next[i].ResetTime5h
		resetTimeWeekly := next[i].ResetTimeWeekly
		if liveSuccess {
			usageWeekly = int(math.Round(clampPercent(remainingPercent(record, record.RemainingWeekly, record.UsageWeekly))))
		}
		lastRefresh := ""
		if !record.LastRefresh.IsZero() {
			lastRefresh = record.LastRefresh.UTC().Format(time.RFC3339)
		}
		if next[i].Usage5h != usage5h ||
			next[i].UsageWeekly != usageWeekly ||
			next[i].ResetTime5h != resetTime5h ||
			next[i].ResetTimeWeekly != resetTimeWeekly ||
			next[i].UsageSource != record.Source ||
			next[i].UsageLastRefresh != lastRefresh ||
			next[i].UsageStale != record.Stale ||
			next[i].UsageError != record.Error {
			next[i].Usage5h = usage5h
			next[i].UsageWeekly = usageWeekly
			next[i].ResetTime5h = resetTime5h
			next[i].ResetTimeWeekly = resetTimeWeekly
			next[i].UsageSource = record.Source
			next[i].UsageLastRefresh = lastRefresh
			next[i].UsageStale = record.Stale
			next[i].UsageError = record.Error
			changed = true
		}
		if liveSuccess && !sameEntitlement(next[i], record) {
			next[i].HasWeeklyWindow = record.HasWeeklyWindow
			next[i].Availability = record.Availability
			next[i].PlanType = record.PlanType
			next[i].RateLimitAllowed = record.RateLimitAllowed
			next[i].RateLimitReachedType = record.RateLimitReachedType
			next[i].CreditsAvailable = record.CreditsAvailable
			next[i].CreditsUnlimited = record.CreditsUnlimited
			next[i].CreditsBalance = record.CreditsBalance
			next[i].CreditsApproxLocalMessages = record.CreditsApproxLocalMessages
			next[i].CreditsApproxCloudMessages = record.CreditsApproxCloudMessages
			next[i].CreditsOverageLimitReached = record.CreditsOverageLimitReached
			next[i].SpendControlReached = record.SpendControlReached
			changed = true
		}
	}
	if changed {
		_ = s.sink(next)
	}
}

func sameEntitlement(account accounts.Account, record UsageRecord) bool {
	return account.HasWeeklyWindow == record.HasWeeklyWindow &&
		account.Availability == record.Availability &&
		account.PlanType == record.PlanType &&
		optionalBoolEqual(account.RateLimitAllowed, record.RateLimitAllowed) &&
		account.RateLimitReachedType == record.RateLimitReachedType &&
		optionalBoolEqual(account.CreditsAvailable, record.CreditsAvailable) &&
		optionalBoolEqual(account.CreditsUnlimited, record.CreditsUnlimited) &&
		optionalStringEqual(account.CreditsBalance, record.CreditsBalance) &&
		optionalIntEqual(account.CreditsApproxLocalMessages, record.CreditsApproxLocalMessages) &&
		optionalIntEqual(account.CreditsApproxCloudMessages, record.CreditsApproxCloudMessages) &&
		optionalBoolEqual(account.CreditsOverageLimitReached, record.CreditsOverageLimitReached) &&
		optionalBoolEqual(account.SpendControlReached, record.SpendControlReached)
}

func optionalBoolEqual(left, right *bool) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func optionalStringEqual(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func optionalIntEqual(left, right *int) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func remainingPercent(record UsageRecord, remaining, usage float64) float64 {
	if record.UsageBasis == "used" {
		return 100 - usage
	}
	if remaining != 0 || usage == 0 {
		return remaining
	}
	return usage
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func BuildCollector(cfg Config) Collector {
	var primary Collector
	if cfg.APIEnabled && cfg.APIURL != "" {
		var refresher AuthRefresher
		if cfg.AuthRefreshEnabled {
			refresher = CodexCLIAuthRefresher{Timeout: cfg.AuthRefreshTimeout}
		}
		primary = APICollector{URL: cfg.APIURL, Unit: cfg.Unit, ActiveAuthPath: cfg.ActiveAuthPath, AuthRefresher: refresher}
	}
	fallbacks := []Collector{}
	if cfg.FallbackCommand != "" {
		fallbacks = append(fallbacks, CommandCollector{Command: cfg.FallbackCommand, Unit: cfg.Unit, Timeout: cfg.FallbackTimeout, ActiveAuthPath: cfg.ActiveAuthPath})
	}
	if cfg.SessionGlob != "" {
		fallbacks = append(fallbacks, SessionLogCollector{Glob: cfg.SessionGlob, Unit: cfg.Unit, ActiveAuthPath: cfg.ActiveAuthPath})
	}
	return Orchestrator{
		Primary:         primary,
		Fallbacks:       fallbacks,
		AccountFallback: AccountCollector{Unit: cfg.Unit, ActiveAuthPath: cfg.ActiveAuthPath},
	}
}

func unitOrPercent(unit string) string {
	if unit != "" {
		return unit
	}
	return UnitPercent
}

func accountKeys(accountsList []accounts.Account) map[string]struct{} {
	keys := make(map[string]struct{}, len(accountsList))
	for _, account := range accountsList {
		key := accountIdentity(account)
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}
	return keys
}
