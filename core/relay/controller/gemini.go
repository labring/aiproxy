package controller

import (
	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func GetGeminiRequestUsage(c *gin.Context, mc model.ModelConfig) (model.Usage, error) {
	var geminiReq relaymodel.GeminiChatRequest

	err := common.UnmarshalRequestReusable(c.Request, &geminiReq)
	if err != nil {
		return model.Usage{}, err
	}

	// Count tokens from all content parts
	totalTokens := int64(0)

	// Count system instruction tokens
	if geminiReq.SystemInstruction != nil {
		for _, part := range geminiReq.SystemInstruction.Parts {
			if part.Text != "" {
				totalTokens += countTokensForText(part.Text, mc.Model)
			}
		}
	}

	// Count tokens from all messages
	for _, content := range geminiReq.Contents {
		for _, part := range content.Parts {
			if part.Text != "" {
				totalTokens += countTokensForText(part.Text, mc.Model)
			}
			// Function calls and responses also consume tokens
			if part.FunctionCall != nil {
				// Approximate token count for function call
				if data, err := sonic.Marshal(part.FunctionCall); err == nil {
					totalTokens += countTokensForText(string(data), mc.Model)
				}
			}

			if part.FunctionResponse != nil {
				// Approximate token count for function response
				if data, err := sonic.Marshal(part.FunctionResponse); err == nil {
					totalTokens += countTokensForText(string(data), mc.Model)
				}
			}
		}
	}

	return model.Usage{
		InputTokens: model.ZeroNullInt64(totalTokens),
	}, nil
}

// countTokensForText provides a rough estimate of token count
// This is a simplified version - in production you might want to use a proper tokenizer
func countTokensForText(text, _ string) int64 {
	// Rough approximation: 1 token ≈ 4 characters for English
	// This is a simplified estimate and should be replaced with proper tokenization
	// for production use
	// Note: modelName parameter is reserved for future use with model-specific tokenizers
	return int64(len(text) / 4)
}
