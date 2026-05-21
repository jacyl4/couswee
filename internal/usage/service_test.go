package usage

import (
	"context"
	"errors"
	"testing"
	"time"

	"couswee/internal/accounts"
)

func TestCacheSnapshotIsolation(t *testing.T) {
	cache := NewCache()
	cache.Replace([]UsageRecord{{Account: "Dev1", Usage5h: 1}})
	snapshot := cache.Snapshot()
	snapshot[0].Usage5h = 99
	if got := cache.Snapshot()[0].Usage5h; got != 1 {
		t.Fatalf("cache mutated through snapshot: %v", got)
	}
}

func TestServiceRefreshSuccess(t *testing.T) {
	svc := NewService(DefaultConfig(), collectorFunc(func(context.Context, accounts.Account) (UsageRecord, error) {
		return UsageRecord{Account: "Dev1", Usage5h: 3, Unit: UnitTokens, Source: SourceAPI}, nil
	}), func() []accounts.Account { return []accounts.Account{{Nickname: "Dev1"}} })
	svc.Refresh(context.Background())
	records := svc.Records()
	if len(records) != 1 || records[0].Usage5h != 3 || records[0].Stale {
		t.Fatalf("records = %#v", records)
	}
}

func TestServicePreservesStaleOnFailure(t *testing.T) {
	fail := false
	svc := NewService(DefaultConfig(), collectorFunc(func(context.Context, accounts.Account) (UsageRecord, error) {
		if fail {
			return UsageRecord{}, errors.New("boom")
		}
		return UsageRecord{Account: "Dev1", Usage5h: 3, Unit: UnitTokens, Source: SourceAPI}, nil
	}), func() []accounts.Account { return []accounts.Account{{Nickname: "Dev1"}} })
	svc.now = func() time.Time { return time.Unix(20, 0).UTC() }
	svc.Refresh(context.Background())
	fail = true
	svc.Refresh(context.Background())
	record := svc.Records()[0]
	if !record.Stale || record.Error == "" || record.Usage5h != 3 {
		t.Fatalf("record = %#v", record)
	}
}

func TestServicePartialFailure(t *testing.T) {
	svc := NewService(DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (UsageRecord, error) {
		if account.Nickname == "Dev1" {
			return UsageRecord{}, errors.New("boom")
		}
		return UsageRecord{Account: account.Nickname, Usage5h: 4, Unit: UnitTokens, Source: SourceAPI}, nil
	}), func() []accounts.Account { return []accounts.Account{{Nickname: "Dev1"}, {Nickname: "Dev2"}} })
	svc.Refresh(context.Background())
	records := svc.Records()
	if len(records) != 2 {
		t.Fatalf("records = %#v", records)
	}
	seen := map[string]UsageRecord{}
	for _, record := range records {
		seen[record.Account] = record
	}
	if !seen["Dev1"].Stale || seen["Dev2"].Stale || seen["Dev2"].Usage5h != 4 {
		t.Fatalf("records = %#v", records)
	}
}

func TestServicePersistsSuccessfulUsageToAccountSink(t *testing.T) {
	var persisted []accounts.Account
	svc := NewService(DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (UsageRecord, error) {
		return UsageRecord{
			Account:         account.Nickname,
			Remaining5h:     42.4,
			RemainingWeekly: 87.6,
			ResetTime5h:     "2026-05-20T23:00:00+08:00",
			ResetTimeWeekly: "2026-05-24T23:00:00+08:00",
			Unit:            UnitPercent,
			Source:          SourceAPI,
		}, nil
	}), func() []accounts.Account {
		return []accounts.Account{{Nickname: "Dev1", Usage5h: 0, UsageWeekly: 0}}
	})
	svc.SetAccountSink(func(updated []accounts.Account) error {
		persisted = append([]accounts.Account(nil), updated...)
		return nil
	})

	svc.Refresh(context.Background())

	if len(persisted) != 1 || persisted[0].Usage5h != 42 || persisted[0].UsageWeekly != 88 || persisted[0].ResetTime5h == "" || persisted[0].ResetTimeWeekly == "" {
		t.Fatalf("persisted = %#v", persisted)
	}
}

func TestServiceDoesNotPersistErrorUsageToAccountSink(t *testing.T) {
	var persisted []accounts.Account
	svc := NewService(DefaultConfig(), collectorFunc(func(context.Context, accounts.Account) (UsageRecord, error) {
		return UsageRecord{}, errors.New("boom")
	}), func() []accounts.Account {
		return []accounts.Account{{Nickname: "Dev1", Usage5h: 33, UsageWeekly: 44}}
	})
	svc.SetAccountSink(func(updated []accounts.Account) error {
		persisted = append([]accounts.Account(nil), updated...)
		return nil
	})

	svc.Refresh(context.Background())

	if len(persisted) != 1 || persisted[0].Usage5h != 33 || persisted[0].UsageWeekly != 44 || !persisted[0].UsageStale || persisted[0].UsageError == "" {
		t.Fatalf("persisted = %#v", persisted)
	}
}

func TestServiceDoesNotPersistAccountFallbackToAccountSink(t *testing.T) {
	var persisted []accounts.Account
	svc := NewService(DefaultConfig(), AccountCollector{Unit: UnitPercent}, func() []accounts.Account {
		return []accounts.Account{{Nickname: "Dev1", Usage5h: 33, UsageWeekly: 44}}
	})
	svc.SetAccountSink(func(updated []accounts.Account) error {
		persisted = append([]accounts.Account(nil), updated...)
		return nil
	})

	svc.Refresh(context.Background())

	if len(persisted) != 1 || persisted[0].Usage5h != 33 || persisted[0].UsageWeekly != 44 || !persisted[0].UsageStale || persisted[0].UsageSource != SourceAccount {
		t.Fatalf("persisted = %#v", persisted)
	}
}

func TestServiceRefreshAccountOnlyCollectsMatchingAccount(t *testing.T) {
	var collected []string
	svc := NewService(DefaultConfig(), collectorFunc(func(_ context.Context, account accounts.Account) (UsageRecord, error) {
		collected = append(collected, account.Nickname)
		return UsageRecord{Account: account.Nickname, Usage5h: 3, Unit: UnitPercent, Source: SourceAPI}, nil
	}), func() []accounts.Account {
		return []accounts.Account{{ID: "1", Nickname: "Dev1"}, {ID: "2", Nickname: "Dev2"}}
	})

	if !svc.RefreshAccount(context.Background(), "2") {
		t.Fatal("RefreshAccount returned false")
	}
	if len(collected) != 1 || collected[0] != "Dev2" {
		t.Fatalf("collected = %#v", collected)
	}
	records := svc.Records()
	if len(records) != 1 || records[0].Account != "Dev2" {
		t.Fatalf("records = %#v", records)
	}
}
