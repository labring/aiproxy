package cachefollow

import (
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/noop"
	"github.com/labring/aiproxy/core/relay/render"
)

const (
	defaultFollowedChannelTTL          = 3 * time.Minute
	defaultRecentChannelUpdateDebounce = 30 * time.Second
)

var _ plugin.Plugin = (*Plugin)(nil)

type Plugin struct {
	noop.Noop
}

func NewCacheFollowPlugin() plugin.Plugin {
	return &Plugin{}
}

func (p *Plugin) getConfig(meta *meta.Meta) (*Config, error) {
	pluginConfig := &Config{}
	if err := meta.ModelConfig.LoadPluginConfig(PluginName, pluginConfig); err != nil {
		return nil, err
	}

	return pluginConfig, nil
}

func getFollowedChannelTTL(retention string, defaultTTL time.Duration) time.Duration {
	retention = strings.TrimSpace(strings.ToLower(retention))
	if retention == "" || retention == "in-memory" || retention == "in_memory" {
		return defaultTTL
	}

	ttl, err := time.ParseDuration(retention)
	if err != nil || ttl <= 0 {
		return defaultTTL
	}

	return ttl
}

func getNodeStringField(node *ast.Node, key string) (string, bool) {
	field := node.Get(key)
	if field == nil || !field.Exists() {
		return "", false
	}

	if field.TypeSafe() == ast.V_NULL {
		return "", true
	}

	value, err := field.String()
	if err != nil {
		return "", true
	}

	return value, true
}

type retentionResponseWriter struct {
	gin.ResponseWriter
	parsed    bool
	retention string
}

func (rw *retentionResponseWriter) Write(b []byte) (int, error) {
	if !rw.parsed {
		rw.tryParseRetention(b)
	}

	return rw.ResponseWriter.Write(b)
}

func (rw *retentionResponseWriter) WriteString(s string) (int, error) {
	return rw.Write([]byte(s))
}

func (rw *retentionResponseWriter) tryParseRetention(data []byte) {
	if render.IsValidSSEData(data) {
		data = render.ExtractSSEData(data)
		if len(data) == 0 || render.IsSSEDone(data) {
			return
		}
	}

	node, err := sonic.Get(data)
	if err != nil || !node.Valid() {
		return
	}

	retention, ok := getNodeStringField(&node, "prompt_cache_retention")
	if ok {
		rw.retention = retention
		rw.parsed = true
		return
	}

	if !ok {
		responseNode := node.Get("response")
		if responseNode != nil && responseNode.Exists() && responseNode.TypeSafe() != ast.V_NULL {
			rw.retention, rw.parsed = getNodeStringField(responseNode, "prompt_cache_retention")
		}
	}
}

func shouldRecord(
	meta *meta.Meta,
	c *gin.Context,
	result adaptor.DoResponseResult,
	relayErr adaptor.Error,
) bool {
	if relayErr != nil ||
		(result.Usage.CachedTokens <= 0 && result.Usage.CacheCreationTokens <= 0) {
		return false
	}

	if meta.Channel.ID == 0 || meta.OriginModel == "" {
		return false
	}

	status := c.Writer.Status()
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return false
	}

	if !c.Writer.Written() || c.Writer.Size() <= 0 {
		return false
	}

	return true
}

func saveStableStoreMapping(
	store adaptor.Store,
	id string,
	meta *meta.Meta,
	expiresAt time.Time,
) error {
	if id == "" {
		return nil
	}

	now := time.Now()

	return store.SaveIfNotExistStore(adaptor.StoreCache{
		ID:        id,
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: expiresAt,
	})
}

func saveRecentStoreMapping(
	store adaptor.Store,
	id string,
	meta *meta.Meta,
	expiresAt time.Time,
	minInterval time.Duration,
) error {
	if id == "" {
		return nil
	}

	now := time.Now()

	return store.SaveStoreWithOption(adaptor.StoreCache{
		ID:        id,
		GroupID:   meta.Group.ID,
		TokenID:   meta.Token.ID,
		ChannelID: meta.Channel.ID,
		Model:     meta.OriginModel,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: expiresAt,
	}, adaptor.SaveStoreOption{
		MinUpdateInterval: minInterval,
	})
}

