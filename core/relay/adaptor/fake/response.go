package fake

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/render"
)

func writeOpenAI(
	meta *meta.Meta,
	c *gin.Context,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
) (adaptor.DoResponseResult, adaptor.Error) {
	now := time.Now().Unix()

	text := synthesizeText(cfg, reqCtx)
	if reqCtx.Stream {
		chunks := splitChunks(text, cfg.StreamChunks, cfg.StreamChunkSize)
		first := relaymodel.ChatCompletionsStreamResponse{
			ID:      fakeID("chatcmpl", meta.RequestID+reqCtx.Model),
			Object:  relaymodel.ChatCompletionChunkObject,
			Created: now,
			Model:   actualModel(meta, reqCtx),
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
				{Index: 0, Delta: relaymodel.Message{Role: relaymodel.RoleAssistant}},
			},
		}

		_ = render.OpenaiObjectData(c, first)
		for _, part := range chunks {
			_ = render.OpenaiObjectData(c, relaymodel.ChatCompletionsStreamResponse{
				ID:      first.ID,
				Object:  relaymodel.ChatCompletionChunkObject,
				Created: now,
				Model:   actualModel(meta, reqCtx),
				Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
					{Index: 0, Delta: relaymodel.Message{Content: part}},
				},
			})
		}

		finalChunk := relaymodel.ChatCompletionsStreamResponse{
			ID:      first.ID,
			Object:  relaymodel.ChatCompletionChunkObject,
			Created: now,
			Model:   actualModel(meta, reqCtx),
			Choices: []*relaymodel.ChatCompletionsStreamResponseChoice{
				{Index: 0, Delta: relaymodel.Message{}, FinishReason: relaymodel.FinishReasonStop},
			},
			Usage: new(usageToChatUsage(usage)),
		}
		_ = render.OpenaiObjectData(c, finalChunk)
		render.OpenaiDone(c)

		return adaptor.DoResponseResult{Usage: usage, UpstreamID: first.ID}, nil
	}

	resp := relaymodel.TextResponse{
		ID:      fakeID("chatcmpl", meta.RequestID+reqCtx.Model),
		Object:  relaymodel.ChatCompletionObject,
		Model:   actualModel(meta, reqCtx),
		Created: now,
		Choices: []*relaymodel.TextResponseChoice{
			{
				Index:        0,
				FinishReason: relaymodel.FinishReasonStop,
				Message: relaymodel.Message{
					Role:    relaymodel.RoleAssistant,
					Content: text,
				},
				Text: text,
			},
		},
		Usage: usageToChatUsage(usage),
	}

	if err := writeJSON(c, http.StatusOK, resp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	return adaptor.DoResponseResult{Usage: usage, UpstreamID: resp.ID}, nil
}

