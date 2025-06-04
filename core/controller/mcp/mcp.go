package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/mcpproxy"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/labring/aiproxy/openapi-mcp/convert"
	"github.com/mark3labs/mcp-go/mcp"
)

type EndpointProvider interface {
	NewEndpoint(newSession string) (newEndpoint string)
	LoadEndpoint(endpoint string) (session string)
}

// handleSSEMCPServer handles the SSE connection for an MCP server
func handleSSEMCPServer(
	c *gin.Context,
	s mcpservers.Server,
	mcpType string,
	endpoint EndpointProvider,
) {
	// Store the session
	store := getStore()
	newSession := store.New()

	newEndpoint := endpoint.NewEndpoint(newSession)
	server := mcpproxy.NewSSEServer(
		s,
		mcpproxy.WithMessageEndpoint(newEndpoint),
	)

	store.Set(newSession, mcpType)
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

// processMCPSSEMpscMessages handles message processing for OpenAPI
func processMCPSSEMpscMessages(
	ctx context.Context,
	sessionID string,
	server *mcpproxy.SSEServer,
) {
	mpscInstance := getMCPMpsc()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			data, err := mpscInstance.recv(ctx, sessionID)
			if err != nil {
				return
			}
			if err := server.HandleMessage(ctx, data); err != nil {
				continue
			}
		}
	}
}

func handleEmbedSSEMCP(
	c *gin.Context,
	mcpID string,
	config *model.MCPEmbeddingConfig,
	endpoint EndpointProvider,
) {
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
	handleSSEMCPServer(c, server, string(model.PublicMCPTypeEmbed), endpoint)
}

func sendMCPSSEMessage(c *gin.Context, mcpType, sessionID string) {
	backend, ok := getStore().Get(sessionID)
	if !ok || backend != mcpType {
		http.Error(c.Writer, "invalid session", http.StatusBadRequest)
		return
	}
	mpscInstance := getMCPMpsc()
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
	err = mpscInstance.send(c.Request.Context(), sessionID, body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Writer.WriteHeader(http.StatusAccepted)
}

// handleStreamableMCPServer handles the streamable connection for an MCP server
func handleStreamableMCPServer(c *gin.Context, s mcpservers.Server) {
	if c.Request.Method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.METHOD_NOT_FOUND,
			"method not allowed",
		))
		return
	}
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.PARSE_ERROR,
			err.Error(),
		))
		return
	}
	respMessage := s.HandleMessage(c.Request.Context(), reqBody)
	if respMessage == nil {
		// For notifications, just send 202 Accepted with no body
		c.Status(http.StatusAccepted)
		return
	}
	c.JSON(http.StatusOK, respMessage)
}

func handleGroupSSEMessage(c *gin.Context, mcpType model.GroupMCPType, sessionID string) {
	switch mcpType {
	case model.GroupMCPTypeProxySSE:
		sendMCPSSEMessage(c, string(mcpType), sessionID)
	case model.GroupMCPTypeProxyStreamable:
		sendMCPSSEMessage(c, string(mcpType), sessionID)
	case model.GroupMCPTypeOpenAPI:
		sendMCPSSEMessage(c, string(mcpType), sessionID)
	default:
		http.Error(c.Writer, "unknown mcp type", http.StatusBadRequest)
	}
}

func handleGroupSSEStreamable(c *gin.Context, groupMcp *model.GroupMCPCache) {
	switch groupMcp.Type {
	case model.GroupMCPTypeProxyStreamable:
		handleGroupProxyStreamable(c, groupMcp.ProxyConfig)
	case model.GroupMCPTypeOpenAPI:
		server, err := newOpenAPIMCPServer(groupMcp.OpenAPIConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))
			return
		}
		handleStreamableMCPServer(c, server)
	default:
		c.JSON(http.StatusBadRequest, mcpservers.CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unsupported mcp type",
		))
	}
}

// newOpenAPIMCPServer creates a new MCP server from OpenAPI configuration
func newOpenAPIMCPServer(config *model.MCPOpenAPIConfig) (mcpservers.Server, error) {
	if config == nil || (config.OpenAPISpec == "" && config.OpenAPIContent == "") {
		return nil, errors.New("invalid OpenAPI configuration")
	}

	// Parse OpenAPI specification
	parser := convert.NewParser()
	var err error
	var openAPIFrom string

	if config.OpenAPISpec != "" {
		openAPIFrom, err = parseOpenAPIFromURL(config, parser)
	} else {
		err = parseOpenAPIFromContent(config, parser)
	}

	if err != nil {
		return nil, err
	}

	// Convert to MCP server
	converter := convert.NewConverter(parser, convert.Options{
		OpenAPIFrom:   openAPIFrom,
		ServerAddr:    config.ServerAddr,
		Authorization: config.Authorization,
	})
	s, err := converter.Convert()
	if err != nil {
		return nil, err
	}

	return s, nil
}

// parseOpenAPIFromURL parses OpenAPI spec from a URL
func parseOpenAPIFromURL(config *model.MCPOpenAPIConfig, parser *convert.Parser) (string, error) {
	spec, err := url.Parse(config.OpenAPISpec)
	if err != nil || (spec.Scheme != "http" && spec.Scheme != "https") {
		return "", errors.New("invalid OpenAPI spec URL")
	}

	openAPIFrom := spec.String()
	if config.V2 {
		err = parser.ParseFileV2(openAPIFrom)
	} else {
		err = parser.ParseFile(openAPIFrom)
	}

	return openAPIFrom, err
}

// parseOpenAPIFromContent parses OpenAPI spec from content string
func parseOpenAPIFromContent(config *model.MCPOpenAPIConfig, parser *convert.Parser) error {
	if config.V2 {
		return parser.ParseV2([]byte(config.OpenAPIContent))
	}
	return parser.Parse([]byte(config.OpenAPIContent))
}
