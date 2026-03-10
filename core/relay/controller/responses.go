package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func GetResponsesRequestUsage(c *gin.Context, _ model.ModelConfig) (model.Usage, error) {
	var request relaymodel.CreateResponseRequest

	err := common.UnmarshalRequestReusable(c.Request, &request)
	if err != nil {
		return model.Usage{}, err
	}

	return model.Usage{}, nil
}

func GetResponsesRequestServiceTier(c *gin.Context) (string, error) {
	var request relaymodel.CreateResponseRequest

	err := common.UnmarshalRequestReusable(c.Request, &request)
	if err != nil {
		return "", err
	}

	if request.ServiceTier == nil {
		return "", nil
	}

	return *request.ServiceTier, nil
}
