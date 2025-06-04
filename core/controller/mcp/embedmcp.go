package controller

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
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

type EmbedMCP struct {
	ID              string                  `json:"id"`
	Enabled         bool                    `json:"enabled"`
	Name            string                  `json:"name"`
	Readme          string                  `json:"readme"`
	Tags            []string                `json:"tags"`
	ConfigTemplates EmbedMCPConfigTemplates `json:"config_templates"`
}

func newEmbedMCP(mcp *mcpservers.McpServer, enabled bool) *EmbedMCP {
	emcp := &EmbedMCP{
		ID:              mcp.ID,
		Enabled:         enabled,
		Name:            mcp.Name,
		Readme:          mcp.Readme,
		Tags:            mcp.Tags,
		ConfigTemplates: newEmbedMCPConfigTemplates(mcp.ConfigTemplates),
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
	enabledMCPs, err := model.GetPublicMCPsEnabled(slices.Collect(maps.Keys(embeds)))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	emcps := make([]*EmbedMCP, 0, len(embeds))
	for _, mcp := range embeds {
		emcps = append(emcps, newEmbedMCP(&mcp, slices.Contains(enabledMCPs, mcp.ID)))
	}

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
	reusingConfig := make(map[string]model.MCPEmbeddingReusingConfig)
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
			reusingConfig[key] = model.MCPEmbeddingReusingConfig{
				Name:        value.Name,
				Description: value.Description,
				Required:    true,
			}
		case mcpservers.ConfigRequiredTypeInitOrReusingOnly:
			if v, ok := initConfig[key]; ok {
				if v == "" {
					return nil, fmt.Errorf("config %s is required", key)
				}
				continue
			}
			reusingConfig[key] = model.MCPEmbeddingReusingConfig{
				Name:        value.Name,
				Description: value.Description,
				Required:    true,
			}
		}
	}
	embedConfig.Reusing = reusingConfig
	return embedConfig, nil
}

func ToPublicMCP(
	e mcpservers.McpServer,
	initConfig map[string]string,
	enabled bool,
) (*model.PublicMCP, error) {
	embedConfig, err := GetEmbedConfig(e.ConfigTemplates, initConfig)
	if err != nil {
		return nil, err
	}
	pmcp := &model.PublicMCP{
		ID:          e.ID,
		Name:        e.Name,
		LogoURL:     e.LogoURL,
		Readme:      e.Readme,
		Tags:        e.Tags,
		EmbedConfig: embedConfig,
	}
	if enabled {
		pmcp.Status = model.PublicMCPStatusEnabled
	} else {
		pmcp.Status = model.PublicMCPStatusDisabled
	}
	switch e.Type {
	case mcpservers.McpTypeEmbed:
		pmcp.Type = model.PublicMCPTypeEmbed
	case mcpservers.McpTypeDocs:
		pmcp.Type = model.PublicMCPTypeDocs
	}
	return pmcp, nil
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

type testEmbedMcpEndpointProvider struct {
	key string
}

func newTestEmbedMcpEndpoint(key string) EndpointProvider {
	return &testEmbedMcpEndpointProvider{
		key: key,
	}
}

func (m *testEmbedMcpEndpointProvider) NewEndpoint(session string) (newEndpoint string) {
	endpoint := fmt.Sprintf("/api/test-embedmcp/message?sessionId=%s&key=%s", session, m.key)
	return endpoint
}

func (m *testEmbedMcpEndpointProvider) LoadEndpoint(endpoint string) (session string) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return parsedURL.Query().Get("sessionId")
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
	token := middleware.GetToken(c)

	// Store the session
	store := getStore()
	newSession := store.New()

	newEndpoint := newTestEmbedMcpEndpoint(token.Key).NewEndpoint(newSession)
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
	go processMCPSSEMpscMessages(ctx, newSession, server)

	// Handle SSE connection
	server.ServeHTTP(c.Writer, c.Request)
}

// TestEmbedMCPMessage godoc
//
//	@Summary		Test Embed MCP Message
//	@Description	Send a message to the test embed MCP server
//	@Tags			embedmcp
//	@Security		ApiKeyAuth
//	@Param			sessionId	query	string	true	"Session ID"
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	nil
//	@Failure		400	{object}	nil
//	@Router			/api/test-embedmcp/message [post]
func TestEmbedMCPMessage(c *gin.Context) {
	sessionID, _ := c.GetQuery("sessionId")
	if sessionID == "" {
		http.Error(c.Writer, "missing sessionId", http.StatusBadRequest)
		return
	}

	sendMCPSSEMessage(c, testEmbedMcpType, sessionID)
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
