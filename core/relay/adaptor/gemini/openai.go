package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/image"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
	"github.com/labring/aiproxy/core/relay/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

var toolChoiceTypeMap = map[string]string{
	relaymodel.ToolChoiceNone:     relaymodel.GeminiFunctionCallingModeNone,
	relaymodel.ToolChoiceAuto:     relaymodel.GeminiFunctionCallingModeAuto,
	relaymodel.ToolChoiceRequired: relaymodel.GeminiFunctionCallingModeAny,
}

var mimeTypeMap = map[string]string{
	"json_object": "application/json",
	"text":        "text/plain",
}

var geminiAudioMimeTypes = map[string]string{
	"aac":  "audio/aac",
	"flac": "audio/flac",
	"mp3":  "audio/mp3",
	"m4a":  "audio/mp4",
	"mp4":  "audio/mp4",
	"mpeg": "audio/mpeg",
	"mpga": "audio/mpeg",
	"oga":  "audio/ogg",
	"ogg":  "audio/ogg",
	"opus": "audio/opus",
	"wav":  "audio/wav",
	"webm": "audio/webm",
}

var geminiVideoMimeTypes = map[string]string{
	"avi":       "video/x-msvideo",
	"flv":       "video/x-flv",
	"mov":       "video/quicktime",
	"mp4":       "video/mp4",
	"mpeg":      "video/mpeg",
	"mpg":       "video/mpeg",
	"mpg4":      "video/mp4",
	"ogv":       "video/ogg",
	"webm":      "video/webm",
	"wmv":       "video/x-ms-wmv",
	"x-ms-wmv":  "video/x-ms-wmv",
	"x-msvideo": "video/x-msvideo",
}

const maxGeminiMediaSize = 1024 * 1024 * 50

func shouldAutoIncludeThoughts(modelName string) bool {
	modelName = strings.ToLower(modelName)

	if !strings.Contains(modelName, "-2.5") && !strings.Contains(modelName, "-3") {
		return false
	}

	// Gemini 2.5 Flash Lite defaults to non-thinking mode, so includeThoughts
	// must not be injected unless thinking is explicitly enabled.
	if strings.Contains(modelName, "2.5-flash-lite") {
		return false
	}

	if strings.Contains(modelName, "tts") {
		return false
	}

	return true
}

func resolveGeminiFeatureModel(meta *meta.Meta) string {
	if meta == nil {
		return ""
	}

	if modelName := utils.FirstMatchingModelName(
		func(modelName string) bool {
			return strings.Contains(strings.ToLower(modelName), "gemini")
		},
		meta.OriginModel,
		meta.ActualModel,
	); modelName != "" {
		return modelName
	}

	return utils.PreferredModelName(meta.OriginModel, meta.ActualModel)
}

func isGeminiImageModel(meta *meta.Meta) bool {
	return isGeminiImageMeta(meta)
}

func isGeminiImageMeta(meta *meta.Meta) bool {
	if meta != nil && meta.ModelConfig.Type == mode.GeminiImage {
		return true
	}

	return strings.Contains(strings.ToLower(resolveGeminiFeatureModel(meta)), "image")
}

func IsImageMetaForAdaptor(meta *meta.Meta) bool {
	return isGeminiImageMeta(meta)
}

func isGeminiTTSModel(meta *meta.Meta) bool {
	return isGeminiTTSMeta(meta)
}

func isGeminiTTSMeta(meta *meta.Meta) bool {
	if meta != nil && meta.ModelConfig.Type == mode.GeminiTTS {
		return true
	}

	return strings.Contains(strings.ToLower(resolveGeminiFeatureModel(meta)), "tts")
}

func IsTTSMetaForAdaptor(meta *meta.Meta) bool {
	return isGeminiTTSMeta(meta)
}

func autoImageURLToBase64Disabled(meta *meta.Meta, cfg Config) bool {
	if meta != nil {
		switch meta.Channel.Type {
		case model.ChannelTypeVertexAI, model.ChannelTypeAWS:
			return false
		}
	}

	return cfg.DisableAutoImageURLToBase64
}

func autoAudioURLToBase64Disabled(meta *meta.Meta, cfg Config) bool {
	if meta != nil && meta.Channel.Type == model.ChannelTypeVertexAI {
		return false
	}

	return cfg.DisableAutoAudioURLToBase64
}

func autoVideoURLToBase64Disabled(meta *meta.Meta, cfg Config) bool {
	if meta != nil && meta.Channel.Type == model.ChannelTypeVertexAI {
		return false
	}

	return cfg.DisableAutoVideoURLToBase64
}

type CountTokensResponse struct {
	Error       *relaymodel.GeminiError `json:"error,omitempty"`
	TotalTokens int                     `json:"totalTokens"`
}

func buildSafetySettings(safetySetting string) []relaymodel.GeminiChatSafetySettings {
	if safetySetting == "" {
		safetySetting = relaymodel.GeminiSafetyThresholdBlockNone
	}

	return []relaymodel.GeminiChatSafetySettings{
		{Category: relaymodel.GeminiSafetyCategoryHarassment, Threshold: safetySetting},
		{Category: relaymodel.GeminiSafetyCategoryHateSpeech, Threshold: safetySetting},
		{Category: relaymodel.GeminiSafetyCategorySexuallyExplicit, Threshold: safetySetting},
		{Category: relaymodel.GeminiSafetyCategoryDangerousContent, Threshold: safetySetting},
		{Category: relaymodel.GeminiSafetyCategoryCivicIntegrity, Threshold: safetySetting},
	}
}

