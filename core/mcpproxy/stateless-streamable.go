package mcpproxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
)

type StreamableHTTPOption func(*StreamableHTTPServer)

type StreamableHTTPServer struct {
	server mcpservers.Server
}

// NewStatelessStreamableHTTPServer creates a new streamable-http server instance
func NewStatelessStreamableHTTPServer(
	server mcpservers.Server,
	opts ...StreamableHTTPOption,
) *StreamableHTTPServer {
	s := &StreamableHTTPServer{
		server: server,
	}

	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServeHTTP implements the http.Handler interface.
func (s *StreamableHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		s.handleGet(w, r)
	case http.MethodDelete:
		s.handleDelete(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *StreamableHTTPServer) handlePost(w http.ResponseWriter, r *http.Request) {
	// post request carry request/notification message

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Invalid content type: must be 'application/json'", http.StatusBadRequest)
		return
	}

	// Check the request body is valid json, meanwhile, get the request Method
	rawData, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeJSONRPCError(
			w,
			nil,
			mcp.PARSE_ERROR,
			fmt.Sprintf("read request body error: %v", err),
		)
		return
	}
	var baseMessage struct {
		Method mcp.MCPMethod `json:"method"`
	}
	if err := json.Unmarshal(rawData, &baseMessage); err != nil {
		s.writeJSONRPCError(w, nil, mcp.PARSE_ERROR, "request body is not valid json")
		return
	}

	// Process message through MCPServer
	response := s.server.HandleMessage(r.Context(), rawData)
	if response == nil {
		// For notifications, just send 202 Accepted with no body
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = sonic.ConfigDefault.NewEncoder(w).Encode(response)
}

func (s *StreamableHTTPServer) handleGet(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "get request is not supported", http.StatusMethodNotAllowed)
}

func (s *StreamableHTTPServer) handleDelete(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "delete request is not supported", http.StatusMethodNotAllowed)
}

func (s *StreamableHTTPServer) writeJSONRPCError(
	w http.ResponseWriter,
	id any,
	code int,
	message string,
) {
	response := mcpservers.CreateMCPErrorResponse(id, code, message)
	jsonBody, err := sonic.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(jsonBody)))
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write(jsonBody)
}
