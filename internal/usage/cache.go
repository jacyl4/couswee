package usage

import "sync"

type Cache struct {
	mu      sync.RWMutex
	records map[string]UsageRecord
}

func NewCache() *Cache {
	return &Cache{records: map[string]UsageRecord{}}
}

func (c *Cache) Snapshot() []UsageRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]UsageRecord, 0, len(c.records))
	for _, record := range c.records {
		out = append(out, record)
	}
	return out
}

func (c *Cache) SnapshotAllowed(allowed map[string]struct{}) []UsageRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]UsageRecord, 0, len(c.records))
	for key, record := range c.records {
		if _, ok := allowed[key]; !ok {
			continue
		}
		out = append(out, record)
	}
	return out
}

func (c *Cache) Replace(records []UsageRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := make(map[string]UsageRecord, len(records))
	for _, record := range records {
		next[record.Account] = record
	}
	c.records = next
}

func (c *Cache) Merge(records []UsageRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, record := range records {
		c.records[record.Account] = record
	}
}

func (c *Cache) Prune(allowed map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.records {
		if _, ok := allowed[key]; !ok {
			delete(c.records, key)
		}
	}
}

func (c *Cache) Get(account string) (UsageRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	record, ok := c.records[account]
	return record, ok
}