func buildGenerationConfig(
	meta *meta.Meta,
	httpReq *http.Request,
	req *relaymodel.GeneralOpenAIRequest,
	textRequest *relaymodel.GeneralOpenAIRequest,
) *relaymodel.GeminiChatGenerationConfig {
	// First unmarshal generationConfig from request body if present
	var reqWithConfig struct {
		GenerationConfig *relaymodel.GeminiChatGenerationConfig `json:"generationConfig,omitempty"`
	}

	_ = common.UnmarshalRequestReusable(httpReq, &reqWithConfig)

	var config relaymodel.GeminiChatGenerationConfig
	if reqWithConfig.GenerationConfig != nil {
		config = *reqWithConfig.GenerationConfig
	}

	// Override with OpenAI-style parameters if provided
	if config.Temperature != nil && textRequest.Temperature != nil {
		config.Temperature = textRequest.Temperature
	}

	if config.TopP != nil && textRequest.TopP != nil {
		config.TopP = textRequest.TopP
	}

	// Convert MaxTokens (int) to MaxOutputTokens (*int)
	if config.MaxOutputTokens == nil && textRequest.MaxTokens != 0 {
		config.MaxOutputTokens = &textRequest.MaxTokens
	}

	if len(config.ResponseModalities) == 0 &&
		isGeminiImageModel(meta) {
		config.ResponseModalities = []string{
			"Text",
			"Image",
		}
	}

	if isGeminiTTSModel(meta) {
		if len(config.ResponseModalities) == 0 {
			config.ResponseModalities = []string{relaymodel.GeminiModalityAudio}
		}

		if config.SpeechConfig == nil {
			config.SpeechConfig = buildGeminiSpeechConfig(textRequest.Audio)
		}
	}

	if config.ResponseMimeType == "" && textRequest.ResponseFormat != nil {
		if mimeType, ok := mimeTypeMap[textRequest.ResponseFormat.Type]; ok {
			config.ResponseMimeType = mimeType
		}

		if textRequest.ResponseFormat.JSONSchema != nil {
			config.ResponseSchema = textRequest.ResponseFormat.JSONSchema.Schema
			cleanJSONSchema(config.ResponseSchema)
			config.ResponseMimeType = mimeTypeMap["json_object"]
		}
	}

	if config.ThinkingConfig == nil && !isGeminiTTSModel(meta) {
		utils.ApplyReasoningToGeminiConfig(
			meta.OriginModel,
			meta.ActualModel,
			&config,
			utils.ParseOpenAIReasoning(req),
		)
	}

	// https://ai.google.dev/gemini-api/docs/thinking
	if config.ThinkingConfig == nil && shouldAutoIncludeThoughts(resolveGeminiFeatureModel(meta)) {
		// disable vertexai image model include thoughts
		// because error call gemini-3-pro-image-preview model
		if meta.Channel.Type != model.ChannelTypeVertexAI ||
			!isGeminiImageModel(meta) {
			config.ThinkingConfig = &relaymodel.GeminiThinkingConfig{
				IncludeThoughts: true,
			}
		}
	}

	if isGeminiTTSModel(meta) {
		config.ThinkingConfig = nil
	}

	return &config
}

func buildGeminiSpeechConfig(audio *relaymodel.Audio) *relaymodel.GeminiSpeechConfig {
	voiceName := "Kore"
	if audio != nil && audio.Voice != "" {
		voiceName = audio.Voice
	}

	return &relaymodel.GeminiSpeechConfig{
		VoiceConfig: &relaymodel.GeminiVoiceConfig{
			PrebuiltVoiceConfig: &relaymodel.GeminiPrebuiltVoiceConfig{
				VoiceName: voiceName,
			},
		},
	}
}

func buildTTSSpeechConfig(voice string) *relaymodel.GeminiSpeechConfig {
	return buildGeminiSpeechConfig(&relaymodel.Audio{Voice: voice})
}

func buildTools(textRequest *relaymodel.GeneralOpenAIRequest) []relaymodel.GeminiChatTools {
	if textRequest.Tools != nil {
		functions := make([]relaymodel.Function, 0, len(textRequest.Tools))
		for _, tool := range textRequest.Tools {
			cleanedFunction := cleanFunctionParameters(tool.Function)
			functions = append(functions, cleanedFunction)
		}

		return []relaymodel.GeminiChatTools{{FunctionDeclarations: functions}}
	}

	if textRequest.Functions != nil {
		return []relaymodel.GeminiChatTools{{FunctionDeclarations: textRequest.Functions}}
	}

	return nil
}

func cleanFunctionParameters(function relaymodel.Function) relaymodel.Function {
	if function.Parameters == nil {
		return function
	}

	parameters, ok := function.Parameters.(map[string]any)
	if !ok {
		return function
	}

	cleanJSONSchema(parameters)

	if properties, ok := parameters["properties"].(map[string]any); ok {
		if len(properties) == 0 {
			function.Parameters = nil
			return function
		}
	}

	function.Parameters = parameters

	return function
}

var unsupportedFields = []string{
	"additionalProperties",
	"$schema",
	"$id",
	"$ref",
	"$defs",
	"exclusiveMinimum",
	"exclusiveMaximum",
}

var supportedFormats = map[string]struct{}{
	"enum":      {},
	"date-time": {},
}

