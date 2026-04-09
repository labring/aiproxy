package model

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
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

func CacheDeleteGroupMCP(groupID, mcpID string) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, common.RedisKeyf(GroupMCPCacheKey, groupID, mcpID)).Err()
}

func CacheSetGroupMCP(groupMCP *GroupMCPCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := common.RedisKeyf(GroupMCPCacheKey, groupMCP.GroupID, groupMCP.ID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, groupMCP)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

func CacheGetGroupMCP(groupID, mcpID string) (*GroupMCPCache, error) {
	if !common.RedisEnabled {
		groupMCP, err := GetGroupMCPByID(mcpID, groupID)
		if err != nil {
			return nil, err
		}

		return groupMCP.ToGroupMCPCache(), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cacheKey := common.RedisKeyf(GroupMCPCacheKey, groupID, mcpID)
	groupMCPCache := &GroupMCPCache{}

	err := common.RDB.HGetAll(ctx, cacheKey).Scan(groupMCPCache)
	if err == nil && groupMCPCache.ID != "" {
		return groupMCPCache, nil
	} else if err != nil && !errors.Is(err, redis.Nil) {
		log.Errorf("get group mcp (%s:%s) from redis error: %s", groupID, mcpID, err.Error())
	}

	groupMCP, err := GetGroupMCPByID(mcpID, groupID)
	if err != nil {
		return nil, err
	}

	gmc := groupMCP.ToGroupMCPCache()

	if err := CacheSetGroupMCP(gmc); err != nil {
		log.Error("redis set group mcp error: " + err.Error())
	}

	return gmc, nil
}

var updateGroupMCPStatusScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "s") then
		redis.call("HSet", KEYS[1], "s", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdateGroupMCPStatus(groupID, mcpID string, status GroupMCPStatus) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updateGroupMCPStatusScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(GroupMCPCacheKey, groupID, mcpID)},
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

func CacheDeletePublicMCP(mcpID string) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, common.RedisKeyf(PublicMCPCacheKey, mcpID)).Err()
}

func CacheSetPublicMCP(publicMCP *PublicMCPCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := common.RedisKeyf(PublicMCPCacheKey, publicMCP.ID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, publicMCP)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

func CacheGetPublicMCP(mcpID string) (*PublicMCPCache, error) {
	if !common.RedisEnabled {
		publicMCP, err := GetPublicMCPByID(mcpID)
		if err != nil {
			return nil, err
		}

		return publicMCP.ToPublicMCPCache(), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cacheKey := common.RedisKeyf(PublicMCPCacheKey, mcpID)
	publicMCPCache := &PublicMCPCache{}

	err := common.RDB.HGetAll(ctx, cacheKey).Scan(publicMCPCache)
	if err == nil && publicMCPCache.ID != "" {
		return publicMCPCache, nil
	} else if err != nil && !errors.Is(err, redis.Nil) {
		log.Errorf("get public mcp (%s) from redis error: %s", mcpID, err.Error())
	}

	publicMCP, err := GetPublicMCPByID(mcpID)
	if err != nil {
		return nil, err
	}

	pmc := publicMCP.ToPublicMCPCache()

	if err := CacheSetPublicMCP(pmc); err != nil {
		log.Error("redis set public mcp error: " + err.Error())
	}

	return pmc, nil
}

var updatePublicMCPStatusScript = redis.NewScript(`
	if redis.call("HExists", KEYS[1], "s") then
		redis.call("HSet", KEYS[1], "s", ARGV[1])
	end
	return redis.status_reply("ok")
`)

func CacheUpdatePublicMCPStatus(mcpID string, status PublicMCPStatus) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return updatePublicMCPStatusScript.Run(
		ctx,
		common.RDB,
		[]string{common.RedisKeyf(PublicMCPCacheKey, mcpID)},
		status,
	).Err()
}

const (
	PublicMCPReusingParamCacheKey = "public_mcp_param:%s:%s"
)

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
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	return common.RDB.Del(ctx, common.RedisKeyf(PublicMCPReusingParamCacheKey, mcpID, groupID)).
		Err()
}

func CacheSetPublicMCPReusingParam(param PublicMCPReusingParamCache) error {
	if !common.RedisEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	key := common.RedisKeyf(PublicMCPReusingParamCacheKey, param.MCPID, param.GroupID)
	pipe := common.RDB.Pipeline()
	pipe.HSet(ctx, key, param)

	expireTime := SyncFrequency + time.Duration(rand.Int64N(60)-30)*time.Second
	pipe.Expire(ctx, key, expireTime)
	_, err := pipe.Exec(ctx)

	return err
}

func CacheGetPublicMCPReusingParam(mcpID, groupID string) (PublicMCPReusingParamCache, error) {
	if groupID == "" {
		return PublicMCPReusingParamCache{
			MCPID:   mcpID,
			GroupID: groupID,
			Params:  make(map[string]string),
		}, nil
	}

	if !common.RedisEnabled {
		param, err := GetPublicMCPReusingParam(mcpID, groupID)
		if err != nil {
			return PublicMCPReusingParamCache{}, err
		}

		return param.ToPublicMCPReusingParamCache(), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	cacheKey := common.RedisKeyf(PublicMCPReusingParamCacheKey, mcpID, groupID)
	paramCache := PublicMCPReusingParamCache{}

	err := common.RDB.HGetAll(ctx, cacheKey).Scan(&paramCache)
	if err == nil && paramCache.MCPID != "" {
		return paramCache, nil
	} else if err != nil && !errors.Is(err, redis.Nil) {
		log.Errorf(
			"get public mcp reusing param (%s:%s) from redis error: %s",
			mcpID,
			groupID,
			err.Error(),
		)
	}

	param, err := GetPublicMCPReusingParam(mcpID, groupID)
	if err != nil {
		return PublicMCPReusingParamCache{}, err
	}

	prc := param.ToPublicMCPReusingParamCache()

	if err := CacheSetPublicMCPReusingParam(prc); err != nil {
		log.Error("redis set public mcp reusing param error: " + err.Error())
	}

	return prc, nil
}
