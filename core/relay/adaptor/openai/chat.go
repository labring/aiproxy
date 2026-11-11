package openai

import (
	"bytes"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
)

// chatCompletionStreamState manages state for ChatCompletion stream conversion
type chatCompletionStreamState struct {
	messageID                  string
	created                    int64
	serviceTier                *string
	meta                       *meta.Meta
	c                          *gin.Context
	toolCallIndexByItemID      map[string]int
	toolCallIndexByOutputIndex map[int]int
	nextToolCallIndex          int
	hasToolCall                bool
}

func (s *chatCompletionStreamState) createdAt() int64 {
	if s.created == 0 {
		s.created = time.Now().Unix()
	}

	return s.created
}

func (s *chatCompletionStreamState) registerToolCall(
	event *relaymodel.ResponseStreamEvent,
) int {
	if event.Item.ID != "" {
		if index, ok := s.toolCallIndexByItemID[event.Item.ID]; ok {
			return index
		}
	}

	if event.OutputIndex != nil {
		if index, ok := s.toolCallIndexByOutputIndex[*event.OutputIndex]; ok {
			return index
		}
	}

	index := s.nextToolCallIndex
	s.nextToolCallIndex++

	if event.Item.ID != "" {
		s.toolCallIndexByItemID[event.Item.ID] = index
	}

	if event.OutputIndex != nil {
		s.toolCallIndexByOutputIndex[*event.OutputIndex] = index
	}

	return index
}

func (s *chatCompletionStreamState) toolCallIndex(
	event *relaymodel.ResponseStreamEvent,
) (int, bool) {
	if event.ItemID != "" {
		index, ok := s.toolCallIndexByItemID[event.ItemID]
		if ok {
			return index, true
		}
	}

	if event.OutputIndex != nil {
		index, ok := s.toolCallIndexByOutputIndex[*event.OutputIndex]
		return index, ok
	}

	if event.ItemID == "" && s.nextToolCallIndex > 0 {
		return s.nextToolCallIndex - 1, true
	}

	return 0, false
}

func responseModelName(meta *meta.Meta) string {
	if meta == nil {
		return ""
	}

	if meta.OriginModel != "" {
		return meta.OriginModel
	}

	return meta.ActualModel
}

func responseToChatFinishReason(response *relaymodel.Response) relaymodel.FinishReason {
	if response == nil {
		return relaymodel.FinishReasonStop
	}

	if response.Status != relaymodel.ResponseStatusIncomplete {
		return relaymodel.FinishReasonStop
	}

	if response.IncompleteDetails == nil {
		return relaymodel.FinishReasonStop
	}

	switch response.IncompleteDetails.Reason {
	case "max_output_tokens":
		return relaymodel.FinishReasonLength
	case "content_filter":
		return relaymodel.FinishReasonContentFilter
	default:
		return relaymodel.FinishReasonStop
	}
}

func responseReasoningSummaryText(response *relaymodel.Response) string {
	if response == nil {
		return ""
	}

	var summaryParts []string

	for _, outputItem := range response.Output {
		if outputItem.Type != relaymodel.InputItemTypeReasoning {
			continue
		}

		summaryParts = append(summaryParts, reasoningSummaryText(outputItem.Summary)...)
	}

	return strings.Join(summaryParts, "\n")
}

func reasoningSummaryText(summary any) []string {
	switch value := summary.(type) {
	case string:
		if value == "" {
			return nil
		}

		return []string{value}
	case []relaymodel.SummaryPart:
		parts := make([]string, 0, len(value))
		for _, part := range value {
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}

		return parts
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			switch typedItem := item.(type) {
			case relaymodel.SummaryPart:
				if typedItem.Text != "" {
					parts = append(parts, typedItem.Text)
				}
			case map[string]any:
				if text, ok := typedItem["text"].(string); ok && text != "" {
					parts = append(parts, text)
				}
			}
		}

		return parts
	default:
		return nil
	}
}

// handleResponseCreated handles response.created event for ChatCompletion
func (s *chatCompletionStreamState) handleResponseCreated(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Response == nil {
		return nil
	}

	s.messageID = event.Response.ID
	if event.Response.ServiceTier != nil {
		s.serviceTier = event.Response.ServiceTier
	}

	if event.Response.CreatedAt != 0 {
		s.created = event.Response.CreatedAt
	}

	return &relaymodel.ChatCompletionsStreamResponse{
		ID:          s.messageID,
		Object:      relaymodel.ChatCompletionChunkObject,
		Created:     s.createdAt(),
		Model:       responseModelName(s.meta),
		ServiceTier: s.serviceTier,
		Obfuscation: event.Obfuscation,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					Role: relaymodel.RoleAssistant,
				},
			},
		},
	}
}

// handleOutputTextDelta handles response.output_text.delta event for ChatCompletion
func (s *chatCompletionStreamState) handleOutputTextDelta(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Delta == "" {
		return nil
	}

	choice := &relaymodel.ChatCompletionsStreamResponseChoice{
		Index: 0,
		Delta: relaymodel.Message{
			Content: event.Delta,
		},
	}
	if len(event.Logprobs) > 0 {
		choice.Logprobs = &relaymodel.ChoiceLogprobs{Content: event.Logprobs}
	}

	return &relaymodel.ChatCompletionsStreamResponse{
		ID:          s.messageID,
		Object:      relaymodel.ChatCompletionChunkObject,
		Created:     s.createdAt(),
		Model:       responseModelName(s.meta),
		ServiceTier: s.serviceTier,
		Obfuscation: event.Obfuscation,
		Choices:     []*relaymodel.ChatCompletionsStreamResponseChoice{choice},
	}
}

func (s *chatCompletionStreamState) handleRefusalDelta(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Delta == "" {
		return nil
	}

	choice := &relaymodel.ChatCompletionsStreamResponseChoice{
		Index: 0,
		Delta: relaymodel.Message{
			Refusal: event.Delta,
		},
	}
	if len(event.Logprobs) > 0 {
		choice.Logprobs = &relaymodel.ChoiceLogprobs{Refusal: event.Logprobs}
	}

	return &relaymodel.ChatCompletionsStreamResponse{
		ID:          s.messageID,
		Object:      relaymodel.ChatCompletionChunkObject,
		Created:     s.createdAt(),
		Model:       responseModelName(s.meta),
		ServiceTier: s.serviceTier,
		Obfuscation: event.Obfuscation,
		Choices:     []*relaymodel.ChatCompletionsStreamResponseChoice{choice},
	}
}

