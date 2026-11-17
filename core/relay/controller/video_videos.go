package controller

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
)

type videosRequestUsageParams struct {
	seconds int
	size    string
}

func ValidateVideosRequest(c *gin.Context, mc model.ModelConfig) error {
	params, err := getVideosRequestUsageParams(c)
	if err != nil {
		return err
	}

	return validateVideosRequestUsageParams(params, mc)
}

func GetVideosRequestPrice(c *gin.Context, mc model.ModelConfig) (model.Price, error) {
	params, err := getVideosRequestUsageParams(c)
	if err != nil {
		return model.Price{}, err
	}

	if err := validateVideosRequestUsageParams(params, mc); err != nil {
		return model.Price{}, err
	}

	return getVideoRequestPrice(mc.Price), nil
}

func GetVideosRequestUsage(c *gin.Context, mc model.ModelConfig) (RequestUsage, error) {
	params, err := getVideosRequestUsageParams(c)
	if err != nil {
		return RequestUsage{}, err
	}

	if err := validateVideosRequestUsageParams(params, mc); err != nil {
		return RequestUsage{}, err
	}

	return RequestUsage{
		// Video usage is provider-specific and often async. Do not use requested
		// seconds as a preflight balance estimate; final usage is supplied by the
		// response or async usage fetcher.
		Usage: model.Usage{},
		Context: model.UsageContext{
			Resolution: params.size,
		},
	}, nil
}

func getVideosRequestUsageParams(c *gin.Context) (videosRequestUsageParams, error) {
	if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		if err := common.ParseMultipartFormWithLimit(c.Request); err != nil {
			return videosRequestUsageParams{}, NewBadRequestParamError(err.Error())
		}

		seconds, err := parseOptionalPositiveInt(c.PostForm("seconds"), "seconds")
		if err != nil {
			return videosRequestUsageParams{}, err
		}

		return videosRequestUsageParams{
			seconds: seconds,
			size:    c.PostForm("size"),
		}, nil
	}

	node, err := common.UnmarshalRequest2ValidatedNode(c.Request)
	if err != nil {
		return videosRequestUsageParams{}, NewBadRequestParamError(err.Error())
	}

	seconds, _, err := intValueFromNode(&node, "seconds")
	if err != nil {
		return videosRequestUsageParams{}, err
	}

	return videosRequestUsageParams{
		seconds: seconds,
		size:    firstNonEmptyStringValueFromNode(&node, "size", "resolution"),
	}, nil
}

func validateVideosRequestUsageParams(params videosRequestUsageParams, mc model.ModelConfig) error {
	fuzzy := !mc.DisableResolutionFuzzyMatch
	if err := validateOpenAIVideoSizeFormat(params.size, mc.AllowedResolutions, fuzzy); err != nil {
		return err
	}

	if err := validateVideoGenerationSeconds(
		params.seconds,
		mc.MaxVideoGenerationSeconds,
	); err != nil {
		return err
	}

	return validateSupportedVideoResolution(
		params.size,
		mc,
		openAIVideoSupportedResolutionOptions(mc.AllowedResolutions, fuzzy),
	)
}
