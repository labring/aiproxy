package model

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type GroupMCPCache struct {
	ID            string               `json:"id"             redis:"i"`
	GroupID       string               `json:"group_id"       redis:"g"`
	Status        GroupMCPStatus       `json:"status"         redis:"s"`
	Type          GroupMCPType         `json:"type"           redis:"t"`
	ProxyConfig   *GroupMCPProxyConfig `json:"proxy_config"   redis:"pc"`
	OpenAPIConfig *MCPOpenAPIConfig    `json:"openapi_config" redis:"oc"`
}

func (g *GroupMCP) ToGroupMCPCache() *GroupMCPCache {
	return &GroupMCPCache{
		ID:            g.ID,
		GroupID:       g.GroupID,
		Status:        g.Status,
		Type:          g.Type,
		ProxyConfig:   g.ProxyConfig,
		OpenAPIConfig: g.OpenAPIConfig,
	}
}

const (
	GroupMCPCacheKey = "group_mcp:%s:%s"
)

func getGroupMCPCacheKey(groupID, mcpID string) string {
	return common.RedisKeyf(GroupMCPCacheKey, groupID, mcpID)
}

func updateGroupMCPLocalCache(groupID, mcpID string, update func(*GroupMCPCache) bool) {
	cacheUpdateModelLocal(getGroupMCPCacheKey(groupID, mcpID), cloneGroupMCPCache, update)
}

func CacheDeleteGroupMCP(groupID, mcpID string) error {
	cacheDeleteModelLocal(getGroupMCPCacheKey(groupID, mcpID))

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, getGroupMCPCacheKey(groupID, mcpID)).Err()
}

func CacheSetGroupMCP(groupMCP *GroupMCPCache) error {
	key := getGroupMCPCacheKey(groupMCP.GroupID, groupMCP.ID)
	cacheSetModelLocal(key, groupMCP, cloneGroupMCPCache)
	return cacheSetGroupMCPRedis(groupMCP)
}

func CacheGetGroupMCP(groupID, mcpID string) (*GroupMCPCache, error) {
	cacheKey := getGroupMCPCacheKey(groupID, mcpID)
	if groupMCPCache, notFound, ok := cacheGetModelLocal(cacheKey, cloneGroupMCPCache); ok {
		if notFound {
			return nil, NotFoundError(ErrGroupMCPNotFound)
		}

		return groupMCPCache, nil
	}

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		groupMCPCache := &GroupMCPCache{}

		err := common.RDB.HGetAll(ctx, cacheKey).Scan(groupMCPCache)
		if err == nil && groupMCPCache.ID != "" {
			cacheSetModelLocal(cacheKey, groupMCPCache, cloneGroupMCPCache)
			return groupMCPCache, nil
		} else if err != nil && !errors.Is(err, redis.Nil) {
			log.Errorf("get group mcp (%s:%s) from redis error: %s", groupID, mcpID, err.Error())
		}
	}

	groupMCPCache, notFound, loaded, err := loadWithLocalKeyLock(
		modelCacheLoadLocker,
		cacheKey,
		func() (*GroupMCPCache, bool, bool) {
			return cacheGetModelLocal(cacheKey, cloneGroupMCPCache)
		},
		func() (*GroupMCPCache, error) {
			groupMCP, err := GetGroupMCPByID(mcpID, groupID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					cacheSetModelNotFoundLocalUnlocked(cacheKey)
				}

				return nil, err
			}

			gmc := groupMCP.ToGroupMCPCache()
			cacheSetModelLocalUnlocked(cacheKey, gmc, cloneGroupMCPCache)

			return gmc, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if notFound {
		return nil, NotFoundError(ErrGroupMCPNotFound)
	}

	if loaded {
		if err := cacheSetGroupMCPRedis(groupMCPCache); err != nil {
			log.Error("redis set group mcp error: " + err.Error())
		}
	}

	return groupMCPCache, nil
}