func (s *chatCompletionStreamState) handleReasoningSummaryTextDelta(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Delta == "" {
		return nil
	}

	return &relaymodel.ChatCompletionsStreamResponse{
		ID:          s.messageID,
		Object:      relaymodel.ChatCompletionChunkObject,
		Created:     s.createdAt(),
		Model:       responseModelName(s.meta),
		ServiceTier: s.serviceTier,
		Obfuscation: event.Obfuscation,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					ReasoningContent: event.Delta,
				},
			},
		},
	}
}

// handleOutputItemAdded handles response.output_item.added event for ChatCompletion
func (s *chatCompletionStreamState) handleOutputItemAdded(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Item == nil {
		return nil
	}

	// Track function calls
	if event.Item.Type == relaymodel.InputItemTypeFunctionCall {
		s.hasToolCall = true
		toolCallIndex := s.registerToolCall(event)

		// Send tool call start
		return &relaymodel.ChatCompletionsStreamResponse{
			ID:          s.messageID,
			Object:      relaymodel.ChatCompletionChunkObject,
			Created:     s.createdAt(),
			Model:       responseModelName(s.meta),
			ServiceTier: s.serviceTier,
			Obfuscation: event.Obfuscation,
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: relaymodel.Message{
						ToolCalls: []relaymodel.ToolCall{
							{
								Index: toolCallIndex,
								ID:    event.Item.CallID,
								Type:  relaymodel.ToolChoiceTypeFunction,
								Function: relaymodel.Function{
									Name:      event.Item.Name,
									Arguments: "",
								},
							},
						},
					},
				},
			},
		}
	}

	if event.Item.Type == relaymodel.InputItemTypeMessage {
		return &relaymodel.ChatCompletionsStreamResponse{
			ID:          s.messageID,
			Object:      relaymodel.ChatCompletionChunkObject,
			Created:     s.createdAt(),
			Model:       responseModelName(s.meta),
			ServiceTier: s.serviceTier,
			Obfuscation: event.Obfuscation,
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: relaymodel.Message{
						Role: relaymodel.RoleAssistant,
					},
				},
			},
		}
	}

	return nil
}

// handleFunctionCallArgumentsDelta handles response.function_call_arguments.delta event for ChatCompletion
func (s *chatCompletionStreamState) handleFunctionCallArgumentsDelta(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Delta == "" {
		return nil
	}

	toolCallIndex, ok := s.toolCallIndex(event)
	if !ok {
		return nil
	}

	// Send delta
	return &relaymodel.ChatCompletionsStreamResponse{
		ID:          s.messageID,
		Object:      relaymodel.ChatCompletionChunkObject,
		Created:     s.createdAt(),
		Model:       responseModelName(s.meta),
		ServiceTier: s.serviceTier,
		Obfuscation: event.Obfuscation,
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: relaymodel.Message{
					ToolCalls: []relaymodel.ToolCall{
						{
							Index: toolCallIndex,
							Function: relaymodel.Function{
								Arguments: event.Delta,
							},
						},
					},
				},
			},
		},
	}
}

// handleResponseCompleted handles response.completed/done event for ChatCompletion
func (s *chatCompletionStreamState) handleResponseCompleted(
	event *relaymodel.ResponseStreamEvent,
) *relaymodel.ChatCompletionsStreamResponse {
	if event.Response == nil || event.Response.Usage == nil {
		return nil
	}

	if event.Response.ServiceTier != nil {
		s.serviceTier = event.Response.ServiceTier
	}

	chatUsage := event.Response.Usage.ToChatUsage()

	finishReason := responseToChatFinishReason(event.Response)
	if finishReason == relaymodel.FinishReasonStop && s.hasToolCall {
		finishReason = relaymodel.FinishReasonToolCalls
	}

	return &relaymodel.ChatCompletionsStreamResponse{
		ID:      s.messageID,
		Object:  relaymodel.ChatCompletionChunkObject,
		Created: s.createdAt(),
		Model:   responseModelName(s.meta),
		Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: finishReason,
			},
		},
		Usage:       &chatUsage,
		Moderation:  event.Response.Moderation,
		ServiceTier: s.serviceTier,
		Obfuscation: event.Obfuscation,
	}
}

func ConvertCompletionsRequest(
	meta *meta.Meta,
	req *http.Request,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

		if err := callback(&node); err != nil {
			return adaptor.ConvertResult{}, err
		}
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	jsonData, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonData))},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
}

func ConvertChatCompletionsRequest(
	meta *meta.Meta,
	req *http.Request,
	doNotPatchStreamOptionsIncludeUsage bool,
	callback ...func(node *ast.Node) error,
) (adaptor.ConvertResult, error) {
	node, err := common.UnmarshalRequest2NodeReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
	}

	// Clean tool parameters (remove null/empty required fields)
	// This should be done before other callbacks to ensure consistency
	if err := CleanToolParametersFromNode(&node); err != nil {
		return adaptor.ConvertResult{}, err
	}

	for _, callback := range callback {
		if callback == nil {
			continue
		}

		if err := callback(&node); err != nil {
			return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
		}
	}

	if !doNotPatchStreamOptionsIncludeUsage {
		if err := patchStreamOptions(&node); err != nil {
			return adaptor.ConvertResult{}, convertRequestError(meta, err.Error())
		}
	}

	_, err = node.Set("model", ast.NewString(meta.ActualModel))
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	jsonData, err := node.MarshalJSON()
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonData))},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
}

