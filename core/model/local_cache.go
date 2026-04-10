package model

import (
	"maps"
	"slices"
	"time"

	"github.com/labring/aiproxy/core/common"
	gcache "github.com/patrickmn/go-cache"
)

const (
	modelLocalTTL                 = time.Second
	modelLocalMissTTL             = 500 * time.Millisecond
	modelLocalCacheCleanup        = 5 * time.Second
	modelLocalCacheDefaultExpires = 2 * time.Second
)

var (
	modelLocalCache      = gcache.New(modelLocalCacheDefaultExpires, modelLocalCacheCleanup)
	modelCacheLoadLocker = common.NewKeyedLocker()
)

type modelLocalCacheItem struct {
	Value    any
	NotFound bool
}

func cacheSetModelLocal[T any](key string, value T, clone func(T) T) {
	common.WithKeyLock(modelCacheLoadLocker, key, func() {
		cacheSetModelLocalUnlocked(key, value, clone)
	})
}

func cacheSetModelLocalUnlocked[T any](key string, value T, clone func(T) T) {
	modelLocalCache.Set(key, modelLocalCacheItem{Value: clone(value)}, modelLocalTTL)
}

func cacheSetModelNotFoundLocalUnlocked(key string) {
	modelLocalCache.Set(key, modelLocalCacheItem{NotFound: true}, modelLocalMissTTL)
}

func cacheDeleteModelLocal(key string) {
	common.WithKeyLock(modelCacheLoadLocker, key, func() {
		cacheDeleteModelLocalUnlocked(key)
	})
}

func cacheDeleteModelLocalUnlocked(key string) {
	modelLocalCache.Delete(key)
}

func cacheGetModelLocal[T any](key string, clone func(T) T) (T, bool, bool) {
	var zero T

	v, ok := modelLocalCache.Get(key)
	if !ok {
		return zero, false, false
	}

	item, ok := v.(modelLocalCacheItem)
	if !ok {
		panic("model local cache type mismatch")
	}

	if item.NotFound {
		return zero, true, true
	}

	value, ok := item.Value.(T)
	if !ok {
		panic("model local cache value type mismatch")
	}

	return clone(value), false, true
}

func cacheUpdateModelLocal[T any](key string, clone func(T) T, update func(T) bool) bool {
	var updated bool
	common.WithKeyLock(modelCacheLoadLocker, key, func() {
		v, ok := modelLocalCache.Get(key)
		if !ok {
			return
		}

		item, ok := v.(modelLocalCacheItem)
		if !ok {
			panic("model local cache type mismatch")
		}

		if item.NotFound {
			cacheDeleteModelLocalUnlocked(key)
			return
		}

		value, ok := item.Value.(T)
		if !ok {
			panic("model local cache value type mismatch")
		}

		value = clone(value)
		if !update(value) {
			return
		}

		modelLocalCache.Set(key, modelLocalCacheItem{Value: value}, modelLocalTTL)

		updated = true
	})

	return updated
}

func loadWithLocalKeyLock[T any](
	locker *common.KeyedLocker,
	key string,
	getLocal func() (T, bool, bool),
	load func() (T, error),
) (T, bool, bool, error) {
	unlock := locker.Lock(key)
	defer unlock()

	if value, notFound, ok := getLocal(); ok {
		return value, notFound, false, nil
	}

	value, err := load()
	if err != nil {
		var zero T
		return zero, false, true, err
	}

	return value, false, true, nil
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	return slices.Clone(values)
}

func cloneStringSliceMap(values map[string][]string) map[string][]string {
	if values == nil {
		return nil
	}

	cloned := make(map[string][]string, len(values))
	for key, items := range values {
		cloned[key] = cloneStringSlice(items)
	}

	return cloned
}

func cloneStringStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	return maps.Clone(values)
}

func cloneStringFloat64Map(values map[string]float64) map[string]float64 {
	if values == nil {
		return nil
	}

	return maps.Clone(values)
}

func clonePrice(price Price) Price {
	cloned := price
	if len(price.ConditionalPrices) == 0 {
		return cloned
	}

	cloned.ConditionalPrices = make([]ConditionalPrice, len(price.ConditionalPrices))
	for i, conditionalPrice := range price.ConditionalPrices {
		cloned.ConditionalPrices[i] = conditionalPrice
		cloned.ConditionalPrices[i].Price = clonePrice(conditionalPrice.Price)
	}

	return cloned
}