func cleanJSONSchema(schema map[string]any) {
	for _, field := range unsupportedFields {
		delete(schema, field)
	}

	if format, exists := schema["format"]; exists {
		if formatStr, ok := format.(string); ok {
			if _, ok := supportedFormats[formatStr]; !ok {
				delete(schema, "format")
			}
		}
	}

	for _, field := range schema {
		switch v := field.(type) {
		case map[string]any:
			cleanJSONSchema(v)
		case []any:
			for _, item := range v {
				if itemMap, ok := item.(map[string]any); ok {
					cleanJSONSchema(itemMap)
				}
			}
		}
	}
}

func buildToolConfig(textRequest *relaymodel.GeneralOpenAIRequest) *relaymodel.GeminiToolConfig {
	if textRequest.ToolChoice == nil {
		return nil
	}

	defaultMode := relaymodel.GeminiFunctionCallingModeAuto
	if strings.Contains(textRequest.Model, "gemini-3") {
		defaultMode = ""
	}

	toolConfig := relaymodel.GeminiToolConfig{
		FunctionCallingConfig: relaymodel.GeminiFunctionCallingConfig{
			Mode: defaultMode,
		},
	}
	switch mode := textRequest.ToolChoice.(type) {
	case string:
		if toolChoiceType, ok := toolChoiceTypeMap[mode]; ok {
			toolConfig.FunctionCallingConfig.Mode = toolChoiceType
		}
	case map[string]any:
		toolConfig.FunctionCallingConfig.Mode = relaymodel.GeminiFunctionCallingModeAny
		if fn, ok := mode["function"].(map[string]any); ok {
			if fnName, ok := fn["name"].(string); ok {
				toolConfig.FunctionCallingConfig.AllowedFunctionNames = []string{fnName}
			}
		}
	}

	return &toolConfig
}

func buildMessageParts(
	message relaymodel.MessageContent,
) *relaymodel.GeminiPart {
	part := &relaymodel.GeminiPart{
		Text: message.Text,
	}
	if message.ImageURL != nil {
		imageURL := message.ImageURL.URL
		switch {
		case strings.HasPrefix(imageURL, "data:image/"):
			mimeType, data, err := image.GetImageFromURL(context.Background(), imageURL)
			if err == nil {
				part.InlineData = &relaymodel.GeminiInlineData{
					MimeType: mimeType,
					Data:     data,
				}
			} else {
				part.FileData = &relaymodel.GeminiFileData{
					FileURI: imageURL,
				}
			}
		default:
			part.FileData = &relaymodel.GeminiFileData{
				FileURI: imageURL,
			}
		}
	}

	if message.InputAudio != nil {
		return buildGeminiMediaPart(
			message.InputAudio.Data,
			message.InputAudio.URL,
			message.InputAudio.Format,
			"audio",
		)
	}

	if message.VideoURL != nil {
		return buildGeminiMediaPart("", message.VideoURL.URL, "", "video")
	}

	return part
}

func buildGeminiMediaPart(data, uri, format, mediaType string) *relaymodel.GeminiPart {
	part := &relaymodel.GeminiPart{}

	if uri == "" && isHTTPURL(data) {
		uri = data
		data = ""
	}

	if data != "" {
		if mimeType, base64Data, ok := parseMediaDataURL(data, mediaType); ok {
			part.InlineData = &relaymodel.GeminiInlineData{
				MimeType: mimeType,
				Data:     base64Data,
			}

			return part
		}

		mimeType := geminiMediaMIMEType(format, mediaType)
		part.InlineData = &relaymodel.GeminiInlineData{
			MimeType: mimeType,
			Data:     normalizeBase64Data(data),
		}

		return part
	}

	if uri == "" {
		return part
	}

	if mimeType, data, ok := parseMediaDataURL(uri, mediaType); ok {
		part.InlineData = &relaymodel.GeminiInlineData{
			MimeType: mimeType,
			Data:     data,
		}

		return part
	}

	part.FileData = &relaymodel.GeminiFileData{
		MimeType: firstNonEmpty(
			geminiMediaMIMEType(format, mediaType),
			mimeTypeFromURL(uri, mediaType),
		),
		FileURI: uri,
	}

	return part
}

func normalizeBase64Data(data string) string {
	if _, base64Data, ok := strings.Cut(data, ";base64,"); ok {
		return base64Data
	}

	return data
}

func parseMediaDataURL(dataURL, mediaType string) (string, string, bool) {
	prefix := "data:" + mediaType + "/"
	if !strings.HasPrefix(dataURL, prefix) {
		return "", "", false
	}

	mimeType, data, ok := strings.Cut(strings.TrimPrefix(dataURL, "data:"), ";base64,")
	if !ok || data == "" {
		return "", "", false
	}

	return mimeType, data, true
}

func geminiMediaMIMEType(format, mediaType string) string {
	format = strings.TrimSpace(strings.ToLower(format))
	format = strings.TrimPrefix(format, ".")

	if strings.Contains(format, "/") {
		return format
	}

	if format == "" {
		return ""
	}

	switch mediaType {
	case "audio":
		if mimeType, ok := geminiAudioMimeTypes[format]; ok {
			return mimeType
		}

		return "audio/" + format
	case "video":
		if mimeType, ok := geminiVideoMimeTypes[format]; ok {
			return mimeType
		}

		return "video/" + format
	default:
		return ""
	}
}