func patchStreamOptions(node *ast.Node) error {
	streamNode := node.Get("stream")
	if !streamNode.Exists() {
		return nil
	}

	streamBool, err := streamNode.Bool()
	if err != nil {
		return errors.New("stream is not a boolean")
	}

	if !streamBool {
		return nil
	}

	streamOptionsNode := node.Get("stream_options")
	if !streamOptionsNode.Exists() {
		_, err = node.Set("stream_options", ast.NewObject([]ast.Pair{
			ast.NewPair("include_usage", ast.NewBool(true)),
		}))
		return err
	}

	if streamOptionsNode.TypeSafe() != ast.V_OBJECT {
		return errors.New("stream_options is not an object")
	}

	_, err = streamOptionsNode.Set("include_usage", ast.NewBool(true))

	return err
}

func GetUsageOrChatChoicesResponseFromNode(
	node *ast.Node,
) (*relaymodel.ChatUsage, []*relaymodel.ChatCompletionsStreamResponseChoice, error) {
	usageNode := node.Get("usage")
	if usageNode != nil && usageNode.TypeSafe() != ast.V_NULL {
		usageRaw, err := usageNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			var usage relaymodel.ChatUsage

			err = sonic.UnmarshalString(usageRaw, &usage)
			if err != nil {
				return nil, nil, err
			}

			return &usage, nil, nil
		}
	}

	var choices []*relaymodel.ChatCompletionsStreamResponseChoice

	choicesNode := node.Get("choices")
	if choicesNode != nil && choicesNode.TypeSafe() != ast.V_NULL {
		choicesRaw, err := choicesNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			err = sonic.UnmarshalString(choicesRaw, &choices)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return nil, choices, nil
}

type PreHandler func(meta *meta.Meta, node *ast.Node) error

func StreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
	preHandler PreHandler,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	responseText := strings.Builder{}

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.OriginModel, meta.ActualModel)
	defer cleanup()

	var (
		usage      relaymodel.ChatUsage
		upstreamID string
	)

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		node, err := common.GetJSONNodeNoCopy(data)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		if preHandler != nil {
			err := preHandler(meta, &node)
			if err != nil {
				log.Error("error pre handler: " + err.Error())
				continue
			}
		}

		u, ch, err := GetUsageOrChatChoicesResponseFromNode(&node)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		if u != nil {
			usage = *u

			responseText.Reset()
		}

		// Extract upstream ID from response if available
		if upstreamID == "" {
			if idNode := node.Get("id"); idNode.Exists() && idNode.TypeSafe() != ast.V_NULL {
				if id, err := idNode.String(); err == nil && id != "" {
					upstreamID = id
				}
			}
		}

		for _, choice := range ch {
			if usage.TotalTokens == 0 {
				if choice.Text != "" {
					responseText.WriteString(choice.Text)
				} else {
					responseText.WriteString(choice.Delta.StringContent())
				}
			}
		}

		_, err = node.Set("model", ast.NewString(meta.OriginModel))
		if err != nil {
			log.Error("error set model: " + err.Error())
		}

		_ = render.OpenaiObjectData(c, &node)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	if usage.TotalTokens == 0 && responseText.Len() > 0 {
		usage = ResponseText2Usage(
			responseText.String(),
			meta.ActualModel,
			int64(meta.RequestUsage.InputTokens),
		)
		_ = render.OpenaiObjectData(c, &relaymodel.ChatCompletionsStreamResponse{
			ID:      ChatCompletionID(),
			Model:   meta.OriginModel,
			Object:  relaymodel.ChatCompletionChunkObject,
			Created: time.Now().Unix(),
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{},
			Usage:   &usage,
		})
	} else if usage.TotalTokens != 0 && usage.PromptTokens == 0 { // some channels don't return prompt tokens & completion tokens
		usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
		usage.CompletionTokens = usage.TotalTokens - int64(meta.RequestUsage.InputTokens)
	}

	render.OpenaiDone(c)

	return adaptor.DoResponseResult{
		Usage:      usage.ToModelUsage(),
		UpstreamID: upstreamID,
	}, nil
}

func GetUsageOrChoicesResponseFromNode(
	node *ast.Node,
) (*relaymodel.ChatUsage, []*relaymodel.TextResponseChoice, error) {
	usageNode := node.Get("usage")
	if usageNode != nil && usageNode.TypeSafe() != ast.V_NULL {
		usageRaw, err := usageNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			var usage relaymodel.ChatUsage

			err = sonic.UnmarshalString(usageRaw, &usage)
			if err != nil {
				return nil, nil, err
			}

			return &usage, nil, nil
		}
	}

	var choices []*relaymodel.TextResponseChoice

	choicesNode := node.Get("choices")
	if choicesNode != nil && choicesNode.TypeSafe() != ast.V_NULL {
		choicesRaw, err := choicesNode.Raw()
		if err != nil {
			if !errors.Is(err, ast.ErrNotExist) {
				return nil, nil, err
			}
		} else {
			err = sonic.UnmarshalString(choicesRaw, &choices)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return nil, choices, nil
}

func Handler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
	preHandler PreHandler,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	node, err := common.UnmarshalResponse2Node(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	if preHandler != nil {
		err := preHandler(meta, &node)
		if err != nil {
			return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
				err,
				"pre_handler_failed",
				http.StatusInternalServerError,
			)
		}
	}

	usage, choices, err := GetUsageOrChoicesResponseFromNode(&node)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Extract upstream ID from response if available
	var upstreamID string
	if idNode := node.Get("id"); idNode.Exists() && idNode.TypeSafe() != ast.V_NULL {
		if id, err := idNode.String(); err == nil && id != "" {
			upstreamID = id
		}
	}

	if usage == nil ||
		usage.TotalTokens == 0 ||
		(usage.PromptTokens == 0 && usage.CompletionTokens == 0) {
		var completionTokens int64
		for _, choice := range choices {
			if choice.Text != "" {
				completionTokens += CountTokenText(choice.Text, meta.ActualModel)
				continue
			}

			completionTokens += CountTokenText(choice.Message.StringContent(), meta.ActualModel)
		}

		usage = &relaymodel.ChatUsage{
			PromptTokens:     int64(meta.RequestUsage.InputTokens),
			CompletionTokens: completionTokens,
			TotalTokens:      int64(meta.RequestUsage.InputTokens) + completionTokens,
		}

		_, err = node.Set("usage", ast.NewAny(usage))
		if err != nil {
			responseErr := relaymodel.WrapperOpenAIError(
				err,
				"set_usage_failed",
				http.StatusInternalServerError,
			)

			return adaptor.DoResponseResult{
				Usage:      usage.ToModelUsage(),
				UpstreamID: upstreamID,
			}, responseErr
		}
	} else if usage.TotalTokens != 0 && usage.PromptTokens == 0 { // some channels don't return prompt tokens & completion tokens
		usage.PromptTokens = int64(meta.RequestUsage.InputTokens)
		usage.CompletionTokens = usage.TotalTokens - int64(meta.RequestUsage.InputTokens)

		_, err = node.Set("usage", ast.NewAny(usage))
		if err != nil {
			responseErr := relaymodel.WrapperOpenAIError(
				err,
				"set_usage_failed",
				http.StatusInternalServerError,
			)

			return adaptor.DoResponseResult{
				Usage:      usage.ToModelUsage(),
				UpstreamID: upstreamID,
			}, responseErr
		}
	}

	_, err = node.Set("model", ast.NewString(meta.OriginModel))
	if err != nil {
		responseErr := relaymodel.WrapperOpenAIError(
			err,
			"set_model_failed",
			http.StatusInternalServerError,
		)

		return adaptor.DoResponseResult{
			Usage:      usage.ToModelUsage(),
			UpstreamID: upstreamID,
		}, responseErr
	}

	newData, err := sonic.Marshal(&node)
	if err != nil {
		responseErr := relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)

		return adaptor.DoResponseResult{
			Usage:      usage.ToModelUsage(),
			UpstreamID: upstreamID,
		}, responseErr
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(newData)))

	_, err = c.Writer.Write(newData)
	if err != nil {
		log.Warnf("write response body failed: %v", err)
	}

	return adaptor.DoResponseResult{
		Usage:      usage.ToModelUsage(),
		UpstreamID: upstreamID,
	}, nil
}

