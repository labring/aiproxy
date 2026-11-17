package ali

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

func aliModelMatches(meta *meta.Meta, match func(string) bool) bool {
	if meta == nil {
		return false
	}
	return utils.FirstMatchingModelName(match, meta.OriginModel, meta.ActualModel) != ""
}

func isAliQwen3Model(meta *meta.Meta) bool {
	return aliModelMatches(meta, func(modelName string) bool {
		return strings.HasPrefix(strings.ToLower(modelName), "qwen3-")
	})
}

func isAliQwqModel(meta *meta.Meta) bool {
	return aliModelMatches(meta, func(modelName string) bool {
		return strings.HasPrefix(strings.ToLower(modelName), "qwq-")
	})
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

// https://help.aliyun.com/zh/model-studio/deep-thinking
func patchReasoningFromNode(meta *meta.Meta, node *ast.Node) error {
	reasoning, err := utils.ParseOpenAIReasoningFromNode(node)
	if err != nil {
		return err
	}

	return utils.ApplyReasoningToAliNode(meta.OriginModel, meta.ActualModel, node, reasoning)
}

func ConvertCompletionsRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	callbacks := []func(node *ast.Node) error{
		func(node *ast.Node) error {
			return patchReasoningFromNode(meta, node)
		},
	}
	if isAliQwen3Model(meta) {
		callbacks = append(callbacks, patchQwen3EnableThinking)
	}

	if isAliQwqModel(meta) {
		callbacks = append(callbacks, patchQwqOnlySupportStream)
	}

	return openai.ConvertCompletionsRequest(meta, req, callbacks...)
}

func ConvertChatCompletionsRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	callbacks := []func(node *ast.Node) error{
		func(node *ast.Node) error {
			return patchReasoningFromNode(meta, node)
		},
	}
	if isAliQwen3Model(meta) {
		callbacks = append(callbacks, patchQwen3EnableThinking)
	}

	if isAliQwqModel(meta) {
		callbacks = append(callbacks, patchQwqOnlySupportStream)
	}

	return openai.ConvertChatCompletionsRequest(meta, req, false, callbacks...)
}

func ConvertGeminiRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	hooks := []openai.OpenAIRequestHook{
		func(openAIReq *relaymodel.GeneralOpenAIRequest) error {
			reasoning := utils.ParseOpenAIReasoning(openAIReq)
			utils.ApplyReasoningToAliRequest(
				meta.OriginModel,
				meta.ActualModel,
				openAIReq,
				reasoning,
			)

			return nil
		},
	}

	if isAliQwen3Model(meta) {
		hooks = append(hooks, func(openAIReq *relaymodel.GeneralOpenAIRequest) error {
			if !openAIReq.Stream {
				enableThinking := false
				openAIReq.EnableThinking = &enableThinking
				openAIReq.ThinkingBudget = nil
			}

			return nil
		})
	}

	if isAliQwqModel(meta) {
		hooks = append(hooks, func(openAIReq *relaymodel.GeneralOpenAIRequest) error {
			openAIReq.Stream = true
			return nil
		})
	}

	return openai.ConvertGeminiRequest(meta, req, hooks...)
}

func getEnableSearch(node *ast.Node) bool {
	enableSearch, _ := node.Get("enable_search").Bool()
	return enableSearch
}

func ChatHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	node, err := common.UnmarshalRequest2ValidatedNode(c.Request)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("get request body failed: %s", err),
			"get_request_body_failed",
			http.StatusInternalServerError,
		)
	}

	result, e := openai.DoResponse(meta, store, c, resp)
	if e != nil {
		return adaptor.DoResponseResult{}, e
	}

	if getEnableSearch(&node) {
		result.Usage.WebSearchCount++
	}

	result.UsageContext = aliChatUsageContextWithDefaults(
		aliChatUsageContext(result.Usage).
			WithFallback(aliChatRequestUsageContext(&node)).
			WithFallback(result.UsageContext),
	)

	return result, nil
}

