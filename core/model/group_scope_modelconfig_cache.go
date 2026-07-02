package model

import (
	"context"
	"errors"
	"maps"
	"math/rand/v2"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const GroupScopeModelConfigsCacheKey = "group_scope_model_configs:%s"

type GroupScopeModelConfigsCache struct {
	GroupID string                  `json:"group_id" redis:"g"`
	Models  []string                `json:"models"   redis:"m"`
	Configs map[string]ModelConfig  `json:"configs"  redis:"c"`
	List    []GroupScopeModelConfig `json:"list"     redis:"l"`
}

type (
	redisGroupScopeModelConfigMap  map[string]ModelConfig
	redisGroupScopeModelConfigList []GroupScopeModelConfig
)

func (r *redisGroupScopeModelConfigMap) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, r)
}

func (r redisGroupScopeModelConfigMap) MarshalBinary() ([]byte, error) {
	return sonic.Marshal(r)
}

func (r *redisGroupScopeModelConfigList) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, r)
}

func (r redisGroupScopeModelConfigList) MarshalBinary() ([]byte, error) {
	return sonic.Marshal(r)
}

func getGroupScopeModelConfigsCacheKey(group string) string {
	return common.RedisKeyf(GroupScopeModelConfigsCacheKey, group)
}

func cloneModelConfig(config ModelConfig) ModelConfig {
	cloned := config
	cloned.Config = maps.Clone(config.Config)
	cloned.Plugin = clonePluginConfig(config.Plugin)
	cloned.Price = clonePrice(config.Price)
	cloned.AllowedResolutions = cloneStringSlice(config.AllowedResolutions)

	return cloned
}

func clonePluginConfig(values map[string]map[string]any) map[string]map[string]any {
	if values == nil {
		return nil
	}

	cloned := make(map[string]map[string]any, len(values))
	for key, value := range values {
		cloned[key] = maps.Clone(value)
	}

	return cloned
}

func cloneGroupScopeModelConfig(config GroupScopeModelConfig) GroupScopeModelConfig {
	cloned := config
	cloned.ModelConfig = cloneModelConfig(config.ModelConfig)
	return cloned
}

func cloneGroupScopeModelConfigs(configs []GroupScopeModelConfig) []GroupScopeModelConfig {
	if configs == nil {
		return nil
	}

	cloned := make([]GroupScopeModelConfig, len(configs))
	for i, config := range configs {
		cloned[i] = cloneGroupScopeModelConfig(config)
	}

	return cloned
}

func cloneModelConfigMap(values map[string]ModelConfig) map[string]ModelConfig {
	if values == nil {
		return nil
	}

	cloned := make(map[string]ModelConfig, len(values))
	for key, value := range values {
		cloned[key] = cloneModelConfig(value)
	}

	return cloned
}

func cloneGroupScopeModelConfigsCache(
	cache *GroupScopeModelConfigsCache,
) *GroupScopeModelConfigsCache {
	if cache == nil {
		return nil
	}

	return &GroupScopeModelConfigsCache{
		GroupID: cache.GroupID,
		Models:  cloneStringSlice(cache.Models),
		Configs: cloneModelConfigMap(cache.Configs),
		List:    cloneGroupScopeModelConfigs(cache.List),
	}
}

func CacheDeleteGroupScopeModelConfig(group string) error {
	cacheDeleteModelLocal(getGroupScopeModelConfigsCacheKey(group))

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, getGroupScopeModelConfigsCacheKey(group)).Err()
}

func CacheSetGroupScopeModelConfigs(cache *GroupScopeModelConfigsCache) error {
	key := getGroupScopeModelConfigsCacheKey(cache.GroupID)
	cacheSetModelLocal(key, cache, cloneGroupScopeModelConfigsCache)
	return cacheSetGroupScopeModelConfigsRedis(cache)
}

