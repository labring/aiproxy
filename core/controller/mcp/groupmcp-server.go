package controller

import (
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

type groupMcpEndpointProvider struct {
	key string
	t   model.GroupMCPType
}

func newGroupMcpEndpoint(key string, t model.GroupMCPType) EndpointProvider {
	return &groupMcpEndpointProvider{
		key: key,
		t:   t,
	}
}

func (m *groupMcpEndpointProvider) NewEndpoint(session string) (newEndpoint string) {
	endpoint := fmt.Sprintf("/mcp/group/message?sessionId=%s&key=%s&type=%s", session, m.key, m.t)
	return endpoint
}

func (m *groupMcpEndpointProvider) LoadEndpoint(endpoint string) (session string) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return parsedURL.Query().Get("sessionId")
}

// GroupMCPSSEServer godoc
//
//	@Summary	Group MCP SSE Server
//	@Security	ApiKeyAuth
//	@Router		/mcp/group/{id}/sse [get]
func GroupMCPSSEServer(c *gin.Context) {
	mcpID := c.Param("id")
	if mcpID == "" {
		http.Error(c.Writer, "mcp id is required", http.StatusBadRequest)
		return
	}

	group := middleware.GetGroup(c)

	groupMcp, err := model.CacheGetGroupMCP(group.ID, mcpID)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusNotFound)
		return
	}
	if groupMcp.Status != model.GroupMCPStatusEnabled {
		http.Error(c.Writer, "mcp is not enabled", http.StatusNotFound)
		return
	}

	token := middleware.GetToken(c)
	endpoint := newGroupMcpEndpoint(token.Key, groupMcp.Type)

	handleGroupSSEMCPServer(c, groupMcp, endpoint)
}

func handleGroupSSEMCPServer(
	c *gin.Context,
	groupMcp *model.GroupMCPCache,
	endpoint EndpointProvider,
) {
	switch groupMcp.Type {
	case model.GroupMCPTypeProxySSE:
		client, err := transport.NewSSE(
			groupMcp.ProxyConfig.URL,
			transport.WithHeaders(groupMcp.ProxyConfig.Headers),
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
		handleSSEMCPServer(c,
			mcpservers.WrapMCPClient2Server(client),
			string(model.GroupMCPTypeProxySSE),
			endpoint,
		)
	case model.GroupMCPTypeProxyStreamable:
		client, err := transport.NewStreamableHTTP(
			groupMcp.ProxyConfig.URL,
			transport.WithHTTPHeaders(groupMcp.ProxyConfig.Headers),
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
			string(model.GroupMCPTypeProxyStreamable),
			endpoint,
		)
	case model.GroupMCPTypeOpenAPI:
		server, err := newOpenAPIMCPServer(groupMcp.OpenAPIConfig)
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		handleSSEMCPServer(c, server, string(model.GroupMCPTypeOpenAPI), endpoint)
	default:
		http.Error(c.Writer, "unsupported mcp type", http.StatusBadRequest)
	}
}

// GroupMCPMessage godoc
//
//	@Summary	MCP SSE Proxy
//	@Security	ApiKeyAuth
//	@Router		/mcp/group/message [post]
func GroupMCPMessage(c *gin.Context) {
	mcpTypeStr, _ := c.GetQuery("type")
	if mcpTypeStr == "" {
		http.Error(c.Writer, "missing mcp type", http.StatusBadRequest)
		return
	}
	mcpType := model.GroupMCPType(mcpTypeStr)

	sessionID, _ := c.GetQuery("sessionId")
	if sessionID == "" {
		http.Error(c.Writer, "missing sessionId", http.StatusBadRequest)
		return
	}

	handleGroupSSEMessage(c, mcpType, sessionID)
}

// GroupMCPStreamable godoc
//
//	@Summary	Group MCP Streamable Server
//	@Security	ApiKeyAuth
//	@Router		/mcp/group/{id} [get]
//	@Router		/mcp/group/{id} [post]
//	@Router		/mcp/group/{id} [delete]
func GroupMCPStreamable(c *gin.Context) {
	mcpID := c.Param("id")
	if mcpID == "" {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"mcp id is required",
		))
		return
	}

	group := middleware.GetGroup(c)

	groupMcp, err := model.CacheGetGroupMCP(group.ID, mcpID)
	if err != nil {
		c.JSON(http.StatusNotFound, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))
		return
	}
	if groupMcp.Status != model.GroupMCPStatusEnabled {
		c.JSON(http.StatusNotFound, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"mcp is not enabled",
		))
		return
	}

	handleGroupSSEStreamable(c, groupMcp)
}

// handleGroupProxyStreamable processes Streamable proxy requests for group
func handleGroupProxyStreamable(c *gin.Context, config *model.GroupMCPProxyConfig) {
	if config == nil || config.URL == "" {
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