func mimeTypeFromURL(rawURL, mediaType string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	path := parsedURL.Path

	index := strings.LastIndex(path, ".")
	if index < 0 || index == len(path)-1 {
		return ""
	}

	return geminiMediaMIMEType(path[index+1:], mediaType)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func isHTTPURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

func parseToolCallArguments(arguments string) map[string]any {
	if arguments == "" {
		return make(map[string]any)
	}

	var args map[string]any
	if err := sonic.UnmarshalString(arguments, &args); err != nil {
		return make(map[string]any)
	}

	return args
}

func appendAssistantToolCalls(
	content *relaymodel.GeminiChatContent,
	toolCalls []relaymodel.ToolCall,
	toolCallMap map[string]string,
) {
	for _, toolCall := range toolCalls {
		toolCallMap[toolCall.ID] = toolCall.Function.Name

		part := &relaymodel.GeminiPart{
			FunctionCall: &relaymodel.GeminiFunctionCall{
				Name: toolCall.Function.Name,
				Args: parseToolCallArguments(toolCall.Function.Arguments),
			},
		}

		if toolCall.ExtraContent != nil &&
			toolCall.ExtraContent.Google != nil &&
			toolCall.ExtraContent.Google.ThoughtSignature != "" {
			part.ThoughtSignature = toolCall.ExtraContent.Google.ThoughtSignature
		} else {
			part.ThoughtSignature = ThoughtSignatureDummySkipValidator
		}

		content.Parts = append(content.Parts, part)
	}
}

func getToolResponseName(
	message relaymodel.Message,
	toolCallMap map[string]string,
) string {
	toolName := toolCallMap[message.ToolCallID]
	if toolName != "" {
		return toolName
	}

	if message.Name != nil {
		return *message.Name
	}

	return "tool_" + message.ToolCallID
}

func parseToolResponseContent(content any) map[string]any {
	if content == nil {
		return make(map[string]any)
	}

	switch c := content.(type) {
	case map[string]any:
		return c
	case string:
		var contentMap map[string]any
		if err := sonic.UnmarshalString(c, &contentMap); err != nil {
			return map[string]any{"result": c}
		}

		return contentMap
	default:
		return make(map[string]any)
	}
}

func appendToolResponse(
	content *relaymodel.GeminiChatContent,
	message relaymodel.Message,
	toolCallMap map[string]string,
) {
	toolName := getToolResponseName(message, toolCallMap)
	content.Parts = append(content.Parts, &relaymodel.GeminiPart{
		FunctionResponse: &relaymodel.GeminiFunctionResponse{
			Name: toolName,
			Response: map[string]any{
				"name":    toolName,
				"content": parseToolResponseContent(message.Content),
			},
		},
	})
}

func buildRegularMessageParts(
	message relaymodel.Message,
	collectImageTasks bool,
	collectAudioTasks bool,
	collectVideoTasks bool,
) (
	[]*relaymodel.GeminiPart,
	[]*relaymodel.GeminiPart,
	[]*relaymodel.GeminiPart,
	[]*relaymodel.GeminiPart,
) {
	openaiContent := message.ParseContent()
	if len(openaiContent) == 0 {
		return nil, nil, nil, nil
	}

	parts := make([]*relaymodel.GeminiPart, 0, len(openaiContent))

	imageTasks := make([]*relaymodel.GeminiPart, 0)
	audioTasks := make([]*relaymodel.GeminiPart, 0)
	videoTasks := make([]*relaymodel.GeminiPart, 0)

	for _, part := range openaiContent {
		msgPart := buildMessageParts(part)
		if msgPart.Text == "" && msgPart.InlineData == nil && msgPart.FileData == nil {
			continue
		}

		switch part.Type {
		case relaymodel.ContentTypeImageURL:
			if collectImageTasks && msgPart.FileData != nil {
				imageTasks = append(imageTasks, msgPart)
			}
		case relaymodel.ContentTypeInputAudio:
			if collectAudioTasks && msgPart.FileData != nil {
				audioTasks = append(audioTasks, msgPart)
			}
		case relaymodel.ContentTypeVideoURL:
			if collectVideoTasks && msgPart.FileData != nil {
				videoTasks = append(videoTasks, msgPart)
			}
		}

		parts = append(parts, msgPart)
	}

	return parts, imageTasks, audioTasks, videoTasks
}

func normalizeGeminiRole(role string) string {
	switch role {
	case relaymodel.RoleAssistant:
		return relaymodel.GeminiRoleModel
	case "tool":
		return relaymodel.GeminiRoleUser
	default:
		return role
	}
}

func mergeConsecutiveContents(
	contents []*relaymodel.GeminiChatContent,
) []*relaymodel.GeminiChatContent {
	mergedContents := make([]*relaymodel.GeminiChatContent, 0, len(contents))
	for _, content := range contents {
		if len(mergedContents) > 0 &&
			mergedContents[len(mergedContents)-1].Role == content.Role {
			mergedContents[len(mergedContents)-1].Parts = append(
				mergedContents[len(mergedContents)-1].Parts,
				content.Parts...,
			)

			continue
		}

		mergedContents = append(mergedContents, content)
	}

	return mergedContents
}

func buildContents(
	textRequest *relaymodel.GeneralOpenAIRequest,
	collectImageTasks bool,
	collectAudioTasks bool,
	collectVideoTasks bool,
) (
	*relaymodel.GeminiChatContent,
	[]*relaymodel.GeminiChatContent,
	[]*relaymodel.GeminiPart,
	[]*relaymodel.GeminiPart,
	[]*relaymodel.GeminiPart,
) {
	contents := make([]*relaymodel.GeminiChatContent, 0, len(textRequest.Messages))

	var (
		imageTasks    []*relaymodel.GeminiPart
		audioTasks    []*relaymodel.GeminiPart
		videoTasks    []*relaymodel.GeminiPart
		systemContent *relaymodel.GeminiChatContent
	)

	toolCallMap := make(map[string]string)

	for _, message := range textRequest.Messages {
		content := relaymodel.GeminiChatContent{
			Role: message.Role,
		}

		switch {
		case message.Role == relaymodel.RoleAssistant && len(message.ToolCalls) > 0:
			appendAssistantToolCalls(&content, message.ToolCalls, toolCallMap)
		case message.Role == "tool" && message.ToolCallID != "":
			appendToolResponse(&content, message, toolCallMap)
		case message.Role == relaymodel.RoleSystem:
			systemContent = &relaymodel.GeminiChatContent{
				Role: relaymodel.RoleUser,
				Parts: []*relaymodel.GeminiPart{{
					Text: message.StringContent(),
				}},
			}

			continue
		default:
			parts, imageTaskParts, audioTaskParts, videoTaskParts := buildRegularMessageParts(
				message,
				collectImageTasks,
				collectAudioTasks,
				collectVideoTasks,
			)
			content.Parts = append(content.Parts, parts...)
			imageTasks = append(imageTasks, imageTaskParts...)
			audioTasks = append(audioTasks, audioTaskParts...)
			videoTasks = append(videoTasks, videoTaskParts...)
		}

		content.Role = normalizeGeminiRole(content.Role)

		if len(content.Parts) > 0 {
			contents = append(contents, &content)
		}
	}

	return systemContent, mergeConsecutiveContents(contents), imageTasks, audioTasks, videoTasks
}

func processImageTasks(
	ctx context.Context,
	imageTasks []*relaymodel.GeminiPart,
) error {
	if len(imageTasks) == 0 {
		return nil
	}

	sem := semaphore.NewWeighted(3)

	var wg sync.WaitGroup

	for _, task := range imageTasks {
		if task.FileData == nil || task.FileData.FileURI == "" {
			continue
		}

		wg.Go(func() {
			if err := sem.Acquire(ctx, 1); err != nil {
				log.Warnf("convert gemini image url to base64 skipped, keep original url: %v", err)
				return
			}
			defer sem.Release(1)

			mimeType, data, err := image.GetImageFromURL(ctx, task.FileData.FileURI)
			if err != nil {
				log.Warnf("convert gemini image url to base64 failed, keep original url: %v", err)
				return
			}

			task.InlineData = &relaymodel.GeminiInlineData{
				MimeType: mimeType,
				Data:     data,
			}
			task.FileData = nil
		})
	}

	wg.Wait()

	return nil
}

func processMediaTasks(
	ctx context.Context,
	mediaType string,
	mediaTasks []*relaymodel.GeminiPart,
) {
	if len(mediaTasks) == 0 {
		return
	}

	sem := semaphore.NewWeighted(3)

	var wg sync.WaitGroup

	for _, task := range mediaTasks {
		if task.FileData == nil || task.FileData.FileURI == "" {
			continue
		}

		wg.Go(func() {
			if err := sem.Acquire(ctx, 1); err != nil {
				log.Warnf(
					"convert gemini %s url to base64 skipped, keep original url: %v",
					mediaType,
					err,
				)

				return
			}
			defer sem.Release(1)

			mimeType, data, err := getGeminiMediaFromURL(
				ctx,
				task.FileData.FileURI,
				mediaType,
				task.FileData.MimeType,
			)
			if err != nil {
				log.Warnf(
					"convert gemini %s url to base64 failed, keep original url: %v",
					mediaType,
					err,
				)

				return
			}

			task.InlineData = &relaymodel.GeminiInlineData{
				MimeType: mimeType,
				Data:     data,
			}
			task.FileData = nil
		})
	}

	wg.Wait()
}

func getGeminiMediaFromURL(
	ctx context.Context,
	rawURL string,
	mediaType string,
	fallbackMimeType string,
) (string, string, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "", "", fmt.Errorf("download %s error: not an http url", mediaType)
	}

	// #nosec G704 -- media URL download is explicit adaptor behavior.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", err
	}

	// #nosec G704 -- media URL download is explicit adaptor behavior.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download %s error: status code: %d", mediaType, resp.StatusCode)
	}

	buf, err := common.GetResponseBodyLimit(resp, maxGeminiMediaSize)
	if err != nil {
		return "", "", err
	}

	mimeType := geminiMediaResponseMIMEType(resp.Header.Get("Content-Type"), rawURL, mediaType)
	if mimeType == "" {
		mimeType = fallbackMimeType
	}

	if !strings.HasPrefix(mimeType, mediaType+"/") {
		return "", "", fmt.Errorf(
			"download %s error: unsupported content type: %s",
			mediaType,
			mimeType,
		)
	}

	return mimeType, base64.StdEncoding.EncodeToString(buf), nil
}

