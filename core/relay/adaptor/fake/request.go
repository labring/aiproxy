package fake

import (
	"bytes"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func (a *Adaptor) loadConfig(meta *meta.Meta) Config {
	cfg := defaultConfig()

	loaded, err := a.configCache.Load(meta, cfg)
	if err != nil {
		return cfg
	}

	return loaded
}

func parseRequest(
	m mode.Mode,
	body []byte,
) (requestContext, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return requestContext{}, nil
	}

	switch m {
	case mode.ChatCompletions, mode.Completions:
		var req relaymodel.GeneralOpenAIRequest
		if err := sonic.Unmarshal(body, &req); err != nil {
			return requestContext{}, err
		}

		text := extractMessagesText(req.Messages)
		if text == "" {
			text = anyToString(req.Prompt)
		}

		return requestContext{
			Text:   text,
			Model:  req.Model,
			Stream: req.Stream,
		}, nil
	case mode.Embeddings:
		var req relaymodel.EmbeddingRequest
		if err := sonic.Unmarshal(body, &req); err != nil {
			return requestContext{}, err
		}

		return requestContext{
			Text:  req.Input,
			Model: req.Model,
		}, nil
	case mode.ImagesGenerations:
		var req relaymodel.ImageRequest
		if err := sonic.Unmarshal(body, &req); err != nil {
			return requestContext{}, err
		}

		return requestContext{
			Text:                req.Prompt,
			Model:               req.Model,
			ImageResponseFormat: req.ResponseFormat,
			ImageSize:           req.Size,
		}, nil
	case mode.Rerank:
		var req relaymodel.RerankRequest
		if err := sonic.Unmarshal(body, &req); err != nil {
			return requestContext{}, err
		}

		return requestContext{
			Text:  req.Query,
			Model: req.Model,
		}, nil
	case mode.Anthropic:
		var req relaymodel.ClaudeAnyContentRequest
		if err := sonic.Unmarshal(body, &req); err != nil {
			return requestContext{}, err
		}

		return requestContext{
			Text:   extractClaudeMessagesText(req.Messages),
			Model:  req.Model,
			Stream: req.Stream,
		}, nil
	case mode.Gemini:
		var req relaymodel.GeminiChatRequest
		if err := sonic.Unmarshal(body, &req); err != nil {
			return requestContext{}, err
		}

		return requestContext{
			Text: extractGeminiText(req.Contents),
		}, nil
	case mode.Responses:
		var req relaymodel.CreateResponseRequest
		if err := sonic.Unmarshal(body, &req); err != nil {
			return requestContext{}, err
		}

		return requestContext{
			Text:       anyToString(req.Input),
			Model:      req.Model,
			Stream:     req.Stream,
			InputItems: responseInputItems(req.Input),
		}, nil
	default:
		return requestContext{}, nil
	}
}

func responseInputItems(input any) []relaymodel.InputItem {
	switch v := input.(type) {
	case string:
		return []relaymodel.InputItem{
			{
				ID:   fakeID("in", v),
				Type: relaymodel.InputItemTypeMessage,
				Role: relaymodel.RoleUser,
				Content: []relaymodel.InputContent{
					{Type: relaymodel.InputContentTypeInputText, Text: v},
				},
			},
		}
	default:
		return nil
	}
}

func getRequestContext(meta *meta.Meta) requestContext {
	v, ok := meta.Get("fake_request_context")
	if !ok {
		return requestContext{}
	}

	ctx, _ := v.(requestContext)

	return ctx
}