// CleanToolParameters removes null or empty required field from tool parameters
// Responses API requires the 'required' field to be either:
// - A non-empty array of strings
// - Completely absent from the schema
// It cannot be null or an empty array
func CleanToolParameters(parameters any) any {
	if params, ok := parameters.(map[string]any); ok {
		if required, hasRequired := params["required"]; hasRequired {
			// Remove if null or empty array
			if required == nil {
				delete(params, "required")
			} else if reqArray, ok := required.([]any); ok && len(reqArray) == 0 {
				delete(params, "required")
			}
		}

		return params
	}

	return parameters
}

// CleanToolParametersFromNode cleans tool parameters in an AST node
// It removes null or empty required fields from tool function parameters
func CleanToolParametersFromNode(node *ast.Node) error {
	toolsNode := node.Get("tools")
	if !toolsNode.Exists() || toolsNode.TypeSafe() == ast.V_NULL {
		return nil
	}

	if toolsNode.TypeSafe() != ast.V_ARRAY {
		return nil
	}

	// Iterate through each tool using ForEach
	err := toolsNode.ForEach(func(path ast.Sequence, toolNode *ast.Node) bool {
		// Get function node
		functionNode := toolNode.Get("function")
		if !functionNode.Exists() {
			return true // Continue to next tool
		}

		// Get parameters node
		parametersNode := functionNode.Get("parameters")
		if !parametersNode.Exists() || parametersNode.TypeSafe() == ast.V_NULL {
			return true // Continue to next tool
		}

		// Get required node
		requiredNode := parametersNode.Get("required")
		if !requiredNode.Exists() {
			return true // Continue to next tool
		}

		// Check if required is null or empty array
		shouldRemove := false
		if requiredNode.TypeSafe() == ast.V_NULL {
			shouldRemove = true
		} else if requiredNode.TypeSafe() == ast.V_ARRAY {
			requiredArray, err := requiredNode.ArrayUseNode()
			if err == nil && len(requiredArray) == 0 {
				shouldRemove = true
			}
		}

		// Remove required field if needed
		if shouldRemove {
			// Use Unset to directly remove the required field
			_, _ = parametersNode.Unset("required")
		}

		return true // Continue to next tool
	})

	return err
}

// ConvertToolsToResponseTools converts OpenAI Tool format to Responses API format
func ConvertToolsToResponseTools(tools []relaymodel.Tool) []relaymodel.ResponseTool {
	responseTools := make([]relaymodel.ResponseTool, 0, len(tools))

	for _, tool := range tools {
		name := tool.Name
		if name == "" {
			name = tool.Function.Name
		}

		description := tool.Description
		if description == "" {
			description = tool.Function.Description
		}

		parameters := tool.Parameters
		if parameters == nil {
			parameters = tool.Function.Parameters
		}

		strict := tool.Strict
		if strict == nil {
			strict = tool.Function.Strict
		}

		responseTool := relaymodel.ResponseTool{
			Type:         tool.Type,
			Name:         name,
			Execution:    tool.Execution,
			Description:  description,
			Parameters:   CleanToolParameters(parameters),
			Strict:       strict,
			DeferLoading: tool.DeferLoading,
		}
		responseTools = append(responseTools, responseTool)
	}

	return responseTools
}

func convertLegacyFunctionsToResponseTools(functions any) []relaymodel.ResponseTool {
	functionList, ok := functions.([]any)
	if !ok || len(functionList) == 0 {
		return nil
	}

	responseTools := make([]relaymodel.ResponseTool, 0, len(functionList))
	for _, function := range functionList {
		functionMap, ok := function.(map[string]any)
		if !ok {
			continue
		}

		name, _ := functionMap["name"].(string)
		if name == "" {
			continue
		}

		description, _ := functionMap["description"].(string)

		responseTools = append(responseTools, relaymodel.ResponseTool{
			Type:        relaymodel.ToolChoiceTypeFunction,
			Name:        name,
			Description: description,
			Parameters:  CleanToolParameters(functionMap["parameters"]),
		})
	}

	return responseTools
}