func geminiMediaResponseMIMEType(contentType, rawURL, mediaType string) string {
	contentType, _, _ = strings.Cut(strings.TrimSpace(strings.ToLower(contentType)), ";")
	if strings.HasPrefix(contentType, mediaType+"/") {
		return contentType
	}

	return mimeTypeFromURL(rawURL, mediaType)
}

func geminiInlineDataToMessageContent(part *relaymodel.GeminiPart) relaymodel.MessageContent {
	url := fmt.Sprintf(
		"data:%s;base64,%s",
		part.InlineData.MimeType,
		part.InlineData.Data,
	)

	switch {
	case strings.HasPrefix(part.InlineData.MimeType, "audio/"):
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeInputAudio,
			InputAudio: &relaymodel.InputAudio{
				Data:   part.InlineData.Data,
				Format: strings.TrimPrefix(part.InlineData.MimeType, "audio/"),
			},
		}
	case strings.HasPrefix(part.InlineData.MimeType, "video/"):
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeVideoURL,
			VideoURL: &relaymodel.VideoURL{
				URL: url,
			},
		}
	default:
		return relaymodel.MessageContent{
			Type: relaymodel.ContentTypeImageURL,
			ImageURL: &relaymodel.ImageURL{
				URL: url,
			},
		}
	}
}

