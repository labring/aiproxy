package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

func GetVideoGenerationJobRequestPrice(_ *gin.Context, mc model.ModelConfig) (model.Price, error) {
	return mc.Price, nil
}

func GetVideoGenerationJobRequestUsage(c *gin.Context, _ model.ModelConfig) (model.Usage, error) {
	_, err := utils.UnmarshalVideoGenerationJobRequest(c.Request)
	if err != nil {
		return model.Usage{}, err
	}

	return model.Usage{}, nil
}
