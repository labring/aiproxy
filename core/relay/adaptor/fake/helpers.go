package fake

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
)

func buildUsage(cfg Config) model.Usage {
	inputTokens := cfg.Usage.InputTokens
	outputTokens := cfg.Usage.OutputTokens

	if inputTokens == 0 {
		inputTokens = 24
	}

	if outputTokens == 0 {
		outputTokens = 12
	}

	totalTokens := inputTokens + outputTokens

	return model.Usage{
		InputTokens:     model.ZeroNullInt64(inputTokens),
		OutputTokens:    model.ZeroNullInt64(outputTokens),
		TotalTokens:     model.ZeroNullInt64(totalTokens),
		CachedTokens:    model.ZeroNullInt64(cfg.Usage.CachedTokens),
		ReasoningTokens: model.ZeroNullInt64(cfg.Usage.ReasoningTokens),
		ImageInputTokens: model.ZeroNullInt64(
			firstNonZero(cfg.Usage.ImageInputTokens, cfg.Image.ImageTokensIn),
		),
		ImageOutputTokens: model.ZeroNullInt64(
			firstNonZero(cfg.Usage.ImageOutputTokens, cfg.Image.ImageTokensOut),
		),
		WebSearchCount: model.ZeroNullInt64(cfg.Usage.WebSearchCount),
	}
}

func usageToChatUsage(usage model.Usage) relaymodel.ChatUsage {
	chatUsage := relaymodel.ChatUsage{
		PromptTokens:     int64(usage.InputTokens),
		CompletionTokens: int64(usage.OutputTokens),
		TotalTokens:      int64(usage.TotalTokens),
	}
	if usage.CachedTokens > 0 {
		chatUsage.PromptTokensDetails = &relaymodel.PromptTokensDetails{
			CachedTokens: int64(usage.CachedTokens),
		}
	}

	if usage.ReasoningTokens > 0 || usage.ImageOutputTokens > 0 {
		chatUsage.CompletionTokensDetails = &relaymodel.CompletionTokensDetails{
			ReasoningTokens: int64(usage.ReasoningTokens),
			ImageTokens:     int64(usage.ImageOutputTokens),
		}
	}

	if usage.WebSearchCount > 0 {
		chatUsage.WebSearchCount = int64(usage.WebSearchCount)
	}

	return chatUsage
}

func usageToResponseUsage(usage model.Usage) relaymodel.ResponseUsage {
	respUsage := relaymodel.ResponseUsage{
		InputTokens:  int64(usage.InputTokens),
		OutputTokens: int64(usage.OutputTokens),
		TotalTokens:  int64(usage.TotalTokens),
	}
	if usage.CachedTokens > 0 {
		respUsage.InputTokensDetails = &relaymodel.ResponseUsageDetails{
			CachedTokens: int64(usage.CachedTokens),
		}
	}

	if usage.ReasoningTokens > 0 {
		respUsage.OutputTokensDetails = &relaymodel.ResponseUsageDetails{
			ReasoningTokens: int64(usage.ReasoningTokens),
		}
	}

	return respUsage
}

func usageToGeminiUsage(usage model.Usage) relaymodel.GeminiUsageMetadata {
	return relaymodel.GeminiUsageMetadata{
		PromptTokenCount:        int64(usage.InputTokens),
		CandidatesTokenCount:    int64(usage.OutputTokens),
		TotalTokenCount:         int64(usage.TotalTokens),
		ThoughtsTokenCount:      int64(usage.ReasoningTokens),
		CachedContentTokenCount: int64(usage.CachedTokens),
		PromptTokensDetails: []relaymodel.GeminiTokensDetail{
			{Modality: relaymodel.GeminiModalityText, TokenCount: int64(usage.InputTokens)},
		},
		CandidatesTokensDetails: []relaymodel.GeminiTokensDetail{
			{Modality: relaymodel.GeminiModalityText, TokenCount: int64(usage.OutputTokens)},
		},
	}
}

func actualModel(meta *meta.Meta, reqCtx requestContext) string {
	return firstNonEmpty(meta.ActualModel, reqCtx.Model, meta.OriginModel, "fake-model")
}

func synthesizeText(cfg Config, reqCtx requestContext) string {
	base := strings.TrimSpace(cfg.StaticText)
	if base == "" {
		input := strings.TrimSpace(reqCtx.Text)
		if input == "" {
			input = "empty input"
		}

		base = "fake response for " + input
	}

	return firstNonEmpty(cfg.ResponsePrefix, "") + base + firstNonEmpty(cfg.ResponseSuffix, "")
}