func CacheGetGroupScopeModelConfigs(group string) (*GroupScopeModelConfigsCache, error) {
	if group == "" {
		return nil, errors.New("group id is required")
	}

	cacheKey := getGroupScopeModelConfigsCacheKey(group)
	if cached, _, ok := cacheGetModelLocal(cacheKey, cloneGroupScopeModelConfigsCache); ok {
		return cached, nil
	}

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		cache := &GroupScopeModelConfigsCache{}

		err := scanGroupScopeModelConfigsCache(ctx, cacheKey, cache)
		if err == nil && cache.GroupID != "" {
			cacheSetModelLocal(cacheKey, cache, cloneGroupScopeModelConfigsCache)
			return cache, nil
		} else if err != nil && !errors.Is(err, redis.Nil) {
			log.Errorf(
				"get group scope model configs (%s) from redis error: %s",
				group,
				err.Error(),
			)
		}
	}

	cache, _, loaded, err := loadWithLocalKeyLock(
		modelCacheLoadLocker,
		cacheKey,
		func() (*GroupScopeModelConfigsCache, bool, bool) {
			return cacheGetModelLocal(cacheKey, cloneGroupScopeModelConfigsCache)
		},
		func() (*GroupScopeModelConfigsCache, error) {
			configs, err := GetAllGroupScopeModelConfigs(group)
			if err != nil {
				return nil, err
			}

			cache := buildGroupScopeModelConfigsCache(group, configs)
			cacheSetModelLocalUnlocked(cacheKey, cache, cloneGroupScopeModelConfigsCache)

			return cache, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if loaded {
		if err := cacheSetGroupScopeModelConfigsRedis(cache); err != nil {
			log.Error("redis set group scope model configs error: " + err.Error())
		}
	}

	return cache, nil
}

func CacheGetGroupScopeModelConfig(group, modelName string) (ModelConfig, bool) {
	cache, err := CacheGetGroupScopeModelConfigs(group)
	if err != nil {
		return ModelConfig{}, false
	}

	config, ok := cache.Configs[modelName]

	return config, ok
}

func ResolveGroupScopeModelConfig(group, modelName string) (ModelConfig, bool) {
	if config.DisableModelConfig {
		return NewDefaultModelConfig(modelName), true
	}

	modelConfig, ok := CacheGetGroupScopeModelConfig(group, modelName)
	if ok {
		return modelConfig, true
	}

	return ModelConfig{}, false
}

func buildGroupScopeModelConfigsCache(
	group string,
	configs []GroupScopeModelConfig,
) *GroupScopeModelConfigsCache {
	configMap := make(map[string]ModelConfig, len(configs))
	for _, config := range configs {
		configMap[config.Model] = config.ToModelConfig()
	}

	return &GroupScopeModelConfigsCache{
		GroupID: group,
		Models:  groupScopeModelConfigModels(configs),
		Configs: configMap,
		List:    cloneGroupScopeModelConfigs(configs),
	}
}

func cacheSetGroupScopeModelConfigsRedis(cache *GroupScopeModelConfigsCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := getGroupScopeModelConfigsCacheKey(cache.GroupID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, "g", cache.GroupID)
	pipe.HSet(ctx, key, "m", redisStringSlice(cache.Models))

	configs, err := sonic.Marshal(redisGroupScopeModelConfigMap(cache.Configs))
	if err != nil {
		cancel()
		return err
	}

	pipe.HSet(ctx, key, "c", configs)

	list, err := sonic.Marshal(redisGroupScopeModelConfigList(cache.List))
	if err != nil {
		cancel()
		return err
	}

	pipe.HSet(ctx, key, "l", list)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err = pipe.Exec(ctx)

	return err
}

func scanGroupScopeModelConfigsCache(
	ctx context.Context,
	key string,
	cache *GroupScopeModelConfigsCache,
) error {
	values, err := common.RDB.HGetAll(ctx, key).Result()
	if err != nil {
		return err
	}

	if len(values) == 0 {
		return redis.Nil
	}

	cache.GroupID = values["g"]
	if raw := values["m"]; raw != "" {
		if err := (*redisStringSlice)(&cache.Models).ScanRedis(raw); err != nil {
			return err
		}
	}

	if raw := values["c"]; raw != "" {
		var configs redisGroupScopeModelConfigMap
		if err := configs.ScanRedis(raw); err != nil {
			return err
		}

		cache.Configs = map[string]ModelConfig(configs)
	}

	if raw := values["l"]; raw != "" {
		var list redisGroupScopeModelConfigList
		if err := list.ScanRedis(raw); err != nil {
			return err
		}

		cache.List = []GroupScopeModelConfig(list)
	}

	return nil
}
