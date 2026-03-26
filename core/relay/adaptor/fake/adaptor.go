package fake

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

func init() {
	registry.Register(model.ChannelTypeFake, &Adaptor{})
}

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Embeddings ||
		m == mode.ImagesGenerations ||
		m == mode.Rerank ||
		m == mode.Anthropic ||
		m == mode.Gemini ||
		m == mode.Responses ||
		m == mode.ResponsesGet ||
		m == mode.ResponsesDelete ||
		m == mode.ResponsesCancel ||
		m == mode.ResponsesInputItems
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    meta.Channel.BaseURL,
	}, nil
}

func (a *Adaptor) SetupRequestHeader(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	_ *http.Request,
) error {
	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	body, err := common.GetRequestBodyReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	reqCtx, err := parseRequest(meta.Mode, body)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if meta.Mode == mode.Gemini && utils.IsGeminiStreamRequest(req.URL.Path) {
		reqCtx.Stream = true
	}

	meta.Set("fake_request_context", reqCtx)

	if reqCtx.Stream {
		meta.Set("stream", true)
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(body))},
		},
		Body: bytes.NewReader(body),
	}, nil
}

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	store adaptor.Store,
	_ *gin.Context,
	_ *http.Request,
) (*http.Response, error) {
	cfg := loadConfig(meta)
	reqCtx := getRequestContext(meta)
	usage := buildUsage(cfg)

	body, contentType, statusCode, err := buildResponseBody(meta, store, cfg, reqCtx, usage)
	if err != nil {
		return nil, err
	}

	if cfg.DelayMS > 0 {
		time.Sleep(time.Duration(cfg.DelayMS) * time.Millisecond)
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type":   {contentType},
			"Content-Length": {strconv.Itoa(len(body))},
			"x-request-id":   {fakeID("req", meta.RequestID+reqCtx.Model)},
		},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}

	return resp, nil
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	_ *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	cfg := loadConfig(meta)
	usage := buildUsage(cfg)
	reqCtx := getRequestContext(meta)

	switch meta.Mode {
	case mode.ChatCompletions, mode.Completions:
		return writeOpenAI(meta, c, cfg, reqCtx, usage)
	case mode.Embeddings:
		return writeEmbeddings(meta, c, cfg, reqCtx, usage)
	case mode.ImagesGenerations:
		return writeImage(meta, c, cfg, reqCtx, usage)
	case mode.Rerank:
		return writeRerank(meta, c, cfg, reqCtx, usage)
	case mode.Anthropic:
		return writeAnthropic(meta, c, cfg, reqCtx, usage)
	case mode.Gemini:
		return writeGemini(meta, c, cfg, reqCtx, usage)
	case mode.Responses:
		return writeResponses(meta, store, c, cfg, reqCtx, usage)
	case mode.ResponsesGet:
		return writeResponsesGet(meta, c, cfg, usage), nil
	case mode.ResponsesDelete:
		c.Status(http.StatusNoContent)
		return adaptor.DoResponseResult{}, nil
	case mode.ResponsesCancel:
		return writeResponsesCancel(meta, c, cfg, usage), nil
	case mode.ResponsesInputItems:
		writeResponsesInputItems(meta, c, cfg)
		return adaptor.DoResponseResult{}, nil
	default:
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusBadRequest,
			"fake adaptor unsupported mode",
		)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp:      "Any non-empty value. The fake adaptor does not call an upstream provider.",
		Readme:       "Fake adaptor for protocol debugging and integration testing. Supports chat, completions, responses, anthropic, gemini native, embeddings, images generations, and rerank. All outputs are synthesized locally and controlled by channel configs.",
		ConfigSchema: configSchema(),
		Models: []model.ModelConfig{
			{Model: "fake-chat", Owner: "fake", Type: mode.ChatCompletions},
			{Model: "fake-completion", Owner: "fake", Type: mode.Completions},
			{Model: "fake-response", Owner: "fake", Type: mode.Responses},
			{Model: "fake-anthropic", Owner: "fake", Type: mode.Anthropic},
			{Model: "fake-gemini", Owner: "fake", Type: mode.Gemini},
			{Model: "fake-embedding", Owner: "fake", Type: mode.Embeddings},
			{Model: "fake-image", Owner: "fake", Type: mode.ImagesGenerations},
			{Model: "fake-rerank", Owner: "fake", Type: mode.Rerank},
		},
	}
}
