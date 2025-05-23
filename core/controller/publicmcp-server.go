package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/mcpproxy"
	statelessmcp "github.com/labring/aiproxy/core/common/stateless-mcp"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/openapi-mcp/convert"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/redis/go-redis/v9"
)

// publicMcpEndpointProvider implements the EndpointProvider interface for MCP
type publicMcpEndpointProvider struct {
	key string
	t   model.PublicMCPType
}

func newPublicMcpEndpoint(key string, t model.PublicMCPType) mcpproxy.EndpointProvider {
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

// Global variables for session management
var (
	memStore       mcpproxy.SessionManager = mcpproxy.NewMemStore()
	redisStore     mcpproxy.SessionManager
	redisStoreOnce = &sync.Once{}
)

func getStore() mcpproxy.SessionManager {
	if common.RedisEnabled {
		redisStoreOnce.Do(func() {
			redisStore = newRedisStoreManager(common.RDB)
		})
		return redisStore
	}
	return memStore
}

// Redis-based session manager
type redisStoreManager struct {
	rdb *redis.Client
}

func newRedisStoreManager(rdb *redis.Client) mcpproxy.SessionManager {
	return &redisStoreManager{
		rdb: rdb,
	}
}

var redisStoreManagerScript = redis.NewScript(`
local key = KEYS[1]
local value = redis.call('GET', key)
if not value then
	return nil
end
redis.call('EXPIRE', key, 300)
return value
`)

func (r *redisStoreManager) New() string {
	return common.ShortUUID()
}

func (r *redisStoreManager) Get(sessionID string) (string, bool) {
	ctx := context.Background()

	result, err := redisStoreManagerScript.Run(ctx, r.rdb, []string{"mcp:session:" + sessionID}).Result()
	if err != nil || result == nil {
		return "", false
	}

	return result.(string), true
}

func (r *redisStoreManager) Set(sessionID, endpoint string) {
	ctx := context.Background()
	r.rdb.Set(ctx, "mcp:session:"+sessionID, endpoint, time.Minute*5)
}

func (r *redisStoreManager) Delete(session string) {
	ctx := context.Background()
	r.rdb.Del(ctx, "mcp:session:"+session)
}

// PublicMCPSseServer godoc
//
//	@Summary	Public MCP SSE Server
//	@Router		/mcp/public/{id}/sse [get]
func PublicMCPSseServer(c *gin.Context) {
	mcpID := c.Param("id")

	publicMcp, err := model.GetPublicMCPByID(mcpID)
	if err != nil {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))
		return
	}

	switch publicMcp.Type {
	case model.PublicMCPTypeProxySSE:
		handlePublicProxySSE(c, publicMcp.ID, publicMcp.ProxyConfig)
	case model.PublicMCPTypeOpenAPI:
		server, err := newOpenAPIMCPServer(publicMcp.OpenAPIConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))
			return
		}
		handleSSEMCPServer(c, server, model.PublicMCPTypeOpenAPI)
	default:
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unknown mcp type",
		))
		return
	}
}