func geminiInlineDataToOutputAudio(part *relaymodel.GeminiPart) *relaymodel.OutputAudio {
	if part.InlineData == nil || !strings.HasPrefix(part.InlineData.MimeType, "audio/") {
		return nil
	}

	return &relaymodel.OutputAudio{
		Data: part.InlineData.Data,
	}
}

func isGeminiAudioInlineData(part *relaymodel.GeminiPart) bool {
	return part.InlineData != nil && strings.HasPrefix(part.InlineData.MimeType, "audio/")
}

// Setting safety to the lowest possible values since Gemini is already powerless enough
func ConvertRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	cfg, err := loadConfig(meta)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return convertRequest(meta, req, cfg)
}

func (a *Adaptor) convertRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	cfg, err := a.loadConfig(meta)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return convertRequest(meta, req, cfg)
}

func convertRequest(
	meta *meta.Meta,
	req *http.Request,
	adaptorConfig Config,
) (adaptor.ConvertResult, error) {
	textRequest, err := utils.UnmarshalGeneralOpenAIRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	textRequest.Model = meta.ActualModel
	meta.Set("stream", textRequest.Stream)

	disableAutoImageURLToBase64 := autoImageURLToBase64Disabled(meta, adaptorConfig)
	disableAutoAudioURLToBase64 := autoAudioURLToBase64Disabled(meta, adaptorConfig)
	disableAutoVideoURLToBase64 := autoVideoURLToBase64Disabled(meta, adaptorConfig)

	systemContent, contents, imageTasks, audioTasks, videoTasks := buildContents(
		textRequest,
		!disableAutoImageURLToBase64,
		!disableAutoAudioURLToBase64,
		!disableAutoVideoURLToBase64,
	)

	// Process image tasks concurrently
	if len(imageTasks) > 0 {
		if err := processImageTasks(
			req.Context(),
			imageTasks,
		); err != nil {
			common.GetLoggerFromReq(req).Warnf("process gemini image tasks failed: %v", err)
		}
	}

	processMediaTasks(req.Context(), "audio", audioTasks)
	processMediaTasks(req.Context(), "video", videoTasks)

	config := buildGenerationConfig(meta, req, textRequest, textRequest)

	// Build actual request
	geminiRequest := relaymodel.GeminiChatRequest{
		Contents:          contents,
		SystemInstruction: systemContent,
		SafetySettings:    buildSafetySettings(adaptorConfig.Safety),
		GenerationConfig:  config,
		Tools:             buildTools(textRequest),
		ToolConfig:        buildToolConfig(textRequest),
	}

	data, err := sonic.Marshal(geminiRequest)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(data))},
		},
		Body: bytes.NewReader(data),
	}, nil
}

func ConvertTTSRequest(meta *meta.Meta, req *http.Request) (adaptor.ConvertResult, error) {
	request, err := utils.UnmarshalTTSRequest(req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	if meta != nil {
		meta.Set("stream_format", request.StreamFormat)
	}

	geminiRequest := relaymodel.GeminiChatRequest{
		Contents: []*relaymodel.GeminiChatContent{
			{
				Role: relaymodel.GeminiRoleUser,
				Parts: []*relaymodel.GeminiPart{
					{Text: request.Input},
				},
			},
		},
		GenerationConfig: &relaymodel.GeminiChatGenerationConfig{
			ResponseModalities: []string{relaymodel.GeminiModalityAudio},
			SpeechConfig:       buildTTSSpeechConfig(request.Voice),
		},
	}

	data, err := sonic.Marshal(geminiRequest)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	return adaptor.ConvertResult{
		Header: http.Header{
			"Content-Type":   {"application/json"},
			"Content-Length": {strconv.Itoa(len(data))},
		},
		Body: bytes.NewReader(data),
	}, nil
}

func TTSHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var geminiResponse relaymodel.GeminiChatResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&geminiResponse); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	var (
		audioData string
		mimeType  string
	)

	for _, candidate := range geminiResponse.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || !strings.HasPrefix(part.InlineData.MimeType, "audio/") {
				continue
			}

			audioData = part.InlineData.Data
			mimeType = part.InlineData.MimeType

			break
		}

		if audioData != "" {
			break
		}
	}

	usage := relaymodel.TextToSpeechUsage{
		InputTokens: int64(meta.RequestUsage.InputTokens),
		TotalTokens: int64(meta.RequestUsage.InputTokens),
	}
	if geminiResponse.UsageMetadata != nil {
		modelUsage := geminiResponse.UsageMetadata.ToModelUsage()
		usage.InputTokens = int64(modelUsage.InputTokens)
		usage.OutputTokens = int64(modelUsage.AudioOutputTokens)

		if usage.OutputTokens == 0 {
			usage.OutputTokens = int64(modelUsage.OutputTokens)
		}

		usage.TotalTokens = int64(modelUsage.TotalTokens)
	}

	if audioData == "" {
		return adaptor.DoResponseResult{Usage: usage.ToModelUsage()},
			relaymodel.WrapperOpenAIErrorWithMessage(
				"gemini tts response audio is empty",
				"empty_audio",
				http.StatusInternalServerError,
			)
	}

	if meta.GetString("stream_format") == "sse" {
		render.OpenaiAudioData(c, audioData)
		render.OpenaiAudioDone(c, usage)
		render.OpenaiDone(c)

		return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, nil
	}

	audioBytes, err := base64.StdEncoding.DecodeString(audioData)
	if err != nil {
		return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, relaymodel.WrapperOpenAIError(
			err,
			"decode_audio_failed",
			http.StatusInternalServerError,
		)
	}

	c.Writer.Header().Set("Content-Type", firstNonEmpty(mimeType, "audio/wav"))
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(audioBytes)))
	_, _ = io.Copy(c.Writer, bytes.NewReader(audioBytes))

	return adaptor.DoResponseResult{Usage: usage.ToModelUsage()}, nil
}