func writeEmbeddings(
	meta *meta.Meta,
	c *gin.Context,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
) (adaptor.DoResponseResult, adaptor.Error) {
	dims := cfg.Embedding.Dimensions
	if dims <= 0 {
		dims = 8
	}

	data := []*relaymodel.EmbeddingResponseItem{
		{
			Object:    "embedding",
			Index:     0,
			Embedding: makeEmbedding(reqCtx.Text, dims, cfg.Embedding.Base),
		},
	}

	resp := relaymodel.EmbeddingResponse{
		Object: "list",
		Model:  actualModel(meta, reqCtx),
		Data:   data,
		Usage: relaymodel.EmbeddingUsage{
			PromptTokens: int64(usage.InputTokens),
			TotalTokens:  int64(usage.TotalTokens),
			PromptTokensDetails: &relaymodel.EmbeddingPromptTokensDetails{
				TextTokens:  int64(usage.InputTokens),
				ImageTokens: int64(usage.ImageInputTokens),
			},
		},
	}
	if err := writeJSON(c, http.StatusOK, resp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}

func writeImage(
	meta *meta.Meta,
	c *gin.Context,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
) (adaptor.DoResponseResult, adaptor.Error) {
	responseFormat := strings.TrimSpace(strings.ToLower(reqCtx.ImageResponseFormat))

	imageURL := ""
	imageB64 := cfg.Image.B64JSON

	switch {
	case responseFormat == "url":
		imageURL = cfg.Image.URL
		if imageURL == "" {
			imageURL = fmt.Sprintf("https://fake.local/images/%s.png", fakeID("img", reqCtx.Text))
		}

		imageB64 = ""
	case responseFormat == "b64_json":
		var err error

		imageB64, err = makeFakePNGBase64(reqCtx.Text, reqCtx.ImageSize)
		if err != nil {
			return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusInternalServerError,
				err.Error(),
			)
		}
	case cfg.Image.URL != "":
		imageURL = cfg.Image.URL

		imageB64 = ""
	case cfg.Image.B64JSON != "":
	default:
		var err error

		imageB64, err = makeFakePNGBase64(reqCtx.Text, reqCtx.ImageSize)
		if err != nil {
			return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusInternalServerError,
				err.Error(),
			)
		}
	}

	resp := relaymodel.ImageResponse{
		Created: time.Now().Unix(),
		Data: []*relaymodel.ImageData{
			{
				URL:           imageURL,
				B64Json:       imageB64,
				RevisedPrompt: firstNonEmpty(cfg.Image.RevisedPrompt, reqCtx.Text),
			},
		},
		Usage: &relaymodel.ImageUsage{
			InputTokens:  int64(usage.InputTokens),
			OutputTokens: int64(usage.OutputTokens),
			TotalTokens:  int64(usage.TotalTokens),
			InputTokensDetails: relaymodel.ImageInputTokensDetails{
				TextTokens:  int64(usage.InputTokens) - int64(usage.ImageInputTokens),
				ImageTokens: int64(usage.ImageInputTokens),
			},
			OutputTokensDetails: &relaymodel.ImageOutputTokensDetails{
				TextTokens:  int64(usage.OutputTokens) - int64(usage.ImageOutputTokens),
				ImageTokens: int64(usage.ImageOutputTokens),
			},
		},
	}
	if err := writeJSON(c, http.StatusOK, resp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}

