package monitor

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"
)

var memModelMonitor *MemModelMonitor

func init() {
	memModelMonitor = NewMemModelMonitor()
}

const (
	timeWindow      = 10 * time.Second
	maxSliceCount   = 12
	banDuration     = 5 * time.Minute
	minRequestCount = 10
	cleanupInterval = time.Minute
)

func getBanDuration() time.Duration {
	jitter := banDuration / 10
	if jitter <= 0 {
		return banDuration
	}

	return banDuration + time.Duration(rand.Int64N(int64(jitter)*2+1)) - jitter
}

type MemModelMonitor struct {
	mu     sync.RWMutex
	models map[string]*ModelData
}

type ModelData struct {
	channels   map[int64]*ChannelStats
	totalStats *TimeWindowStats
}

type ChannelStats struct {
	timeWindows *TimeWindowStats
	bannedUntil time.Time
}

type ModelChannelStatsSnapshot struct {
	Requests int64 `json:"requests"`
	Errors   int64 `json:"errors"`
	Banned   bool  `json:"banned"`
}

type TimeWindowStats struct {
	slices               []*timeSlice
	mu                   sync.Mutex
	totalRequests        int
	totalErrors          int
	lastCleanedWindowEnd time.Time
	cacheInitialized     bool
}

type timeSlice struct {
	windowStart time.Time
	requests    int
	errors      int
}

func NewTimeWindowStats() *TimeWindowStats {
	return &TimeWindowStats{
		slices: make([]*timeSlice, 0, maxSliceCount),
	}
}

func NewMemModelMonitor() *MemModelMonitor {
	mm := &MemModelMonitor{
		models: make(map[string]*ModelData),
	}

	go mm.periodicCleanup()

	return mm
}

func (m *MemModelMonitor) periodicCleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpiredData()
	}
}

func (m *MemModelMonitor) cleanupExpiredData() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for modelName, modelData := range m.models {
		for channelID, channelStats := range modelData.channels {
			hasValidSlices := channelStats.timeWindows.HasValidSlices()
			if !hasValidSlices && !channelStats.bannedUntil.After(now) {
				delete(modelData.channels, channelID)
			}
		}

		hasValidSlices := modelData.totalStats.HasValidSlices()
		if !hasValidSlices && len(modelData.channels) == 0 {
			delete(m.models, modelName)
		}
	}
}

func (m *MemModelMonitor) AddRequest(
	model string,
	channelID int64,
	isError, tryBan bool,
	maxErrorRate float64,
) (errorRate float64, banExecution bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	var (
		modelData *ModelData
		exists    bool
	)

	if modelData, exists = m.models[model]; !exists {
		modelData = &ModelData{
			channels:   make(map[int64]*ChannelStats),
			totalStats: NewTimeWindowStats(),
		}
		m.models[model] = modelData
	}

	var channel *ChannelStats
	if channel, exists = modelData.channels[channelID]; !exists {
		channel = &ChannelStats{
			timeWindows: NewTimeWindowStats(),
		}
		modelData.channels[channelID] = channel
	}

	modelData.totalStats.AddRequest(now, isError)
	channel.timeWindows.AddRequest(now, isError)

	return m.checkAndBan(now, channel, tryBan, maxErrorRate)
}

func (m *MemModelMonitor) checkAndBan(
	now time.Time,
	channel *ChannelStats,
	tryBan bool,
	maxErrorRate float64,
) (errorRate float64, banExecution bool) {
	errorRate = getErrorRateFromStats(channel.timeWindows)

	if tryBan {
		if channel.bannedUntil.After(now) {
			return errorRate, false
		}

		channel.bannedUntil = now.Add(getBanDuration())

		return errorRate, true
	}

	// Check if we should ban (maxErrorRate <= 0 disables banning)
	if maxErrorRate > 0 && errorRate >= maxErrorRate {
		if channel.bannedUntil.After(now) {
			return errorRate, false
		}

		channel.bannedUntil = now.Add(getBanDuration())

		return errorRate, true
	}

	return errorRate, false
}

func getErrorRateFromStats(stats *TimeWindowStats) float64 {
	req, err := stats.GetStats()
	if req < minRequestCount {
		return 0
	}

	return float64(err) / float64(req)
}

func (m *MemModelMonitor) GetModelsErrorRate(_ context.Context) (map[string]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]float64)
	for model, data := range m.models {
		result[model] = getErrorRateFromStats(data.totalStats)
	}

	return result, nil
}

func (m *MemModelMonitor) GetModelChannelErrorRate(
	_ context.Context,
	model string,
) (map[int64]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[int64]float64)
	if data, exists := m.models[model]; exists {
		for channelID, channel := range data.channels {
			result[channelID] = getErrorRateFromStats(channel.timeWindows)
		}
	}

	return result, nil
}

func (m *MemModelMonitor) GetChannelModelErrorRate(
	_ context.Context,
	model string,
	channelID int64,
) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.models[model]
	if !exists {
		return 0, nil
	}

	channel, exists := data.channels[channelID]
	if !exists {
		return 0, nil
	}

	return getErrorRateFromStats(channel.timeWindows), nil
}

func (m *MemModelMonitor) GetChannelModelErrorRates(
	_ context.Context,
	channelID int64,
) (map[string]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]float64)
	for model, data := range m.models {
		if channel, exists := data.channels[channelID]; exists {
			result[model] = getErrorRateFromStats(channel.timeWindows)
		}
	}

	return result, nil
}