// Type aliases for usage-related types to use unified definitions from relaymodel
var finishReason2OpenAI = map[string]string{
	relaymodel.GeminiFinishReasonStop:      relaymodel.FinishReasonStop,
	relaymodel.GeminiFinishReasonMaxTokens: relaymodel.FinishReasonLength,
}

func FinishReason2OpenAI(reason string) string {
	if openaiReason, ok := finishReason2OpenAI[reason]; ok {
		return openaiReason
	}
	return reason
}

func getToolCall(item *relaymodel.GeminiPart) (*relaymodel.ToolCall, error) {
	if item.FunctionCall == nil {
		return nil, nil
	}

	argsBytes, err := sonic.Marshal(item.FunctionCall.Args)
	if err != nil {
		return nil, err
	}

	toolCall := relaymodel.ToolCall{
		ID:   openai.CallID(),
		Type: "function",
		Function: relaymodel.Function{
			Arguments: conv.BytesToString(argsBytes),
			Name:      item.FunctionCall.Name,
		},
	}

	// Preserve Gemini thought signature if present (OpenAI format)
	if item.ThoughtSignature != "" {
		toolCall.ExtraContent = &relaymodel.ExtraContent{
			Google: &relaymodel.GoogleExtraContent{
				ThoughtSignature: item.ThoughtSignature,
			},
		}
	}

	return &toolCall, nil
}

func responseChat2OpenAI(
	meta *meta.Meta,
	response *relaymodel.GeminiChatResponse,
) *relaymodel.TextResponse {
	fullTextResponse := relaymodel.TextResponse{
		ID:      openai.ChatCompletionID(),
		Model:   meta.OriginModel,
		Object:  relaymodel.ChatCompletionObject,
		Created: time.Now().Unix(),
		Choices: make([]*relaymodel.TextResponseChoice, 0, len(response.Candidates)),
	}
	if response.UsageMetadata != nil {
		fullTextResponse.Usage = response.UsageMetadata.ToUsage()
	}

	for i, candidate := range response.Candidates {
		choice := relaymodel.TextResponseChoice{
			Index: i,
			Message: relaymodel.Message{
				Role: relaymodel.RoleAssistant,
			},
			FinishReason: FinishReason2OpenAI(candidate.FinishReason),
		}
		if len(candidate.Content.Parts) > 0 {
			var (
				contents         []relaymodel.MessageContent
				reasoningContent strings.Builder
				builder          strings.Builder
			)

			hasStructuredContent := false
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil && !isGeminiAudioInlineData(part) {
					hasStructuredContent = true
					break
				}
			}

			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					toolCall, err := getToolCall(part)
					if err != nil {
						log.Error("get tool call failed: " + err.Error())
					}

					if toolCall != nil {
						choice.Message.ToolCalls = append(choice.Message.ToolCalls, *toolCall)
					}
				}

				if part.Text != "" {
					if hasStructuredContent {
						if part.Thought {
							reasoningContent.WriteString(part.Text)

							if part.ThoughtSignature != "" {
								choice.Message.Signature = part.ThoughtSignature
							}
						} else {
							contents = append(contents, relaymodel.MessageContent{
								Type: relaymodel.ContentTypeText,
								Text: part.Text,
							})
						}
					} else {
						if part.Thought {
							reasoningContent.WriteString(part.Text)

							if part.ThoughtSignature != "" {
								choice.Message.Signature = part.ThoughtSignature
							}
						} else {
							builder.WriteString(part.Text)
						}
					}
				}

				if part.InlineData != nil {
					if outputAudio := geminiInlineDataToOutputAudio(part); outputAudio != nil {
						choice.Message.Audio = outputAudio
					} else {
						contents = append(contents, geminiInlineDataToMessageContent(part))
					}
				}
			}

			if hasStructuredContent {
				choice.Message.Content = contents
			} else {
				choice.Message.Content = builder.String()
			}

			choice.Message.ReasoningContent = reasoningContent.String()
		}

		fullTextResponse.Choices = append(fullTextResponse.Choices, &choice)
	}

	return &fullTextResponse
}