func writeRerank(
	meta *meta.Meta,
	c *gin.Context,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
) (adaptor.DoResponseResult, adaptor.Error) {
	topN := 3
	docs := []string{reqCtx.Text, reqCtx.Text + " alt", reqCtx.Text + " summary"}

	results := make([]*relaymodel.RerankResult, 0, topN)
	for i := range topN {
		score := cfg.Rerank.BaseScore - float64(i)*cfg.Rerank.Step
		if score == 0 {
			score = 0.99 - float64(i)*0.1
		}

		result := &relaymodel.RerankResult{
			Index:          i,
			RelevanceScore: score,
		}

		includeDocument := true
		if cfg.Rerank.ReturnDocuments != nil {
			includeDocument = *cfg.Rerank.ReturnDocuments
		}

		if includeDocument {
			result.Document = &relaymodel.Document{Text: docs[i%len(docs)]}
		}

		results = append(results, result)
	}

	resp := relaymodel.RerankResponse{
		ID: fakeID("rerank", reqCtx.Text),
		Meta: relaymodel.RerankMeta{
			Model: actualModel(meta, reqCtx),
			Tokens: &relaymodel.RerankMetaTokens{
				InputTokens:  int64(usage.InputTokens),
				OutputTokens: int64(usage.OutputTokens),
			},
		},
		Results: results,
	}
	if err := writeJSON(c, http.StatusOK, resp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	return adaptor.DoResponseResult{Usage: usage}, nil
}

func writeAnthropic(
	meta *meta.Meta,
	c *gin.Context,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
) (adaptor.DoResponseResult, adaptor.Error) {
	resp := relaymodel.ClaudeResponse{
		ID:    fakeID("msg", reqCtx.Text),
		Type:  firstNonEmpty(cfg.Anthropic.Type, relaymodel.ClaudeTypeMessage),
		Role:  relaymodel.RoleAssistant,
		Model: actualModel(meta, reqCtx),
		Content: []relaymodel.ClaudeContent{
			{Type: relaymodel.ClaudeContentTypeText, Text: synthesizeText(cfg, reqCtx)},
		},
		StopReason: firstNonEmpty(cfg.Anthropic.StopReason, relaymodel.ClaudeStopReasonEndTurn),
		Usage:      relaymodel.ClaudeFromModelUsage(usage),
	}
	if reqCtx.Stream {
		_ = render.ClaudeEventObjectData(c, "message_start", relaymodel.ClaudeStreamResponse{
			Type:    relaymodel.ClaudeStreamTypeMessageStart,
			Message: &resp,
		})

		_ = render.ClaudeEventObjectData(c, "content_block_start", relaymodel.ClaudeStreamResponse{
			Type:  relaymodel.ClaudeStreamTypeContentBlockStart,
			Index: 0,
			ContentBlock: &relaymodel.ClaudeContent{
				Type: relaymodel.ClaudeContentTypeText,
				Text: "",
			},
		})
		for _, part := range splitChunks(synthesizeText(cfg, reqCtx), cfg.StreamChunks, cfg.StreamChunkSize) {
			_ = render.ClaudeEventObjectData(
				c,
				"content_block_delta",
				relaymodel.ClaudeStreamResponse{
					Type:  relaymodel.ClaudeStreamTypeContentBlockDelta,
					Index: 0,
					Delta: &relaymodel.ClaudeDelta{
						Type: relaymodel.ClaudeContentTypeText,
						Text: part,
					},
				},
			)
		}

		_ = render.ClaudeEventObjectData(c, "content_block_stop", relaymodel.ClaudeStreamResponse{
			Type:  relaymodel.ClaudeStreamTypeContentBlockStop,
			Index: 0,
		})
		_ = render.ClaudeEventObjectData(c, "message_delta", relaymodel.ClaudeStreamResponse{
			Type:  relaymodel.ClaudeStreamTypeMessageDelta,
			Usage: new(relaymodel.ClaudeFromModelUsage(usage)),
			Delta: &relaymodel.ClaudeDelta{StopReason: new(resp.StopReason)},
		})
		_ = render.ClaudeEventObjectData(c, "message_stop", relaymodel.ClaudeStreamResponse{
			Type: relaymodel.ClaudeStreamTypeMessageStop,
		})

		return adaptor.DoResponseResult{Usage: usage, UpstreamID: resp.ID}, nil
	}

	if err := writeJSON(c, http.StatusOK, resp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	return adaptor.DoResponseResult{Usage: usage, UpstreamID: resp.ID}, nil
}

func writeGemini(
	meta *meta.Meta,
	c *gin.Context,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
) (adaptor.DoResponseResult, adaptor.Error) {
	resp := relaymodel.GeminiChatResponse{
		Candidates: []*relaymodel.GeminiChatCandidate{
			{
				Index:        0,
				FinishReason: firstNonEmpty(cfg.Gemini.FinishReason, "STOP"),
				Content: relaymodel.GeminiChatContent{
					Role: "model",
					Parts: []*relaymodel.GeminiPart{
						{Text: synthesizeText(cfg, reqCtx)},
					},
				},
			},
		},
		UsageMetadata: new(usageToGeminiUsage(usage)),
		ModelVersion:  firstNonEmpty(cfg.Gemini.ModelVersion, "fake-1.0"),
	}
	if reqCtx.Stream {
		for _, part := range splitChunks(synthesizeText(cfg, reqCtx), cfg.StreamChunks, cfg.StreamChunkSize) {
			chunk := relaymodel.GeminiChatResponse{
				Candidates: []*relaymodel.GeminiChatCandidate{
					{
						Index:        0,
						FinishReason: "",
						Content: relaymodel.GeminiChatContent{
							Role:  "model",
							Parts: []*relaymodel.GeminiPart{{Text: part}},
						},
					},
				},
			}
			_ = render.GeminiObjectData(c, chunk)
		}

		_ = render.GeminiObjectData(c, resp)

		return adaptor.DoResponseResult{
			Usage:      usage,
			UpstreamID: fakeID("gemini", reqCtx.Text),
		}, nil
	}

	if err := writeJSON(c, http.StatusOK, resp); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	return adaptor.DoResponseResult{Usage: usage, UpstreamID: fakeID("gemini", reqCtx.Text)}, nil
}

func writeResponses(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	cfg Config,
	reqCtx requestContext,
	usage model.Usage,
) (adaptor.DoResponseResult, adaptor.Error) {
	responseID := firstNonEmpty(meta.ResponseID, fakeID("resp", reqCtx.Text+meta.RequestID))

	storeResponse := true
	if cfg.Response.Store != nil {
		storeResponse = *cfg.Response.Store
	}

	respObj := buildResponseObject(meta, cfg, reqCtx, usage, responseID, storeResponse)
	if storeResponse {
		_ = store.SaveStore(adaptor.StoreCache{
			ID:        responseID,
			GroupID:   meta.Group.ID,
			TokenID:   meta.Token.ID,
			ChannelID: meta.Channel.ID,
			Model:     actualModel(meta, reqCtx),
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		})
	}

	if reqCtx.Stream {
		if err := streamResponses(c, cfg, respObj, synthesizeText(cfg, reqCtx)); err != nil {
			return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusInternalServerError,
				err.Error(),
			)
		}

		return adaptor.DoResponseResult{Usage: usage, UpstreamID: responseID}, nil
	}

	if err := writeJSON(c, http.StatusCreated, respObj); err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			err.Error(),
		)
	}

	return adaptor.DoResponseResult{Usage: usage, UpstreamID: responseID}, nil
}

