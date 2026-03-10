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

	usage := model.Usage{}
	if request.ServiceTier != nil {
		usage.ServiceTier = *request.ServiceTier
	}

	return usage, nil
}
