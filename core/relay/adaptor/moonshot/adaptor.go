package moonshot

import (
	"fmt"
	"net/http"
	"strings"

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
	registry.Register(model.ChannelTypeMoonshot, &Adaptor{})
}

const baseURL = "https://api.moonshot.cn/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	return m == mode.ChatCompletions ||
		m == mode.Completions ||
		m == mode.Embeddings ||
		m == mode.Moderations ||
		m == mode.ImagesGenerations ||
		m == mode.ImagesEdits ||
		m == mode.AudioSpeech ||
		m == mode.AudioTranscription ||
		m == mode.AudioTranslation ||
		m == mode.Rerank ||
		m == mode.ParsePdf ||
		m == mode.VideoGenerationsJobs ||
		m == mode.VideoGenerationsGetJobs ||
		m == mode.VideoGenerationsContent ||
		m == mode.Anthropic ||
		m == mode.Gemini
}

func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	switch meta.Mode {
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
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
	case mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	case mode.ChatCompletions:
		return openai.ConvertChatCompletionsRequest(
			meta,
			req,
			false,
			func(node *ast.Node) error {
				return patchReasoningFromNode(meta, node)
			},
		)
	case mode.Anthropic:
		return openai.ConvertClaudeRequest(meta, req, func(openAIReq *relaymodel.GeneralOpenAIRequest) error {
			return patchReasoningRequest(meta, openAIReq)
		})
	case mode.Gemini:
		return openai.ConvertGeminiRequest(meta, req, func(openAIReq *relaymodel.GeneralOpenAIRequest) error {
			return patchReasoningRequest(meta, openAIReq)
		})
	default:
		return a.Adaptor.ConvertRequest(meta, store, req)
	}
}

func patchReasoningFromNode(meta *meta.Meta, node *ast.Node) error {
	reasoning, err := utils.ParseOpenAIReasoningFromNode(node)
	if err != nil {
		return err
	}

	return applyReasoningToMoonshotNode(meta, node, reasoning)
}

func patchReasoningRequest(meta *meta.Meta, openAIReq *relaymodel.GeneralOpenAIRequest) error {
	reasoning := utils.ParseOpenAIReasoning(openAIReq)
	applyReasoningToMoonshotRequest(meta, openAIReq, reasoning)
	return nil
}

func applyReasoningToMoonshotNode(
	meta *meta.Meta,
	node *ast.Node,
	reasoning relaymodel.NormalizedReasoning,
) error {
	if node == nil || !reasoning.Specified {
		return nil
	}

	_, _ = node.Unset("reasoning_effort")

	if !supportsThinkingToggle(meta) {
		_, _ = node.Unset("thinking")
		return nil
	}

	thinkingType := relaymodel.ClaudeThinkingTypeEnabled
	if reasoning.Disabled || utils.ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
		thinkingType = relaymodel.ClaudeThinkingTypeDisabled
	}

	_, err := node.SetAny("thinking", relaymodel.ClaudeThinking{Type: thinkingType})

	return err
}

func applyReasoningToMoonshotRequest(
	meta *meta.Meta,
	req *relaymodel.GeneralOpenAIRequest,
	reasoning relaymodel.NormalizedReasoning,
) {
	if req == nil || !reasoning.Specified {
		return
	}

	req.ReasoningEffort = nil

	if !supportsThinkingToggle(meta) {
		req.Thinking = nil
		return
	}

	thinkingType := relaymodel.ClaudeThinkingTypeEnabled
	if reasoning.Disabled || utils.ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
		thinkingType = relaymodel.ClaudeThinkingTypeDisabled
	}

	req.Thinking = &relaymodel.ClaudeThinking{Type: thinkingType}
}

func supportsThinkingToggle(meta *meta.Meta) bool {
	modelName := moonshotModelName(meta)
	modelName = strings.ToLower(modelName)

	return strings.HasPrefix(modelName, "kimi-k2.5") ||
		strings.HasPrefix(modelName, "kimi-k2.6")
}

func moonshotModelName(meta *meta.Meta) string {
	if meta == nil {
		return ""
	}

	if meta.ActualModel != "" {
		return meta.ActualModel
	}

	return meta.OriginModel
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Moonshot API\nOpenAI-compatible endpoint\nSupports Gemini-compatible request conversion",
		Models: ModelList,
	}
}
