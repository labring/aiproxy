package ali

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

// https://help.aliyun.com/zh/dashscope/developer-reference/api-details

type Adaptor struct{}

const baseURL = "https://dashscope.aliyuncs.com"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL
	if u == "" {
		u = baseURL
	}
	switch meta.Mode {
	case mode.ImagesGenerations:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api/v1/services/aigc/text2image/image-synthesis",
		}, nil
	case mode.ChatCompletions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/compatible-mode/v1/chat/completions",
		}, nil
	case mode.Completions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/compatible-mode/v1/completions",
		}, nil
	case mode.Embeddings:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/compatible-mode/v1/embeddings",
		}, nil
	case mode.AudioSpeech, mode.AudioTranscription:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api-ws/v1/inference",
		}, nil
	case mode.Rerank:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api/v1/services/rerank/text-rerank/text-rerank",
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)

	// req.Header.Set("X-Dashscope-Plugin", meta.Channel.Config.Plugin)
	return nil
}

// qwen3 enable_thinking must be set to false for non-streaming calls
func patchQwen3EnableThinking(node *ast.Node) error {
	streamNode := node.Get("stream")
	isStreaming := false

	if streamNode.Exists() {
		streamBool, err := streamNode.Bool()
		if err != nil {
			return errors.New("stream is not a boolean")
		}
		isStreaming = streamBool
	}

	// Set enable_thinking to false for non-streaming requests
	if !isStreaming {
		_, err := node.Set("enable_thinking", ast.NewBool(false))
		return err
	}

	return nil
}

// qwq only support stream mode
func patchQwqOnlySupportStream(node *ast.Node) error {
	_, err := node.Set("stream", ast.NewBool(true))
	return err
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.ImagesGenerations:
		return ConvertImageRequest(meta, req)
	case mode.Rerank:
		return ConvertRerankRequest(meta, req)
	case mode.ChatCompletions:
		if strings.HasPrefix(meta.ActualModel, "qwen3-") {
			return openai.ConvertChatCompletionsRequest(meta, req, patchQwen3EnableThinking, false)
		}
		if strings.HasPrefix(meta.ActualModel, "qwq-") {
			return openai.ConvertChatCompletionsRequest(meta, req, patchQwqOnlySupportStream, false)
		}
		return openai.ConvertChatCompletionsRequest(meta, req, nil, false)
	case mode.Completions:
		if strings.HasPrefix(meta.ActualModel, "qwen3-") {
			return openai.ConvertCompletionsRequest(meta, req, patchQwen3EnableThinking)
		}
		if strings.HasPrefix(meta.ActualModel, "qwq-") {
			return openai.ConvertCompletionsRequest(meta, req, patchQwqOnlySupportStream)
		}
		return openai.ConvertCompletionsRequest(meta, req, nil)
	case mode.Embeddings:
		return openai.ConvertRequest(meta, store, req)
	case mode.AudioSpeech:
		return ConvertTTSRequest(meta, req)
	case mode.AudioTranscription:
		return ConvertSTTRequest(meta, req)
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
	switch meta.Mode {
	case mode.AudioSpeech:
		return TTSDoRequest(meta, req)
	case mode.AudioTranscription:
		return STTDoRequest(meta, req)
	case mode.ChatCompletions:
		fallthrough
	default:
		return utils.DoRequest(req)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	switch meta.Mode {
	case mode.ImagesGenerations:
		return ImageHandler(meta, c, resp)
	case mode.Embeddings, mode.Completions:
		return openai.DoResponse(meta, store, c, resp)
	case mode.ChatCompletions:
		reqBody, err := common.GetRequestBody(c.Request)
		if err != nil {
			return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
				fmt.Sprintf("get request body failed: %s", err),
				"get_request_body_failed",
				http.StatusInternalServerError,
			)
		}
		enableSearch, err := getEnableSearch(reqBody)
		if err != nil {
			return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
				fmt.Sprintf("get enable_search failed: %s", err),
				"get_enable_search_failed",
				http.StatusInternalServerError,
			)
		}
		u, e := openai.DoResponse(meta, store, c, resp)
		if e != nil {
			return model.Usage{}, e
		}
		if enableSearch {
			u.WebSearchCount++
		}
		return u, nil
	case mode.Rerank:
		return RerankHandler(meta, c, resp)
	case mode.AudioSpeech:
		return TTSDoResponse(meta, c, resp)
	case mode.AudioTranscription:
		return STTDoResponse(meta, c, resp)
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}
}

func getEnableSearch(reqBody []byte) (bool, error) {
	searchNode, err := sonic.Get(reqBody, "enable_search")
	if err != nil {
		if errors.Is(err, ast.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("get enable_search failed: %w", err)
	}
	enableSearch, err := searchNode.Bool()
	if err != nil {
		if errors.Is(err, ast.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("get enable_search failed: %w", err)
	}
	return enableSearch, nil
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"OpenAI compatibility",
			"Network search metering support",
			"Rerank support: https://help.aliyun.com/zh/model-studio/text-rerank-api",
			"STT support: https://help.aliyun.com/zh/model-studio/sambert-speech-synthesis/",
		},
		Models: ModelList,
	}
}