func convertLegacyFunctionCallToResponseToolChoice(functionCall any) any {
	switch value := functionCall.(type) {
	case string:
		switch value {
		case "auto", "none":
			return value
		default:
			return nil
		}
	case map[string]any:
		name, _ := value["name"].(string)
		if name == "" {
			return nil
		}

		return map[string]any{
			"type": "function",
			"name": name,
		}
	default:
		return nil
	}
}

func convertChatToolChoiceToResponseToolChoice(toolChoice any) any {
	toolChoiceMap, ok := toolChoice.(map[string]any)
	if !ok {
		return toolChoice
	}

	toolType, _ := toolChoiceMap["type"].(string)

	functionMap, ok := toolChoiceMap["function"].(map[string]any)
	if !ok || toolType != relaymodel.ToolChoiceTypeFunction {
		return toolChoice
	}

	name, _ := functionMap["name"].(string)
	if name == "" {
		return toolChoice
	}

	return map[string]any{
		"type": relaymodel.ToolChoiceTypeFunction,
		"name": name,
	}
}

func convertChatResponseFormatToResponseText(
	responseFormat *relaymodel.ResponseFormat,
) *relaymodel.ResponseText {
	if responseFormat == nil || responseFormat.Type == "" {
		return nil
	}

	format := relaymodel.ResponseTextFormat{
		Type: responseFormat.Type,
	}

	if responseFormat.JSONSchema != nil {
		format.Name = responseFormat.JSONSchema.Name
		format.Schema = responseFormat.JSONSchema.Schema
		format.Strict = responseFormat.JSONSchema.Strict
		format.Description = responseFormat.JSONSchema.Description
	}

	return &relaymodel.ResponseText{Format: format}
}

func appendUniqueString(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}

	return append(values, value)
}

func appendChatContentPartToResponseInput(
	inputItem *relaymodel.InputItem,
	contentType relaymodel.InputContentType,
	part map[string]any,
) {
	partType, _ := part["type"].(string)
	switch partType {
	case relaymodel.ContentTypeText:
		text, _ := part["text"].(string)
		if text == "" {
			return
		}

		inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
			Type:                  contentType,
			Text:                  text,
			PromptCacheBreakpoint: part["prompt_cache_breakpoint"],
		})
	case relaymodel.ContentTypeImageURL:
		imageURL, _ := part["image_url"].(map[string]any)

		url, _ := imageURL["url"].(string)
		if url == "" {
			return
		}

		detail, _ := imageURL["detail"].(string)
		inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
			Type:                  "input_image",
			ImageURL:              url,
			Detail:                detail,
			PromptCacheBreakpoint: part["prompt_cache_breakpoint"],
		})
	}
}

func appendMessageContentToResponseInput(
	inputItem *relaymodel.InputItem,
	contentType relaymodel.InputContentType,
	content any,
) {
	switch content := content.(type) {
	case string:
		if content != "" {
			inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
				Type: contentType,
				Text: content,
			})
		}
	case []relaymodel.MessageContent:
		for _, part := range content {
			switch part.Type {
			case relaymodel.ContentTypeText:
				if part.Text != "" {
					inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
						Type:                  contentType,
						Text:                  part.Text,
						PromptCacheBreakpoint: part.PromptCacheBreakpoint,
					})
				}
			case relaymodel.ContentTypeImageURL:
				if part.ImageURL != nil && part.ImageURL.URL != "" {
					inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
						Type:                  "input_image",
						ImageURL:              part.ImageURL.URL,
						Detail:                part.ImageURL.Detail,
						PromptCacheBreakpoint: part.PromptCacheBreakpoint,
					})
				}
			}
		}
	case []any:
		for _, part := range content {
			if partMap, ok := part.(map[string]any); ok {
				appendChatContentPartToResponseInput(inputItem, contentType, partMap)
			}
		}
	}
}

// ConvertMessagesToInputItems converts Message array to InputItem array for Responses API
func ConvertMessagesToInputItems(messages []relaymodel.Message) []relaymodel.InputItem {
	inputItems := make([]relaymodel.InputItem, 0, len(messages))

	for _, msg := range messages {
		// Handle tool responses (function results from tool role)
		if msg.Role == relaymodel.RoleTool && msg.ToolCallID != "" {
			// Extract the actual content from the tool message
			var output string
			switch content := msg.Content.(type) {
			case string:
				output = content
			default:
				// Try to marshal non-string content
				if data, err := sonic.MarshalString(content); err == nil {
					output = data
				}
			}

			// Create separate InputItem for function call output
			inputItems = append(inputItems, relaymodel.InputItem{
				Type:   relaymodel.InputItemTypeFunctionCallOutput,
				CallID: msg.ToolCallID,
				Output: output,
			})

			continue
		}

		// Handle tool calls (function calls from assistant)
		if len(msg.ToolCalls) > 0 {
			// Create separate InputItems for each function call
			for _, toolCall := range msg.ToolCalls {
				inputItems = append(inputItems, relaymodel.InputItem{
					Type:      relaymodel.InputItemTypeFunctionCall,
					CallID:    toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: toolCall.Function.Arguments,
				})
			}

			messageItem := relaymodel.InputItem{
				Type:    relaymodel.InputItemTypeMessage,
				Role:    msg.Role,
				Content: make([]relaymodel.InputContent, 0),
			}
			appendMessageContentToResponseInput(
				&messageItem,
				relaymodel.InputContentTypeOutputText,
				msg.Content,
			)

			if msg.Refusal != "" {
				messageItem.Content = append(messageItem.Content, relaymodel.InputContent{
					Type:    "refusal",
					Refusal: msg.Refusal,
				})
			}

			if len(messageItem.Content) > 0 {
				inputItems = append(inputItems, messageItem)
			}

			continue
		}

		// Handle regular messages
		role := msg.Role
		// Tool role without ToolCallID is treated as user role
		switch role {
		case relaymodel.RoleTool:
			role = relaymodel.RoleUser
		case relaymodel.RoleSystem:
			role = relaymodel.RoleDeveloper
		}

		inputItem := relaymodel.InputItem{
			Type:    relaymodel.InputItemTypeMessage,
			Role:    role,
			Content: make([]relaymodel.InputContent, 0),
		}

		// Determine content type based on role
		// assistant uses 'output_text', others use 'input_text'
		contentType := relaymodel.InputContentTypeInputText
		if role == relaymodel.RoleAssistant {
			contentType = relaymodel.InputContentTypeOutputText
		}

		appendMessageContentToResponseInput(&inputItem, contentType, msg.Content)

		if role == relaymodel.RoleAssistant && msg.Refusal != "" {
			inputItem.Content = append(inputItem.Content, relaymodel.InputContent{
				Type:    "refusal",
				Refusal: msg.Refusal,
			})
		}

		// Only append the message if it has content
		if len(inputItem.Content) > 0 {
			inputItems = append(inputItems, inputItem)
		}
	}

	return inputItems
}