func cloneGroupModelConfig(config GroupModelConfig) GroupModelConfig {
	cloned := config
	cloned.ImagePrices = cloneStringFloat64Map(config.ImagePrices)
	cloned.Price = clonePrice(config.Price)

	return cloned
}

func cloneGroupCache(group *GroupCache) *GroupCache {
	if group == nil {
		return nil
	}

	cloned := *group

	cloned.AvailableSets = redisStringSlice(cloneStringSlice([]string(group.AvailableSets)))
	if group.ModelConfigs != nil {
		cloned.ModelConfigs = make(redisGroupModelConfigMap, len(group.ModelConfigs))
		for key, config := range group.ModelConfigs {
			cloned.ModelConfigs[key] = cloneGroupModelConfig(config)
		}
	}

	return &cloned
}

func cloneTokenCache(token *TokenCache) *TokenCache {
	if token == nil {
		return nil
	}

	cloned := *token
	cloned.Subnets = redisStringSlice(cloneStringSlice([]string(token.Subnets)))
	cloned.Models = redisStringSlice(cloneStringSlice([]string(token.Models)))
	cloned.availableSets = cloneStringSlice(token.availableSets)
	cloned.modelsBySet = cloneStringSliceMap(token.modelsBySet)

	return &cloned
}

func cloneGroupMCPProxyConfig(config *GroupMCPProxyConfig) *GroupMCPProxyConfig {
	if config == nil {
		return nil
	}

	cloned := *config
	cloned.Querys = cloneStringStringMap(config.Querys)
	cloned.Headers = cloneStringStringMap(config.Headers)

	return &cloned
}

func cloneOpenAPIConfig(config *MCPOpenAPIConfig) *MCPOpenAPIConfig {
	if config == nil {
		return nil
	}

	cloned := *config

	return &cloned
}

func cloneGroupMCPCache(groupMCP *GroupMCPCache) *GroupMCPCache {
	if groupMCP == nil {
		return nil
	}

	cloned := *groupMCP
	cloned.ProxyConfig = cloneGroupMCPProxyConfig(groupMCP.ProxyConfig)
	cloned.OpenAPIConfig = cloneOpenAPIConfig(groupMCP.OpenAPIConfig)

	return &cloned
}

func cloneMCPPrice(price MCPPrice) MCPPrice {
	cloned := price
	cloned.ToolsCallPrices = cloneStringFloat64Map(price.ToolsCallPrices)

	return cloned
}

func clonePublicMCPProxyConfig(config *PublicMCPProxyConfig) *PublicMCPProxyConfig {
	if config == nil {
		return nil
	}

	cloned := *config
	cloned.Querys = cloneStringStringMap(config.Querys)

	cloned.Headers = cloneStringStringMap(config.Headers)
	if config.Reusing != nil {
		cloned.Reusing = maps.Clone(config.Reusing)
	}

	return &cloned
}

func cloneEmbeddingConfig(config *MCPEmbeddingConfig) *MCPEmbeddingConfig {
	if config == nil {
		return nil
	}

	cloned := *config

	cloned.Init = cloneStringStringMap(config.Init)
	if config.Reusing != nil {
		cloned.Reusing = maps.Clone(config.Reusing)
	}

	return &cloned
}

func clonePublicMCPCache(publicMCP *PublicMCPCache) *PublicMCPCache {
	if publicMCP == nil {
		return nil
	}

	cloned := *publicMCP
	cloned.Price = cloneMCPPrice(publicMCP.Price)
	cloned.ProxyConfig = clonePublicMCPProxyConfig(publicMCP.ProxyConfig)
	cloned.OpenAPIConfig = cloneOpenAPIConfig(publicMCP.OpenAPIConfig)
	cloned.EmbedConfig = cloneEmbeddingConfig(publicMCP.EmbedConfig)

	return &cloned
}

func clonePublicMCPReusingParamCache(
	param PublicMCPReusingParamCache,
) PublicMCPReusingParamCache {
	cloned := param
	if param.Params != nil {
		cloned.Params = redisMap[string, string](
			cloneStringStringMap(map[string]string(param.Params)),
		)
	}

	return cloned
}
