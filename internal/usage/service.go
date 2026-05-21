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
	return s.cache.Snapshot()
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
		if account.ID != selector && account.Nickname != selector && account.ProfileName != selector {
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
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	record, err := s.collector.Collect(ctx, account)
	if err != nil {
		if cached, ok := s.cache.Get(account.Nickname); ok {
			cached.Stale = true
			cached.Error = err.Error()
			cached.LastRefresh = s.now()
			return cached
		}
		fallback := AccountCollector{Unit: cfgUnitOrPercent(), Now: s.now}.Collect
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
		record, ok := byAccount[next[i].Nickname]
		if !ok {
			continue
		}
		liveSuccess := !record.Stale && record.Error == "" && record.Source != "error" && record.Source != SourceAccount
		usage5h := next[i].Usage5h
		usageWeekly := next[i].UsageWeekly
		resetTime5h := record.ResetTime5h
		if resetTime5h == "" {
			resetTime5h = record.ResetTime
		}
		resetTimeWeekly := record.ResetTimeWeekly
		if liveSuccess {
			usage5h = int(math.Round(clampPercent(recordValue(record.Remaining5h, record.Usage5h))))
			usageWeekly = int(math.Round(clampPercent(recordValue(record.RemainingWeekly, record.UsageWeekly))))
		} else {
			resetTime5h = next[i].ResetTime5h
			resetTimeWeekly = next[i].ResetTimeWeekly
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
	}
	if changed {
		_ = s.sink(next)
	}
}

func recordValue(primary, fallback float64) float64 {
	if primary != 0 {
		return primary
	}
	return fallback
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
		primary = APICollector{URL: cfg.APIURL, Unit: cfg.Unit, ActiveAuthPath: cfg.ActiveAuthPath}
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

func cfgUnitOrPercent() string {
	return UnitPercent
}
