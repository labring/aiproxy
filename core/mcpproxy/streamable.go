package mcpproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	headerKeySessionID = "Mcp-Session-Id"
)

// StreamableProxy represents a proxy for the MCP Streamable HTTP transport
type StreamableProxy struct {
	store   SessionManager
	backend string
	headers map[string]string
}

// NewStreamableProxy creates a new proxy for the Streamable HTTP transport
func NewStreamableProxy(
	backend string,
	headers map[string]string,
	store SessionManager,
) *StreamableProxy {
	return &StreamableProxy{
		store:   store,
		backend: backend,
		headers: headers,
	}
}

// ServeHTTP handles both GET and POST requests for the Streamable HTTP transport
func (p *StreamableProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id")
	w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		p.handleGetRequest(w, r)
	case http.MethodPost:
		p.handlePostRequest(w, r)
	case http.MethodDelete:
		p.handleDeleteRequest(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRequest handles GET requests for SSE streaming
func (p *StreamableProxy) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	// Check if Accept header includes text/event-stream
	acceptHeader := r.Header.Get("Accept")
	if !strings.Contains(acceptHeader, "text/event-stream") {
		http.Error(w, "Accept header must include text/event-stream", http.StatusBadRequest)
		return
	}

	// Get proxy session ID from header
	proxySessionID := r.Header.Get(headerKeySessionID)
	if proxySessionID == "" {
		// This might be an initialization request
		p.proxyInitialOrNoSessionRequest(w, r)
		return
	}

	// Look up the backend endpoint and session ID
	backendInfo, ok := p.store.Get(proxySessionID)
	if !ok {
		http.Error(w, "Invalid or expired session ID", http.StatusNotFound)
		return
	}

	// Create a request to the backend
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, backendInfo, nil)
	if err != nil {
		http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
		return
	}

	// Extract the real backend session ID from the stored URL
	parts := strings.Split(backendInfo, "?sessionId=")
	if len(parts) > 1 {
		req.Header.Set(headerKeySessionID, parts[1])
	}

	// Add any additional headers
	for name, value := range p.headers {
		req.Header.Set(name, value)
	}

	//nolint:bodyclose
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Failed to connect to backend", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check if we got an SSE response
	if resp.StatusCode != http.StatusOK ||
		!strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		// Add our proxy session ID
		w.Header().Set(headerKeySessionID, proxySessionID)

		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}

	// Set SSE headers for the client response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a context that cancels when the client disconnects
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Monitor client disconnection
	go func() {
		<-ctx.Done()
		resp.Body.Close()
	}()

	// Stream the SSE events to the client
	reader := bufio.NewReader(resp.Body)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return
		}

		// Write the line to the client
		fmt.Fprint(w, line)
		flusher.Flush()
	}
}

// handlePostRequest handles POST requests for JSON-RPC messages
func (p *StreamableProxy) handlePostRequest(w http.ResponseWriter, r *http.Request) {
	// Check if this is an initialization request
	proxySessionID := r.Header.Get(headerKeySessionID)
	if proxySessionID == "" {
		p.proxyInitialOrNoSessionRequest(w, r)
		return
	}

	// Look up the backend endpoint and session ID
	backendInfo, ok := p.store.Get(proxySessionID)
	if !ok {
		http.Error(w, "Invalid or expired session ID", http.StatusNotFound)
		return
	}

	// Create a request to the backend
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, backendInfo, r.Body)
	if err != nil {
		http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
		return
	}

	// Extract the real backend session ID from the stored URL
	parts := strings.Split(backendInfo, "?sessionId=")
	if len(parts) > 1 {
		req.Header.Set(headerKeySessionID, parts[1])
	}

	// Add any additional headers
	for name, value := range p.headers {
		req.Header.Set(name, value)
	}

	//nolint:bodyclose
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Failed to connect to backend", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Add our proxy session ID
	w.Header().Set(headerKeySessionID, proxySessionID)

	contentType := resp.Header.Get("Content-Type")

	w.Header().Set("Content-Type", contentType)

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Check if the response is an SSE stream
	if strings.Contains(contentType, "text/event-stream") {
		// Handle SSE response
		reader := bufio.NewReader(resp.Body)
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Create a context that cancels when the client disconnects
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Monitor client disconnection
		go func() {
			<-ctx.Done()
			resp.Body.Close()
		}()

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				return
			}

			// Write the line to the client
			_, _ = fmt.Fprint(w, line)
			flusher.Flush()
		}
	} else {
		// Copy regular response body
		_, _ = io.Copy(w, resp.Body)
	}
}

// handleDeleteRequest handles DELETE requests for session termination
func (p *StreamableProxy) handleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	// Get proxy session ID from header
	proxySessionID := r.Header.Get(headerKeySessionID)
	if proxySessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// Look up the backend endpoint and session ID
	backendInfo, ok := p.store.Get(proxySessionID)
	if !ok {
		http.Error(w, "Invalid or expired session ID", http.StatusNotFound)
		return
	}

	// Create a request to the backend
	req, err := http.NewRequestWithContext(r.Context(), http.MethodDelete, backendInfo, nil)
	if err != nil {
		http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
		return
	}

	// Extract the real backend session ID from the stored URL
	parts := strings.Split(backendInfo, "?sessionId=")
	if len(parts) > 1 {
		req.Header.Set(headerKeySessionID, parts[1])
	}

	// Add any additional headers
	for name, value := range p.headers {
		req.Header.Set(name, value)
	}

	// Make the request to the backend
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to connect to backend", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Remove the session from our store
	p.store.Delete(proxySessionID)

	contentType := resp.Header.Get("Content-Type")
	w.Header().Set("Content-Type", contentType)

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, _ = io.Copy(w, resp.Body)
}

// proxyInitialOrNoSessionRequest handles the initial request that doesn't have a session ID yet
func (p *StreamableProxy) proxyInitialOrNoSessionRequest(w http.ResponseWriter, r *http.Request) {
	// Create a request to the backend
	req, err := http.NewRequestWithContext(r.Context(), r.Method, p.backend, r.Body)
	if err != nil {
		http.Error(w, "Failed to create backend request", http.StatusInternalServerError)
		return
	}

	// Add any additional headers
	for name, value := range p.headers {
		req.Header.Set(name, value)
	}

	//nolint:bodyclose
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Failed to connect to backend", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check if we received a session ID from the backend
	backendSessionID := resp.Header.Get(headerKeySessionID)
	if backendSessionID != "" {
		// Generate a new proxy session ID
		proxySessionID := p.store.New()

		// Store the mapping between our proxy session ID and the backend endpoint with its session
		// ID
		backendURL := p.backend
		if strings.Contains(backendURL, "?") {
			backendURL += "&sessionId=" + backendSessionID
		} else {
			backendURL += "?sessionId=" + backendSessionID
		}
		p.store.Set(proxySessionID, backendURL)

		// Replace the backend session ID with our proxy session ID in the response
		w.Header().Set(headerKeySessionID, proxySessionID)
	}

	contentType := resp.Header.Get("Content-Type")

	w.Header().Set("Content-Type", contentType)

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Check if the response is an SSE stream
	if strings.Contains(contentType, "text/event-stream") {
		// Handle SSE response
		reader := bufio.NewReader(resp.Body)
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Create a context that cancels when the client disconnects
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Monitor client disconnection
		go func() {
			<-ctx.Done()
			resp.Body.Close()
		}()

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				return
			}

			// Write the line to the client
			fmt.Fprint(w, line)
			flusher.Flush()
		}
	} else {
		// Copy regular response body
		_, _ = io.Copy(w, resp.Body)
	}
}
