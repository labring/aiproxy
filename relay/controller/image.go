package controller

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/model"
	relaymodel "github.com/labring/aiproxy/relay/model"
	"github.com/labring/aiproxy/relay/utils"
)

func getImageRequest(c *gin.Context) (*relaymodel.ImageRequest, error) {
	imageRequest, err := utils.UnmarshalImageRequest(c.Request)
	if err != nil {
		return nil, err
	}
	if imageRequest.Prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if imageRequest.Size == "" {
		return nil, errors.New("size is required")
	}
	if imageRequest.N == 0 {
		imageRequest.N = 1
	}
	return imageRequest, nil
}

func GetImageSizePrice(modelConfig *model.ModelConfig, size string) (float64, bool) {
	if len(modelConfig.ImagePrices) == 0 {
		return modelConfig.Price.InputPrice, true
	}
	price, ok := modelConfig.ImagePrices[size]
	return price, ok
}

func GetImageRequestPrice(c *gin.Context, mc *model.ModelConfig) (model.Price, error) {
	imageRequest, err := getImageRequest(c)
	if err != nil {
		return model.Price{}, err
	}

	imageCostPrice, ok := GetImageSizePrice(mc, imageRequest.Size)
	if !ok {
		return model.Price{}, fmt.Errorf("invalid image size: %s", imageRequest.Size)
	}

	return model.Price{
		InputPrice: imageCostPrice,
	}, nil
}

func GetImageRequestUsage(c *gin.Context, _ *model.ModelConfig) (model.Usage, error) {
	imageRequest, err := getImageRequest(c)
	if err != nil {
		return model.Usage{}, err
	}

	return model.Usage{
		InputTokens: int64(imageRequest.N),
	}, nil
}