func splitChunks(text string, count, size int) []string {
	if text == "" {
		return []string{""}
	}

	if size <= 0 {
		if count <= 1 {
			return []string{text}
		}

		size = int(math.Ceil(float64(len(text)) / float64(count)))
	}

	if size <= 0 {
		size = len(text)
	}

	result := make([]string, 0, (len(text)+size-1)/size)
	for start := 0; start < len(text); start += size {
		end := min(start+size, len(text))

		result = append(result, text[start:end])
	}

	return result
}

func makeEmbedding(text string, dims int, base string) []float64 {
	if dims <= 0 {
		dims = 8
	}

	if base == "" {
		base = text
	}

	sum := sha256.Sum256([]byte(base))

	result := make([]float64, dims)
	for i := range dims {
		b := sum[i%len(sum)]
		result[i] = float64(int(b)%200-100) / 100
	}

	return result
}

func makeFakePNGBase64(prompt, size string) (string, error) {
	width, height := parseImageSize(size)
	if width <= 0 || height <= 0 {
		width, height = 1024, 1024
	}

	seed := prompt
	if strings.TrimSpace(seed) == "" {
		seed = "fake-image"
	}

	sum := sha256.Sum256([]byte(seed))
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	base := color.RGBA{R: sum[0], G: sum[1], B: sum[2], A: 0xff}
	accent := color.RGBA{R: sum[3], G: sum[4], B: sum[5], A: 0xff}
	highlight := color.RGBA{R: sum[6], G: sum[7], B: sum[8], A: 0xff}

	for y := range height {
		for x := range width {
			pixel := base
			if (x/32+y/32)%2 == 0 {
				pixel = accent
			}

			if (x+y)%97 < 11 {
				pixel = highlight
			}

			img.SetRGBA(x, y, pixel)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func parseImageSize(size string) (int, int) {
	switch strings.TrimSpace(strings.ToLower(size)) {
	case "", "auto":
		return 1024, 1024
	}

	widthText, heightText, ok := strings.Cut(size, "x")
	if !ok {
		return 1024, 1024
	}

	width, err := strconv.Atoi(strings.TrimSpace(widthText))
	if err != nil || width <= 0 {
		return 1024, 1024
	}

	height, err := strconv.Atoi(strings.TrimSpace(heightText))
	if err != nil || height <= 0 {
		return 1024, 1024
	}

	return width, height
}

func writeJSON(c *gin.Context, statusCode int, data any) error {
	raw, err := sonic.Marshal(data)
	if err != nil {
		return err
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Length", strconv.Itoa(len(raw)))
	c.Status(statusCode)
	_, err = c.Writer.Write(raw)

	return err
}

func buildResponseBody(
	meta *meta.Meta,
	_ adaptor.Store,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
) ([]byte, string, int, error) {
	rec := httptestRecorder{header: http.Header{}}

	gc, _ := gin.CreateTestContext(&rec)
	switch meta.Mode {
	case mode.ChatCompletions, mode.Completions:
		_, err := writeOpenAI(meta, gc, cfg, reqCtx, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, err
	case mode.Embeddings:
		_, err := writeEmbeddings(meta, gc, cfg, reqCtx, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, err
	case mode.ImagesGenerations:
		_, err := writeImage(meta, gc, cfg, reqCtx, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, err
	case mode.Rerank:
		_, err := writeRerank(meta, gc, cfg, reqCtx, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, err
	case mode.Anthropic:
		_, err := writeAnthropic(meta, gc, cfg, reqCtx, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, err
	case mode.Gemini:
		_, err := writeGemini(meta, gc, cfg, reqCtx, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, err
	case mode.Responses:
		_, err := writeResponses(meta, discardStore{}, gc, cfg, reqCtx, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusCreated, err
	case mode.ResponsesGet:
		_ = writeResponsesGet(meta, gc, cfg, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, nil
	case mode.ResponsesDelete:
		gc.Status(http.StatusNoContent)
		return nil, "application/json", http.StatusNoContent, nil
	case mode.ResponsesCancel:
		_ = writeResponsesCancel(meta, gc, cfg, usage)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, nil
	case mode.ResponsesInputItems:
		writeResponsesInputItems(meta, gc, cfg)
		return rec.body.Bytes(), contentTypeOrJSON(rec.header), http.StatusOK, nil
	default:
		return nil, "", 0, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func contentTypeOrJSON(header http.Header) string {
	if v := header.Get("Content-Type"); v != "" {
		return v
	}
	return "application/json"
}

type httptestRecorder struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func (r *httptestRecorder) Header() http.Header         { return r.header }
func (r *httptestRecorder) WriteHeader(code int)        { r.code = code }
func (r *httptestRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }

type discardStore struct{}

func (discardStore) GetStore(string, int, string) (adaptor.StoreCache, error) {
	return adaptor.StoreCache{}, nil
}

func (discardStore) SaveStore(adaptor.StoreCache) error {
	return nil
}

func (discardStore) SaveStoreWithOption(adaptor.StoreCache, adaptor.SaveStoreOption) error {
	return nil
}

func (discardStore) SaveIfNotExistStore(adaptor.StoreCache) error {
	return nil
}

func buildResponseObject(
	meta *meta.Meta,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
	responseID string,
	store bool,
) relaymodel.Response {
	status := relaymodel.ResponseStatusCompleted
	if cfg.Response.Status != "" {
		status = cfg.Response.Status
	}

	parallelToolCalls := true
	if cfg.Response.ParallelToolCalls != nil {
		parallelToolCalls = *cfg.Response.ParallelToolCalls
	}

	return relaymodel.Response{
		ID:                responseID,
		Object:            "response",
		CreatedAt:         time.Now().Unix(),
		Status:            status,
		Model:             actualModel(meta, reqCtx),
		ParallelToolCalls: parallelToolCalls,
		Store:             store,
		Output: []relaymodel.OutputItem{
			{
				ID:     fakeID("out", responseID),
				Type:   relaymodel.InputItemTypeMessage,
				Status: relaymodel.ResponseStatusCompleted,
				Role:   relaymodel.RoleAssistant,
				Content: []relaymodel.OutputContent{
					{
						Type: relaymodel.OutputContentTypeText,
						Text: synthesizeText(cfg, reqCtx),
					},
				},
			},
		},
		Reasoning: relaymodel.ResponseReasoning{
			Summary: []relaymodel.SummaryPart{
				{
					Type: "summary_text",
					Text: firstNonEmpty(cfg.ReasoningText, "fake reasoning summary"),
				},
			},
		},
		Text: relaymodel.ResponseText{
			Format: relaymodel.ResponseTextFormat{Type: "text"},
		},
		Usage:      new(usageToResponseUsage(usage)),
		Metadata:   cfg.Metadata,
		Truncation: "disabled",
	}
}

func streamResponses(
	c *gin.Context,
	cfg Config,
	respObj relaymodel.Response,
	text string,
) error {
	if err := render.ResponsesEventObjectData(
		c,
		relaymodel.EventResponseCreated,
		relaymodel.ResponseStreamEvent{
			Type:     relaymodel.EventResponseCreated,
			Response: &respObj,
		},
	); err != nil {
		return err
	}

	item := respObj.Output[0]
	if err := render.ResponsesEventObjectData(
		c,
		relaymodel.EventOutputItemAdded,
		relaymodel.ResponseStreamEvent{
			Type:        relaymodel.EventOutputItemAdded,
			OutputIndex: new(0),
			Item:        &item,
		},
	); err != nil {
		return err
	}

	for _, chunk := range splitChunks(text, cfg.StreamChunks, cfg.StreamChunkSize) {
		if err := render.ResponsesEventObjectData(
			c,
			relaymodel.EventOutputTextDelta,
			relaymodel.ResponseStreamEvent{
				Type:         relaymodel.EventOutputTextDelta,
				OutputIndex:  new(0),
				ContentIndex: new(0),
				Delta:        chunk,
			},
		); err != nil {
			return err
		}
	}

	if err := render.ResponsesEventObjectData(
		c,
		relaymodel.EventOutputTextDone,
		relaymodel.ResponseStreamEvent{
			Type:         relaymodel.EventOutputTextDone,
			OutputIndex:  new(0),
			ContentIndex: new(0),
			Text:         text,
		},
	); err != nil {
		return err
	}

	if err := render.ResponsesEventObjectData(
		c,
		relaymodel.EventOutputItemDone,
		relaymodel.ResponseStreamEvent{
			Type:        relaymodel.EventOutputItemDone,
			OutputIndex: new(0),
			Item:        &item,
		},
	); err != nil {
		return err
	}

	if err := render.ResponsesEventObjectData(
		c,
		relaymodel.EventResponseCompleted,
		relaymodel.ResponseStreamEvent{
			Type:     relaymodel.EventResponseCompleted,
			Response: &respObj,
		},
	); err != nil {
		return err
	}

	return render.ResponsesEventObjectData(
		c,
		relaymodel.EventResponseDone,
		relaymodel.ResponseStreamEvent{
			Type:     relaymodel.EventResponseDone,
			Response: &respObj,
		},
	)
}

func extractMessagesText(messages []relaymodel.Message) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(message.StringContent())
		if content != "" {
			parts = append(parts, content)
		}
	}

	return strings.Join(parts, "\n")
}

func extractClaudeMessagesText(messages []relaymodel.ClaudeAnyContentMessage) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(anyToString(message.Content))
		if content != "" {
			parts = append(parts, content)
		}
	}

	return strings.Join(parts, "\n")
}

func extractGeminiText(contents []*relaymodel.GeminiChatContent) string {
	parts := make([]string, 0, len(contents))
	for _, content := range contents {
		if content == nil {
			continue
		}

		for _, part := range content.Parts {
			if part == nil {
				continue
			}

			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		}
	}

	return strings.Join(parts, "\n")
}

func anyToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []string:
		return strings.Join(t, "\n")
	default:
		raw, err := sonic.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}

		return string(raw)
	}
}

func fakeID(prefix, seed string) string {
	if seed == "" {
		seed = strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	sum := sha256.Sum256([]byte(seed))

	return fmt.Sprintf("%s_%x", prefix, sum[:6])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}

	return 0
}

func defaultConfig() Config {
	return Config{
		StaticText:        "This is a fake adaptor response.",
		SystemFingerprint: "fake-adaptor",
		ResponsePrefix:    "",
		ResponseSuffix:    "",
		ReasoningText:     "This answer was generated by the local fake adaptor.",
		StreamChunks:      3,
		Embedding: EmbeddingCfg{
			Dimensions: 8,
		},
		Image: ImageCfg{
			URL:            "https://fake.local/images/default.png",
			B64JSON:        "",
			RevisedPrompt:  "fake revised prompt",
			InputTokens:    64,
			OutputTokens:   128,
			ImageTokensIn:  16,
			ImageTokensOut: 128,
		},
		Rerank: RerankCfg{
			BaseScore: 0.99,
			Step:      0.1,
		},
		Usage: UsageCfg{
			InputTokens:     24,
			OutputTokens:    12,
			CachedTokens:    0,
			ReasoningTokens: 0,
		},
		Response: ResponseCfg{
			Status: relaymodel.ResponseStatusCompleted,
		},
		Anthropic: AnthropicCfg{
			StopReason: relaymodel.ClaudeStopReasonEndTurn,
			Type:       relaymodel.ClaudeTypeMessage,
		},
		Gemini: GeminiCfg{
			FinishReason: "STOP",
			ModelVersion: "fake-1.0",
		},
		OpenAPI: OpenAPICfg{
			SpecVersion: "3.1.0",
		},
	}
}

func configSchema() map[string]any {
	return map[string]any{
		"type":  "object",
		"title": "Fake Adaptor Config",
		"properties": map[string]any{
			"static_text": map[string]any{
				"type":        "string",
				"title":       "Static Text",
				"description": "Main fake answer body returned by chat, completion, responses, anthropic, and gemini endpoints.",
			},
			"response_prefix": map[string]any{
				"type":        "string",
				"title":       "Response Prefix",
				"description": "Prepended to synthesized text.",
			},
			"response_suffix": map[string]any{
				"type":        "string",
				"title":       "Response Suffix",
				"description": "Appended to synthesized text.",
			},
			"reasoning_text": map[string]any{
				"type":        "string",
				"title":       "Reasoning Text",
				"description": "Reasoning summary used in Responses API output.",
			},
			"delay_ms": map[string]any{
				"type":        "integer",
				"title":       "Delay Milliseconds",
				"description": "Artificial upstream delay added before the local fake response is returned.",
			},
			"stream_chunks": map[string]any{
				"type":        "integer",
				"title":       "Stream Chunks",
				"description": "Number of chunks used when synthesizing streaming responses.",
			},
			"stream_chunk_size": map[string]any{
				"type":        "integer",
				"title":       "Stream Chunk Size",
				"description": "Fixed chunk size for streaming text. If empty, chunk count is used.",
			},
			"usage": map[string]any{
				"type":        "object",
				"title":       "Usage",
				"description": "Token usage template applied to all fake responses.",
				"properties": map[string]any{
					"input_tokens": map[string]any{
						"type":  "integer",
						"title": "Input Tokens",
					},
					"output_tokens": map[string]any{
						"type":  "integer",
						"title": "Output Tokens",
					},
					"cached_tokens": map[string]any{
						"type":  "integer",
						"title": "Cached Tokens",
					},
					"reasoning_tokens": map[string]any{
						"type":  "integer",
						"title": "Reasoning Tokens",
					},
					"image_input_tokens": map[string]any{
						"type":  "integer",
						"title": "Image Input Tokens",
					},
					"image_output_tokens": map[string]any{
						"type":  "integer",
						"title": "Image Output Tokens",
					},
					"web_search_count": map[string]any{
						"type":  "integer",
						"title": "Web Search Count",
					},
				},
			},
			"embedding": map[string]any{
				"type":        "object",
				"title":       "Embedding",
				"description": "Controls fake embedding vector generation.",
				"properties": map[string]any{
					"dimensions": map[string]any{"type": "integer", "title": "Dimensions"},
					"base": map[string]any{
						"type":        "string",
						"title":       "Embedding Base",
						"description": "Optional seed string used to stabilize generated vectors.",
					},
				},
			},
			"image": map[string]any{
				"type":        "object",
				"title":       "Image",
				"description": "Controls fake image generation payloads.",
				"properties": map[string]any{
					"url":            map[string]any{"type": "string", "title": "Image URL"},
					"b64_json":       map[string]any{"type": "string", "title": "Base64 Image"},
					"revised_prompt": map[string]any{"type": "string", "title": "Revised Prompt"},
					"input_tokens":   map[string]any{"type": "integer", "title": "Input Tokens"},
					"output_tokens":  map[string]any{"type": "integer", "title": "Output Tokens"},
					"image_tokens_in": map[string]any{
						"type":  "integer",
						"title": "Image Tokens In",
					},
					"image_tokens_out": map[string]any{
						"type":  "integer",
						"title": "Image Tokens Out",
					},
				},
			},
			"rerank": map[string]any{
				"type":        "object",
				"title":       "Rerank",
				"description": "Controls fake rerank scores and document echoing.",
				"properties": map[string]any{
					"base_score": map[string]any{"type": "number", "title": "Base Score"},
					"step":       map[string]any{"type": "number", "title": "Score Step"},
					"return_documents": map[string]any{
						"type":  "boolean",
						"title": "Return Documents",
					},
				},
			},
			"response": map[string]any{
				"type":        "object",
				"title":       "Responses",
				"description": "Controls OpenAI Responses API fake behavior.",
				"properties": map[string]any{
					"store": map[string]any{"type": "boolean", "title": "Store Response"},
					"status": map[string]any{
						"type":  "string",
						"title": "Response Status",
						"enum": []string{
							relaymodel.ResponseStatusCompleted,
							relaymodel.ResponseStatusInProgress,
							relaymodel.ResponseStatusCancelled,
							relaymodel.ResponseStatusIncomplete,
						},
					},
					"parallel_tool_calls": map[string]any{
						"type":  "boolean",
						"title": "Parallel Tool Calls",
					},
				},
			},
			"anthropic": map[string]any{
				"type":        "object",
				"title":       "Anthropic",
				"description": "Controls native Claude response fields.",
				"properties": map[string]any{
					"stop_reason": map[string]any{"type": "string", "title": "Stop Reason"},
					"type":        map[string]any{"type": "string", "title": "Type"},
				},
			},
			"gemini": map[string]any{
				"type":        "object",
				"title":       "Gemini",
				"description": "Controls native Gemini response fields.",
				"properties": map[string]any{
					"finish_reason": map[string]any{"type": "string", "title": "Finish Reason"},
					"model_version": map[string]any{"type": "string", "title": "Model Version"},
				},
			},
			"metadata": map[string]any{
				"type":        "object",
				"title":       "Metadata",
				"description": "Arbitrary metadata copied into Responses API objects.",
			},
			"openapi": map[string]any{
				"type":        "object",
				"title":       "OpenAPI Template",
				"description": "Reserved section for OpenAPI-style configuration templating and documentation.",
				"properties": map[string]any{
					"spec_version": map[string]any{"type": "string", "title": "Spec Version"},
					"info":         map[string]any{"type": "object", "title": "Info"},
					"components":   map[string]any{"type": "object", "title": "Components"},
				},
			},
		},
	}
}
