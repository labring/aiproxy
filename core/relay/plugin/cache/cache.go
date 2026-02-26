package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/noop"
	gcache "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
)

// Constants for cache metadata keys
const (
	cacheKey   = "cache_key"
	cacheHit   = "cache_hit"
	cacheValue = "cache_value"
)

// Constants for plugin configuration
const (
	pluginConfigCacheKey = "cache-config"
	cacheHeader          = "X-Aiproxy-Cache"
	redisCachePrefix     = "cache:"
)

// Buffer size constants
const (
	defaultBufferSize = 512 * 1024
	maxBufferSize     = 4 * defaultBufferSize
)

// Item represents a cached response
type Item struct {
	Body   []byte              `json:"body"`
	Header map[string][]string `json:"header"`
	Usage  model.Usage         `json:"usage"`
}

// Cache implements caching functionality for AI requests
type Cache struct {
	noop.Noop
	rdb *redis.Client
}

var (
	_ plugin.Plugin = (*Cache)(nil)
	// Global cache instance with 5 minute default TTL and 10 minute cleanup interval
	cache = gcache.New(30*time.Second, 5*time.Minute)
	// Buffer pool for response writers
	bufferPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
		},
	}
)

// NewCachePlugin creates a new cache plugin
func NewCachePlugin(rdb *redis.Client) plugin.Plugin {
	return &Cache{rdb: rdb}
}

// Cache metadata helpers
func getCacheKey(meta *meta.Meta) string {
	return meta.GetString(cacheKey)
}

func setCacheKey(meta *meta.Meta, key string) {
	meta.Set(cacheKey, key)
}

func isCacheHit(meta *meta.Meta) bool {
	return meta.GetBool(cacheHit)
}

func getCacheItem(meta *meta.Meta) *Item {
	v, ok := meta.Get(cacheValue)
	if !ok {
		return nil
	}

	item, ok := v.(*Item)
	if !ok {
		panic(fmt.Sprintf("cache item type not match: %T", v))
	}

	return item
}

func setCacheHit(meta *meta.Meta, item *Item) {
	meta.Set(cacheHit, true)
	meta.Set(cacheValue, item)
}

// Buffer pool helpers
func getBuffer() *bytes.Buffer {
	v, ok := bufferPool.Get().(*bytes.Buffer)
	if !ok {
		panic(fmt.Sprintf("buffer type error: %T, %v", v, v))
	}

	return v
}

func putBuffer(buf *bytes.Buffer) {
	buf.Reset()

	if buf.Cap() > maxBufferSize {
		return
	}

	bufferPool.Put(buf)
}

// getPluginConfig retrieves the plugin configuration from metadata
func getPluginConfig(meta *meta.Meta) (config *Config, err error) {
	v, ok := meta.Get(pluginConfigCacheKey)
	if ok {
		config, ok := v.(*Config)
		if !ok {
			panic(fmt.Sprintf("cache config type not match: %T", v))
		}

		return config, nil
	}

	pluginConfig := Config{}
	if err := meta.ModelConfig.LoadPluginConfig("cache", &pluginConfig); err != nil {
		return nil, err
	}

	meta.Set(pluginConfigCacheKey, &pluginConfig)

	return &pluginConfig, nil
}

