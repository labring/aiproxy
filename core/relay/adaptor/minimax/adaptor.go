package minimax

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

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeMinimax, &Adaptor{})
}

const (
	baseURL          = "https://api.minimaxi.com/v1"
	legacyBaseURL    = "https://api.minimax.chat/v1"
	anthropicBaseURL = "https://api.minimaxi.com/anthropic/v1"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.ChatCompletions ||
		m == mode.Embeddings ||
		m == mode.AudioSpeech ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme:  "MiniMax API\nOpenAI-compatible base URL: https://api.minimaxi.com/v1\nAnthropic-compatible base URL: https://api.minimaxi.com/anthropic/v1\nSupports chat, embeddings, TTS, Gemini-compatible requests, and Anthropic-compatible requests\nKey format supports `api_key` or `api_key|group_id`; `group_id` remains optional for backward compatibility",
		KeyHelp: "api_key or api_key|group_id",
		Models:  ModelList,
	}
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	req *http.Request,
) error {
	apiKey, _, err := GetAPIKeyAndGroupID(meta.Channel.Key)
	if err != nil {
		return err
	}

	if meta.Mode != mode.Anthropic {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}

	req.Header.Set(anthropic.AnthropicTokenHeader, apiKey)

	anthropicVersion := anthropic.AnthropicVersion
	if c != nil && c.Request != nil {
		if v := c.Request.Header.Get("Anthropic-Version"); v != "" {
			anthropicVersion = v
		}

		if rawBetas := c.Request.Header.Get(anthropic.AnthropicBeta); rawBetas != "" {
			req.Header.Set(
				anthropic.AnthropicBeta,
				anthropic.FixBetasStringWithModel(meta.ActualModel, rawBetas),
			)
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
	switch meta.Mode {
	case mode.ChatCompletions, mode.Gemini:
		url, err := url.JoinPath(meta.Channel.BaseURL, "/text/chatcompletion_v2")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.Anthropic:
		targetBaseURL, err := resolveAnthropicBaseURL(meta.Channel.BaseURL)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		targetURL, err := url.JoinPath(targetBaseURL, "/messages")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		if c != nil {
			if beta := c.Query("beta"); beta != "" {
				parsedURL, err := url.Parse(targetURL)
				if err != nil {
					return adaptor.RequestURL{}, err
				}

				queryValues := parsedURL.Query()
				queryValues.Set("beta", beta)
				parsedURL.RawQuery = queryValues.Encode()
				targetURL = parsedURL.String()
			}
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    targetURL,
		}, nil
	case mode.Embeddings:
		_, groupID, err := GetAPIKeyAndGroupID(meta.Channel.Key)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		url, err := url.JoinPath(meta.Channel.BaseURL, "/embeddings")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		url, err = appendGroupID(url, groupID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	case mode.AudioSpeech:
		_, groupID, err := GetAPIKeyAndGroupID(meta.Channel.Key)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		url, err := url.JoinPath(meta.Channel.BaseURL, "/t2a_v2")
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		url, err = appendGroupID(url, groupID)
		if err != nil {
			return adaptor.RequestURL{}, err
		}

		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    url,
		}, nil
	default:
		return a.Adaptor.GetRequestURL(meta, store, c)
	}
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		return openai.ConvertChatCompletionsRequest(meta, req, true)
	case mode.Gemini:
		return openai.ConvertGeminiRequest(meta, req)
	case mode.Anthropic:
		return anthropic.ConvertRequest(meta, req)
	case mode.AudioSpeech:
		return ConvertTTSRequest(meta, req)
	default:
		return a.Adaptor.ConvertRequest(meta, store, req)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	switch meta.Mode {
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			return anthropic.StreamHandler(meta, c, resp)
		}
		return anthropic.Handler(meta, c, resp)
	case mode.AudioSpeech:
		return TTSHandler(meta, c, resp)
	default:
		if !utils.IsStreamResponse(resp) {
			if err := TryErrorHanlder(resp); err != nil {
				return adaptor.DoResponseResult{}, err
			}
		}

		return a.Adaptor.DoResponse(meta, store, c, resp)
	}
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}

func resolveAnthropicBaseURL(rawBaseURL string) (string, error) {
	parsedURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return "", err
	}

	if strings.TrimRight(rawBaseURL, "/") == legacyBaseURL ||
		strings.EqualFold(parsedURL.Host, "api.minimax.chat") {
		parsedURL.Scheme = "https"
		parsedURL.Host = "api.minimaxi.com"
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

func appendGroupID(rawURL, groupID string) (string, error) {
	if groupID == "" {
		return rawURL, nil
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	queryValues := parsedURL.Query()
	queryValues.Set("GroupId", groupID)
	parsedURL.RawQuery = queryValues.Encode()

	return parsedURL.String(), nil
}
