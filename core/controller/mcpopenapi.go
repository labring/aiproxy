package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SSEServer implements a Server-Sent Events (SSE) based MCP server.
// It provides real-time communication capabilities over HTTP using the SSE protocol.
type SSEServer struct {
	server          *server.MCPServer
	messageEndpoint string
	srv             *http.Server
	eventQueue      chan string

	keepAlive         bool
	keepAliveInterval time.Duration
}

// SSEOption defines a function type for configuring SSEServer
type SSEOption func(*SSEServer)

// WithMessageEndpoint sets the message endpoint path
func WithMessageEndpoint(endpoint string) SSEOption {
	return func(s *SSEServer) {
		s.messageEndpoint = endpoint
	}
}

func WithKeepAliveInterval(keepAliveInterval time.Duration) SSEOption {
	return func(s *SSEServer) {
		s.keepAlive = true
		s.keepAliveInterval = keepAliveInterval
	}
}

func WithKeepAlive(keepAlive bool) SSEOption {
	return func(s *SSEServer) {
		s.keepAlive = keepAlive
	}
}

// NewSSEServer creates a new SSE server instance with the given MCP server and options.
func NewSSEServer(server *server.MCPServer, opts ...SSEOption) *SSEServer {
	s := &SSEServer{
		server:            server,
		messageEndpoint:   "/message",
		keepAlive:         false,
		keepAliveInterval: 10 * time.Second,
		eventQueue:        make(chan string, 100),
	}

	// Apply all options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// handleSSE handles incoming SSE connection requests.
// It sets up appropriate headers and creates a new session for the client.
func (s *SSEServer) HandleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Start keep alive : ping
	if s.keepAlive {
		go func() {
			ticker := time.NewTicker(s.keepAliveInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					//: ping - 2025-03-27 07:44:38.682659+00:00
					s.eventQueue <- fmt.Sprintf(":ping - %s\n\n", time.Now().Format(time.RFC3339))
				case <-r.Context().Done():
					return
				}
			}
		}()
	}

	// Send the initial endpoint event
	fmt.Fprintf(w, "event: endpoint\ndata: %s\r\n\r\n", s.messageEndpoint)
	flusher.Flush()

	// Main event loop - this runs in the HTTP handler goroutine
	for {
		select {
		case event := <-s.eventQueue:
			// Write the event to the response
			fmt.Fprint(w, event)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleMessage processes incoming JSON-RPC messages from clients and sends responses
// back through both the SSE connection and HTTP response.
func (s *SSEServer) HandleMessage(req []byte) error {
	// Parse message as raw JSON
	var rawMessage json.RawMessage
	if err := json.Unmarshal([]byte(req), &rawMessage); err != nil {
		return errors.New("parse error")
	}

	// Process message through MCPServer
	response := s.server.HandleMessage(context.Background(), rawMessage)

	// Only send response if there is one (not for notifications)
	if response != nil {
		eventData, _ := json.Marshal(response)

		// Queue the event for sending via SSE
		select {
		case s.eventQueue <- fmt.Sprintf("event: message\ndata: %s\n\n", eventData):
			// Event queued successfully
		default:
			// Queue is full, could log this
		}
	}

	return nil
}

// writeJSONRPCError writes a JSON-RPC error response with the given error details.
func (s *SSEServer) writeJSONRPCError(
	w http.ResponseWriter,
	id interface{},
	code int,
	message string,
) {
	response := createErrorResponse(id, code, message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

func JSONRPCError(
	id interface{},
	code int,
	message string,
) ([]byte, error) {
	return json.Marshal(createErrorResponse(id, code, message))
}

func createErrorResponse(
	id interface{},
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