// ConvertChatCompletionToResponsesRequest converts a ChatCompletion request to Responses API format
func ConvertChatCompletionToResponsesRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	// Parse ChatCompletion request
	var chatReq relaymodel.GeneralOpenAIRequest

	err := common.UnmarshalRequestReusable(req, &chatReq)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if chatReq.N > 1 {
		return adaptor.ConvertResult{}, convertRequestError(
			meta,
			"n must be 1 when converting Chat Completions requests to the Responses API",
		)
	}

	// Create Responses API request
	responsesReq := relaymodel.CreateResponseRequest{
		Model:  meta.ActualModel,
		Input:  ConvertMessagesToInputItems(chatReq.Messages),
		Stream: chatReq.Stream,
	}

	// Map common fields
	if chatReq.Temperature != nil {
		responsesReq.Temperature = chatReq.Temperature
	}

	if chatReq.TopP != nil {
		responsesReq.TopP = chatReq.TopP
	}

	if chatReq.ResponseFormat != nil {
		responsesReq.Text = convertChatResponseFormatToResponseText(chatReq.ResponseFormat)
	}

	if chatReq.Verbosity != "" {
		if responsesReq.Text == nil {
			responsesReq.Text = &relaymodel.ResponseText{
				Format: relaymodel.ResponseTextFormat{Type: "text"},
			}
		}

		responsesReq.Text.Verbosity = chatReq.Verbosity
	}

	if chatReq.TopLogprobs != nil {
		responsesReq.TopLogprobs = chatReq.TopLogprobs
	}

	if chatReq.Logprobs != nil && *chatReq.Logprobs {
		responsesReq.Include = appendUniqueString(
			responsesReq.Include,
			"message.output_text.logprobs",
		)
	}

	if chatReq.MaxTokens > 0 {
		responsesReq.MaxOutputTokens = &chatReq.MaxTokens
	} else if chatReq.MaxCompletionTokens > 0 {
		responsesReq.MaxOutputTokens = &chatReq.MaxCompletionTokens
	}

	// Map tools
	if len(chatReq.Tools) > 0 {
		responsesReq.Tools = ConvertToolsToResponseTools(chatReq.Tools)
	} else if chatReq.Functions != nil {
		responsesReq.Tools = convertLegacyFunctionsToResponseTools(chatReq.Functions)
	}

	if chatReq.ToolChoice != nil {
		responsesReq.ToolChoice = convertChatToolChoiceToResponseToolChoice(chatReq.ToolChoice)
	} else if chatReq.FunctionCall != nil {
		responsesReq.ToolChoice = convertLegacyFunctionCallToResponseToolChoice(
			chatReq.FunctionCall,
		)
	}

	if chatReq.ParallelToolCalls != nil {
		responsesReq.ParallelToolCalls = chatReq.ParallelToolCalls
	}

	// Map service tier
	if chatReq.ServiceTier != "" {
		responsesReq.ServiceTier = &chatReq.ServiceTier
	}

	// Map prompt cache key
	if chatReq.PromptCacheKey != "" {
		responsesReq.PromptCacheKey = &chatReq.PromptCacheKey
	}

	responsesReq.PromptCacheOptions = chatReq.PromptCacheOptions

	// Map prompt cache retention
	if chatReq.PromptCacheRetention != "" {
		responsesReq.PromptCacheRetention = &chatReq.PromptCacheRetention
	}

	// Map user
	if chatReq.User != "" {
		responsesReq.User = &chatReq.User
	}

	if chatReq.SafetyIdentifier != "" {
		responsesReq.SafetyIdentifier = &chatReq.SafetyIdentifier
	}

	responsesReq.Moderation = chatReq.Moderation

	if chatReq.StreamOptions != nil {
		if chatReq.StreamOptions.IncludeObfuscation != nil {
			responsesReq.StreamOptions = &relaymodel.ResponseStreamOptions{
				IncludeObfuscation: chatReq.StreamOptions.IncludeObfuscation,
			}
		}
	}

	reasoning := utils.ParseOpenAIReasoning(&chatReq)
	applyReasoningToResponsesRequestForModel(meta, &responsesReq, reasoning)

	// Map metadata
	if chatReq.Metadata != nil {
		if metadata, ok := chatReq.Metadata.(map[string]any); ok {
			responsesReq.Metadata = metadata
		}
	}

	// Force non-store mode
	storeValue := false
	responsesReq.Store = &storeValue

	// Marshal to JSON
	jsonData, err := sonic.Marshal(responsesReq)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(jsonData))},
		},
		Body: bytes.NewReader(jsonData),
	}, nil
}

