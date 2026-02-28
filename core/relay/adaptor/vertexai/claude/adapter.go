package vertexai

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/pkg/errors"
)

const anthropicVersion = "vertex-2023-10-16"

type Adaptor struct{}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	if request == nil {
		return adaptor.ConvertResult{}, errors.New("request is nil")
	}

	var (
		data []byte
		err  error
	)

	switch meta.Mode {
	case mode.ChatCompletions:
		data, err = handleChatCompletionsRequest(meta, request)
	case mode.Anthropic:
		data, err = handleAnthropicRequest(meta, request)
	case mode.Gemini:
		data, err = handleGeminiRequest(meta, request)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}

	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(data))},
		},
		Body: bytes.NewReader(data),
	}, nil
}

var unsupportedBetas = map[string]struct{}{
	"tool-examples-2025-10-29":        {},
	"context-management-2025-06-27":   {},
	"prompt-caching-scope-2026-01-05": {},
}

func fixBetas(model string, betas []string) []string {
	return anthropic.FixBetasWithModel(model, betas, func(e string) bool {
		_, ok := unsupportedBetas[e]
		return ok
	})
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	req *http.Request,
) error {
	betas := c.Request.Header.Get(anthropic.AnthropicBeta)
	if betas != "" {
		req.Header.Set(
			anthropic.AnthropicBeta,
			strings.Join(fixBetas(meta.ActualModel, strings.Split(betas, ",")), ","),
		)
	}

	return nil
}

func handleChatCompletionsRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	claudeReq, err := anthropic.OpenAIConvertRequest(meta, request)
	if err != nil {
		return nil, err
	}

	meta.Set("stream", claudeReq.Stream)

	req := Request{
		AnthropicVersion: anthropicVersion,
		ClaudeRequest:    claudeReq,
	}
	req.Model = ""

	return sonic.Marshal(req)
}

func handleAnthropicRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	return anthropic.ConvertRequestToBytes(meta, request, func(node *ast.Node) error {
		stream, _ := node.Get("stream").Bool()
		meta.Set("stream", stream)

		if _, err := node.Unset("model"); err != nil {
			return err
		}

		_, _ = node.Unset("context_management")
		anthropic.RemoveToolsExamples(node)

		if _, err := node.Set("anthropic_version", ast.NewString(anthropicVersion)); err != nil {
			return err
		}

		return nil
	})
}

func handleGeminiRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	// Convert Gemini format to Claude format
	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(meta, request)
	if err != nil {
		return nil, err
	}

	meta.Set("stream", claudeReq.Stream)

	req := Request{
		AnthropicVersion: anthropicVersion,
		ClaudeRequest:    claudeReq,
	}
	req.Model = ""

	return sonic.Marshal(req)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			return anthropic.OpenAIStreamHandler(meta, c, resp)
		}
		return anthropic.OpenAIHandler(meta, c, resp)
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			return anthropic.StreamHandler(meta, c, resp)
		}
		return anthropic.Handler(meta, c, resp)
	case mode.Gemini:
		if utils.IsStreamResponse(resp) {
			return anthropic.GeminiStreamHandler(meta, c, resp)
		}
		return anthropic.GeminiHandler(meta, c, resp)
	default:
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}
}
