package controller

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
)

type aliVideoRequestUsageParams struct {
	seconds    int
	resolution string
}

func ValidateAliVideoRequest(c *gin.Context, mc model.ModelConfig) error {
	params, err := getAliVideoRequestUsageParams(c)
	if err != nil {
		return err
	}

	return validateAliVideoRequestUsageParams(params, mc)
}

func GetAliVideoRequestPrice(c *gin.Context, mc model.ModelConfig) (model.Price, error) {
	params, err := getAliVideoRequestUsageParams(c)
	if err != nil {
		return model.Price{}, err
	}

	if err := validateAliVideoRequestUsageParams(params, mc); err != nil {
		return model.Price{}, err
	}

	return getVideoRequestPrice(mc.Price), nil
}

func GetAliVideoRequestUsage(c *gin.Context, mc model.ModelConfig) (RequestUsage, error) {
	params, err := getAliVideoRequestUsageParams(c)
	if err != nil {
		return RequestUsage{}, err
	}

	if err := validateAliVideoRequestUsageParams(params, mc); err != nil {
		return RequestUsage{}, err
	}

	return aliVideoRequestUsage(params), nil
}

func getAliVideoRequestUsageParams(c *gin.Context) (aliVideoRequestUsageParams, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(c.Request)
	if err != nil {
		return aliVideoRequestUsageParams{}, NewBadRequestParamError(err.Error())
	}

	parameters := node.Get("parameters")

	seconds, _, err := intValueFromNode(parameters, "duration")
	if err != nil {
		return aliVideoRequestUsageParams{}, err
	}

	return aliVideoRequestUsageParams{
		seconds:    seconds,
		resolution: firstNonEmptyStringValueFromNode(parameters, "size", "resolution"),
	}, nil
}

func validateAliVideoRequestUsageParams(
	params aliVideoRequestUsageParams,
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
		aliVideoSupportedResolutionOptions(mc.AllowedResolutions),
	)
}

func aliVideoRequestUsage(params aliVideoRequestUsageParams) RequestUsage {
	return RequestUsage{
		Usage: model.Usage{},
		Context: model.UsageContext{
			Resolution:       params.resolution,
			NativeResolution: params.resolution,
		},
	}
}

func aliVideoSupportedResolutionOptions(supported []string) string {
	options := normalizeSupportedResolutionValues(supported)
	if len(options) == 0 {
		return noResolutionOptions
	}

	return strings.Join(options, ", ")
}
