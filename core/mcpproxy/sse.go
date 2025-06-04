package mcpproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
)

// SSEServer implements a Server-Sent Events (SSE) based MCP server.
// It provides real-time communication capabilities over HTTP using the SSE protocol.
type SSEServer struct {
	server          mcpservers.Server
	messageEndpoint string
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
func NewSSEServer(server mcpservers.Server, opts ...SSEOption) *SSEServer {
	s := &SSEServer{
		server:            server,
		messageEndpoint:   "/message",
		keepAlive:         false,
		keepAliveInterval: 30 * time.Second,
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
func (s *SSEServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			id := 0
			for {
				id++
				select {
				case <-ticker.C:
					message := mcp.JSONRPCRequest{
						JSONRPC: "2.0",
						ID:      mcp.NewRequestId(id),
						Request: mcp.Request{
							Method: "ping",
						},
					}
					messageBytes, _ := sonic.Marshal(message)
					pingMsg := fmt.Sprintf("event: message\ndata:%s\n\n", messageBytes)
					select {
					case s.eventQueue <- pingMsg:
					case <-r.Context().Done():
						return
					}
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
func (s *SSEServer) HandleMessage(ctx context.Context, req []byte) error {
	// Process message through MCPServer
	response := s.server.HandleMessage(ctx, req)

	// Only send response if there is one (not for notifications)
	if response != nil {
		var message string
		eventData, err := sonic.Marshal(response)
		if err != nil {
			message = "event: message\ndata: {\"error\": \"internal error\",\"jsonrpc\": \"2.0\", \"id\": null}\n\n"
		} else {
			message = fmt.Sprintf("event: message\ndata: %s\n\n", eventData)
		}

		// Queue the event for sending via SSE
		select {
		case s.eventQueue <- message:
			// Event queued successfully
		default:
			// Queue is full
			return errors.New("event queue is full")
		}
	}

	return nil
}
