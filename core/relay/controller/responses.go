package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
)

func GetResponsesRequestUsage(c *gin.Context, _ model.ModelConfig) (model.Usage, error) {
	return model.Usage{}, nil
}