func aliChatUsageContext(usage coremodel.Usage) coremodel.UsageContext {
	usageContext := coremodel.UsageContext{}

	if usage.ImageInputTokens > 0 || usage.AudioInputTokens > 0 || usage.VideoInputTokens > 0 {
		usageContext.InputMedia = new(bool)
		*usageContext.InputMedia = true
	}

	if usage.VideoInputTokens > 0 {
		usageContext.InputVideo = new(bool)
		*usageContext.InputVideo = true
	}

	if usage.AudioOutputTokens > 0 {
		usageContext.OutputAudio = new(bool)
		*usageContext.OutputAudio = true
	}

	return usageContext
}

func aliChatRequestUsageContext(node *ast.Node) coremodel.UsageContext {
	usageContext := coremodel.UsageContext{}

	if aliChatRequestHasInputMedia(node) {
		usageContext.InputMedia = new(bool)
		*usageContext.InputMedia = true
	}

	if aliChatRequestHasInputVideo(node) {
		usageContext.InputVideo = new(bool)
		*usageContext.InputVideo = true
	}

	if aliChatRequestWantsOutputAudio(node) {
		usageContext.OutputAudio = new(bool)
		*usageContext.OutputAudio = true
	}

	return usageContext
}

func aliChatUsageContextWithDefaults(usageContext coremodel.UsageContext) coremodel.UsageContext {
	if usageContext.InputMedia == nil {
		usageContext.InputMedia = new(bool)
	}

	if usageContext.OutputAudio == nil {
		usageContext.OutputAudio = new(bool)
	}

	return usageContext
}

func aliChatRequestHasInputMedia(node *ast.Node) bool {
	return aliChatRequestContentMatches(node, func(content *ast.Node) bool {
		return aliChatContentHasField(content, "image_url") ||
			aliChatContentHasField(content, "input_audio") ||
			aliChatContentHasField(content, "audio") ||
			aliChatContentHasField(content, "video") ||
			aliChatContentHasField(content, "video_url") ||
			aliChatContentTypeContains(content, "image") ||
			aliChatContentTypeContains(content, "audio") ||
			aliChatContentTypeContains(content, "video")
	})
}

func aliChatRequestHasInputVideo(node *ast.Node) bool {
	return aliChatRequestContentMatches(node, func(content *ast.Node) bool {
		return aliChatContentHasField(content, "video") ||
			aliChatContentHasField(content, "video_url") ||
			aliChatContentTypeContains(content, "video")
	})
}

func aliChatRequestWantsOutputAudio(node *ast.Node) bool {
	modalities := node.Get("modalities")
	if modalities.Exists() && modalities.TypeSafe() == ast.V_ARRAY {
		hasAudio := false
		_ = modalities.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
			value, err := item.String()
			if err == nil && strings.EqualFold(value, "audio") {
				hasAudio = true
				return false
			}

			return true
		})

		if hasAudio {
			return true
		}
	}

	return node.Get("audio").Exists()
}

func aliChatRequestContentMatches(node *ast.Node, match func(*ast.Node) bool) bool {
	messages := node.Get("messages")
	if !messages.Exists() || messages.TypeSafe() != ast.V_ARRAY {
		return false
	}

	matched := false
	_ = messages.ForEach(func(_ ast.Sequence, message *ast.Node) bool {
		content := message.Get("content")
		if !content.Exists() {
			return true
		}

		if content.TypeSafe() == ast.V_ARRAY {
			_ = content.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
				if match(item) {
					matched = true
					return false
				}

				return true
			})

			return !matched
		}

		if match(content) {
			matched = true
			return false
		}

		return true
	})

	return matched
}

func aliChatContentHasField(content *ast.Node, field string) bool {
	return content.TypeSafe() == ast.V_OBJECT && content.Get(field).Exists()
}

func aliChatContentTypeContains(content *ast.Node, pattern string) bool {
	if content.TypeSafe() != ast.V_OBJECT {
		return false
	}

	contentType, err := content.Get("type").String()
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(contentType), pattern)
}
