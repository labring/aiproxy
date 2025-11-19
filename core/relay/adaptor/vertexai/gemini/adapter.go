package vertexai

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct{}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	request *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.Anthropic:
		return gemini.ConvertClaudeRequest(meta, request)
	case mode.Gemini:
		// For Gemini mode (native format), pass through the body as-is
		// stream flag is already set in middleware from URL action
		body, err := common.GetRequestBodyReusable(request)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}

		return adaptor.ConvertResult{
			Body: bytes.NewReader(body),
		}, nil
	default:
		return gemini.ConvertRequest(meta, request)
	}
}

func (a *Adaptor) SetupRequestHeader(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	_ *http.Request,
) error {
	return nil
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			usage, err = gemini.ClaudeStreamHandler(meta, c, resp)
		} else {
			usage, err = gemini.ClaudeHandler(meta, c, resp)
		}
	case mode.Gemini:
		// For Gemini mode (native format), pass through the response as-is
		if utils.IsStreamResponse(resp) {
			usage, err = gemini.NativeStreamHandler(meta, c, resp)
		} else {
			usage, err = gemini.NativeHandler(meta, c, resp)
		}
	default:
		if utils.IsStreamResponse(resp) {
			usage, err = gemini.StreamHandler(meta, c, resp)
		} else {
			usage, err = gemini.Handler(meta, c, resp)
		}
	}

	return usage, err
}
