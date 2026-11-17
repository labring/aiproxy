package controller

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
)

type doubaoVideoRequestUsageParams struct {
	seconds    int
	resolution string
}

func ValidateDoubaoVideoRequest(c *gin.Context, mc model.ModelConfig) error {
	params, err := getDoubaoVideoRequestUsageParams(c)
	if err != nil {
		return err
	}

	return validateDoubaoVideoRequestUsageParams(params, mc)
}

func GetDoubaoVideoRequestPrice(c *gin.Context, mc model.ModelConfig) (model.Price, error) {
	params, err := getDoubaoVideoRequestUsageParams(c)
	if err != nil {
		return model.Price{}, err
	}

	if err := validateDoubaoVideoRequestUsageParams(params, mc); err != nil {
		return model.Price{}, err
	}

	return getVideoRequestPrice(mc.Price), nil
}

func GetDoubaoVideoRequestUsage(c *gin.Context, mc model.ModelConfig) (RequestUsage, error) {
	params, err := getDoubaoVideoRequestUsageParams(c)
	if err != nil {
		return RequestUsage{}, err
	}

	if err := validateDoubaoVideoRequestUsageParams(params, mc); err != nil {
		return RequestUsage{}, err
	}

	return doubaoVideoRequestUsage(params), nil
}

func getDoubaoVideoRequestUsageParams(c *gin.Context) (doubaoVideoRequestUsageParams, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(c.Request)
	if err != nil {
		return doubaoVideoRequestUsageParams{}, NewBadRequestParamError(err.Error())
	}

	seconds, _, err := intValueFromNode(&node, "duration")
	if err != nil {
		return doubaoVideoRequestUsageParams{}, err
	}

	return doubaoVideoRequestUsageParams{
		seconds:    seconds,
		resolution: stringValueFromNode(&node, "resolution"),
	}, nil
}

func validateDoubaoVideoRequestUsageParams(
	params doubaoVideoRequestUsageParams,
	mc model.ModelConfig,
) error {
	if err := validateVideoGenerationSeconds(
		params.seconds,
		mc.MaxVideoGenerationSeconds,
	); err != nil {
		return err
	}

	return validateSupportedVideoResolution(
		params.resolution,
		mc,
		doubaoVideoSupportedResolutionOptions(mc.AllowedResolutions),
	)
}

func doubaoVideoRequestUsage(params doubaoVideoRequestUsageParams) RequestUsage {
	return RequestUsage{
		Usage: model.Usage{},
		Context: model.UsageContext{
			Resolution:       params.resolution,
			NativeResolution: params.resolution,
		},
	}
}

func doubaoVideoSupportedResolutionOptions(supported []string) string {
	options := normalizeSupportedResolutionValues(supported)
	if len(options) == 0 {
		return noResolutionOptions
	}

	return strings.Join(options, ", ")
}