func (m *MemModelMonitor) GetAllChannelModelErrorRates(
	_ context.Context,
) (map[int64]map[string]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[int64]map[string]float64)
	for model, data := range m.models {
		for channelID, channel := range data.channels {
			if _, exists := result[channelID]; !exists {
				result[channelID] = make(map[string]float64)
			}

			result[channelID][model] = getErrorRateFromStats(channel.timeWindows)
		}
	}

	return result, nil
}

func (m *MemModelMonitor) GetAllModelChannelStats(
	_ context.Context,
) (map[string]map[int64]ModelChannelStatsSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]map[int64]ModelChannelStatsSnapshot)
	now := time.Now()

	for model, data := range m.models {
		if _, exists := result[model]; !exists {
			result[model] = make(map[int64]ModelChannelStatsSnapshot)
		}

		for channelID, channel := range data.channels {
			req, err := channel.timeWindows.GetStats()
			result[model][channelID] = ModelChannelStatsSnapshot{
				Requests: int64(req),
				Errors:   int64(err),
				Banned:   channel.bannedUntil.After(now),
			}
		}
	}

	return result, nil
}

func (m *MemModelMonitor) GetBannedChannelsWithModel(
	_ context.Context,
	model string,
) ([]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var banned []int64
	if data, exists := m.models[model]; exists {
		now := time.Now()
		for channelID, channel := range data.channels {
			if channel.bannedUntil.After(now) {
				banned = append(banned, channelID)
			}
		}
	}

	return banned, nil
}

func (m *MemModelMonitor) GetBannedChannelsMapWithModel(
	_ context.Context,
	model string,
) (map[int64]struct{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	banned := make(map[int64]struct{})
	if data, exists := m.models[model]; exists {
		now := time.Now()
		for channelID, channel := range data.channels {
			if channel.bannedUntil.After(now) {
				banned[channelID] = struct{}{}
			}
		}
	}

	return banned, nil
}

func (m *MemModelMonitor) GetAllBannedModelChannels(_ context.Context) (map[string][]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]int64)
	now := time.Now()

	for model, data := range m.models {
		for channelID, channel := range data.channels {
			if channel.bannedUntil.After(now) {
				if _, exists := result[model]; !exists {
					result[model] = []int64{}
				}

				result[model] = append(result[model], channelID)
			}
		}
	}

	return result, nil
}

func (m *MemModelMonitor) ClearChannelModelErrors(
	_ context.Context,
	model string,
	channelID int,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if data, exists := m.models[model]; exists {
		delete(data.channels, int64(channelID))
	}

	return nil
}

func (m *MemModelMonitor) ClearChannelAllModelErrors(_ context.Context, channelID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, data := range m.models {
		delete(data.channels, int64(channelID))
	}

	return nil
}

func (m *MemModelMonitor) ClearAllModelErrors(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.models = make(map[string]*ModelData)

	return nil
}

func (t *TimeWindowStats) rebuildLocked(cutoff time.Time) {
	validSlices := t.slices[:0]
	totalReq := 0

	totalErr := 0
	for _, s := range t.slices {
		if s.windowStart.Before(cutoff) {
			continue
		}

		validSlices = append(validSlices, s)
		totalReq += s.requests
		totalErr += s.errors
	}

	t.slices = validSlices
	t.totalRequests = totalReq
	t.totalErrors = totalErr
	t.lastCleanedWindowEnd = cutoff
	t.cacheInitialized = true
}

func (t *TimeWindowStats) cleanupLocked(now time.Time, callback func(slice *timeSlice)) {
	cutoff := now.Truncate(timeWindow).Add(-timeWindow * time.Duration(maxSliceCount-1))

	if !t.cacheInitialized {
		t.rebuildLocked(cutoff)
	} else if t.lastCleanedWindowEnd.Before(cutoff) {
		validSlices := t.slices[:0]
		for _, s := range t.slices {
			if s.windowStart.Before(cutoff) {
				t.totalRequests -= s.requests
				t.totalErrors -= s.errors
				continue
			}

			validSlices = append(validSlices, s)
		}

		t.slices = validSlices
		t.lastCleanedWindowEnd = cutoff
	}

	if callback != nil {
		for _, s := range t.slices {
			callback(s)
		}
	}
}

func (t *TimeWindowStats) AddRequest(now time.Time, isError bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cleanupLocked(now, nil)

	currentWindow := now.Truncate(timeWindow)

	var slice *timeSlice
	for i := range t.slices {
		if t.slices[i].windowStart.Equal(currentWindow) {
			slice = t.slices[i]
			break
		}
	}

	if slice == nil {
		slice = &timeSlice{windowStart: currentWindow}
		t.slices = append(t.slices, slice)
	}

	slice.requests++

	t.totalRequests++
	if isError {
		slice.errors++
		t.totalErrors++
	}
}

func (t *TimeWindowStats) GetStats() (totalReq, totalErr int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cleanupLocked(time.Now(), nil)

	return t.totalRequests, t.totalErrors
}

func (t *TimeWindowStats) HasValidSlices() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cleanupLocked(time.Now(), nil)

	return len(t.slices) > 0
}