// handlePublicProxySSE processes SSE proxy requests
func handlePublicProxySSE(c *gin.Context, mcpID string, config *model.PublicMCPProxyConfig) {
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
	group := middleware.GetGroup(c)
	token := middleware.GetToken(c)

	// Process reusing parameters if any
	if err := processReusingParams(config.ReusingParams, mcpID, group.ID, headers, backendQuery); err != nil {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
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
	mcpproxy.SSEHandler(
		c.Writer,
		c.Request,
		getStore(),
		newPublicMcpEndpoint(token.Key, model.PublicMCPTypeProxySSE),
		backendURL.String(),
		headers,
	)
}

// newOpenAPIMCPServer creates a new MCP server from OpenAPI configuration
func newOpenAPIMCPServer(config *model.MCPOpenAPIConfig) (*server.MCPServer, error) {
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

// handleSSEMCPServer handles the SSE connection for an MCP server
func handleSSEMCPServer(c *gin.Context, s *server.MCPServer, mcpType model.PublicMCPType) {
	token := middleware.GetToken(c)

	// Store the session
	store := getStore()
	newSession := store.New()

	newEndpoint := newPublicMcpEndpoint(token.Key, mcpType).NewEndpoint(newSession)
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

// processMCPSseMpscMessages handles message processing for OpenAPI
func processMCPSseMpscMessages(ctx context.Context, sessionID string, server *statelessmcp.SSEServer) {
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

// processReusingParams handles the reusing parameters for MCP proxy
func processReusingParams(reusingParams map[string]model.ReusingParam, mcpID string, groupID string, headers map[string]string, backendQuery *url.Values) error {
	if len(reusingParams) == 0 {
		return nil
	}

	param, err := model.GetGroupPublicMCPReusingParam(mcpID, groupID)
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
//	@Router		/mcp/public/message [post]
func PublicMCPMessage(c *gin.Context) {
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
	mcpType := model.PublicMCPType(mcpTypeStr)
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
	case model.PublicMCPTypeProxySSE:
		mcpproxy.SSEProxyHandler(
			c.Writer,
			c.Request,
			getStore(),
			newPublicMcpEndpoint(token.Key, mcpType),
		)
	case model.PublicMCPTypeOpenAPI:
		sendMCPSSEMessage(c, mcpTypeStr, sessionID)
	default:
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unknown mcp type",
		))
	}
}

func sendMCPSSEMessage(c *gin.Context, mcpType, sessionID string) {
	backend, ok := getStore().Get(sessionID)
	if !ok || backend != mcpType {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"invalid session",
		))
		return
	}
	mpscInstance := getMCPMpsc()
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INTERNAL_ERROR,
			err.Error(),
		))
		return
	}
	err = mpscInstance.send(c.Request.Context(), sessionID, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INTERNAL_ERROR,
			err.Error(),
		))
		return
	}
	c.Writer.WriteHeader(http.StatusAccepted)
}

// PublicMCPStreamable godoc
//
//	@Summary	Public MCP Streamable Server
//	@Router		/mcp/public/{id}/streamable [get]
//	@Router		/mcp/public/{id}/streamable [post]
//	@Router		/mcp/public/{id}/streamable [delete]
//
// TODO: batch and sse support
func PublicMCPStreamable(c *gin.Context) {
	mcpID := c.Param("id")
	publicMcp, err := model.GetPublicMCPByID(mcpID)
	if err != nil {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			err.Error(),
		))
		return
	}

	switch publicMcp.Type {
	case model.PublicMCPTypeProxyStreamable:
		handlePublicProxyStreamable(c, mcpID, publicMcp.ProxyConfig)
	case model.PublicMCPTypeOpenAPI:
		server, err := newOpenAPIMCPServer(publicMcp.OpenAPIConfig)
		if err != nil {
			c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
				mcp.NewRequestId(nil),
				mcp.INVALID_REQUEST,
				err.Error(),
			))
			return
		}
		handleStreamableMCPServer(c, server)
	default:
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"unknown mcp type",
		))
	}
}

