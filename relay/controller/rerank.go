package controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/common/config"
	"github.com/labring/aiproxy/relay/meta"
	model "github.com/labring/aiproxy/relay/model"
	"github.com/labring/aiproxy/relay/utils"
)

func RerankHelper(meta *meta.Meta, c *gin.Context) *HandleResult {
	return Handle(meta, c, func() (*PreCheckGroupBalanceReq, error) {
		if !config.GetBillingEnabled() {
			return &PreCheckGroupBalanceReq{}, nil
		}

		inputPrice, outputPrice, cachedPrice, cacheCreationPrice, ok := GetModelPrice(meta.ModelConfig)
		if !ok {
			return nil, fmt.Errorf("model price not found: %s", meta.OriginModel)
		}

		rerankRequest, err := getRerankRequest(c)
		if err != nil {
			return nil, err
		}

		return &PreCheckGroupBalanceReq{
			InputTokens:        rerankPromptTokens(rerankRequest),
			InputPrice:         inputPrice,
			OutputPrice:        outputPrice,
			CachedPrice:        cachedPrice,
			CacheCreationPrice: cacheCreationPrice,
		}, nil
	})
}

func getRerankRequest(c *gin.Context) (*model.RerankRequest, error) {
	rerankRequest, err := utils.UnmarshalRerankRequest(c.Request)
	if err != nil {
		return nil, err
	}
	if rerankRequest.Model == "" {
		return nil, errors.New("model parameter must be provided")
	}
	if rerankRequest.Query == "" {
		return nil, errors.New("query must not be empty")
	}
	if len(rerankRequest.Documents) == 0 {
		return nil, errors.New("document list must not be empty")
	}

	return rerankRequest, nil
}

func rerankPromptTokens(rerankRequest *model.RerankRequest) int {
	return len(rerankRequest.Query) + len(strings.Join(rerankRequest.Documents, ""))
}