func streamResponseChat2OpenAI(
	meta *meta.Meta,
	geminiResponse *relaymodel.GeminiChatResponse,
) *relaymodel.ChatCompletionsStreamResponse {
	response := &relaymodel.ChatCompletionsStreamResponse{
		ID:      openai.ChatCompletionID(),
		Created: time.Now().Unix(),
		Model:   meta.OriginModel,
		Object:  relaymodel.ChatCompletionChunkObject,
		Choices: make(
			[]*relaymodel.ChatCompletionsStreamResponseChoice,
			0,
			len(geminiResponse.Candidates),
		),
	}
	if geminiResponse.UsageMetadata != nil {
		usage := geminiResponse.UsageMetadata.ToUsage()
		response.Usage = &usage
	}

	for i, candidate := range geminiResponse.Candidates {
		choice := relaymodel.ChatCompletionsStreamResponseChoice{
			Index: i,
			Delta: relaymodel.Message{
				Content: "",
			},
			FinishReason: FinishReason2OpenAI(candidate.FinishReason),
		}
		if len(candidate.Content.Parts) > 0 {
			var (
				contents         []relaymodel.MessageContent
				reasoningContent strings.Builder
				builder          strings.Builder
			)

			hasStructuredContent := false
			for _, part := range candidate.Content.Parts {
				if part.InlineData != nil && !isGeminiAudioInlineData(part) {
					hasStructuredContent = true
					break
				}
			}

			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					toolCall, err := getToolCall(part)
					if err != nil {
						log.Error("get tool call failed: " + err.Error())
					}

					if toolCall != nil {
						choice.Delta.ToolCalls = append(choice.Delta.ToolCalls, *toolCall)
					}
				}

				if part.Text != "" {
					if hasStructuredContent {
						if part.Thought {
							reasoningContent.WriteString(part.Text)

							if part.ThoughtSignature != "" {
								choice.Delta.Signature = part.ThoughtSignature
							}
						} else {
							contents = append(contents, relaymodel.MessageContent{
								Type: relaymodel.ContentTypeText,
								Text: part.Text,
							})
						}
					} else {
						if part.Thought {
							reasoningContent.WriteString(part.Text)

							if part.ThoughtSignature != "" {
								choice.Delta.Signature = part.ThoughtSignature
							}
						} else {
							builder.WriteString(part.Text)
						}
					}
				}

				if part.InlineData != nil {
					if outputAudio := geminiInlineDataToOutputAudio(part); outputAudio != nil {
						choice.Delta.Audio = outputAudio
					} else {
						contents = append(contents, geminiInlineDataToMessageContent(part))
					}
				}
			}

			if hasStructuredContent {
				choice.Delta.Content = contents
			} else {
				choice.Delta.Content = builder.String()
			}

			choice.Delta.ReasoningContent = reasoningContent.String()
		}

		response.Choices = append(response.Choices, &choice)
	}

	return response
}

func StreamHandler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	log := common.GetLogger(c)

	scanner, cleanup := utils.NewStreamScanner(resp.Body, meta.ActualModel)
	defer cleanup()

	usage := model.Usage{}
	webSearchQueries := map[string]struct{}{}
	webSearchGrounded := false
	webSearchGemini3 := isGemini3Meta(meta)

	for scanner.Scan() {
		data := scanner.Bytes()
		if !render.IsValidSSEData(data) {
			continue
		}

		data = render.ExtractSSEData(data)
		if render.IsSSEDone(data) {
			break
		}

		var geminiResponse relaymodel.GeminiChatResponse

		err := sonic.Unmarshal(data, &geminiResponse)
		if err != nil {
			log.Error("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := streamResponseChat2OpenAI(meta, &geminiResponse)
		if response.Usage != nil {
			usage = geminiResponse.UsageMetadata.ToModelUsage()
		}

		trackGeminiWebSearch(
			&geminiResponse,
			webSearchQueries,
			&webSearchGrounded,
			&webSearchGemini3,
		)

		_ = render.OpenaiObjectData(c, response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("error reading stream: " + err.Error())
	}

	render.OpenaiDone(c)

	usage.WebSearchCount = model.ZeroNullInt64(
		geminiWebSearchCount(webSearchQueries, webSearchGrounded, webSearchGemini3),
	)

	return adaptor.DoResponseResult{Usage: usage}, nil
}

func Handler(
	meta *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHandler(resp)
	}

	defer resp.Body.Close()

	var geminiResponse relaymodel.GeminiChatResponse

	err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&geminiResponse)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIError(
			err,
			"unmarshal_response_body_failed",
			http.StatusInternalServerError,
		)
	}

	fullTextResponse := responseChat2OpenAI(meta, &geminiResponse)

	jsonResponse, err := sonic.Marshal(fullTextResponse)
	if err != nil {
		return adaptor.DoResponseResult{
				Usage: fullTextResponse.Usage.ToModelUsage(),
			}, relaymodel.WrapperOpenAIError(
				err,
				"marshal_response_body_failed",
				http.StatusInternalServerError,
			)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(jsonResponse)))
	_, _ = c.Writer.Write(jsonResponse)

	modelUsage := fullTextResponse.Usage.ToModelUsage()
	modelUsage.WebSearchCount = model.ZeroNullInt64(geminiResponse.GetWebSearchCount())

	return adaptor.DoResponseResult{Usage: modelUsage}, nil
}