func savePromptCacheMappings(
	store adaptor.Store,
	meta *meta.Meta,
	expiresAt time.Time,
	minInterval time.Duration,
) error {
	if err := saveStableStoreMapping(
		store,
		model.PromptCacheStoreID(
			meta.OriginModel,
			meta.PromptCacheKey,
			model.CacheKeyTypeStable,
		),
		meta,
		expiresAt,
	); err != nil {
		return err
	}

	return saveRecentStoreMapping(
		store,
		model.PromptCacheStoreID(
			meta.OriginModel,
			meta.PromptCacheKey,
			model.CacheKeyTypeRecent,
		),
		meta,
		expiresAt,
		minInterval,
	)
}

func saveCacheFollowMappings(
	store adaptor.Store,
	meta *meta.Meta,
	expiresAt time.Time,
	minInterval time.Duration,
	enableGenericFollow bool,
) error {
	if meta.User != "" {
		if err := saveStableStoreMapping(
			store,
			model.CacheFollowUserStoreID(meta.OriginModel, meta.User, model.CacheKeyTypeStable),
			meta,
			expiresAt,
		); err != nil {
			return err
		}

		if err := saveRecentStoreMapping(
			store,
			model.CacheFollowUserStoreID(meta.OriginModel, meta.User, model.CacheKeyTypeRecent),
			meta,
			expiresAt,
			minInterval,
		); err != nil {
			return err
		}
	}

	if !enableGenericFollow {
		return nil
	}

	if err := saveStableStoreMapping(
		store,
		model.CacheFollowStoreID(meta.OriginModel, model.CacheKeyTypeStable),
		meta,
		expiresAt,
	); err != nil {
		return err
	}

	return saveRecentStoreMapping(
		store,
		model.CacheFollowStoreID(meta.OriginModel, model.CacheKeyTypeRecent),
		meta,
		expiresAt,
		minInterval,
	)
}

func supportsPromptCacheStore(m mode.Mode) bool {
	switch m {
	case mode.Responses, mode.ChatCompletions:
		return true
	default:
		return false
	}
}

func supportsCacheFollowStore(m mode.Mode) bool {
	switch m {
	case mode.Responses, mode.ChatCompletions, mode.Gemini, mode.Anthropic:
		return true
	default:
		return false
	}
}

func (p *Plugin) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
) (adaptor.DoResponseResult, adaptor.Error) {
	if !supportsCacheFollowStore(meta.Mode) {
		return do.DoResponse(meta, store, c, resp)
	}

	pluginConfig, err := p.getConfig(meta)
	if err != nil || !pluginConfig.Enable {
		return do.DoResponse(meta, store, c, resp)
	}

	supportsPromptCache := supportsPromptCacheStore(meta.Mode)

	var retentionWriter *retentionResponseWriter
	if meta.PromptCacheKey != "" && supportsPromptCache {
		retentionWriter = &retentionResponseWriter{ResponseWriter: c.Writer}

		c.Writer = retentionWriter
		defer func() {
			c.Writer = retentionWriter.ResponseWriter
		}()
	}

	result, relayErr := do.DoResponse(meta, store, c, resp)
	if !shouldRecord(meta, c, result, relayErr) {
		return result, relayErr
	}

	if meta.PromptCacheKey != "" && supportsPromptCache {
		retention := ""
		if retentionWriter != nil {
			retention = retentionWriter.retention
		}

		expiresAt := time.Now().
			Add(getFollowedChannelTTL(retention, pluginConfig.GetFollowedChannelTTL()))
		if err := savePromptCacheMappings(
			store,
			meta,
			expiresAt,
			pluginConfig.GetRecentChannelUpdateDebounce(),
		); err != nil {
			common.GetLogger(c).Warnf("save prompt cache key store failed: %v", err)
		}
	}

	expiresAt := time.Now().Add(pluginConfig.GetFollowedChannelTTL())
	if err := saveCacheFollowMappings(
		store,
		meta,
		expiresAt,
		pluginConfig.GetRecentChannelUpdateDebounce(),
		pluginConfig.EnableGenericFollow,
	); err != nil {
		common.GetLogger(c).Warnf("save cachefollow store failed: %v", err)
	}

	return result, relayErr
}
