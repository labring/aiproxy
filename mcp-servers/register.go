package mcpservers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labring/aiproxy/core/model"
	"github.com/mark3labs/mcp-go/mcp"
)

type mcpServerCacheItem struct {
	MCPServer         Server
	LastUsedTimestamp atomic.Int64
}

var (
	servers             = make(map[string]McpServer)
	mcpServerCache      = make(map[string]*mcpServerCacheItem)
	mcpServerCacheLock  = sync.RWMutex{}
	cacheExpirationTime = 3 * time.Minute
)

func startCacheCleaner(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			cleanupExpiredCache()
		}
	}()
}

func cleanupExpiredCache() {
	now := time.Now().Unix()
	expiredTime := now - int64(cacheExpirationTime.Seconds())

	mcpServerCacheLock.Lock()
	defer mcpServerCacheLock.Unlock()

	for key, item := range mcpServerCache {
		if item.LastUsedTimestamp.Load() < expiredTime {
			delete(mcpServerCache, key)
		}
	}
}

func init() {
	startCacheCleaner(time.Minute)
}

func Register(mcp McpServer) {
	if mcp.ID == "" {
		panic("mcp id is required")
	}

	if mcp.Name == "" {
		panic("mcp name is required")
	}

	if mcp.Description == "" &&
		mcp.DescriptionCN == "" &&
		mcp.Readme == "" &&
		mcp.ReadmeURL == "" &&
		mcp.ReadmeCN == "" &&
		mcp.ReadmeCNURL == "" {
		panic(
			fmt.Sprintf(
				"mcp %s description or description_cn readme or readme_url or readme_cn or readme_cn_url is required",
				mcp.ID,
			),
		)
	}

	switch mcp.Type {
	case model.PublicMCPTypeEmbed:
		if mcp.newServer == nil {
			panic(fmt.Sprintf("mcp %s new server is required", mcp.ID))
		}
	case model.PublicMCPTypeProxySSE,
		model.PublicMCPTypeProxyStreamable:
		if len(mcp.ProxyConfigTemplates) == 0 {
			panic(fmt.Sprintf("mcp %s proxy config templates is required", mcp.ID))
		}
	default:
	}

	if len(mcp.ConfigTemplates) != 0 {
		if err := CheckConfigTemplatesValidate(mcp.ConfigTemplates); err != nil {
			panic(fmt.Sprintf("mcp %s config templates example is invalid: %v", mcp.ID, err))
		}
	}

	if len(mcp.ProxyConfigTemplates) != 0 {
		if err := CheckProxyConfigTemplatesValidate(mcp.ProxyConfigTemplates); err != nil {
			panic(fmt.Sprintf("mcp %s config templates example is invalid: %v", mcp.ID, err))
		}
	}

	if _, ok := servers[mcp.ID]; ok {
		panic(fmt.Sprintf("mcp %s already registered", mcp.ID))
	}

	servers[mcp.ID] = mcp
}

func ListTools(ctx context.Context, id string) ([]mcp.Tool, error) {
	embedServer, ok := servers[id]
	if !ok {
		return nil, fmt.Errorf("mcp %s not found", id)
	}

	tools, err := embedServer.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp %s list tools error: %w", id, err)
	}

	return tools, nil
}

func GetMCPServer(id string, config, reusingConfig map[string]string) (Server, error) {
	embedServer, ok := servers[id]
	if !ok {
		return nil, fmt.Errorf("mcp %s not found", id)
	}

	if len(embedServer.ConfigTemplates) == 0 {
		if embedServer.disableCache {
			return embedServer.NewServer(config, reusingConfig)
		}
		return loadCacheServer(embedServer, nil)
	}

	if err := ValidateConfigTemplatesConfig(
		embedServer.ConfigTemplates,
		config,
		reusingConfig,
	); err != nil {
		return nil, fmt.Errorf("mcp %s config is invalid: %w", id, err)
	}

	if embedServer.disableCache {
		return embedServer.NewServer(config, reusingConfig)
	}

	if len(reusingConfig) == 0 {
		return loadCacheServer(embedServer, config)
	}

	for _, template := range embedServer.ConfigTemplates {
		switch template.Required {
		case ConfigRequiredTypeReusingOptional,
			ConfigRequiredTypeReusingOnly,
			ConfigRequiredTypeInitOrReusingOnly:
			return embedServer.NewServer(config, reusingConfig)
		}
	}

	return loadCacheServer(embedServer, config)
}

func buildNoReusingConfigCacheKey(config map[string]string) string {
	keys := make([]string, 0, len(config))
	for key, value := range config {
		keys = append(keys, fmt.Sprintf("%s:%s", key, value))
	}

	sort.Strings(keys)

	return strings.Join(keys, ":")
}

func loadCacheServer(embedServer McpServer, config map[string]string) (Server, error) {
	cacheKey := embedServer.ID
	if len(config) > 0 {
		cacheKey = fmt.Sprintf("%s:%s", embedServer.ID, buildNoReusingConfigCacheKey(config))
	}

	mcpServerCacheLock.RLock()

	server, ok := mcpServerCache[cacheKey]

	mcpServerCacheLock.RUnlock()

	if ok {
		server.LastUsedTimestamp.Store(time.Now().Unix())
		return server.MCPServer, nil
	}

	mcpServerCacheLock.Lock()
	defer mcpServerCacheLock.Unlock()

	server, ok = mcpServerCache[cacheKey]
	if ok {
		server.LastUsedTimestamp.Store(time.Now().Unix())
		return server.MCPServer, nil
	}

	mcpServer, err := embedServer.NewServer(config, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp %s new server is invalid: %w", embedServer.ID, err)
	}

	mcpServerCacheItem := &mcpServerCacheItem{
		MCPServer:         mcpServer,
		LastUsedTimestamp: atomic.Int64{},
	}
	mcpServerCacheItem.LastUsedTimestamp.Store(time.Now().Unix())
	mcpServerCache[cacheKey] = mcpServerCacheItem

	return mcpServer, nil
}

func Servers() map[string]McpServer {
	return servers
}

func GetEmbedMCP(id string) (McpServer, bool) {
	mcp, ok := servers[id]
	return mcp, ok
}
