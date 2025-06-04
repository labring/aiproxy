package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
)

// hostMcpEndpointProvider implements the EndpointProvider interface for MCP
type hostMcpEndpointProvider struct {
	key string
	t   string
}

func newHostMcpEndpoint(key, t string) EndpointProvider {
	return &hostMcpEndpointProvider{
		key: key,
		t:   t,
	}
}

func (m *hostMcpEndpointProvider) NewEndpoint(session string) (newEndpoint string) {
	endpoint := fmt.Sprintf("/message?sessionId=%s&key=%s&type=%s", session, m.key, m.t)
	return endpoint
}

func (m *hostMcpEndpointProvider) LoadEndpoint(endpoint string) (session string) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return parsedURL.Query().Get("sessionId")
}

func routeHostMCP(
	c *gin.Context,
	publicHandler, groupHandler func(c *gin.Context, mcpID string),
) {
	log := middleware.GetLogger(c)
	host := c.Request.Host

	log.Debugf("route host mcp: %s", host)

	publicMCPHost := config.GetPublicMCPHost()
	groupMCPHost := config.GetGroupMCPHost()

	switch {
	case publicMCPHost != "" && strings.HasSuffix(host, publicMCPHost):
		mcpID := strings.TrimSuffix(host, "."+publicMCPHost)
		publicHandler(c, mcpID)
	case groupMCPHost != "" && strings.HasSuffix(host, groupMCPHost):
		mcpID := strings.TrimSuffix(host, "."+groupMCPHost)
		groupHandler(c, mcpID)
	default:
		http.Error(c.Writer, "invalid host", http.StatusNotFound)
	}
}

// HostMCPSSEServer godoc
//
//	@Summary	Public MCP SSE Server
//	@Security	ApiKeyAuth
//	@Router		/sse [get]
func HostMCPSSEServer(c *gin.Context) {
	routeHostMCP(c, func(c *gin.Context, mcpID string) {
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
		endpoint := newHostMcpEndpoint(token.Key, string(publicMcp.Type))

		handlePublicSSEMCP(c, publicMcp, endpoint)
	}, func(c *gin.Context, mcpID string) {
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
		endpoint := newHostMcpEndpoint(token.Key, string(groupMcp.Type))

		handleGroupSSEMCPServer(c, groupMcp, endpoint)
	})
}

// HostMCPMessage godoc
//
//	@Summary	Public MCP SSE Server
//	@Security	ApiKeyAuth
//	@Router		/message [post]
func HostMCPMessage(c *gin.Context) {
	routeHostMCP(c, func(c *gin.Context, _ string) {
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
	}, func(c *gin.Context, _ string) {
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
	})
}

// HostMCPStreamable godoc
//
//	@Summary	Host MCP Streamable Server
//	@Security	ApiKeyAuth
//	@Router		/mcp [get]
//	@Router		/mcp [post]
//	@Router		/mcp [delete]
func HostMCPStreamable(c *gin.Context) {
	routeHostMCP(c, func(c *gin.Context, mcpID string) {
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
	}, func(c *gin.Context, mcpID string) {
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
	})
}
