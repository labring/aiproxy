package deepseek

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeDeepseek, &Adaptor{})
}

const (
	baseURL          = "https://api.deepseek.com/v1"
	anthropicBaseURL = "https://api.deepseek.com/anthropic"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) error {
	if meta.Mode != mode.Anthropic {
		return a.Adaptor.SetupRequestHeader(meta, store, c, req)
	}

	req.Header.Set(anthropic.AnthropicTokenHeader, meta.Channel.Key)

	anthropicVersion := anthropic.AnthropicVersion
	if c != nil && c.Request != nil {
		if v := c.Request.Header.Get("Anthropic-Version"); v != "" {
			anthropicVersion = v
		}

		if rawBetas := anthropicBetaHeader(c.Request.Header); rawBetas != "" {
			req.Header.Set(anthropic.AnthropicBeta, rawBetas)
		}
	}

	req.Header.Set("Anthropic-Version", anthropicVersion)

	return nil
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	if meta.Mode != mode.Anthropic {
		return a.Adaptor.GetRequestURL(meta, store, c)
	}

	targetBaseURL, err := resolveAnthropicBaseURL(meta.Channel.BaseURL)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	targetURL, err := url.JoinPath(targetBaseURL, "/messages")
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	targetURL, err = appendBetaQuery(targetURL, c)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    targetURL,
	}, nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	if meta.Mode != mode.Anthropic {
		return a.Adaptor.ConvertRequest(meta, store, req)
	}

	return anthropic.ConvertRequest(meta, req)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if meta.Mode != mode.Anthropic {
		return a.Adaptor.DoResponse(meta, store, c, resp)
	}

	if utils.IsStreamResponse(resp) {
		return anthropic.StreamHandler(meta, c, resp)
	}

	return anthropic.Handler(meta, c, resp)
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "DeepSeek API\nOpenAI-compatible chat and completions endpoints\nSupports native Anthropic-compatible endpoint and Gemini-compatible request conversion",
		Models: ModelList,
	}
}

func resolveAnthropicBaseURL(rawBaseURL string) (string, error) {
	if rawBaseURL == "" {
		rawBaseURL = baseURL
	}

	parsedURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return "", err
	}

	if strings.EqualFold(parsedURL.Host, "api.deepseek.com") {
		parsedURL.Scheme = "https"
	}

	trimmedPath := strings.TrimRight(parsedURL.Path, "/")

	switch {
	case trimmedPath == "":
		parsedURL.Path = "/anthropic/v1"
	case strings.HasSuffix(trimmedPath, "/anthropic/v1"):
		parsedURL.Path = trimmedPath
	case strings.HasSuffix(trimmedPath, "/anthropic"):
		parsedURL.Path = trimmedPath + "/v1"
	case strings.HasSuffix(trimmedPath, "/v1"):
		parsedURL.Path = strings.TrimSuffix(trimmedPath, "/v1") + "/anthropic/v1"
	default:
		parsedURL.Path = trimmedPath + "/anthropic/v1"
	}

	parsedURL.RawPath = ""
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	return parsedURL.String(), nil
}

func anthropicBetaHeader(header http.Header) string {
	return strings.Join(header.Values(anthropic.AnthropicBeta), ",")
}

func appendBetaQuery(rawURL string, c *gin.Context) (string, error) {
	if c == nil || c.Request == nil {
		return rawURL, nil
	}

	beta := c.Query("beta")
	if beta == "" {
		return rawURL, nil
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	queryValues := parsedURL.Query()
	queryValues.Set("beta", beta)
	parsedURL.RawQuery = queryValues.Encode()

	return parsedURL.String(), nil
}
