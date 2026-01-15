package controller

import (
	"context"
	"encoding"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/model"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const (
	ToolCacheKey = "tool:%s:%d" // mcp_id:updated_at
	redisTimeout = 2 * time.Second
)

type redisToolSlice []mcp.Tool

var (
	_ redis.Scanner            = (*redisToolSlice)(nil)
	_ encoding.BinaryMarshaler = (*redisToolSlice)(nil)
)

func (r *redisToolSlice) ScanRedis(value string) error {
	return sonic.Unmarshal(conv.StringToBytes(value), r)
}

func (r redisToolSlice) MarshalBinary() ([]byte, error) {
	return sonic.Marshal(r)
}

type toolCacheMemItem struct {
	tools      []mcp.Tool
	lastUsedAt time.Time
	expiresAt  time.Time
}

type toolMemoryCache struct {
	mu             sync.Mutex
	items          map[string]toolCacheMemItem
	cleanStartOnce sync.Once
}

var toolMemCache = &toolMemoryCache{
	items: make(map[string]toolCacheMemItem),
}

func getToolCacheKey(mcpID string, updatedAt int64) string {
	return common.RedisKeyf(ToolCacheKey, mcpID, updatedAt)
}

func (c *toolMemoryCache) set(key string, tools []mcp.Tool) {
	c.startCleanupOnStart()

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.items[key] = toolCacheMemItem{
		tools:      tools,
		lastUsedAt: now,
		expiresAt:  now.Add(time.Hour),
	}
}

func (c *toolMemoryCache) get(key string) ([]mcp.Tool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	now := time.Now()
	if now.After(item.lastUsedAt.Add(time.Minute*10)) ||
		now.After(item.expiresAt) {
		delete(c.items, key)
		return nil, false
	}

	item.lastUsedAt = now

	return item.tools, true
}

func (c *toolMemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.lastUsedAt.Add(time.Minute*10)) ||
			now.After(item.expiresAt) {
			delete(c.items, key)
		}
	}
}

func (c *toolMemoryCache) startCleanupOnStart() {
	c.cleanStartOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute * 10)
			defer ticker.Stop()

			for range ticker.C {
				c.cleanup()
			}
		}()
	})
}

func CacheSetTools(mcpID string, updatedAt int64, tools []mcp.Tool) error {
	key := getToolCacheKey(mcpID, updatedAt)

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		redisKey := common.RedisKeyf(ToolCacheKey, mcpID, updatedAt)
		pipe := common.RDB.Pipeline()
		pipe.HSet(ctx, redisKey, tools)
		pipe.Expire(ctx, redisKey, time.Hour)
		_, err := pipe.Exec(ctx)

		return err
	}

	toolMemCache.set(key, tools)

	return nil
}

func CacheGetTools(mcpID string, updatedAt int64) ([]mcp.Tool, bool) {
	key := getToolCacheKey(mcpID, updatedAt)

	if common.RedisEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()

		tools := redisToolSlice{}

		err := common.RDB.HGetAll(ctx, key).Scan(&tools)
		if err != nil {
			log.Errorf("failed to get tools cache from redis (%s): %v", key, err)
		} else {
			return tools, true
		}
	}

	item, exists := toolMemCache.get(key)
	if exists {
		return item, true
	}

	return nil, false
}

func checkParamsIsFull(params model.Params, reusing map[string]model.ReusingParam) bool {
	for key, r := range reusing {
		if !r.Required {
			continue
		}

		if params == nil {
			return false
		}

		if v, ok := params[key]; !ok || v == "" {
			return false
		}
	}

	return true
}

