package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
)

func GetResponsesRequestUsage(c *gin.Context, _ model.ModelConfig) (model.Usage, error) {
	return model.Usage{}, nil
}

// GetResponsesRequestServiceTier extracts service_tier from the request body
func GetResponsesRequestServiceTier(c *gin.Context) (string, error) {
	return GetRequestServiceTier(c)
}
