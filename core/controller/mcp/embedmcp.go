package controller

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/mcpproxy"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	// init embed mcp
	_ "github.com/labring/aiproxy/mcp-servers/mcpregister"
	"github.com/mark3labs/mcp-go/mcp"
)

type EmbedMCPConfigTemplate struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Example     string `json:"example,omitempty"`
	Description string `json:"description,omitempty"`
}

func newEmbedMCPConfigTemplate(template mcpservers.ConfigTemplate) EmbedMCPConfigTemplate {
	return EmbedMCPConfigTemplate{
		Name:        template.Name,
		Required:    template.Required == mcpservers.ConfigRequiredTypeInitOnly,
		Example:     template.Example,
		Description: template.Description,
	}
}

type EmbedMCPConfigTemplates = map[string]EmbedMCPConfigTemplate

func newEmbedMCPConfigTemplates(templates mcpservers.ConfigTemplates) EmbedMCPConfigTemplates {
	emcpTemplates := make(EmbedMCPConfigTemplates, len(templates))
	for key, template := range templates {
		emcpTemplates[key] = newEmbedMCPConfigTemplate(template)
	}

	return emcpTemplates
}

func newEmbedMCPProxyConfigTemplates(
	templates mcpservers.ProxyConfigTemplates,
) EmbedMCPConfigTemplates {
	emcpTemplates := make(EmbedMCPConfigTemplates, len(templates))
	for key, template := range templates {
		emcpTemplates[key] = newEmbedMCPConfigTemplate(template.ConfigTemplate)
	}

	return emcpTemplates
}

type EmbedMCP struct {
	ID              string                    `json:"id"`
	Enabled         bool                      `json:"enabled"`
	Name            string                    `json:"name"`
	NameCN          string                    `json:"name_cn"`
	Readme          string                    `json:"readme"`
	ReadmeURL       string                    `json:"readme_url"`
	ReadmeCN        string                    `json:"readme_cn"`
	ReadmeCNURL     string                    `json:"readme_cn_url"`
	GitHubURL       string                    `json:"github_url"`
	Tags            []string                  `json:"tags"`
	ConfigTemplates EmbedMCPConfigTemplates   `json:"config_templates"`
	EmbedConfig     *model.MCPEmbeddingConfig `json:"embed_config"`
}

func newEmbedMCP(
	mcp *mcpservers.McpServer,
	enabled bool,
	embedConfig *model.MCPEmbeddingConfig,
) *EmbedMCP {
	emcp := &EmbedMCP{
		ID:          mcp.ID,
		Enabled:     enabled,
		Name:        mcp.Name,
		NameCN:      mcp.NameCN,
		Readme:      mcp.Readme,
		ReadmeURL:   mcp.ReadmeURL,
		ReadmeCN:    mcp.ReadmeCN,
		ReadmeCNURL: mcp.ReadmeCNURL,
		GitHubURL:   mcp.GitHubURL,
		Tags:        mcp.Tags,
		EmbedConfig: embedConfig,
	}
	if len(mcp.ConfigTemplates) != 0 {
		emcp.ConfigTemplates = newEmbedMCPConfigTemplates(mcp.ConfigTemplates)
	}

	if len(mcp.ProxyConfigTemplates) != 0 {
		emcp.ConfigTemplates = newEmbedMCPProxyConfigTemplates(mcp.ProxyConfigTemplates)
	}

	if emcp.ConfigTemplates == nil {
		emcp.ConfigTemplates = make(EmbedMCPConfigTemplates)
	}

	return emcp
}

// GetEmbedMCPs godoc
//
//	@Summary		Get embed mcp
//	@Description	Get embed mcp
//	@Tags			embedmcp
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{array}	EmbedMCP
//	@Router			/api/embedmcp/ [get]
func GetEmbedMCPs(c *gin.Context) {
	embeds := mcpservers.Servers()
	embedIDs := slices.Collect(maps.Keys(embeds))

	enabledMCPs, err := model.GetPublicMCPsEnabled(embedIDs)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	embedConfigs, err := model.GetPublicMCPsEmbedConfig(embedIDs)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	emcps := make([]*EmbedMCP, 0, len(embeds))
	for _, mcp := range embeds {
		enabled := slices.Contains(enabledMCPs, mcp.ID)

		var embedConfig *model.MCPEmbeddingConfig
		if c, ok := embedConfigs[mcp.ID]; ok {
			embedConfig = &c
		}

		emcps = append(
			emcps,
			newEmbedMCP(
				&mcp,
				enabled,
				embedConfig,
			),
		)
	}

	slices.SortFunc(emcps, func(a, b *EmbedMCP) int {
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		}

		if a.Enabled != b.Enabled {
			if a.Enabled {
				return -1
			}
			return 1
		}

		return strings.Compare(a.ID, b.ID)
	})

	middleware.SuccessResponse(c, emcps)
}