func writeResponsesGet(
	meta *meta.Meta,
	c *gin.Context,
	cfg Config,
	usage model.Usage,
) adaptor.DoResponseResult {
	reqCtx := getRequestContext(meta)
	respObj := buildResponseObject(
		meta,
		cfg,
		reqCtx,
		usage,
		firstNonEmpty(meta.ResponseID, fakeID("resp", "get")),
		true,
	)
	_ = writeJSON(c, http.StatusOK, respObj)

	return adaptor.DoResponseResult{Usage: usage, UpstreamID: respObj.ID}
}

func writeResponsesCancel(
	meta *meta.Meta,
	c *gin.Context,
	cfg Config,
	usage model.Usage,
) adaptor.DoResponseResult {
	reqCtx := getRequestContext(meta)
	respObj := buildResponseObject(
		meta,
		cfg,
		reqCtx,
		usage,
		firstNonEmpty(meta.ResponseID, fakeID("resp", "cancel")),
		true,
	)
	respObj.Status = relaymodel.ResponseStatusCancelled
	_ = writeJSON(c, http.StatusOK, respObj)

	return adaptor.DoResponseResult{Usage: usage, UpstreamID: respObj.ID}
}

func writeResponsesInputItems(
	meta *meta.Meta,
	c *gin.Context,
	_ Config,
) {
	reqCtx := getRequestContext(meta)
	if len(reqCtx.InputItems) == 0 {
		reqCtx.InputItems = []relaymodel.InputItem{
			{
				ID:   fakeID("in", meta.ResponseID),
				Type: relaymodel.InputItemTypeMessage,
				Role: relaymodel.RoleUser,
				Content: []relaymodel.InputContent{
					{
						Type: relaymodel.InputContentTypeInputText,
						Text: firstNonEmpty(reqCtx.Text, "fake input"),
					},
				},
			},
		}
	}

	list := relaymodel.InputItemList{
		Object:  "list",
		Data:    reqCtx.InputItems,
		HasMore: false,
	}
	if len(reqCtx.InputItems) > 0 {
		list.FirstID = reqCtx.InputItems[0].ID
		list.LastID = reqCtx.InputItems[len(reqCtx.InputItems)-1].ID
	}

	_ = writeJSON(c, http.StatusOK, list)
}
