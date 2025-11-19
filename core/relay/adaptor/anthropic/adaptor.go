package anthropic

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct{}

const baseURL = "https://api.anthropic.com/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	url, err := url.JoinPath(u, "/messages")
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    url,
	}, nil
}

const (
	AnthropicVersion = "2023-06-01"
	//nolint:gosec
	AnthropicTokenHeader = "X-Api-Key"
	AnthropicBeta        = "Anthropic-Beta"
)

func FixBetasStringWithModel(model, betas string, deleteFunc ...func(e string) bool) string {
	return strings.Join(FixBetasWithModel(model, strings.Split(betas, ","), deleteFunc...), ",")
}

func modelBetaSupported(model, beta string) bool {
	switch beta {
	case "computer-use-2025-01-24":
		return strings.Contains(model, "3-7-sonnet")
	case "token-efficient-tools-2025-02-19":
		return strings.Contains(model, "3-7-sonnet") ||
			strings.Contains(model, "4-") ||
			strings.Contains(model, "-4")
	case "interleaved-thinking-2025-05-14":
		return strings.Contains(model, "4-") ||
			strings.Contains(model, "-4")
	case "output-128k-2025-02-19":
		return strings.Contains(model, "3-7-sonnet")
	case "dev-full-thinking-2025-05-14":
		return strings.Contains(model, "4-") ||
			strings.Contains(model, "-4")
	case "context-1m-2025-08-07":
		return (strings.Contains(model, "4-") ||
			strings.Contains(model, "-4")) &&
			strings.Contains(model, "sonnet")
	case "context-management-2025-06-27":
		return strings.Contains(model, "4-5") ||
			strings.Contains(model, "-4-5")
	default:
		return true
	}
}

func FixBetasWithModel(model string, betas []string, deleteFunc ...func(e string) bool) []string {
	return slices.DeleteFunc(betas, func(beta string) bool {
		for _, v := range deleteFunc {
			if v != nil && v(beta) {
				return true
			}
		}

		return !modelBetaSupported(model, beta)
	})
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	req *http.Request,
) error {
	req.Header.Set(AnthropicTokenHeader, meta.Channel.Key)

	anthropicVersion := c.Request.Header.Get("Anthropic-Version")
	if anthropicVersion == "" {
		anthropicVersion = AnthropicVersion
	}

	req.Header.Set("Anthropic-Version", anthropicVersion)

	rawBetas := c.Request.Header.Get(AnthropicBeta)

	if rawBetas != "" {
		req.Header.Set(AnthropicBeta, FixBetasStringWithModel(meta.ActualModel, rawBetas))
	}

	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		data, err := OpenAIConvertRequest(meta, req)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}

		data2, err := sonic.Marshal(data)
		if err != nil {
			return adaptor.ConvertResult{}, err
		}

		return adaptor.ConvertResult{
			Header: http.Header{
				"Content-Type":   {"application/json"},
				"Content-Length": {strconv.Itoa(len(data2))},
			},
			Body: bytes.NewReader(data2),
		}, nil
	case mode.Anthropic:
		return ConvertRequest(meta, req)
	case mode.Gemini:
		return ConvertGeminiRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequest(req, meta.RequestTimeout)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			usage, err = OpenAIStreamHandler(meta, c, resp)
		} else {
			usage, err = OpenAIHandler(meta, c, resp)
		}
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			usage, err = StreamHandler(meta, c, resp)
		} else {
			usage, err = Handler(meta, c, resp)
		}
	case mode.Gemini:
		if utils.IsStreamResponse(resp) {
			usage, err = GeminiStreamHandler(meta, c, resp)
		} else {
			usage, err = GeminiHandler(meta, c, resp)
		}
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}

	return usage, err
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"Support native Endpoint: /v1/messages",
		},
		Models: ModelList,
	}
}
