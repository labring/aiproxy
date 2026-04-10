package monitor

import (
	"maps"
	"time"

	"github.com/labring/aiproxy/core/common"
	gcache "github.com/patrickmn/go-cache"
)

const (
	monitorLocalTTL = time.Second
)

var (
	modelChannelErrorRateLocalCache = gcache.New(2*time.Second, 5*time.Second)
	modelBannedChannelsLocalCache   = gcache.New(2*time.Second, 5*time.Second)
	monitorLocalLoadLocker          = common.NewKeyedLocker()
)

func modelChannelErrorRateLocalCacheKey(model string) string {
	return "rate:" + model
}

func bannedChannelsLocalCacheKey(model string) string {
	return "banned:" + model
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
}

func deleteModelChannelErrorRateLocal(model string) {
	common.WithKeyLock(monitorLocalLoadLocker, modelChannelErrorRateLocalCacheKey(model), func() {
		modelChannelErrorRateLocalCache.Delete(modelChannelErrorRateLocalCacheKey(model))
	})
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

func deleteMonitorModelLocal(model string) {
	deleteModelChannelErrorRateLocal(model)
	deleteBannedChannelsLocal(model)
}

func flushMonitorLocalCache() {
	modelChannelErrorRateLocalCache.Flush()
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
