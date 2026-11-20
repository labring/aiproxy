package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

const imageRequestParamN = "n"

type RequestParamError struct {
	StatusCode int
	Message    string
}

func (e *RequestParamError) Error() string {
	return e.Message
}

func NewBadRequestParamError(message string) error {
	return &RequestParamError{
		StatusCode: http.StatusBadRequest,
		Message:    message,
	}
}

func validateImageGenerationCount(n, maxCount int) error {
	if maxCount <= 0 || n <= maxCount {
		return nil
	}

	return NewBadRequestParamError(
		fmt.Sprintf("%s must be less than or equal to %d", imageRequestParamN, maxCount),
	)
}

func getImagesRequestN(c *gin.Context) (int, bool, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(c.Request)
	if err != nil {
		return 0, false, err
	}

	iter, err := node.Properties()
	if err != nil {
		return 0, false, err
	}

	count := 0

	var pair ast.Pair
	for iter.Next(&pair) {
		if pair.Key != imageRequestParamN {
			continue
		}

		count++
		if count > 1 {
			return 0, true, NewBadRequestParamError("duplicate " + imageRequestParamN)
		}
	}

	nNode := node.Get(imageRequestParamN)
	if nNode == nil || !nNode.Exists() || nNode.TypeSafe() == ast.V_NULL {
		return 0, false, nil
	}

	n, err := nNode.Int64()
	if err != nil {
		return 0, true, NewBadRequestParamError("invalid " + imageRequestParamN)
	}

	return int(n), true, nil
}

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

func ValidateImagesRequest(c *gin.Context, mc model.ModelConfig) error {
	imageRequest, err := getImagesRequest(c)
	if err != nil {
		return err
	}

	if err := validateSupportedImageResolution(imageRequest.Size, mc); err != nil {
		return err
	}

	n, ok, err := getImagesRequestN(c)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	return validateImageGenerationCount(n, mc.MaxImageGenerationCount)
}

func validateSupportedImageResolution(resolution string, mc model.ModelConfig) error {
	fuzzy := !mc.DisableResolutionFuzzyMatch
	if err := validateOpenAIImageResolutionFormat(
		resolution,
		mc.AllowedResolutions,
		fuzzy,
	); err != nil {
		return err
	}

	if supportedImageResolutionMatches(
		resolution,
		mc.AllowedResolutions,
		fuzzy,
	) {
		return nil
	}

	return NewBadRequestParamError(
		fmt.Sprintf(
			"unsupported image resolution `%s`, supported resolutions: %s",
			resolution,
			openAIImageSupportedResolutionOptions(mc.AllowedResolutions, fuzzy),
		),
	)
}

func GetConditionalImagesOutputPrice(
	price model.Price,
	resolution string,
	quality string,
) (float64, bool) {
	if len(price.ConditionalPrices) == 0 {
		return 0, false
	}

	selectedPrice := price.SelectConditionalPrice(model.Usage{}, model.UsageContext{
		Resolution: resolution,
		Quality:    quality,
	})
	if len(selectedPrice.ConditionalPrices) != 0 {
		return 0, false
	}

	return float64(selectedPrice.OutputPrice), true
}

func setImageOutputPriceUnit(price *model.Price, force bool) {
	if price == nil {
		return
	}

	if (force || len(price.ConditionalPrices) != 0) && price.OutputPriceUnit == 0 {
		price.OutputPriceUnit = 1
	}

	if (force || len(price.ConditionalPrices) != 0) && price.ImageOutputPriceUnit == 0 {
		price.ImageOutputPriceUnit = 1
	}

	for i := range price.ConditionalPrices {
		setImageOutputPriceUnit(&price.ConditionalPrices[i].Price, true)
	}
}

func GetImagesRequestPrice(c *gin.Context, mc model.ModelConfig) (model.Price, error) {
	if _, err := getImagesRequest(c); err != nil {
		return model.Price{}, err
	}

	if len(mc.Price.ConditionalPrices) != 0 {
		price := mc.Price
		setImageOutputPriceUnit(&price, false)

		return price, nil
	}

	return model.Price{
		PerRequestPrice:      mc.Price.PerRequestPrice,
		InputPrice:           mc.Price.InputPrice,
		InputPriceUnit:       mc.Price.InputPriceUnit,
		ImageInputPrice:      mc.Price.ImageInputPrice,
		ImageInputPriceUnit:  mc.Price.ImageInputPriceUnit,
		OutputPrice:          mc.Price.OutputPrice,
		OutputPriceUnit:      mc.Price.OutputPriceUnit,
		ImageOutputPrice:     mc.Price.ImageOutputPrice,
		ImageOutputPriceUnit: mc.Price.ImageOutputPriceUnit,
	}, nil
}

func GetImagesRequestUsage(c *gin.Context, _ model.ModelConfig) (RequestUsage, error) {
	imageRequest, err := getImagesRequest(c)
	if err != nil {
		return RequestUsage{}, err
	}

	return RequestUsage{
		// Image output usage depends on the upstream billing model. Some providers
		// bill by returned image count, while GPT image models return image tokens.
		// Keep preflight usage empty and let the response handler provide final
		// usage from the upstream response or actual returned image count.
		Usage: model.Usage{},
		Context: model.UsageContext{
			Resolution: imageRequest.Size,
			Quality:    imageRequest.Quality,
		},
	}, nil
}