func getPublicMCPTools(
	ctx context.Context,
	publicMcp model.PublicMCP,
	testConfig model.TestConfig,
	params map[string]string,
	reusing map[string]model.ReusingParam,
) (tools []mcp.Tool, err error) {
	tools, exists := CacheGetTools(publicMcp.ID, publicMcp.UpdateAt.Unix())
	if exists {
		return tools, nil
	}

	defer func() {
		if err != nil {
			return
		}

		if err := CacheSetTools(publicMcp.ID, publicMcp.UpdateAt.Unix(), tools); err != nil {
			log.Errorf("failed to set tools cache in redis: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	switch publicMcp.Type {
	case model.PublicMCPTypeEmbed:
		return getEmbedMCPTools(ctx, publicMcp, testConfig, params, reusing)
	case model.PublicMCPTypeOpenAPI:
		return getOpenAPIMCPTools(ctx, publicMcp)
	case model.PublicMCPTypeProxySSE:
		return getProxySSEMCPTools(ctx, publicMcp, testConfig, params, reusing)
	case model.PublicMCPTypeProxyStreamable:
		return getProxyStreamableMCPTools(ctx, publicMcp, testConfig, params, reusing)
	default:
		return nil, nil
	}
}

func getEmbedMCPTools(
	ctx context.Context,
	publicMcp model.PublicMCP,
	testConfig model.TestConfig,
	params map[string]string,
	reusing map[string]model.ReusingParam,
) ([]mcp.Tool, error) {
	tools, err := mcpservers.ListTools(ctx, publicMcp.ID)
	if err == nil {
		return tools, nil
	}

	if publicMcp.EmbedConfig == nil {
		return nil, nil
	}

	var effectiveParams map[string]string
	switch {
	case testConfig.Enabled && checkParamsIsFull(testConfig.Params, reusing):
		effectiveParams = testConfig.Params
	case checkParamsIsFull(params, reusing):
		effectiveParams = params
	default:
		return nil, nil
	}

	server, err := mcpservers.GetMCPServer(
		publicMcp.ID,
		publicMcp.EmbedConfig.Init,
		effectiveParams,
	)
	if err != nil {
		return nil, err
	}

	return mcpservers.ListServerTools(ctx, server)
}

func getOpenAPIMCPTools(ctx context.Context, publicMcp model.PublicMCP) ([]mcp.Tool, error) {
	if publicMcp.OpenAPIConfig == nil {
		return nil, nil
	}

	server, err := newOpenAPIMCPServer(publicMcp.OpenAPIConfig)
	if err != nil {
		return nil, err
	}

	return mcpservers.ListServerTools(ctx, server)
}

func getProxySSEMCPTools(
	ctx context.Context,
	publicMcp model.PublicMCP,
	testConfig model.TestConfig,
	params map[string]string,
	reusing map[string]model.ReusingParam,
) ([]mcp.Tool, error) {
	if publicMcp.ProxyConfig == nil {
		return nil, nil
	}

	var effectiveParams map[string]string
	switch {
	case testConfig.Enabled && checkParamsIsFull(testConfig.Params, reusing):
		effectiveParams = testConfig.Params
	case checkParamsIsFull(params, reusing):
		effectiveParams = params
	default:
		return nil, nil
	}

	url, headers, err := prepareProxyConfig(
		publicMcp.ToPublicMCPCache(),
		staticParams(effectiveParams),
	)
	if err != nil {
		return nil, err
	}

	client, err := transport.NewSSE(url, transport.WithHeaders(headers))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.Start(ctx); err != nil {
		return nil, err
	}

	return mcpservers.ListServerTools(ctx, mcpservers.WrapMCPClient2Server(client))
}

func getProxyStreamableMCPTools(
	ctx context.Context,
	publicMcp model.PublicMCP,
	testConfig model.TestConfig,
	params map[string]string,
	reusing map[string]model.ReusingParam,
) ([]mcp.Tool, error) {
	if publicMcp.ProxyConfig == nil {
		return nil, nil
	}

	var effectiveParams map[string]string
	switch {
	case testConfig.Enabled && checkParamsIsFull(testConfig.Params, reusing):
		effectiveParams = testConfig.Params
	case checkParamsIsFull(params, reusing):
		effectiveParams = params
	default:
		return nil, nil
	}

	url, headers, err := prepareProxyConfig(
		publicMcp.ToPublicMCPCache(),
		staticParams(effectiveParams),
	)
	if err != nil {
		return nil, err
	}

	client, err := transport.NewStreamableHTTP(
		url,
		transport.WithHTTPHeaders(headers),
	)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.Start(ctx); err != nil {
		return nil, err
	}

	return mcpservers.ListServerTools(ctx, mcpservers.WrapMCPClient2Server(client))
}
