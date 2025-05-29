package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/utils"
)

func GetChatRequestPrice(_ *gin.Context, mc model.ModelConfig) (model.Price, error) {
	return mc.Price, nil
}

func GetChatRequestUsage(c *gin.Context, _ model.ModelConfig) (model.Usage, error) {
	textRequest, err := utils.UnmarshalGeneralOpenAIRequest(c.Request)
	if err != nil {
		return model.Usage{}, err
	}

	return model.Usage{
		InputTokens: model.ZeroNullInt64(openai.CountTokenMessages(
			textRequest.Messages,
			textRequest.Model,
		)),
	}, nil
}
