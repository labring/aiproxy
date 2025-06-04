package controller

import (
	"errors"
	"fmt"
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

// publicMcpEndpointProvider implements the EndpointProvider interface for MCP
type publicMcpEndpointProvider struct {
	key string
	t   model.PublicMCPType
}

func newPublicMcpEndpoint(key string, t model.PublicMCPType) EndpointProvider {
	return &publicMcpEndpointProvider{
		key: key,
		t:   t,
	}
}

func (m *publicMcpEndpointProvider) NewEndpoint(session string) (newEndpoint string) {
	endpoint := fmt.Sprintf("/mcp/public/message?sessionId=%s&key=%s&type=%s", session, m.key, m.t)
	return endpoint
}

func (m *publicMcpEndpointProvider) LoadEndpoint(endpoint string) (session string) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return parsedURL.Query().Get("sessionId")
}

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

	token := middleware.GetToken(c)
	endpoint := newPublicMcpEndpoint(token.Key, publicMcp.Type)

	handlePublicSSEMCP(c, publicMcp, endpoint)
}

func handlePublicSSEMCP(
	c *gin.Context,
	publicMcp *model.PublicMCPCache,
	endpoint EndpointProvider,
) {
	switch publicMcp.Type {
	case model.PublicMCPTypeProxySSE:
		client, err := transport.NewSSE(
			publicMcp.ProxyConfig.URL,
			transport.WithHeaders(publicMcp.ProxyConfig.Headers),
		)
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		err = client.Start(c.Request.Context())
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		defer client.Close()
		handleSSEMCPServer(
			c,
			mcpservers.WrapMCPClient2Server(client),
			string(model.PublicMCPTypeProxySSE),
			endpoint,
		)
	case model.PublicMCPTypeProxyStreamable:
		client, err := transport.NewStreamableHTTP(
			publicMcp.ProxyConfig.URL,
			transport.WithHTTPHeaders(publicMcp.ProxyConfig.Headers),
		)
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		err = client.Start(c.Request.Context())
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		defer client.Close()
		handleSSEMCPServer(
			c,
			mcpservers.WrapMCPClient2Server(client),
			string(model.PublicMCPTypeProxyStreamable),
			endpoint,
		)
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
		handleSSEMCPServer(c, server, string(model.PublicMCPTypeOpenAPI), endpoint)
	case model.PublicMCPTypeEmbed:
		handleEmbedSSEMCP(c, publicMcp.ID, publicMcp.EmbedConfig, endpoint)
	default:
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unknown mcp type",
		))
		return
	}
}

// processReusingParams handles the reusing parameters for MCP proxy
func processReusingParams(
	reusingParams map[string]model.ReusingParam,
	mcpID, groupID string,
	headers map[string]string,
	backendQuery *url.Values,
) error {
	if len(reusingParams) == 0 {
		return nil
	}

	param, err := model.CacheGetPublicMCPReusingParam(mcpID, groupID)
	if err != nil {
		return err
	}

	for k, v := range reusingParams {
		paramValue, ok := param.ReusingParams[k]
		if !ok {
			if v.Required {
				return fmt.Errorf("%s required", k)
			}
			continue
		}

		switch v.Type {
		case model.ParamTypeHeader:
			headers[k] = paramValue
		case model.ParamTypeQuery:
			backendQuery.Set(k, paramValue)
		default:
			return errors.New("unknow param type")
		}
	}

	return nil
}

// PublicMCPMessage godoc
//
//	@Summary	Public MCP SSE Server
//	@Security	ApiKeyAuth
//	@Router		/mcp/public/message [post]
func PublicMCPMessage(c *gin.Context) {
	mcpTypeStr, _ := c.GetQuery("type")
	if mcpTypeStr == "" {
		http.Error(c.Writer, "missing mcp type", http.StatusBadRequest)
		return
	}
	mcpType := model.PublicMCPType(mcpTypeStr)
	sessionID, _ := c.GetQuery("sessionId")
	if sessionID == "" {
		http.Error(c.Writer, "missing sessionId", http.StatusBadRequest)
		return
	}

	handlePublicSSEMessage(c, mcpType, sessionID)
}

func handlePublicSSEMessage(c *gin.Context, mcpType model.PublicMCPType, sessionID string) {
	switch mcpType {
	case model.PublicMCPTypeProxySSE:
		sendMCPSSEMessage(c, string(mcpType), sessionID)
	case model.PublicMCPTypeProxyStreamable:
		sendMCPSSEMessage(c, string(mcpType), sessionID)
	case model.PublicMCPTypeOpenAPI:
		sendMCPSSEMessage(c, string(mcpType), sessionID)
	case model.PublicMCPTypeEmbed:
		sendMCPSSEMessage(c, string(mcpType), sessionID)
	default:
		http.Error(c.Writer, "unknown mcp type", http.StatusBadRequest)
	}
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

	handlePublicSSEStreamable(c, publicMcp)
}

func handlePublicSSEStreamable(c *gin.Context, publicMcp *model.PublicMCPCache) {
	switch publicMcp.Type {
	case model.PublicMCPTypeProxySSE:
		client, err := transport.NewSSE(
			publicMcp.ProxyConfig.URL,
			transport.WithHeaders(publicMcp.ProxyConfig.Headers),
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))
			return
		}
		err = client.Start(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))
			return
		}
		defer client.Close()
		mcpproxy.NewStatelessStreamableHTTPServer(
			mcpservers.WrapMCPClient2Server(client),
		).ServeHTTP(c.Writer, c.Request)
	case model.PublicMCPTypeProxyStreamable:
		handlePublicProxyStreamable(c, publicMcp.ID, publicMcp.ProxyConfig)
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
		handlePublicEmbedStreamable(c, publicMcp.ID, publicMcp.EmbedConfig)
	default:
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unknown mcp type",
		))
	}
}

func handlePublicEmbedStreamable(c *gin.Context, mcpID string, config *model.MCPEmbeddingConfig) {
	var reusingConfig map[string]string
	if len(config.Reusing) != 0 {
		group := middleware.GetGroup(c)
		param, err := model.CacheGetPublicMCPReusingParam(mcpID, group.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))
			return
		}
		reusingConfig = param.ReusingParams
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
func handlePublicProxyStreamable(c *gin.Context, mcpID string, config *model.PublicMCPProxyConfig) {
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
	group := middleware.GetGroup(c)

	// Process reusing parameters if any
	if err := processReusingParams(config.ReusingParams, mcpID, group.ID, headers, &backendQuery); err != nil {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))
		return
	}

	for k, v := range config.Headers {
		headers[k] = v
	}
	for k, v := range config.Querys {
		backendQuery.Set(k, v)
	}

	backendURL.RawQuery = backendQuery.Encode()
	mcpproxy.NewStreamableProxy(backendURL.String(), headers, getStore()).
		ServeHTTP(c.Writer, c.Request)
}
