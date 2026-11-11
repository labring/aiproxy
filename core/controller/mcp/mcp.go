package controller

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/mcpproxy"
	"github.com/labring/aiproxy/core/model"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/labring/aiproxy/openapi-mcp/convert"
	"github.com/mark3labs/mcp-go/mcp"
	log "github.com/sirupsen/logrus"
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
	go processMCPSSEMpscMessages(ctx, cancel, newSession, server)

	// Handle SSE connection
	server.ServeHTTP(c.Writer, c.Request)
}

// processMCPSSEMpscMessages handles message processing for OpenAPI
func processMCPSSEMpscMessages(
	ctx context.Context,
	cancel context.CancelFunc,
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
				// The response could not be queued on the SSE stream, so the
				// client would wait for it forever. Close the session so the
				// client reconnects instead of silently losing the response.
				log.Errorf("mcp sse session %s: closing session: %v", sessionID, err)
				cancel()

				return
			}
		}
	}
}

func handleEmbedSSEMCP(
	c *gin.Context,
	mcpID string,
	config *model.MCPEmbeddingConfig,
	paramsFunc ParamsFunc,
	endpoint EndpointProvider,
) {
	reusingConfig, err := prepareEmbedReusingConfig(mcpID, paramsFunc, config.Reusing)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	server, err := mcpservers.GetMCPServer(mcpID, config.Init, reusingConfig)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	handleSSEMCPServer(c, server, string(model.PublicMCPTypeEmbed), endpoint)
}

// prepareEmbedReusingConfig 准备嵌入MCP的reusing配置
func prepareEmbedReusingConfig(
	mcpID string,
	paramsFunc ParamsFunc,
	reusingParams map[string]model.ReusingParam,
) (map[string]string, error) {
	if len(reusingParams) == 0 {
		return nil, nil
	}

	return NewReusingParamProcessor(mcpID, paramsFunc).
		ProcessEmbedReusingParams(reusingParams)
}

func sendMCPSSEMessage(c *gin.Context, sessionID string) {
	_, ok := getStore().Get(sessionID)
	if !ok {
		http.Error(c.Writer, "invalid session", http.StatusBadRequest)
		return
	}

	mpscInstance := getMCPMpsc()

	body, err := common.GetRequestBody(c.Request)
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

	reqBody, err := common.GetRequestBody(c.Request)
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

func handleGroupStreamable(c *gin.Context, groupMcp *model.GroupMCPCache) {
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

	var (
		err         error
		openAPIFrom string
	)

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

// sseEndpointProvider implements the EndpointProvider interface for MCP
type sseEndpointProvider struct{}

var sseEndpoint = &sseEndpointProvider{}

func (m *sseEndpointProvider) NewEndpoint(session string) (newEndpoint string) {
	endpoint := "/message?sessionId=" + session
	return endpoint
}

func (m *sseEndpointProvider) LoadEndpoint(endpoint string) (session string) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}

	return parsedURL.Query().Get("sessionId")
}

// MCPMessage godoc
//
//	@Summary	MCP SSE Message
//	@Router		/message [post]
func MCPMessage(c *gin.Context) {
	sessionID, _ := c.GetQuery("sessionId")
	if sessionID == "" {
		http.Error(c.Writer, "missing sessionId", http.StatusBadRequest)
		return
	}

	sendMCPSSEMessage(c, sessionID)
}
