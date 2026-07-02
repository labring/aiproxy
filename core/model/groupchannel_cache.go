package model

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const GroupChannelsCacheKey = "group_channels:%s"

func getGroupChannelsCacheKey(group string) string {
	return common.RedisKeyf(GroupChannelsCacheKey, group)
}

type GroupChannelsCache struct {
	GroupID  string          `json:"group_id" redis:"g"`
	Channels []*GroupChannel `json:"channels" redis:"c"`
}

type redisGroupChannels []*GroupChannel

func (r *redisGroupChannels) ScanRedis(value string) error {
	return sonic.UnmarshalString(value, r)
}

func (r redisGroupChannels) MarshalBinary() ([]byte, error) {
	return sonic.Marshal(r)
}

func cloneGroupChannel(channel *GroupChannel) *GroupChannel {
	if channel == nil {
		return nil
	}

	cloned := *channel
	cloned.ModelMapping = cloneStringStringMap(channel.ModelMapping)
	cloned.Models = cloneStringSlice(channel.Models)
	cloned.Configs = cloneChannelConfigs(channel.Configs)
	cloned.Sets = cloneStringSlice(channel.Sets)

	return &cloned
}

func cloneGroupChannels(channels []*GroupChannel) []*GroupChannel {
	if channels == nil {
		return nil
	}

	cloned := make([]*GroupChannel, len(channels))
	for i, channel := range channels {
		cloned[i] = cloneGroupChannel(channel)
	}

	return cloned
}

func cloneGroupChannelsCache(cache *GroupChannelsCache) *GroupChannelsCache {
	if cache == nil {
		return nil
	}

	return &GroupChannelsCache{
		GroupID:  cache.GroupID,
		Channels: cloneGroupChannels(cache.Channels),
	}
}

func CacheDeleteGroupChannels(group string) error {
	cacheDeleteModelLocal(getGroupChannelsCacheKey(group))

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, getGroupChannelsCacheKey(group)).Err()
}

func CacheSetGroupChannels(cache *GroupChannelsCache) error {
	key := getGroupChannelsCacheKey(cache.GroupID)
	cacheSetModelLocal(key, cache, cloneGroupChannelsCache)
	return cacheSetGroupChannelsRedis(cache)
}

func CacheGetGroupChannels(group string) (*GroupChannelsCache, error) {
	if group == "" {
		return nil, errors.New("group id is required")
	}

	cacheKey := getGroupChannelsCacheKey(group)
	if cached, _, ok := cacheGetModelLocal(cacheKey, cloneGroupChannelsCache); ok {
		return cached, nil
	}

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		cache := &GroupChannelsCache{}

		err := scanGroupChannelsCache(ctx, cacheKey, cache)
		if err == nil && cache.GroupID != "" {
			cacheSetModelLocal(cacheKey, cache, cloneGroupChannelsCache)
			return cache, nil
		} else if err != nil && !errors.Is(err, redis.Nil) {
			log.Errorf("get group channels (%s) from redis error: %s", group, err.Error())
		}
	}

	cache, _, loaded, err := loadWithLocalKeyLock(
		modelCacheLoadLocker,
		cacheKey,
		func() (*GroupChannelsCache, bool, bool) {
			return cacheGetModelLocal(cacheKey, cloneGroupChannelsCache)
		},
		func() (*GroupChannelsCache, error) {
			channels, err := LoadEnabledGroupChannels(group)
			if err != nil {
				return nil, err
			}

			gc := &GroupChannelsCache{GroupID: group, Channels: channels}
			cacheSetModelLocalUnlocked(cacheKey, gc, cloneGroupChannelsCache)

			return gc, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if loaded {
		if err := cacheSetGroupChannelsRedis(cache); err != nil {
			log.Error("redis set group channels error: " + err.Error())
		}
	}

	return cache, nil
}

func cacheSetGroupChannelsRedis(cache *GroupChannelsCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := getGroupChannelsCacheKey(cache.GroupID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, "g", cache.GroupID)

	channels, err := sonic.Marshal(redisGroupChannels(cache.Channels))
	if err != nil {
		cancel()
		return err
	}

	pipe.HSet(ctx, key, "c", channels)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err = pipe.Exec(ctx)

	return err
}

func scanGroupChannelsCache(ctx context.Context, key string, cache *GroupChannelsCache) error {
	values, err := common.RDB.HGetAll(ctx, key).Result()
	if err != nil {
		return err
	}

	if len(values) == 0 {
		return redis.Nil
	}

	cache.GroupID = values["g"]
	if raw := values["c"]; raw != "" {
		var channels redisGroupChannels
		if err := channels.ScanRedis(raw); err != nil {
			return err
		}

		cache.Channels = []*GroupChannel(channels)
	}

	return nil
}