func cacheSetGroupMCPRedis(groupMCP *GroupMCPCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := getGroupMCPCacheKey(groupMCP.GroupID, groupMCP.ID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, groupMCP)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

var updateGroupMCPStatusScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "s") then
		redis.call("HSet", KEYS[1], "s", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdateGroupMCPStatus(groupID, mcpID string, status GroupMCPStatus) error {
	updateGroupMCPLocalCache(groupID, mcpID, func(groupMCP *GroupMCPCache) bool {
		groupMCP.Status = status
		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateGroupMCPStatusScript.Run(
		ctx,
		common.RDB,
		[]string{getGroupMCPCacheKey(groupID, mcpID)},
		status,
	).Err()
}

type PublicMCPCache struct {
	ID            string                `json:"id"             redis:"i"`
	Status        PublicMCPStatus       `json:"status"         redis:"s"`
	Type          PublicMCPType         `json:"type"           redis:"t"`
	Price         MCPPrice              `json:"price"          redis:"p"`
	ProxyConfig   *PublicMCPProxyConfig `json:"proxy_config"   redis:"pc"`
	OpenAPIConfig *MCPOpenAPIConfig     `json:"openapi_config" redis:"oc"`
	EmbedConfig   *MCPEmbeddingConfig   `json:"embed_config"   redis:"ec"`
}

func (p *PublicMCP) ToPublicMCPCache() *PublicMCPCache {
	return &PublicMCPCache{
		ID:            p.ID,
		Status:        p.Status,
		Type:          p.Type,
		Price:         p.Price,
		ProxyConfig:   p.ProxyConfig,
		OpenAPIConfig: p.OpenAPIConfig,
		EmbedConfig:   p.EmbedConfig,
	}
}

const (
	PublicMCPCacheKey = "public_mcp:%s"
)

func getPublicMCPCacheKey(mcpID string) string {
	return common.RedisKeyf(PublicMCPCacheKey, mcpID)
}

func updatePublicMCPLocalCache(mcpID string, update func(*PublicMCPCache) bool) {
	cacheUpdateModelLocal(getPublicMCPCacheKey(mcpID), clonePublicMCPCache, update)
}

func CacheDeletePublicMCP(mcpID string) error {
	cacheDeleteModelLocal(getPublicMCPCacheKey(mcpID))

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, getPublicMCPCacheKey(mcpID)).Err()
}

func CacheSetPublicMCP(publicMCP *PublicMCPCache) error {
	key := getPublicMCPCacheKey(publicMCP.ID)
	cacheSetModelLocal(key, publicMCP, clonePublicMCPCache)
	return cacheSetPublicMCPRedis(publicMCP)
}

func CacheGetPublicMCP(mcpID string) (*PublicMCPCache, error) {
	cacheKey := getPublicMCPCacheKey(mcpID)
	if publicMCPCache, notFound, ok := cacheGetModelLocal(cacheKey, clonePublicMCPCache); ok {
		if notFound {
			return nil, NotFoundError(ErrPublicMCPNotFound)
		}

		return publicMCPCache, nil
	}

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		publicMCPCache := &PublicMCPCache{}

		err := common.RDB.HGetAll(ctx, cacheKey).Scan(publicMCPCache)
		if err == nil && publicMCPCache.ID != "" {
			cacheSetModelLocal(cacheKey, publicMCPCache, clonePublicMCPCache)
			return publicMCPCache, nil
		} else if err != nil && !errors.Is(err, redis.Nil) {
			log.Errorf("get public mcp (%s) from redis error: %s", mcpID, err.Error())
		}
	}

	publicMCPCache, notFound, loaded, err := loadWithLocalKeyLock(
		modelCacheLoadLocker,
		cacheKey,
		func() (*PublicMCPCache, bool, bool) {
			return cacheGetModelLocal(cacheKey, clonePublicMCPCache)
		},
		func() (*PublicMCPCache, error) {
			publicMCP, err := GetPublicMCPByID(mcpID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					cacheSetModelNotFoundLocalUnlocked(cacheKey)
				}

				return nil, err
			}

			pmc := publicMCP.ToPublicMCPCache()
			cacheSetModelLocalUnlocked(cacheKey, pmc, clonePublicMCPCache)

			return pmc, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if notFound {
		return nil, NotFoundError(ErrPublicMCPNotFound)
	}

	if loaded {
		if err := cacheSetPublicMCPRedis(publicMCPCache); err != nil {
			log.Error("redis set public mcp error: " + err.Error())
		}
	}

	return publicMCPCache, nil
}

func cacheSetPublicMCPRedis(publicMCP *PublicMCPCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := getPublicMCPCacheKey(publicMCP.ID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, publicMCP)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

var updatePublicMCPStatusScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "s") then
		redis.call("HSet", KEYS[1], "s", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdatePublicMCPStatus(mcpID string, status PublicMCPStatus) error {
	updatePublicMCPLocalCache(mcpID, func(publicMCP *PublicMCPCache) bool {
		publicMCP.Status = status
		return true
	})

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updatePublicMCPStatusScript.Run(
		ctx,
		common.RDB,
		[]string{getPublicMCPCacheKey(mcpID)},
		status,
	).Err()
}

const (
	PublicMCPReusingParamCacheKey = "public_mcp_param:%s:%s"
)

func getPublicMCPReusingParamCacheKey(mcpID, groupID string) string {
	return common.RedisKeyf(PublicMCPReusingParamCacheKey, mcpID, groupID)
}

type PublicMCPReusingParamCache struct {
	MCPID   string                   `json:"mcp_id"   redis:"m"`
	GroupID string                   `json:"group_id" redis:"g"`
	Params  redisMap[string, string] `json:"params"   redis:"p"`
}

func (p *PublicMCPReusingParam) ToPublicMCPReusingParamCache() PublicMCPReusingParamCache {
	return PublicMCPReusingParamCache{
		MCPID:   p.MCPID,
		GroupID: p.GroupID,
		Params:  p.Params,
	}
}

func CacheDeletePublicMCPReusingParam(mcpID, groupID string) error {
	cacheDeleteModelLocal(getPublicMCPReusingParamCacheKey(mcpID, groupID))

	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, getPublicMCPReusingParamCacheKey(mcpID, groupID)).
		Err()
}

func CacheSetPublicMCPReusingParam(param PublicMCPReusingParamCache) error {
	key := getPublicMCPReusingParamCacheKey(param.MCPID, param.GroupID)
	cacheSetModelLocal(key, param, clonePublicMCPReusingParamCache)
	return cacheSetPublicMCPReusingParamRedis(param)
}

func CacheGetPublicMCPReusingParam(mcpID, groupID string) (PublicMCPReusingParamCache, error) {
	if groupID == "" {
		return PublicMCPReusingParamCache{
			MCPID:   mcpID,
			GroupID: groupID,
			Params:  make(map[string]string),
		}, nil
	}

	cacheKey := getPublicMCPReusingParamCacheKey(mcpID, groupID)
	if paramCache, notFound, ok := cacheGetModelLocal(
		cacheKey,
		clonePublicMCPReusingParamCache,
	); ok {
		if notFound {
			return PublicMCPReusingParamCache{}, NotFoundError(ErrMCPReusingParamNotFound)
		}

		return paramCache, nil
	}

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		paramCache := PublicMCPReusingParamCache{}

		err := common.RDB.HGetAll(ctx, cacheKey).Scan(&paramCache)
		if err == nil && paramCache.MCPID != "" {
			cacheSetModelLocal(cacheKey, paramCache, clonePublicMCPReusingParamCache)
			return paramCache, nil
		} else if err != nil && !errors.Is(err, redis.Nil) {
			log.Errorf(
				"get public mcp reusing param (%s:%s) from redis error: %s",
				mcpID,
				groupID,
				err.Error(),
			)
		}
	}

	paramCache, notFound, loaded, err := loadWithLocalKeyLock(
		modelCacheLoadLocker,
		cacheKey,
		func() (PublicMCPReusingParamCache, bool, bool) {
			return cacheGetModelLocal(cacheKey, clonePublicMCPReusingParamCache)
		},
		func() (PublicMCPReusingParamCache, error) {
			param, err := GetPublicMCPReusingParam(mcpID, groupID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					cacheSetModelNotFoundLocalUnlocked(cacheKey)
				}

				return PublicMCPReusingParamCache{}, err
			}

			prc := param.ToPublicMCPReusingParamCache()
			cacheSetModelLocalUnlocked(cacheKey, prc, clonePublicMCPReusingParamCache)

			return prc, nil
		},
	)
	if err != nil {
		return PublicMCPReusingParamCache{}, err
	}

	if notFound {
		return PublicMCPReusingParamCache{}, NotFoundError(ErrMCPReusingParamNotFound)
	}

	if loaded {
		if err := cacheSetPublicMCPReusingParamRedis(paramCache); err != nil {
			log.Error("redis set public mcp reusing param error: " + err.Error())
		}
	}

	return paramCache, nil
}

func cacheSetPublicMCPReusingParamRedis(param PublicMCPReusingParamCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := getPublicMCPReusingParamCacheKey(param.MCPID, param.GroupID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, param)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}
