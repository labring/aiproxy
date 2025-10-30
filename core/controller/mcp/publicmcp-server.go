package controller

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/mcpproxy"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// PublicMCPSSEServer godoc
//
//	@Summary	Public MCP SSE Server
//	@Security	ApiKeyAuth
//	@Router		/mcp/public/{id}/sse [get]
func PublicMCPSSEServer(c *gin.Context) {
	mcpID := c.Param("id")
	if mcpID == "" {
		http.Error(c.Writer, "mcp id is required", http.StatusBadRequest)
		return
	}

	publicMcp, err := model.CacheGetPublicMCP(mcpID)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	if publicMcp.Status != model.PublicMCPStatusEnabled {
		http.Error(c.Writer, "mcp is not enabled", http.StatusBadRequest)
		return
	}

	group := middleware.GetGroup(c)
	paramsFunc := newGroupParams(publicMcp.ID, group.ID)

	handlePublicSSEMCP(c, publicMcp, paramsFunc, sseEndpoint)
}

func handlePublicSSEMCP(
	c *gin.Context,
	publicMcp *model.PublicMCPCache,
	paramsFunc ParamsFunc,
	endpoint EndpointProvider,
) {
	switch publicMcp.Type {
	case model.PublicMCPTypeProxySSE:
		if err := handlePublicProxySSE(c, publicMcp, paramsFunc, endpoint); err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
	case model.PublicMCPTypeProxyStreamable:
		if err := handlePublicProxyStreamableSSE(c, publicMcp, paramsFunc, endpoint); err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
	case model.PublicMCPTypeOpenAPI:
		server, err := newOpenAPIMCPServer(publicMcp.OpenAPIConfig)
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}

		handleSSEMCPServer(c, server, string(model.PublicMCPTypeOpenAPI), endpoint)
	case model.PublicMCPTypeEmbed:
		handleEmbedSSEMCP(c, publicMcp.ID, publicMcp.EmbedConfig, paramsFunc, endpoint)
	default:
		http.Error(c.Writer, "unknown mcp type", http.StatusBadRequest)
	}
}

// handlePublicProxySSE 处理公共代理SSE
func handlePublicProxySSE(
	c *gin.Context,
	publicMcp *model.PublicMCPCache,
	paramsFunc ParamsFunc,
	endpoint EndpointProvider,
) error {
	client, err := createProxySSEClient(c, publicMcp, paramsFunc)
	if err != nil {
		return err
	}
	defer client.Close()

	handleSSEMCPServer(
		c,
		mcpservers.WrapMCPClient2Server(client),
		string(model.PublicMCPTypeProxySSE),
		endpoint,
	)

	return nil
}

// handlePublicProxyStreamableSSE 处理公共代理Streamable SSE
func handlePublicProxyStreamableSSE(
	c *gin.Context,
	publicMcp *model.PublicMCPCache,
	paramsFunc ParamsFunc,
	endpoint EndpointProvider,
) error {
	client, err := createProxyStreamableClient(c, publicMcp, paramsFunc)
	if err != nil {
		return err
	}
	defer client.Close()

	handleSSEMCPServer(
		c,
		mcpservers.WrapMCPClient2Server(client),
		string(model.PublicMCPTypeProxyStreamable),
		endpoint,
	)

	return nil
}

// createProxySSEClient 创建代理SSE客户端
func createProxySSEClient(
	c *gin.Context,
	publicMcp *model.PublicMCPCache,
	paramsFunc ParamsFunc,
) (transport.Interface, error) {
	url, headers, err := prepareProxyConfig(publicMcp, paramsFunc)
	if err != nil {
		return nil, err
	}

	client, err := transport.NewSSE(url, transport.WithHeaders(headers))
	if err != nil {
		return nil, err
	}

	if err := client.Start(c.Request.Context()); err != nil {
		return nil, err
	}

	return client, nil
}

// createProxyStreamableClient 创建代理Streamable客户端
func createProxyStreamableClient(
	c *gin.Context,
	publicMcp *model.PublicMCPCache,
	paramsFunc ParamsFunc,
) (transport.Interface, error) {
	url, headers, err := prepareProxyConfig(publicMcp, paramsFunc)
	if err != nil {
		return nil, err
	}

	client, err := transport.NewStreamableHTTP(url, transport.WithHTTPHeaders(headers))
	if err != nil {
		return nil, err
	}

	if err := client.Start(c.Request.Context()); err != nil {
		return nil, err
	}

	return client, nil
}

