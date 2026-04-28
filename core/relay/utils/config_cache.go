package utils

import (
	"strconv"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/meta"
	gcache "github.com/patrickmn/go-cache"
)

const (
	localConfigCacheTTL     = 3 * time.Second
	localConfigCacheCleanup = 10 * time.Second
)

type ChannelConfigCache[T any] struct {
	once   sync.Once
	cache  *gcache.Cache
	locker *common.KeyedLocker
}

func (c *ChannelConfigCache[T]) Load(meta *meta.Meta, defaults T) (T, error) {
	if meta == nil || meta.Channel.ID <= 0 {
		return loadChannelConfig(meta, defaults)
	}

	c.init()

	key := strconv.Itoa(meta.Channel.ID)

	return common.LoadWithKeyLock(
		c.locker,
		key,
		func() (T, bool) {
			return c.get(key)
		},
		func() (T, error) {
			cfg, err := loadChannelConfig(meta, defaults)
			if err != nil {
				var zero T
				return zero, err
			}

			c.cache.Set(key, cfg, localConfigCacheTTL)

			return cfg, nil
		},
	)
}

func (c *ChannelConfigCache[T]) init() {
	c.once.Do(func() {
		c.cache = gcache.New(localConfigCacheTTL, localConfigCacheCleanup)
		c.locker = common.NewKeyedLocker()
	})
}

func (c *ChannelConfigCache[T]) get(key string) (T, bool) {
	var zero T

	value, ok := c.cache.Get(key)
	if !ok {
		return zero, false
	}

	cfg, ok := value.(T)
	if !ok {
		panic("channel config local cache type mismatch")
	}

	return cfg, true
}

func loadChannelConfig[T any](meta *meta.Meta, defaults T) (T, error) {
	cfg := defaults
	if meta == nil {
		return cfg, nil
	}

	if err := meta.ChannelConfigs.LoadConfig(&cfg); err != nil {
		var zero T
		return zero, err
	}

	return cfg, nil
}

type PluginConfigCache[T any] struct {
	once   sync.Once
	cache  *gcache.Cache
	locker *common.KeyedLocker
}

func (c *PluginConfigCache[T]) Load(meta *meta.Meta, pluginName string, defaults T) (T, error) {
	key := pluginConfigCacheKey(meta)
	if key == "" {
		return loadPluginConfig(meta, pluginName, defaults)
	}

	c.init()

	return common.LoadWithKeyLock(
		c.locker,
		key,
		func() (T, bool) {
			return c.get(key)
		},
		func() (T, error) {
			cfg, err := loadPluginConfig(meta, pluginName, defaults)
			if err != nil {
				var zero T
				return zero, err
			}

			c.cache.Set(key, cfg, localConfigCacheTTL)

			return cfg, nil
		},
	)
}

func (c *PluginConfigCache[T]) init() {
	c.once.Do(func() {
		c.cache = gcache.New(localConfigCacheTTL, localConfigCacheCleanup)
		c.locker = common.NewKeyedLocker()
	})
}

func (c *PluginConfigCache[T]) get(key string) (T, bool) {
	var zero T

	value, ok := c.cache.Get(key)
	if !ok {
		return zero, false
	}

	cfg, ok := value.(T)
	if !ok {
		panic("plugin config local cache type mismatch")
	}

	return cfg, true
}

func loadPluginConfig[T any](meta *meta.Meta, pluginName string, defaults T) (T, error) {
	cfg := defaults
	if meta == nil {
		return cfg, nil
	}

	if err := meta.ModelConfig.LoadPluginConfig(pluginName, &cfg); err != nil {
		var zero T
		return zero, err
	}

	return cfg, nil
}

func pluginConfigCacheKey(meta *meta.Meta) string {
	if meta == nil {
		return ""
	}

	return meta.ModelConfig.Model
}
