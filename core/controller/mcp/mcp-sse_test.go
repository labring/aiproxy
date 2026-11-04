//nolint:testpackage
package controller

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/mcpproxy"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

type echoMCPServer struct{}

func (echoMCPServer) HandleMessage(_ context.Context, _ json.RawMessage) mcp.JSONRPCMessage {
	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(1),
		Result:  struct{}{},
	}
}

// When the SSE event queue is full the response cannot be delivered, so the
// session must be closed instead of silently dropping responses.
func TestProcessMCPSSEMpscMessagesClosesSessionWhenEventQueueFull(t *testing.T) {
	const sessionID = "test-sse-queue-full"

	server := mcpproxy.NewSSEServer(echoMCPServer{})
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go processMCPSSEMpscMessages(ctx, cancel, sessionID, server)

	// Nobody drains the SSE event queue (capacity 100), so it overflows.
	go func() {
		for ctx.Err() == nil {
			_ = getMCPMpsc().send(ctx, sessionID, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))

			time.Sleep(time.Millisecond)
		}
	}()

	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		require.Fail(t, "session was not closed after the SSE event queue overflowed")
	}
}
