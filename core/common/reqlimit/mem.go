package reqlimit

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type windowCounts struct {
	normal int64
	over   int64
}

type entry struct {
	sync.Mutex
	windows    map[int64]*windowCounts
	lastAccess atomic.Value
}

type InMemoryRecord struct {
	entries sync.Map
}

func NewInMemoryRecord() *InMemoryRecord {
	rl := &InMemoryRecord{
		entries: sync.Map{},
	}
	go rl.cleanupInactiveEntries(2*time.Minute, 1*time.Minute)
	return rl
}

func (m *InMemoryRecord) getEntry(keys []string) *entry {
	key := strings.Join(keys, ":")
	actual, _ := m.entries.LoadOrStore(key, &entry{
		windows: make(map[int64]*windowCounts),
	})
	e, _ := actual.(*entry)
	if e.lastAccess.Load() == nil {
		e.lastAccess.CompareAndSwap(nil, time.Now())
	}
	return e
}

func (m *InMemoryRecord) cleanupAndCount(e *entry, cutoff int64) (int64, int64) {
	normalCount := int64(0)
	overCount := int64(0)
	for ts, wc := range e.windows {
		if ts < cutoff {
			delete(e.windows, ts)
		} else {
			normalCount += wc.normal
			overCount += wc.over
		}
	}
	return normalCount, overCount
}

func (m *InMemoryRecord) PushRequest(
	overed int64,
	duration time.Duration,
	n int64,
	keys ...string,
) (normalCount, overCount, secondCount int64) {
	e := m.getEntry(keys)

	e.Lock()
	defer e.Unlock()

	now := time.Now()
	e.lastAccess.Store(now)

	windowStart := now.Unix()
	cutoff := windowStart - int64(duration.Seconds())

	normalCount, overCount = m.cleanupAndCount(e, cutoff)

	wc, exists := e.windows[windowStart]
	if !exists {
		wc = &windowCounts{}
		e.windows[windowStart] = wc
	}

	if overed == 0 || normalCount <= overed {
		wc.normal += n
		normalCount += n
	} else {
		wc.over += n
		overCount += n
	}

	return normalCount, overCount, wc.normal + wc.over
}

func (m *InMemoryRecord) GetRequest(
	duration time.Duration,
	keys ...string,
) (totalCount, secondCount int64) {
	nowSecond := time.Now().Unix()
	cutoff := nowSecond - int64(duration.Seconds())

	m.entries.Range(func(key, value any) bool {
		k, _ := key.(string)
		currentKeys := parseKeys(k)

		if matchKeys(keys, currentKeys) {
			e, _ := value.(*entry)
			e.Lock()
			normalCount, overCount := m.cleanupAndCount(e, cutoff)
			nowWindow := e.windows[nowSecond]
			e.Unlock()
			totalCount += normalCount + overCount
			if nowWindow != nil {
				secondCount += nowWindow.normal + nowWindow.over
			}
		}
		return true
	})

	return totalCount, secondCount
}

func (m *InMemoryRecord) cleanupInactiveEntries(interval, maxInactivity time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		m.entries.Range(func(key, value any) bool {
			e, _ := value.(*entry)
			la := e.lastAccess.Load()
			if la == nil {
				return true
			}
			lastAccess, _ := la.(time.Time)
			if time.Since(lastAccess) > maxInactivity {
				m.entries.CompareAndDelete(key, e)
			}
			return true
		})
	}
}

func parseKeys(key string) []string {
	return strings.Split(key, ":")
}

func matchKeys(pattern, keys []string) bool {
	if len(pattern) != len(keys) {
		return false
	}

	for i, p := range pattern {
		if p != "*" && p != keys[i] {
			return false
		}
	}
	return true
}
