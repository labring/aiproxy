package controller

import (
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/model"
	"github.com/labring/aiproxy/relay/utils"
)

func GetTTSRequestPrice(_ *gin.Context, mc *model.ModelConfig) (model.Price, error) {
	return mc.Price, nil
}

func GetTTSRequestUsage(c *gin.Context, _ *model.ModelConfig) (model.Usage, error) {
	ttsRequest, err := utils.UnmarshalTTSRequest(c.Request)
	if err != nil {
		return model.Usage{}, err
	}

	return model.Usage{
		InputTokens: int64(utf8.RuneCountInString(ttsRequest.Input)),
	}, nil
}
