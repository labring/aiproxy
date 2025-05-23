package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/mcpproxy"
	statelessmcp "github.com/labring/aiproxy/core/common/stateless-mcp"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type groupMcpEndpointProvider struct {
	key string
	t   model.GroupMCPType
}

func newGroupMcpEndpoint(key string, t model.GroupMCPType) mcpproxy.EndpointProvider {
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

// GroupMCPSseServer godoc
//
//	@Summary	Group MCP SSE Server
//	@Router		/mcp/group/{id}/sse [get]
func GroupMCPSseServer(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"mcp id is required",
		))
		return
	}

	group := middleware.GetGroup(c)

	groupMcp, err := model.GetGroupMCPByID(id, group.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))
		return
	}

	switch groupMcp.Type {
	case model.GroupMCPTypeProxySSE:
		handleGroupProxySSE(c, groupMcp.ProxyConfig)
	case model.GroupMCPTypeOpenAPI:
		server, err := newOpenAPIMCPServer(groupMcp.OpenAPIConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))
			return
		}
		handleGroupMCPServer(c, server, model.GroupMCPTypeOpenAPI)
	default:
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unsupported mcp type",
		))
	}
}

// handlePublicProxySSE processes SSE proxy requests
func handleGroupProxySSE(c *gin.Context, config *model.GroupMCPProxyConfig) {
	if config == nil || config.URL == "" {
		return
	}

	backendURL, err := url.Parse(config.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))
		return
	}

	headers := make(map[string]string)
	backendQuery := &url.Values{}
	token := middleware.GetToken(c)

	for k, v := range config.Headers {
		headers[k] = v
	}
	for k, v := range config.Querys {
		backendQuery.Set(k, v)
	}

	backendURL.RawQuery = backendQuery.Encode()
	mcpproxy.SSEHandler(
		c.Writer,
		c.Request,
		getStore(),
		newPublicMcpEndpoint(token.Key, model.PublicMCPTypeProxySSE),
		backendURL.String(),
		headers,
	)
}

// handleMCPServer handles the SSE connection for an MCP server
func handleGroupMCPServer(c *gin.Context, s *server.MCPServer, mcpType model.GroupMCPType) {
	token := middleware.GetToken(c)

	// Store the session
	store := getStore()
	newSession := store.New()

	newEndpoint := newGroupMcpEndpoint(token.Key, mcpType).NewEndpoint(newSession)
	server := statelessmcp.NewSSEServer(
		s,
		statelessmcp.WithMessageEndpoint(newEndpoint),
	)

	store.Set(newSession, string(mcpType))
	defer func() {
		store.Delete(newSession)
	}()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Start message processing goroutine
	go processMCPSseMpscMessages(ctx, newSession, server)

	// Handle SSE connection
	server.HandleSSE(c.Writer, c.Request)
}

// GroupMCPMessage godoc
//
//	@Summary	MCP SSE Proxy
//	@Router		/mcp/group/message [post]
func GroupMCPMessage(c *gin.Context) {
	token := middleware.GetToken(c)
	mcpTypeStr, _ := c.GetQuery("type")
	if mcpTypeStr == "" {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"missing mcp type",
		))
		return
	}
	mcpType := model.GroupMCPType(mcpTypeStr)
	sessionID, _ := c.GetQuery("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"missing sessionId",
		))
		return
	}

	switch mcpType {
	case model.GroupMCPTypeProxySSE:
		mcpproxy.SSEProxyHandler(
			c.Writer,
			c.Request,
			getStore(),
			newGroupMcpEndpoint(token.Key, mcpType),
		)
	default:
		sendMCPSSEMessage(c, mcpTypeStr, sessionID)
	}
}

// GroupMCPStreamable godoc
//
//	@Summary	Group MCP Streamable Server
//	@Router		/mcp/group/{id}/streamable [get]
//	@Router		/mcp/group/{id}/streamable [post]
//	@Router		/mcp/group/{id}/streamable [delete]
func GroupMCPStreamable(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"mcp id is required",
		))
		return
	}

	group := middleware.GetGroup(c)

	groupMcp, err := model.GetGroupMCPByID(id, group.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))
		return
	}

	switch groupMcp.Type {
	case model.GroupMCPTypeProxyStreamable:
		handleGroupProxyStreamable(c, groupMcp.ProxyConfig)
	case model.GroupMCPTypeOpenAPI:
		server, err := newOpenAPIMCPServer(groupMcp.OpenAPIConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))
			return
		}
		handleGroupStreamableMCPServer(c, server)
	default:
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unsupported mcp type",
		))
	}
}

// handleGroupProxyStreamable processes Streamable proxy requests for group
func handleGroupProxyStreamable(c *gin.Context, config *model.GroupMCPProxyConfig) {
	if config == nil || config.URL == "" {
		return
	}

	backendURL, err := url.Parse(config.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))
		return
	}

	headers := make(map[string]string)
	backendQuery := &url.Values{}

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

// handleGroupStreamableMCPServer handles the streamable connection for a group MCP server
func handleGroupStreamableMCPServer(c *gin.Context, s *server.MCPServer) {
	if c.Request.Method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.METHOD_NOT_FOUND,
			"method not allowed",
		))
		return
	}
	var rawMessage json.RawMessage
	if err := sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(&rawMessage); err != nil {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.PARSE_ERROR,
			err.Error(),
		))
		return
	}
	respMessage := s.HandleMessage(c.Request.Context(), rawMessage)
	c.JSON(http.StatusOK, respMessage)
}