// handlePublicProxyStreamable processes Streamable proxy requests
func handlePublicProxyStreamable(c *gin.Context, mcpID string, config *model.PublicMCPProxyConfig) {
	if config == nil || config.URL == "" {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
			mcp.NewRequestId(nil),
			mcp.INVALID_REQUEST,
			"invalid proxy configuration",
		))
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
	group := middleware.GetGroup(c)
	token := middleware.GetToken(c)

	// Process reusing parameters if any
	if err := processReusingParams(config.ReusingParams, mcpID, group.ID, headers, backendQuery); err != nil {
		c.JSON(http.StatusBadRequest, CreateMCPErrorResponse(
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
	mcpproxy.SSEHandler(
		c.Writer,
		c.Request,
		getStore(),
		newPublicMcpEndpoint(token.Key, model.PublicMCPTypeProxySSE),
		backendURL.String(),
		headers,
	)

	mcpproxy.NewStreamableProxy(backendURL.String(), headers, getStore()).
		ServeHTTP(c.Writer, c.Request)
}

// handleStreamableMCPServer handles the streamable connection for an MCP server
func handleStreamableMCPServer(c *gin.Context, s *server.MCPServer) {
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

// Interface for multi-producer, single-consumer message passing
type mpsc interface {
	recv(ctx context.Context, id string) ([]byte, error)
	send(ctx context.Context, id string, data []byte) error
}

// Global MPSC instances
var (
	memMCPMpsc       mpsc = newChannelMCPMpsc()
	redisMCPMpsc     mpsc
	redisMCPMpscOnce = &sync.Once{}
)

func getMCPMpsc() mpsc {
	if common.RedisEnabled {
		redisMCPMpscOnce.Do(func() {
			redisMCPMpsc = newRedisMCPMPSC(common.RDB)
		})
		return redisMCPMpsc
	}
	return memMCPMpsc
}

// In-memory channel-based MPSC implementation
type channelMCPMpsc struct {
	channels     map[string]chan []byte
	lastAccess   map[string]time.Time
	channelMutex sync.RWMutex
}

// newChannelMCPMpsc creates a new channel-based mpsc implementation
func newChannelMCPMpsc() *channelMCPMpsc {
	c := &channelMCPMpsc{
		channels:   make(map[string]chan []byte),
		lastAccess: make(map[string]time.Time),
	}

	// Start a goroutine to clean up expired channels
	go c.cleanupExpiredChannels()

	return c
}

// cleanupExpiredChannels periodically checks for and removes channels that haven't been accessed in 5 minutes
func (c *channelMCPMpsc) cleanupExpiredChannels() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.channelMutex.Lock()
		now := time.Now()
		for id, lastAccess := range c.lastAccess {
			if now.Sub(lastAccess) > 5*time.Minute {
				// Close and delete the channel
				if ch, exists := c.channels[id]; exists {
					close(ch)
					delete(c.channels, id)
				}
				delete(c.lastAccess, id)
			}
		}
		c.channelMutex.Unlock()
	}
}

// getOrCreateChannel gets an existing channel or creates a new one for the session
func (c *channelMCPMpsc) getOrCreateChannel(id string) chan []byte {
	c.channelMutex.RLock()
	ch, exists := c.channels[id]
	c.channelMutex.RUnlock()

	if !exists {
		c.channelMutex.Lock()
		if ch, exists = c.channels[id]; !exists {
			ch = make(chan []byte, 10)
			c.channels[id] = ch
		}
		c.lastAccess[id] = time.Now()
		c.channelMutex.Unlock()
	} else {
		c.channelMutex.Lock()
		c.lastAccess[id] = time.Now()
		c.channelMutex.Unlock()
	}

	return ch
}

// recv receives data for the specified session
func (c *channelMCPMpsc) recv(ctx context.Context, id string) ([]byte, error) {
	ch := c.getOrCreateChannel(id)

	select {
	case data, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("channel closed for session %s", id)
		}
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// send sends data to the specified session
func (c *channelMCPMpsc) send(ctx context.Context, id string, data []byte) error {
	ch := c.getOrCreateChannel(id)

	select {
	case ch <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("channel buffer full for session %s", id)
	}
}

// Redis-based MPSC implementation
type redisMCPMPSC struct {
	rdb *redis.Client
}

// newRedisMCPMPSC creates a new Redis MPSC instance
func newRedisMCPMPSC(rdb *redis.Client) *redisMCPMPSC {
	return &redisMCPMPSC{rdb: rdb}
}

func (r *redisMCPMPSC) send(ctx context.Context, id string, data []byte) error {
	// Set expiration to 5 minutes when sending data
	id = "mcp:mpsc:" + id
	pipe := r.rdb.Pipeline()
	pipe.LPush(ctx, id, data)
	pipe.Expire(ctx, id, 5*time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisMCPMPSC) recv(ctx context.Context, id string) ([]byte, error) {
	id = "mcp:mpsc:" + id
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			result, err := r.rdb.BRPop(ctx, time.Second, id).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					runtime.Gosched()
					continue
				}
				return nil, err
			}
			if len(result) != 2 {
				return nil, errors.New("invalid BRPop result")
			}
			return []byte(result[1]), nil
		}
	}
}

func CreateMCPErrorResponse(
	id mcp.RequestId,
	code int,
	message string,
) mcp.JSONRPCMessage {
	return mcp.JSONRPCError{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Error: struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
		},
	}
}