// ConvertResponsesToChatCompletionResponse converts Responses API response to ChatCompletion format
func ConvertResponsesToChatCompletionResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	responseBody, err := common.GetResponseBody(resp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"read_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var responsesResp relaymodel.Response

	err = sonic.Unmarshal(responseBody, &responsesResp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	// Convert to ChatCompletion format
	chatResp := relaymodel.TextResponse{
		ID:          responsesResp.ID,
		Object:      relaymodel.ChatCompletionObject,
		Created:     responsesResp.CreatedAt,
		Model:       responseModelName(meta),
		Choices:     []*relaymodel.TextResponseChoice{},
		Usage:       relaymodel.ChatUsage{},
		Moderation:  responsesResp.Moderation,
		ServiceTier: responsesResp.ServiceTier,
	}

	reasonContent := responseReasoningSummaryText(&responsesResp)
	choice := &relaymodel.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:             relaymodel.RoleAssistant,
			Content:          "",
			ReasoningContent: reasonContent,
		},
		FinishReason: responseToChatFinishReason(&responsesResp),
	}

	var (
		contentParts    []string
		refusalParts    []string
		contentLogprobs []relaymodel.ChatCompletionTokenLogprob
		refusalLogprobs []relaymodel.ChatCompletionTokenLogprob
	)

	// Responses output items belong to one generation and therefore one Chat choice.
	for _, outputItem := range responsesResp.Output {
		switch outputItem.Type {
		case "", relaymodel.InputItemTypeMessage:
			if outputItem.Role != "" {
				choice.Message.Role = outputItem.Role
			}

			for _, content := range outputItem.Content {
				switch content.Type {
				case "text", relaymodel.OutputContentTypeOutputText:
					if content.Text != "" {
						contentParts = append(contentParts, content.Text)
					}

					choice.Message.Annotations = append(
						choice.Message.Annotations,
						content.Annotations...,
					)
					contentLogprobs = append(contentLogprobs, content.Logprobs...)
				case "refusal":
					if content.Refusal != "" {
						refusalParts = append(refusalParts, content.Refusal)
					}

					refusalLogprobs = append(refusalLogprobs, content.Logprobs...)
				}
			}

		case relaymodel.InputItemTypeFunctionCall:
			toolCallID := outputItem.CallID
			if toolCallID == "" {
				toolCallID = outputItem.ID
			}

			if choice.FinishReason == relaymodel.FinishReasonStop {
				choice.FinishReason = relaymodel.FinishReasonToolCalls
			}

			choice.Message.ToolCalls = append(
				choice.Message.ToolCalls,
				relaymodel.ToolCall{
					Index: len(choice.Message.ToolCalls),
					ID:    toolCallID,
					Type:  relaymodel.ToolChoiceTypeFunction,
					Function: relaymodel.Function{
						Name:      outputItem.Name,
						Arguments: outputItem.Arguments.String(),
					},
				},
			)

		default:
			continue
		}
	}

	if len(contentParts) > 0 {
		choice.Message.Content = strings.Join(contentParts, "\n")
	}

	if len(refusalParts) > 0 {
		choice.Message.Refusal = strings.Join(refusalParts, "\n")
	}

	if len(contentLogprobs) > 0 || len(refusalLogprobs) > 0 {
		choice.Logprobs = &relaymodel.ChoiceLogprobs{
			Content: contentLogprobs,
			Refusal: refusalLogprobs,
		}
	}

	chatResp.Choices = append(chatResp.Choices, choice)

	// Convert usage
	if responsesResp.Usage != nil {
		chatResp.Usage = responsesResp.Usage.ToChatUsage()
	}

	// Marshal and return
	chatRespData, err := sonic.Marshal(chatResp)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"marshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(chatRespData)))
	_, _ = c.Writer.Write(chatRespData)

	return adaptor.DoResponseResult{
		Usage:      responsesResp.ToModelUsage(),
		UpstreamID: responsesResp.ID,
		AsyncUsage: responseNeedsAsyncUsage(&responsesResp),
	}, nil
}

type responsesStreamErrorState struct {
	usage          model.Usage
	responseID     string
	lastResponse   *relaymodel.Response
	pendingFailure *relaymodel.ResponseStreamEvent
}

func (s *responsesStreamErrorState) update(event *relaymodel.ResponseStreamEvent) {
	if event.Response == nil {
		return
	}

	if s.responseID == "" {
		s.responseID = event.Response.ID
	}

	s.lastResponse = event.Response
	s.usage = event.Response.ToModelUsage()
}

func (s *responsesStreamErrorState) result() adaptor.DoResponseResult {
	asyncUsage := responseNeedsAsyncUsage(s.lastResponse)
	if s.pendingFailure != nil {
		asyncUsage = false
	}

	return adaptor.DoResponseResult{
		Usage:      s.usage,
		UpstreamID: s.responseID,
		AsyncUsage: asyncUsage,
	}
}

func (s *responsesStreamErrorState) errorBeforeEvent(
	event *relaymodel.ResponseStreamEvent,
) adaptor.Error {
	if s.pendingFailure == nil {
		return nil
	}

	if event.Type == relaymodel.EventError {
		return responseStreamError(event)
	}

	return responseStreamError(s.pendingFailure)
}

func (s *responsesStreamErrorState) handleFailure(
	event *relaymodel.ResponseStreamEvent,
) (adaptor.Error, bool) {
	if event.Type != relaymodel.EventResponseFailed && event.Type != relaymodel.EventError {
		return nil, false
	}

	if event.Type == relaymodel.EventResponseFailed && event.Response != nil &&
		event.Response.Error == nil {
		s.update(event)

		pendingEvent := *event
		s.pendingFailure = &pendingEvent

		return nil, true
	}

	return responseStreamError(event), true
}

func responseStreamEventCanDelay(eventType string) bool {
	switch eventType {
	case relaymodel.EventResponseCreated,
		relaymodel.EventResponseInProgress,
		relaymodel.EventResponseQueued,
		relaymodel.EventKeepAlive:
		return true
	default:
		return false
	}
}

