package mcpservers

import (
	"context"
	"encoding/json"
	"runtime"

	"github.com/bytedance/sonic"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type Server interface {
	HandleMessage(ctx context.Context, message json.RawMessage) mcp.JSONRPCMessage
}

type client2Server struct {
	client transport.Interface
}

func (s *client2Server) HandleMessage(
	ctx context.Context,
	message json.RawMessage,
) mcp.JSONRPCMessage {
	methodNode, err := sonic.Get(message, "method")
	if err != nil {
		return CreateMCPErrorResponse(nil, mcp.PARSE_ERROR, err.Error())
	}
	method, err := methodNode.String()
	if err != nil {
		return CreateMCPErrorResponse(nil, mcp.PARSE_ERROR, err.Error())
	}

	switch method {
	case "notifications/initialized":
		req := mcp.JSONRPCNotification{}
		err := sonic.Unmarshal(message, &req)
		if err != nil {
			return CreateMCPErrorResponse(nil, mcp.PARSE_ERROR, err.Error())
		}
		err = s.client.SendNotification(ctx, req)
		if err != nil {
			return CreateMCPErrorResponse(nil, mcp.PARSE_ERROR, err.Error())
		}
		return nil
	default:
		req := transport.JSONRPCRequest{}
		err := sonic.Unmarshal(message, &req)
		if err != nil {
			return CreateMCPErrorResponse(nil, mcp.PARSE_ERROR, err.Error())
		}
		resp, err := s.client.SendRequest(ctx, req)
		if err != nil {
			return CreateMCPErrorResponse(nil, mcp.INTERNAL_ERROR, err.Error())
		}
		if resp.Error != nil {
			return CreateMCPErrorResponse(
				resp.ID,
				resp.Error.Code,
				resp.Error.Message,
				resp.Error.Data,
			)
		}
		return CreateMCPResultResponse(
			resp.ID,
			resp.Result,
		)
	}
}

func WrapMCPClient2Server(client transport.Interface) Server {
	return &client2Server{client: client}
}

func WrapMCPClient2ServerWithCleanup(client transport.Interface) Server {
	server := &client2Server{client: client}
	_ = runtime.AddCleanup(server, func(client transport.Interface) {
		_ = client.Close()
	}, server.client)
	return server
}

type JSONRPCNoErrorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      mcp.RequestId   `json:"id"`
	Result  json.RawMessage `json:"result"`
}

func CreateMCPResultResponse(
	id any,
	result json.RawMessage,
) mcp.JSONRPCMessage {
	return &JSONRPCNoErrorResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(id),
		Result:  result,
	}
}

func CreateMCPErrorResponse(
	id any,
	code int,
	message string,
	data ...any,
) mcp.JSONRPCMessage {
	var d any
	if len(data) > 0 {
		d = data[0]
	}
	return mcp.JSONRPCError{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(id),
		Error: struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    any    `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
			Data:    d,
		},
	}
}
