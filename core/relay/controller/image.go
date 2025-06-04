package controller

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

func getImagesRequest(c *gin.Context) (*relaymodel.ImageRequest, error) {
	imageRequest, err := utils.UnmarshalImageRequest(c.Request)
	if err != nil {
		return nil, err
	}
	if imageRequest.Prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if imageRequest.N == 0 {
		imageRequest.N = 1
	}
	return imageRequest, nil
}

func GetImagesOutputPrice(modelConfig model.ModelConfig, size, quality string) (float64, bool) {
	switch {
	case len(modelConfig.ImagePrices) == 0 && len(modelConfig.ImageQualityPrices) == 0:
		return float64(modelConfig.Price.OutputPrice), true
	case len(modelConfig.ImageQualityPrices) != 0:
		price, ok := modelConfig.ImageQualityPrices[size][quality]
		return price, ok
	case len(modelConfig.ImagePrices) != 0:
		price, ok := modelConfig.ImagePrices[size]
		return price, ok
	default:
		return 0, false
	}
}

func GetImagesRequestPrice(c *gin.Context, mc model.ModelConfig) (model.Price, error) {
	imageRequest, err := getImagesRequest(c)
	if err != nil {
		return model.Price{}, err
	}

	imageCostPrice, ok := GetImagesOutputPrice(mc, imageRequest.Size, imageRequest.Quality)
	if !ok {
		return model.Price{}, fmt.Errorf(
			"invalid image size `%s` or quality `%s`",
			imageRequest.Size,
			imageRequest.Quality,
		)
	}

	return model.Price{
		PerRequestPrice:     mc.Price.PerRequestPrice,
		InputPrice:          mc.Price.InputPrice,
		InputPriceUnit:      mc.Price.InputPriceUnit,
		ImageInputPrice:     mc.Price.ImageInputPrice,
		ImageInputPriceUnit: mc.Price.ImageInputPriceUnit,
		OutputPrice:         model.ZeroNullFloat64(imageCostPrice),
		OutputPriceUnit:     mc.Price.OutputPriceUnit,
	}, nil
}

func GetImagesRequestUsage(c *gin.Context, _ model.ModelConfig) (model.Usage, error) {
	imageRequest, err := getImagesRequest(c)
	if err != nil {
		return model.Usage{}, err
	}

	return model.Usage{
		InputTokens: model.ZeroNullInt64(openai.CountTokenInput(
			imageRequest.Prompt,
			imageRequest.Model,
		)),
		OutputTokens: model.ZeroNullInt64(imageRequest.N),
	}, nil
}
