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

func bannedChannelsStringLocalCacheKey(model string) string {
	return "banned:" + model + ":string"
}

func channelModelErrorRateLocalCacheKey(model string, channelID int64) string {
	return "rate:" + model + ":channel:" + strconv.FormatInt(channelID, 10)
}

func channelModelErrorRateStringLocalCacheKey(model, channelKey string) string {
	return "rate:" + model + ":channel:" + channelKey
}

func cloneChannelErrorRates(values map[int64]float64) map[int64]float64 {
	if values == nil {
		return nil
	}

	cloned := make(map[int64]float64, len(values))
	maps.Copy(cloned, values)

	return cloned
}

func cloneChannelStringErrorRates(values map[string]float64) map[string]float64 {
	if values == nil {
		return nil
	}

	cloned := make(map[string]float64, len(values))
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

func cloneBannedChannelKeys(values map[string]struct{}) map[string]struct{} {
	if values == nil {
		return nil
	}

	cloned := make(map[string]struct{}, len(values))
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

func getModelChannelStringErrorRateLocal(model string) (map[string]float64, bool) {
	v, ok := modelChannelErrorRateLocalCache.Get(
		modelChannelErrorRateLocalCacheKey(model) + ":string",
	)
	if !ok {
		return nil, false
	}

	rates, ok := v.(map[string]float64)
	if !ok {
		panic("model channel string error rate local cache type mismatch")
	}

	return cloneChannelStringErrorRates(rates), true
}

func setModelChannelStringErrorRateLocalUnlocked(model string, values map[string]float64) {
	modelChannelErrorRateLocalCache.Set(
		modelChannelErrorRateLocalCacheKey(model)+":string",
		cloneChannelStringErrorRates(values),
		monitorLocalTTL,
	)

	for channelKey, rate := range values {
		setChannelModelStringErrorRateLocalUnlocked(model, channelKey, rate)
	}
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
		modelChannelErrorRateLocalCache.Delete(
			modelChannelErrorRateLocalCacheKey(model) + ":string",
		)
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

func getChannelModelStringErrorRateLocal(model, channelKey string) (float64, bool) {
	v, ok := channelModelErrorRateLocalCache.Get(
		channelModelErrorRateStringLocalCacheKey(model, channelKey),
	)
	if !ok {
		return 0, false
	}

	rate, ok := v.(float64)
	if !ok {
		panic("channel model string error rate local cache type mismatch")
	}

	return rate, true
}

func setChannelModelStringErrorRateLocalUnlocked(model, channelKey string, value float64) {
	channelModelErrorRateLocalCache.Set(
		channelModelErrorRateStringLocalCacheKey(model, channelKey),
		value,
		monitorLocalTTL,
	)
}

func setChannelModelErrorRateLocalUnlocked(model string, channelID int64, value float64) {
	channelModelErrorRateLocalCache.Set(
		channelModelErrorRateLocalCacheKey(model, channelID),
		value,
		monitorLocalTTL,
	)
}

func deleteChannelModelStringErrorRateLocal(model, channelKey string) {
	common.WithKeyLock(
		monitorLocalLoadLocker,
		channelModelErrorRateStringLocalCacheKey(model, channelKey),
		func() {
			channelModelErrorRateLocalCache.Delete(
				channelModelErrorRateStringLocalCacheKey(model, channelKey),
			)

			if channelID, err := strconv.ParseInt(channelKey, 10, 64); err == nil {
				channelModelErrorRateLocalCache.Delete(
					channelModelErrorRateLocalCacheKey(model, channelID),
				)
			}
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

func getBannedChannelKeysLocal(model string) (map[string]struct{}, bool) {
	v, ok := modelBannedChannelsLocalCache.Get(bannedChannelsStringLocalCacheKey(model))
	if !ok {
		return nil, false
	}

	banned, ok := v.(map[string]struct{})
	if !ok {
		panic("banned channel keys local cache type mismatch")
	}

	return cloneBannedChannelKeys(banned), true
}

func setBannedChannelKeysLocalUnlocked(model string, values map[string]struct{}) {
	modelBannedChannelsLocalCache.Set(
		bannedChannelsStringLocalCacheKey(model),
		cloneBannedChannelKeys(values),
		monitorLocalTTL,
	)
}

func deleteBannedChannelsLocal(model string) {
	common.WithKeyLock(monitorLocalLoadLocker, bannedChannelsLocalCacheKey(model), func() {
		modelBannedChannelsLocalCache.Delete(bannedChannelsLocalCacheKey(model))
		modelBannedChannelsLocalCache.Delete(bannedChannelsStringLocalCacheKey(model))
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
