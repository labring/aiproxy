package controller

import (
	"fmt"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
)

const (
	defaultGeminiVideoDurationSeconds = 8
	defaultGeminiVideoResolution      = "720p"
)

type geminiVideoRequestUsageParams struct {
	seconds    int
	variants   int
	resolution string
}

func ValidateGeminiVideoRequest(c *gin.Context, mc model.ModelConfig) error {
	params, err := getGeminiVideoRequestUsageParams(c)
	if err != nil {
		return err
	}

	return validateGeminiVideoRequestUsageParams(params, mc)
}

func GetGeminiVideoRequestPrice(_ *gin.Context, mc model.ModelConfig) (model.Price, error) {
	return getVideoRequestPrice(mc.Price), nil
}

func GetGeminiVideoRequestUsage(c *gin.Context, mc model.ModelConfig) (RequestUsage, error) {
	params, err := getGeminiVideoRequestUsageParams(c)
	if err != nil {
		return RequestUsage{}, err
	}

	if err := validateGeminiVideoRequestUsageParams(params, mc); err != nil {
		return RequestUsage{}, err
	}

	return RequestUsage{
		// Native Gemini/Veo polling does not expose final duration or resolution
		// metadata, and OpenAI-compatible video models use different units. Keep
		// preflight usage empty; the Gemini adaptor stores request-side metadata
		// for async completion billing without mutating RequestUsage.
		Usage: model.Usage{},
		Context: model.UsageContext{
			Resolution: params.resolution,
		},
	}, nil
}

func validateGeminiNativeVideoResolutionFormat(
	resolution string,
	supported []string,
	fuzzy bool,
) error {
	resolution = normalizeSupportedResolutionValue(resolution)
	switch resolution {
	case "", "720p", "1080p", "4k":
		return nil
	default:
		return NewBadRequestParamError(
			fmt.Sprintf(
				"invalid gemini video resolution `%s`, supported resolutions: %s",
				resolution,
				geminiVideoSupportedResolutionOptions(supported, fuzzy),
			),
		)
	}
}

func getGeminiVideoRequestUsageParams(c *gin.Context) (geminiVideoRequestUsageParams, error) {
	node, err := common.UnmarshalRequest2ValidatedNode(c.Request)
	if err != nil {
		return geminiVideoRequestUsageParams{}, NewBadRequestParamError(err.Error())
	}

	parameters := node.Get("parameters")
	params := geminiVideoRequestUsageParams{
		seconds:    defaultGeminiVideoDurationSeconds,
		variants:   1,
		resolution: defaultGeminiVideoResolution,
	}

	parsedResolution := geminiVideoStringValueFromNode(&node, parameters, "resolution")
	if parsedResolution != "" {
		params.resolution = parsedResolution
	}

	parsedSeconds, ok, err := geminiVideoIntValueFromNode(&node, parameters, "durationSeconds")
	if err != nil {
		return geminiVideoRequestUsageParams{}, err
	}

	if ok && parsedSeconds > 0 {
		params.seconds = parsedSeconds
	}

	parsedVariants, ok, err := geminiVideoIntValueFromNode(&node, parameters, "numberOfVideos")
	if err != nil {
		return geminiVideoRequestUsageParams{}, err
	}

	if ok && parsedVariants > 0 {
		params.variants = parsedVariants
	}

	return params, nil
}

func geminiVideoIntValueFromNode(
	node *ast.Node,
	parameters *ast.Node,
	name string,
) (int, bool, error) {
	value, ok, err := intValueFromNode(node, name)
	if err != nil || (ok && value != 0) {
		return value, ok, err
	}

	parameterValue, parameterOK, err := intValueFromNode(parameters, name)
	if err != nil || parameterOK {
		return parameterValue, parameterOK, err
	}

	return value, ok, nil
}

func geminiVideoStringValueFromNode(
	node *ast.Node,
	parameters *ast.Node,
	name string,
) string {
	if value := stringValueFromNode(node, name); value != "" {
		return value
	}

	return stringValueFromNode(parameters, name)
}

func validateGeminiVideoRequestUsageParams(
	params geminiVideoRequestUsageParams,
	mc model.ModelConfig,
) error {
	fuzzy := !mc.DisableResolutionFuzzyMatch
	if err := validateGeminiNativeVideoResolutionFormat(
		params.resolution,
		mc.AllowedResolutions,
		fuzzy,
	); err != nil {
		return err
	}

	if err := validateVideoGenerationSeconds(
		params.seconds,
		mc.MaxVideoGenerationSeconds,
	); err != nil {
		return err
	}

	if err := validateVideoGenerationCount(
		params.variants,
		mc.MaxVideoGenerationCount,
	); err != nil {
		return err
	}

	return validateSupportedVideoResolution(
		params.resolution,
		mc,
		geminiVideoSupportedResolutionOptions(mc.AllowedResolutions, fuzzy),
	)
}
