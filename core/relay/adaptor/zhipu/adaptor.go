package zhipu

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeZhipu, &Adaptor{})
}

const baseURL = "https://open.bigmodel.cn/api/paas/v4"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.AudioTranscription ||
		m == mode.AudioSpeech ||
		m == mode.Embeddings ||
		m == mode.Rerank ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL

	switch meta.Mode {
	case mode.ChatCompletions, mode.Anthropic, mode.Gemini:
		return postURL(u, "/chat/completions")
	case mode.Completions:
		return postURL(u, "/completions")
	case mode.AudioTranscription:
		return postURL(u, "/audio/transcriptions")
	case mode.AudioSpeech:
		return postURL(u, "/audio/speech")
	case mode.Embeddings:
		return postURL(u, "/embeddings")
	case mode.Rerank:
		return postURL(u, "/rerank")
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func postURL(baseURL, path string) (adaptor.RequestURL, error) {
	u, err := url.JoinPath(baseURL, path)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    u,
	}, nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		return openai.ConvertChatCompletionsRequest(
			meta,
			req,
			false,
			patchReasoningFromNode,
		)
	case mode.Completions:
		return openai.ConvertCompletionsRequest(meta, req)
	case mode.Anthropic:
		return openai.ConvertClaudeRequest(meta, req, patchReasoningRequest)
	case mode.Gemini:
		return openai.ConvertGeminiRequest(meta, req, patchReasoningRequest)
	case mode.AudioTranscription:
		return openai.ConvertSTTRequest(meta, req)
	case mode.AudioSpeech:
		return openai.ConvertTTSRequest(meta, req, "")
	case mode.Embeddings:
		return openai.ConvertEmbeddingsRequest(meta, req, false)
	case mode.Rerank:
		return openai.ConvertRerankRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func patchReasoningFromNode(node *ast.Node) error {
	reasoning, err := utils.ParseOpenAIReasoningFromNode(node)
	if err != nil {
		return err
	}

	return utils.ApplyReasoningToZhipuNode(node, reasoning)
}

func patchReasoningRequest(openAIReq *relaymodel.GeneralOpenAIRequest) error {
	reasoning := utils.ParseOpenAIReasoning(openAIReq)
	utils.ApplyReasoningToZhipuRequest(openAIReq, reasoning)
	return nil
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	switch meta.Mode {
	case mode.Anthropic:
		if utils.IsStreamResponse(resp) {
			return openai.ClaudeStreamHandler(meta, c, resp)
		}
		return openai.ClaudeHandler(meta, c, resp)
	case mode.Gemini:
		if utils.IsStreamResponse(resp) {
			return openai.GeminiStreamHandler(meta, c, resp)
		}
		return openai.GeminiHandler(meta, c, resp)
	case mode.Embeddings:
		return EmbeddingsHandler(c, resp)
	case mode.ChatCompletions,
		mode.Completions,
		mode.AudioTranscription,
		mode.AudioSpeech,
		mode.Rerank:
		return openai.DoResponse(meta, store, c, resp)
	default:
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			"unsupported_mode",
			http.StatusBadRequest,
		)
	}
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Zhipu AI BigModel API\nOpenAI-compatible endpoint\nSupports Anthropic-compatible and Gemini-compatible request conversion",
		Models: ModelList,
	}
}