type SaveEmbedMCPRequest struct {
	ID         string            `json:"id"`
	Enabled    bool              `json:"enabled"`
	InitConfig map[string]string `json:"init_config"`
}

func GetEmbedConfig(
	ct mcpservers.ConfigTemplates,
	initConfig map[string]string,
) (*model.MCPEmbeddingConfig, error) {
	reusingConfig := make(map[string]model.ReusingParam)

	embedConfig := &model.MCPEmbeddingConfig{
		Init: initConfig,
	}
	for key, value := range ct {
		switch value.Required {
		case mcpservers.ConfigRequiredTypeInitOnly:
			if v, ok := initConfig[key]; !ok || v == "" {
				return nil, fmt.Errorf("config %s is required", key)
			}
		case mcpservers.ConfigRequiredTypeReusingOnly:
			if _, ok := initConfig[key]; ok {
				return nil, fmt.Errorf("config %s is provided, but it is not allowed", key)
			}

			reusingConfig[key] = model.ReusingParam{
				Name:        value.Name,
				Description: value.Description,
				Required:    true,
			}
		case mcpservers.ConfigRequiredTypeInitOrReusingOnly:
			if v, ok := initConfig[key]; ok && v != "" {
				continue
			}

			reusingConfig[key] = model.ReusingParam{
				Name:        value.Name,
				Description: value.Description,
				Required:    true,
			}
		}
	}

	embedConfig.Reusing = reusingConfig

	return embedConfig, nil
}

func GetProxyConfig(
	proxyConfigType mcpservers.ProxyConfigTemplates,
	initConfig map[string]string,
) (*model.PublicMCPProxyConfig, error) {
	if len(proxyConfigType) == 0 {
		return nil, errors.New("proxy config type is empty")
	}

	config := &model.PublicMCPProxyConfig{
		Querys:  make(map[string]string),
		Headers: make(map[string]string),
		Reusing: make(map[string]model.PublicMCPProxyReusingParam),
	}

	for key, param := range proxyConfigType {
		value := initConfig[key]
		if value == "" {
			value = param.Default
		}

		switch param.Required {
		case mcpservers.ConfigRequiredTypeInitOnly:
			// 必须在初始化时提供
			if value == "" {
				return nil, fmt.Errorf("parameter %s is required", key)
			}

			applyParamToConfig(config, key, value, param.Type)
		case mcpservers.ConfigRequiredTypeReusingOnly:
			// 只能通过 reusing 提供，不能在初始化时提供
			if value != "" {
				return nil, fmt.Errorf(
					"parameter %s should not be provided in init config, it should be provided via reusing",
					key,
				)
			}

			config.Reusing[key] = model.PublicMCPProxyReusingParam{
				ReusingParam: model.ReusingParam{
					Name:        param.Name,
					Description: param.Description,
					Required:    true,
				},
				Type: param.Type,
			}
		case mcpservers.ConfigRequiredTypeInitOrReusingOnly:
			// 可以在初始化时提供，也可以通过 reusing 提供
			if value != "" {
				applyParamToConfig(config, key, value, param.Type)
			} else {
				config.Reusing[key] = model.PublicMCPProxyReusingParam{
					ReusingParam: model.ReusingParam{
						Name:        param.Name,
						Description: param.Description,
						Required:    true,
					},
					Type: param.Type,
				}
			}
		default:
			// 可选参数
			if value != "" {
				applyParamToConfig(config, key, value, param.Type)
			}
		}
	}

	if config.URL == "" {
		return nil, errors.New("url is required in proxy config")
	}

	return config, nil
}

// 辅助函数：将参数应用到配置中
func applyParamToConfig(
	config *model.PublicMCPProxyConfig,
	key, value string,
	paramType model.ProxyParamType,
) {
	switch paramType {
	case model.ParamTypeURL:
		config.URL = value
	case model.ParamTypeHeader:
		config.Headers[key] = value
	case model.ParamTypeQuery:
		config.Querys[key] = value
	}
}

func ToPublicMCP(
	e mcpservers.McpServer,
	initConfig map[string]string,
	enabled bool,
) (*model.PublicMCP, error) {
	pmcp := e.PublicMCP
	switch e.Type {
	case model.PublicMCPTypeEmbed:
		embedConfig, err := GetEmbedConfig(e.ConfigTemplates, initConfig)
		if err != nil {
			return nil, err
		}

		pmcp.EmbedConfig = embedConfig
	case model.PublicMCPTypeProxySSE, model.PublicMCPTypeProxyStreamable:
		proxyConfig, err := GetProxyConfig(e.ProxyConfigTemplates, initConfig)
		if err != nil {
			return nil, err
		}

		pmcp.ProxyConfig = proxyConfig
	default:
	}

	if enabled {
		pmcp.Status = model.PublicMCPStatusEnabled
	} else {
		pmcp.Status = model.PublicMCPStatusDisabled
	}

	return &pmcp, nil
}

