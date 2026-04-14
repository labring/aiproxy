package monitor

import (
	"maps"
	"strconv"
	"time"

	"github.com/labring/aiproxy/core/common"
	gcache "github.com/patrickmn/go-cache"
)

const (
	monitorLocalTTL = time.Second
)

var (
	modelChannelErrorRateLocalCache = gcache.New(2*time.Second, 5*time.Second)
	channelModelErrorRateLocalCache = gcache.New(2*time.Second, 5*time.Second)
	modelBannedChannelsLocalCache   = gcache.New(2*time.Second, 5*time.Second)
	monitorLocalLoadLocker          = common.NewKeyedLocker()
)

func modelChannelErrorRateLocalCacheKey(model string) string {
	return "rate:" + model
}

func bannedChannelsLocalCacheKey(model string) string {
	return "banned:" + model
}

func channelModelErrorRateLocalCacheKey(model string, channelID int64) string {
	return "rate:" + model + ":channel:" + strconv.FormatInt(channelID, 10)
}

func cloneChannelErrorRates(values map[int64]float64) map[int64]float64 {
	if values == nil {
		return nil
	}

	cloned := make(map[int64]float64, len(values))
	maps.Copy(cloned, values)

	return cloned
}

func cloneBannedChannels(values map[int64]struct{}) map[int64]struct{} {
	if values == nil {
		return nil
	}

	cloned := make(map[int64]struct{}, len(values))
	for key := range values {
		cloned[key] = struct{}{}
	}

	return cloned
}

func getModelChannelErrorRateLocal(model string) (map[int64]float64, bool) {
	v, ok := modelChannelErrorRateLocalCache.Get(modelChannelErrorRateLocalCacheKey(model))
	if !ok {
		return nil, false
	}

	rates, ok := v.(map[int64]float64)
	if !ok {
		panic("model channel error rate local cache type mismatch")
	}

	return cloneChannelErrorRates(rates), true
}

func setModelChannelErrorRateLocalUnlocked(model string, values map[int64]float64) {
	modelChannelErrorRateLocalCache.Set(
		modelChannelErrorRateLocalCacheKey(model),
		cloneChannelErrorRates(values),
		monitorLocalTTL,
	)

	for channelID, rate := range values {
		setChannelModelErrorRateLocalUnlocked(model, channelID, rate)
	}
}

func deleteModelChannelErrorRateLocal(model string) {
	common.WithKeyLock(monitorLocalLoadLocker, modelChannelErrorRateLocalCacheKey(model), func() {
		modelChannelErrorRateLocalCache.Delete(modelChannelErrorRateLocalCacheKey(model))
	})
}

func getChannelModelErrorRateLocal(model string, channelID int64) (float64, bool) {
	v, ok := channelModelErrorRateLocalCache.Get(
		channelModelErrorRateLocalCacheKey(model, channelID),
	)
	if !ok {
		return 0, false
	}

	rate, ok := v.(float64)
	if !ok {
		panic("channel model error rate local cache type mismatch")
	}

	return rate, true
}

func setChannelModelErrorRateLocalUnlocked(model string, channelID int64, value float64) {
	channelModelErrorRateLocalCache.Set(
		channelModelErrorRateLocalCacheKey(model, channelID),
		value,
		monitorLocalTTL,
	)
}

func deleteChannelModelErrorRateLocal(model string, channelID int64) {
	common.WithKeyLock(
		monitorLocalLoadLocker,
		channelModelErrorRateLocalCacheKey(model, channelID),
		func() {
			channelModelErrorRateLocalCache.Delete(
				channelModelErrorRateLocalCacheKey(model, channelID),
			)
		},
	)
}

func getBannedChannelsLocal(model string) (map[int64]struct{}, bool) {
	v, ok := modelBannedChannelsLocalCache.Get(bannedChannelsLocalCacheKey(model))
	if !ok {
		return nil, false
	}

	banned, ok := v.(map[int64]struct{})
	if !ok {
		panic("banned channels local cache type mismatch")
	}

	return cloneBannedChannels(banned), true
}

func setBannedChannelsLocalUnlocked(model string, values map[int64]struct{}) {
	modelBannedChannelsLocalCache.Set(
		bannedChannelsLocalCacheKey(model),
		cloneBannedChannels(values),
		monitorLocalTTL,
	)
}

func deleteBannedChannelsLocal(model string) {
	common.WithKeyLock(monitorLocalLoadLocker, bannedChannelsLocalCacheKey(model), func() {
		modelBannedChannelsLocalCache.Delete(bannedChannelsLocalCacheKey(model))
	})
}

func flushMonitorLocalCache() {
	modelChannelErrorRateLocalCache.Flush()
	channelModelErrorRateLocalCache.Flush()
	modelBannedChannelsLocalCache.Flush()
}

func loadWithLocalKeyLock[T any](
	locker *common.KeyedLocker,
	key string,
	getLocal func() (T, bool),
	load func() (T, error),
) (T, error) {
	return common.LoadWithKeyLock(locker, key, getLocal, load)
}
