package vertexai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/pkg/errors"
)

const anthropicVersion = "vertex-2023-10-16"

type Adaptor struct{}

func (a *Adaptor) ConvertRequest(meta *meta.Meta, request *http.Request) (string, http.Header, io.Reader, error) {
	if request == nil {
		return "", nil, nil, errors.New("request is nil")
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
	default:
		return "", nil, nil, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}

	if err != nil {
		return "", nil, nil, err
	}

	return http.MethodPost, nil, bytes.NewReader(data), nil
}

func handleChatCompletionsRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	claudeReq, err := anthropic.OpenAIConvertRequest(meta, request)
	if err != nil {
		return nil, err
	}

	meta.Set("stream", claudeReq.Stream)

	req := Request{
		AnthropicVersion: anthropicVersion,
		Request:          claudeReq,
	}
	req.Model = ""

	return sonic.Marshal(req)
}

func handleAnthropicRequest(meta *meta.Meta, request *http.Request) ([]byte, error) {
	reqBody, err := common.GetRequestBody(request)
	if err != nil {
		return nil, err
	}

	node, err := sonic.Get(reqBody)
	if err != nil {
		return nil, err
	}

	if err = anthropic.ConvertImage2Base64(context.Background(), &node); err != nil {
		return nil, err
	}

	stream, _ := node.Get("stream").Bool()
	meta.Set("stream", stream)

	if _, err = node.Unset("model"); err != nil {
		return nil, err
	}

	if _, err = node.Set("anthropic_version", ast.NewString(anthropicVersion)); err != nil {
		return nil, err
	}

	return node.MarshalJSON()
}

func (a *Adaptor) DoResponse(meta *meta.Meta, c *gin.Context, resp *http.Response) (usage *model.Usage, err *relaymodel.ErrorWithStatusCode) {
	switch meta.Mode {
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			usage, err = anthropic.OpenAIStreamHandler(meta, c, resp)
		} else {
			usage, err = anthropic.OpenAIHandler(meta, c, resp)
		}
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			usage, err = anthropic.StreamHandler(meta, c, resp)
		} else {
			usage, err = anthropic.Handler(meta, c, resp)
		}
	default:
		return nil, openai.ErrorWrapperWithMessage(fmt.Sprintf("unsupported mode: %s", meta.Mode), "unsupported_mode", http.StatusBadRequest)
	}
	return
}