// prepareProxyConfig 准备代理配置
func prepareProxyConfig(
	publicMcp *model.PublicMCPCache,
	paramsFunc ParamsFunc,
) (string, map[string]string, error) {
	url, err := url.Parse(publicMcp.ProxyConfig.URL)
	if err != nil {
		return "", nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	headers := make(map[string]string)
	backendQuery := url.Query()

	if len(publicMcp.ProxyConfig.Reusing) > 0 {
		processor := NewReusingParamProcessor(publicMcp.ID, paramsFunc)

		if err := processor.ProcessProxyReusingParams(
			publicMcp.ProxyConfig.Reusing,
			headers,
			&backendQuery,
		); err != nil {
			return "", nil, err
		}
	}

	maps.Copy(headers, publicMcp.ProxyConfig.Headers)

	url.RawQuery = backendQuery.Encode()

	return url.String(), headers, nil
}

// processProxyReusingParams handles the reusing parameters for MCP proxy
func processProxyReusingParams(
	reusingParams map[string]model.PublicMCPProxyReusingParam,
	paramsFunc ParamsFunc,
	headers map[string]string,
	backendQuery *url.Values,
) error {
	if len(reusingParams) == 0 {
		return nil
	}

	params, err := paramsFunc.GetParams()
	if err != nil {
		return err
	}

	for k, v := range reusingParams {
		paramValue, ok := params[k]
		if !ok {
			if v.Required {
				return fmt.Errorf("required reusing parameter %s is missing", k)
			}
			continue
		}

		switch v.Type {
		case model.ParamTypeHeader:
			headers[k] = paramValue
		case model.ParamTypeQuery:
			backendQuery.Set(k, paramValue)
		case model.ParamTypeURL:
			return fmt.Errorf("URL parameter %s cannot be set via reusing", k)
		default:
			return fmt.Errorf("unknown param type: %s", v.Type)
		}
	}

	return nil
}

// PublicMCPStreamable godoc
//
//	@Summary	Public MCP Streamable Server
//	@Security	ApiKeyAuth
//	@Router		/mcp/public/{id} [get]
//	@Router		/mcp/public/{id} [post]
//	@Router		/mcp/public/{id} [delete]
func PublicMCPStreamable(c *gin.Context) {
	mcpID := c.Param("id")

	publicMcp, err := model.CacheGetPublicMCP(mcpID)
	if err != nil {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))

		return
	}

	if publicMcp.Status != model.PublicMCPStatusEnabled {
		c.JSON(http.StatusNotFound, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"mcp is not enabled",
		))

		return
	}

	group := middleware.GetGroup(c)
	paramsFunc := newGroupParams(publicMcp.ID, group.ID)

	handlePublicStreamable(c, publicMcp, paramsFunc)
}

func handlePublicStreamable(
	c *gin.Context,
	publicMcp *model.PublicMCPCache,
	paramsFunc ParamsFunc,
) {
	switch publicMcp.Type {
	case model.PublicMCPTypeProxySSE:
		client, err := createProxySSEClient(c, publicMcp, paramsFunc)
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		defer client.Close()

		mcpproxy.NewStatelessStreamableHTTPServer(
			mcpservers.WrapMCPClient2Server(client),
		).ServeHTTP(c.Writer, c.Request)
	case model.PublicMCPTypeProxyStreamable:
		handlePublicProxyStreamable(c, paramsFunc, publicMcp.ProxyConfig)
	case model.PublicMCPTypeOpenAPI:
		server, err := newOpenAPIMCPServer(publicMcp.OpenAPIConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))

			return
		}

		handleStreamableMCPServer(c, server)
	case model.PublicMCPTypeEmbed:
		handlePublicEmbedStreamable(c, publicMcp.ID, paramsFunc, publicMcp.EmbedConfig)
	default:
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unknown mcp type",
		))
	}
}

func handlePublicEmbedStreamable(
	c *gin.Context,
	mcpID string,
	paramsFunc ParamsFunc,
	config *model.MCPEmbeddingConfig,
) {
	var reusingConfig map[string]string
	if len(config.Reusing) != 0 {
		params, err := paramsFunc.GetParams()
		if err != nil {
			c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))

			return
		}

		reusingConfig = params
	}

	server, err := mcpservers.GetMCPServer(mcpID, config.Init, reusingConfig)
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

// handlePublicProxyStreamable processes Streamable proxy requests
func handlePublicProxyStreamable(
	c *gin.Context,
	paramsFunc ParamsFunc,
	config *model.PublicMCPProxyConfig,
) {
	if config == nil || config.URL == "" {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"invalid proxy configuration",
		))

		return
	}

	backendURL, err := url.Parse(config.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))

		return
	}

	headers := make(map[string]string)
	backendQuery := backendURL.Query()

	// Process reusing parameters if any
	if err := processProxyReusingParams(config.Reusing, paramsFunc, headers, &backendQuery); err != nil {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))

		return
	}

	maps.Copy(headers, config.Headers)

	for k, v := range config.Querys {
		backendQuery.Set(k, v)
	}

	backendURL.RawQuery = backendQuery.Encode()
	mcpproxy.NewStreamableProxy(backendURL.String(), headers, getStore()).
		ServeHTTP(c.Writer, c.Request)
}

// TestPublicMCPSSEServer godoc
//
//	@Summary	Test Public MCP SSE Server
//	@Security	ApiKeyAuth
//	@Param		group	path	string	true	"Group ID"
//	@Param		id		path	string	true	"MCP ID"
//	@Router		/api/test-publicmcp/{group}/{id}/sse [get]
func TestPublicMCPSSEServer(c *gin.Context) {
	mcpID := c.Param("id")
	if mcpID == "" {
		http.Error(c.Writer, "mcp id is required", http.StatusBadRequest)
		return
	}

	groupID := c.Param("group")
	if groupID == "" {
		http.Error(c.Writer, "group id is required", http.StatusBadRequest)
		return
	}

	publicMcp, err := model.CacheGetPublicMCP(mcpID)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	if publicMcp.Status != model.PublicMCPStatusEnabled {
		http.Error(c.Writer, "mcp is not enabled", http.StatusBadRequest)
		return
	}

	paramsFunc := newGroupParams(publicMcp.ID, groupID)

	handlePublicSSEMCP(c, publicMcp, paramsFunc, sseEndpoint)
}
