package controller

import (
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

	handleGroupSSEMCPServer(c, groupMcp, sseEndpoint)
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

	handleGroupStreamable(c, groupMcp)
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

	headers := maps.Clone(config.Headers)
	backendQuery := backendURL.Query()

	for k, v := range config.Querys {
		backendQuery.Set(k, v)
	}

	backendURL.RawQuery = backendQuery.Encode()
	mcpproxy.NewStreamableProxy(backendURL.String(), headers, getStore()).
		ServeHTTP(c.Writer, c.Request)
}
