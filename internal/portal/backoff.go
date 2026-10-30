package portal

import (
	"sync"
	"time"
)

// syncBackoff keeps a failing source from being re-fetched on every scheduler
// tick: its LastSync never advances, so it always looks due. Retries wait
// 1m, 2m, 4m, ... capped at the source interval and at one hour. State is in
// memory and resets on restart.
type syncBackoff struct {
	mu      sync.Mutex
	running map[string]bool
	fails   map[string]int
	next    map[string]time.Time
}

func newSyncBackoff() syncBackoff {
	return syncBackoff{
		running: map[string]bool{},
		fails:   map[string]int{},
		next:    map[string]time.Time{},
	}
}

// begin reports whether a sync for key may start now and marks it running.
func (b *syncBackoff) begin(key string, force bool, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running[key] || (!force && now.Before(b.next[key])) {
		return false
	}
	b.running[key] = true
	return true
}

// end records the outcome and returns the retry delay after a failure (0 on
// success).
func (b *syncBackoff) end(key string, err error, intervalSec int, now time.Time) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.running, key)
	if err == nil {
		delete(b.fails, key)
		delete(b.next, key)
		return 0
	}
	b.fails[key]++
	d := retryDelay(b.fails[key], intervalSec)
	b.next[key] = now.Add(d)
	return d
}

func retryDelay(fails, intervalSec int) time.Duration {
	limit := time.Hour
	if iv := time.Duration(intervalSec) * time.Second; iv > 0 && iv < limit {
		limit = iv
	}
	d := time.Minute
	for i := 1; i < fails && d < limit; i++ {
		d *= 2
	}
	return min(d, limit)
}
