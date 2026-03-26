package reqlimit

import (
	"path"
	"slices"
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
	windows              map[int64]*windowCounts
	lastAccess           atomic.Value
	windowSeconds        int64
	totalNormal          int64
	totalOver            int64
	lastCleanedCutoff    int64
	aggregateInitialized bool
}

type InMemoryRecord struct {
	entries sync.Map
}

type recordSnapshot struct {
	Keys        []string
	TotalCount  int64
	SecondCount int64
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

func (m *InMemoryRecord) rebuildAggregateLocked(e *entry, windowSeconds, cutoff int64) {
	normalCount := int64(0)
	overCount := int64(0)

	for ts, wc := range e.windows {
		if ts < cutoff {
			delete(e.windows, ts)
			continue
		}

		normalCount += wc.normal
		overCount += wc.over
	}

	e.windowSeconds = windowSeconds
	e.totalNormal = normalCount
	e.totalOver = overCount
	e.lastCleanedCutoff = cutoff
	e.aggregateInitialized = true
}

func (m *InMemoryRecord) refreshAggregateLocked(e *entry, nowSecond, windowSeconds int64) {
	cutoff := nowSecond - windowSeconds

	if !e.aggregateInitialized || e.windowSeconds != windowSeconds {
		m.rebuildAggregateLocked(e, windowSeconds, cutoff)
		return
	}

	for ts := e.lastCleanedCutoff; ts < cutoff; ts++ {
		wc, ok := e.windows[ts]
		if !ok {
			continue
		}

		e.totalNormal -= wc.normal
		e.totalOver -= wc.over
		delete(e.windows, ts)
	}

	if e.lastCleanedCutoff < cutoff {
		e.lastCleanedCutoff = cutoff
	}
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
	windowSeconds := int64(duration.Seconds())
	m.refreshAggregateLocked(e, windowStart, windowSeconds)

	wc, exists := e.windows[windowStart]
	if !exists {
		wc = &windowCounts{}
		e.windows[windowStart] = wc
	}

	if overed == 0 || e.totalNormal <= overed {
		wc.normal += n
		e.totalNormal += n
	} else {
		wc.over += n
		e.totalOver += n
	}

	return e.totalNormal, e.totalOver, wc.normal + wc.over
}

func (m *InMemoryRecord) GetRequest(
	duration time.Duration,
	keys ...string,
) (totalCount, secondCount int64) {
	nowSecond := time.Now().Unix()
	windowSeconds := int64(duration.Seconds())

	if !hasWildcard(keys) {
		value, ok := m.entries.Load(strings.Join(keys, ":"))
		if !ok {
			return 0, 0
		}

		e, _ := value.(*entry)
		e.Lock()
		m.refreshAggregateLocked(e, nowSecond, windowSeconds)
		nowWindow := e.windows[nowSecond]
		totalCount = e.totalNormal + e.totalOver
		e.Unlock()

		if nowWindow != nil {
			secondCount = nowWindow.normal + nowWindow.over
		}

		return totalCount, secondCount
	}

	m.entries.Range(func(key, value any) bool {
		k, _ := key.(string)
		currentKeys := parseKeys(k)

		if matchKeys(keys, currentKeys) {
			e, _ := value.(*entry)
			e.Lock()
			m.refreshAggregateLocked(e, nowSecond, windowSeconds)
			nowWindow := e.windows[nowSecond]
			entryTotalCount := e.totalNormal + e.totalOver
			e.Unlock()

			totalCount += entryTotalCount

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

func (m *InMemoryRecord) Snapshot(duration time.Duration) []recordSnapshot {
	nowSecond := time.Now().Unix()
	windowSeconds := int64(duration.Seconds())
	snapshots := make([]recordSnapshot, 0)

	m.entries.Range(func(key, value any) bool {
		k, _ := key.(string)
		e, _ := value.(*entry)
		e.Lock()
		m.refreshAggregateLocked(e, nowSecond, windowSeconds)
		nowWindow := e.windows[nowSecond]
		totalCount := e.totalNormal + e.totalOver
		e.Unlock()

		secondCount := int64(0)
		if nowWindow != nil {
			secondCount = nowWindow.normal + nowWindow.over
		}

		snapshots = append(snapshots, recordSnapshot{
			Keys:        parseKeys(k),
			TotalCount:  totalCount,
			SecondCount: secondCount,
		})

		return true
	})

	return snapshots
}

func (m *InMemoryRecord) SnapshotByPattern(duration time.Duration, keys ...string) []recordSnapshot {
	nowSecond := time.Now().Unix()
	windowSeconds := int64(duration.Seconds())
	snapshots := make([]recordSnapshot, 0)

	if !hasWildcard(keys) {
		value, ok := m.entries.Load(strings.Join(keys, ":"))
		if !ok {
			return snapshots
		}

		e, _ := value.(*entry)
		e.Lock()
		m.refreshAggregateLocked(e, nowSecond, windowSeconds)
		nowWindow := e.windows[nowSecond]
		totalCount := e.totalNormal + e.totalOver
		e.Unlock()

		secondCount := int64(0)
		if nowWindow != nil {
			secondCount = nowWindow.normal + nowWindow.over
		}

		return []recordSnapshot{{
			Keys:        append([]string(nil), keys...),
			TotalCount:  totalCount,
			SecondCount: secondCount,
		}}
	}

	m.entries.Range(func(key, value any) bool {
		k, _ := key.(string)
		currentKeys := parseKeys(k)
		if !matchKeys(keys, currentKeys) {
			return true
		}

		e, _ := value.(*entry)
		e.Lock()
		m.refreshAggregateLocked(e, nowSecond, windowSeconds)
		nowWindow := e.windows[nowSecond]
		totalCount := e.totalNormal + e.totalOver
		e.Unlock()

		secondCount := int64(0)
		if nowWindow != nil {
			secondCount = nowWindow.normal + nowWindow.over
		}

		snapshots = append(snapshots, recordSnapshot{
			Keys:        currentKeys,
			TotalCount:  totalCount,
			SecondCount: secondCount,
		})

		return true
	})

	return snapshots
}

func parseKeys(key string) []string {
	return strings.Split(key, ":")
}

func matchKeys(pattern, keys []string) bool {
	if len(pattern) != len(keys) {
		return false
	}

	for i, p := range pattern {
		if isGlobPattern(p) {
			matched, err := path.Match(p, keys[i])
			if err != nil || !matched {
				return false
			}

			continue
		}

		if p != keys[i] {
			return false
		}
	}

	return true
}

func hasWildcard(keys []string) bool {
	return slices.ContainsFunc(keys, isGlobPattern)
}

func isGlobPattern(key string) bool {
	return strings.ContainsAny(key, "*?[")
}