// ConvertResponsesToChatCompletionStreamResponse converts Responses API stream to ChatCompletion stream
func ConvertResponsesToChatCompletionStreamResponse(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.OriginModel, meta.ActualModel)
	defer cleanup()

	var (
		usage               model.Usage
		responseID          string
		lastResponse        *relaymodel.Response
		pendingInitialChunk *relaymodel.ChatCompletionsStreamResponse
		wroteStream         bool
	)

	errorState := responsesStreamErrorState{}

	state := &chatCompletionStreamState{
		meta:                       meta,
		c:                          c,
		toolCallIndexByItemID:      make(map[string]int),
		toolCallIndexByOutputIndex: make(map[int]int),
	}
	stopStream := false

	var writeChatStreamResp func(*relaymodel.ChatCompletionsStreamResponse)

	writeChatStreamResp = func(chatStreamResp *relaymodel.ChatCompletionsStreamResponse) {
		if chatStreamResp == nil {
			return
		}

		if pendingInitialChunk != nil {
			initialChunk := pendingInitialChunk
			pendingInitialChunk = nil

			writeChatStreamResp(initialChunk)
		}

		chunkData, err := sonic.Marshal(chatStreamResp)
		if err != nil {
			log.Error("error marshalling chat stream response: " + err.Error())
			return
		}

		render.OpenaiBytesData(c, chunkData)

		wroteStream = true
	}

	for scanner.Scan() && !stopStream {
		data := scanner.Bytes()

		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)

		// Parse the stream event
		var event relaymodel.ResponseStreamEvent

		err := sonic.Unmarshal(data, &event)
		if err != nil {
			log.Error("error unmarshalling response stream: " + err.Error())
			continue
		}

		if event.Response != nil {
			if responseID == "" {
				responseID = event.Response.ID
			}

			lastResponse = event.Response
			usage = event.Response.ToModelUsage()
		}

		errorState.usage = usage
		errorState.responseID = responseID
		errorState.lastResponse = lastResponse

		if err := errorState.errorBeforeEvent(&event); err != nil {
			return errorState.result(), err
		}

		// Handle event and get response
		var chatStreamResp *relaymodel.ChatCompletionsStreamResponse

		switch event.Type {
		case relaymodel.EventResponseCreated:
			pendingInitialChunk = state.handleResponseCreated(&event)
		case relaymodel.EventOutputTextDelta:
			chatStreamResp = state.handleOutputTextDelta(&event)
		case relaymodel.EventRefusalDelta:
			chatStreamResp = state.handleRefusalDelta(&event)
		case relaymodel.EventReasoningSummaryTextDelta:
			chatStreamResp = state.handleReasoningSummaryTextDelta(&event)
		case relaymodel.EventOutputItemAdded:
			chatStreamResp = state.handleOutputItemAdded(&event)
		case relaymodel.EventFunctionCallArgumentsDelta:
			chatStreamResp = state.handleFunctionCallArgumentsDelta(&event)
		case relaymodel.EventResponseCompleted,
			relaymodel.EventResponseIncomplete,
			relaymodel.EventResponseDone:
			chatStreamResp = state.handleResponseCompleted(&event)
		case relaymodel.EventResponseFailed, relaymodel.EventError:
			if wroteStream {
				log.Error(
					"response stream failed after data was sent: " + responseStreamErrorMessage(
						&event,
					),
				)

				stopStream = true

				break
			}

			err, handled := errorState.handleFailure(&event)
			if handled && err == nil {
				continue
			}

			if handled {
				return errorState.result(), err
			}
		}

		writeChatStreamResp(chatStreamResp)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading response stream: " + err.Error())
	}

	if errorState.pendingFailure != nil && !wroteStream {
		return errorState.result(), responseStreamError(errorState.pendingFailure)
	}

	if wroteStream {
		render.OpenaiDone(c)
	}

	return adaptor.DoResponseResult{
		Usage:      usage,
		UpstreamID: responseID,
		AsyncUsage: responseNeedsAsyncUsage(lastResponse),
	}, nil
}

func responseStreamError(event *relaymodel.ResponseStreamEvent) adaptor.Error {
	openAIError := relaymodel.OpenAIError{
		Message: responseStreamErrorMessage(event),
		Type:    relaymodel.ErrorTypeUpstream,
		Code:    relaymodel.ErrorCodeBadResponse,
	}
	statusCode := http.StatusBadGateway

	if event.Error != nil {
		openAIError = *event.Error
		if openAIError.Type == "" {
			openAIError.Type = relaymodel.ErrorTypeUpstream
		}

		if openAIError.Message == "" {
			openAIError.Message = responseStreamErrorMessage(event)
		}

		if openAIError.Code == nil {
			openAIError.Code = relaymodel.ErrorCodeBadResponse
		}
	}

	if event.Response != nil && event.Response.Error != nil {
		openAIError.Message = event.Response.Error.Message
		if event.Response.Error.Code != "" {
			openAIError.Code = event.Response.Error.Code
		}
	}

	if status, ok := streamErrorStatusCode(openAIError.Code); ok {
		statusCode = status
	} else if status, ok := streamErrorStatusCode(openAIError.Type); ok {
		statusCode = status
	} else if status, ok := streamErrorStatusCode(openAIError.Message); ok {
		statusCode = status
	}

	return relaymodel.NewOpenAIError(statusCode, openAIError)
}

func streamErrorStatusCode(code any) (int, bool) {
	switch value := code.(type) {
	case int:
		return statusCodeFromNumericCode(value)
	case int64:
		return statusCodeFromNumericCode(int(value))
	case float64:
		if value == float64(int(value)) {
			return statusCodeFromNumericCode(int(value))
		}
	case string:
		if status, err := strconv.Atoi(value); err == nil {
			return statusCodeFromNumericCode(status)
		}

		switch value {
		case "too_many_requests", "rate_limit_exceeded":
			return http.StatusTooManyRequests, true
		case "invalid_request_error", "bad_request", "bad_request_error", "invalid_request":
			return http.StatusBadRequest, true
		}

		lowerValue := strings.ToLower(value)
		if strings.Contains(lowerValue, "system messages are not allowed") {
			return http.StatusBadRequest, true
		}
	}

	return 0, false
}

func statusCodeFromNumericCode(code int) (int, bool) {
	if code >= http.StatusBadRequest && code < 600 {
		return code, true
	}

	return 0, false
}

func responseStreamErrorMessage(event *relaymodel.ResponseStreamEvent) string {
	if event.Error != nil && event.Error.Message != "" {
		return event.Error.Message
	}

	if event.Response != nil && event.Response.Error != nil && event.Response.Error.Message != "" {
		return event.Response.Error.Message
	}

	if event.Type != "" {
		return "response stream failed: " + event.Type
	}

	return "response stream failed"
}