// SaveEmbedMCP godoc
//
//	@Summary		Save embed mcp
//	@Description	Save embed mcp
//	@Tags			embedmcp
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			body	body		SaveEmbedMCPRequest	true	"Save embed mcp request"
//	@Success		200		{object}	nil
//	@Router			/api/embedmcp/ [post]
func SaveEmbedMCP(c *gin.Context) {
	var req SaveEmbedMCPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	emcp, ok := mcpservers.GetEmbedMCP(req.ID)
	if !ok {
		middleware.ErrorResponse(c, http.StatusNotFound, "embed mcp not found")
		return
	}

	pmcp, err := ToPublicMCP(emcp, req.InitConfig, req.Enabled)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := model.SavePublicMCP(pmcp); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// query like:
// /api/test-embedmcp/aiproxy-openapi/sse?key=adminkey&config[key1]=value1&config[key2]=value2&reusing[key3]=value3
func getConfigFromQuery(c *gin.Context) (map[string]string, map[string]string) {
	initConfig := make(map[string]string)
	reusingConfig := make(map[string]string)

	queryParams := c.Request.URL.Query()

	for paramName, paramValues := range queryParams {
		if len(paramValues) == 0 {
			continue
		}

		paramValue := paramValues[0]

		if strings.HasPrefix(paramName, "config[") && strings.HasSuffix(paramName, "]") {
			key := paramName[7 : len(paramName)-1]
			if key != "" {
				initConfig[key] = paramValue
			}
		}

		if strings.HasPrefix(paramName, "reusing[") && strings.HasSuffix(paramName, "]") {
			key := paramName[8 : len(paramName)-1]
			if key != "" {
				reusingConfig[key] = paramValue
			}
		}
	}

	return initConfig, reusingConfig
}

// TestEmbedMCPSseServer godoc
//
//	@Summary		Test Embed MCP SSE Server
//	@Description	Test Embed MCP SSE Server
//	@Tags			embedmcp
//	@Security		ApiKeyAuth
//	@Param			id				path		string	true	"MCP ID"
//	@Param			config[key]		query		string	false	"Initial configuration parameters (e.g. config[host]=http://localhost:3000)"
//	@Param			reusing[key]	query		string	false	"Reusing configuration parameters (e.g. reusing[authorization]=apikey)"
//	@Success		200				{object}	nil
//	@Failure		400				{object}	nil
//	@Router			/api/test-embedmcp/{id}/sse [get]
func TestEmbedMCPSseServer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		http.Error(c.Writer, "mcp id is required", http.StatusBadRequest)
		return
	}

	initConfig, reusingConfig := getConfigFromQuery(c)

	emcp, err := mcpservers.GetMCPServer(id, initConfig, reusingConfig)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	handleTestEmbedMCPServer(c, emcp)
}

const (
	testEmbedMcpType = "test-embedmcp"
)

func handleTestEmbedMCPServer(c *gin.Context, s mcpservers.Server) {
	// Store the session
	store := getStore()
	newSession := store.New()

	newEndpoint := sseEndpoint.NewEndpoint(newSession)
	server := mcpproxy.NewSSEServer(
		s,
		mcpproxy.WithMessageEndpoint(newEndpoint),
	)

	store.Set(newSession, testEmbedMcpType)
	defer func() {
		store.Delete(newSession)
	}()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Start message processing goroutine
	go processMCPSSEMpscMessages(ctx, cancel, newSession, server)

	// Handle SSE connection
	server.ServeHTTP(c.Writer, c.Request)
}

// TestEmbedMCPStreamable godoc
//
//	@Summary		Test Embed MCP Streamable Server
//	@Description	Test Embed MCP Streamable Server with various HTTP methods
//	@Tags			embedmcp
//	@Security		ApiKeyAuth
//	@Param			id				path	string	true	"MCP ID"
//	@Param			config[key]		query	string	false	"Initial configuration parameters (e.g. config[host]=http://localhost:3000)"
//	@Param			reusing[key]	query	string	false	"Reusing configuration parameters (e.g., reusing[authorization]=apikey)"
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	nil
//	@Failure		400	{object}	nil
//	@Router			/api/test-embedmcp/{id} [get]
//	@Router			/api/test-embedmcp/{id} [post]
//	@Router			/api/test-embedmcp/{id} [delete]
func TestEmbedMCPStreamable(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"mcp id is required",
		))

		return
	}

	initConfig, reusingConfig := getConfigFromQuery(c)

	server, err := mcpservers.GetMCPServer(id, initConfig, reusingConfig)
	if err != nil {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))

		return
	}

	handleStreamableMCPServer(c, server)
}