// Redis cache operations
func (c *Cache) getFromRedis(ctx context.Context, key string) (*Item, error) {
	if c.rdb == nil {
		return nil, nil
	}

	data, err := c.rdb.Get(ctx, common.RedisKey(redisCachePrefix, key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var item Item
	if err := sonic.Unmarshal(data, &item); err != nil {
		return nil, err
	}

	return &item, nil
}

func (c *Cache) setToRedis(ctx context.Context, key string, item *Item, ttl time.Duration) error {
	if c.rdb == nil {
		return nil
	}

	data, err := sonic.Marshal(item)
	if err != nil {
		return err
	}

	return c.rdb.Set(ctx, common.RedisKey(redisCachePrefix, key), data, ttl).Err()
}

// getFromCache retrieves item from cache (Redis or memory)
func (c *Cache) getFromCache(ctx context.Context, key string) (*Item, bool) {
	// Try Redis first if available
	if c.rdb != nil {
		item, err := c.getFromRedis(ctx, key)
		if err == nil && item != nil {
			return item, true
		}
		// If Redis fails, fallback to memory cache
	}

	// Try memory cache
	if v, ok := cache.Get(key); ok {
		if item, ok := v.(Item); ok {
			return &item, true
		}
	}

	return nil, false
}

// setToCache stores item in cache (Redis and/or memory)
func (c *Cache) setToCache(ctx context.Context, key string, item Item, ttl time.Duration) {
	// Set to Redis if available
	if c.rdb != nil {
		if err := c.setToRedis(ctx, key, &item, ttl); err == nil {
			// If Redis succeeds, also set to memory cache for faster access
			cache.Set(key, item, ttl)
			return
		}
		// If Redis fails, fallback to memory cache only
	}

	// Set to memory cache
	cache.Set(key, item, ttl)
}

// ConvertRequest handles the request conversion phase
func (c *Cache) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
	do adaptor.ConvertRequest,
) (adaptor.ConvertResult, error) {
	pluginConfig, err := getPluginConfig(meta)
	if err != nil {
		return do.ConvertRequest(meta, store, req)
	}

	if !pluginConfig.Enable {
		return do.ConvertRequest(meta, store, req)
	}

	body, err := common.GetRequestBodyReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if len(body) == 0 {
		return do.ConvertRequest(meta, store, req)
	}

	// Generate hash as cache key
	hash := sha256.Sum256(body)
	cacheKey := fmt.Sprintf("%d:%s", meta.Mode, hex.EncodeToString(hash[:]))
	setCacheKey(meta, cacheKey)

	// Check cache
	ctx := req.Context()
	if item, ok := c.getFromCache(ctx, cacheKey); ok {
		setCacheHit(meta, item)
		return adaptor.ConvertResult{}, nil
	}

	return do.ConvertRequest(meta, store, req)
}

// DoRequest handles the request execution phase
func (c *Cache) DoRequest(
	meta *meta.Meta,
	store adaptor.Store,
	ctx *gin.Context,
	req *http.Request,
	do adaptor.DoRequest,
) (*http.Response, error) {
	if isCacheHit(meta) {
		return &http.Response{}, nil
	}

	return do.DoRequest(meta, store, ctx, req)
}

// Custom response writer to capture response for caching
type responseWriter struct {
	gin.ResponseWriter
	cacheBody *bytes.Buffer
	maxSize   int
	overflow  bool
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.overflow {
		return rw.ResponseWriter.Write(b)
	}

	if rw.maxSize > 0 && rw.cacheBody.Len()+len(b) > rw.maxSize {
		rw.overflow = true
		rw.cacheBody.Reset()
		return rw.ResponseWriter.Write(b)
	}

	rw.cacheBody.Write(b)

	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) WriteString(s string) (int, error) {
	if rw.overflow {
		return rw.ResponseWriter.WriteString(s)
	}

	if rw.maxSize > 0 && rw.cacheBody.Len()+len(s) > rw.maxSize {
		rw.overflow = true
		rw.cacheBody.Reset()
		return rw.ResponseWriter.WriteString(s)
	}

	rw.cacheBody.WriteString(s)

	return rw.ResponseWriter.WriteString(s)
}

func (c *Cache) writeCacheHeader(ctx *gin.Context, pluginConfig *Config, value string) {
	if pluginConfig.AddCacheHitHeader {
		header := pluginConfig.CacheHitHeader
		if header == "" {
			header = cacheHeader
		}

		ctx.Header(header, value)
	}
}

// DoResponse handles the response processing phase
func (c *Cache) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	ctx *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
) (result adaptor.DoResponseResult, adapterErr adaptor.Error) {
	pluginConfig, err := getPluginConfig(meta)
	if err != nil {
		return do.DoResponse(meta, store, ctx, resp)
	}

	// Handle cache hit
	if isCacheHit(meta) {
		item := getCacheItem(meta)
		if item == nil {
			return do.DoResponse(meta, store, ctx, resp)
		}

		// Restore headers from cache
		for k, v := range item.Header {
			for _, val := range v {
				ctx.Header(k, val)
			}
		}

		// Override specific headers
		ctx.Header("Content-Type", item.Header["Content-Type"][0])
		ctx.Header("Content-Length", strconv.Itoa(len(item.Body)))
		c.writeCacheHeader(ctx, pluginConfig, "hit")
		_, _ = ctx.Writer.Write(item.Body)

		return adaptor.DoResponseResult{Usage: item.Usage}, nil
	}

	if !pluginConfig.Enable {
		return do.DoResponse(meta, store, ctx, resp)
	}

	c.writeCacheHeader(ctx, pluginConfig, "miss")

	// Set up response capture for caching
	buf := getBuffer()
	defer putBuffer(buf)

	rw := &responseWriter{
		ResponseWriter: ctx.Writer,
		maxSize:        pluginConfig.ItemMaxSize,
		cacheBody:      buf,
	}

	ctx.Writer = rw
	defer func() {
		ctx.Writer = rw.ResponseWriter
		if adapterErr != nil ||
			rw.overflow ||
			rw.cacheBody.Len() == 0 {
			return
		}

		// Convert http.Header to map[string][]string for JSON serialization
		headerMap := maps.Clone(rw.Header())

		// Store in cache
		item := Item{
			Body:   bytes.Clone(rw.cacheBody.Bytes()),
			Header: headerMap,
			Usage:  result.Usage,
		}

		ttl := time.Duration(pluginConfig.TTL) * time.Second
		c.setToCache(ctx.Request.Context(), getCacheKey(meta), item, ttl)
	}()

	return do.DoResponse(meta, store, ctx, resp)
}
