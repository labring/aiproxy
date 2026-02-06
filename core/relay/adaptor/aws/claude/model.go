package aws

import (
	"fmt"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

type awsModelItem struct {
	ID string
	model.ModelConfig
}

// AwsModelIDMap maps internal model identifiers to AWS model identifiers.
// For more details, see: https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html

var AwsModelIDMap = map[string]awsModelItem{
	"claude-instant-1.2": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-instant-v1",
	},
	"claude-2.0": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-v2",
	},
	"claude-2.1": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-v2:1",
	},
	"claude-3-haiku-20240307": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-3-haiku-20240307-v1:0",
	},
	"claude-3-5-sonnet-latest": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
	},
	"claude-3-5-haiku-20241022": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-3-5-haiku-20241022-v1:0",
	},
	"claude-3-5-sonnet-20241022": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
	},
	"claude-3-5-sonnet-20240620": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-3-5-sonnet-20240620-v1:0",
	},
	"claude-3-7-sonnet-20250219": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-3-7-sonnet-20250219-v1:0",
	},
	"claude-opus-4-20250514": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-opus-4-20250514-v1:0",
	},
	"claude-sonnet-4-20250514": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-sonnet-4-20250514-v1:0",
	},
	"claude-opus-4-1-20250805": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-opus-4-1-20250805-v1:0",
	},
	"claude-haiku-4-5-20251001": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-haiku-4-5-20251001-v1:0",
	},
	"claude-sonnet-4-5-20250929": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-sonnet-4-5-20250929-v1:0",
	},
	"claude-opus-4-5-20251101": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-opus-4-5-20251101-v1:0",
	},
	"claude-opus-4-6": {
		ModelConfig: model.ModelConfig{
			Type:  mode.ChatCompletions,
			Owner: model.ModelOwnerAnthropic,
		},
		ID: "anthropic.claude-opus-4-6-v1",
	},
}

// https://docs.aws.amazon.com/bedrock/latest/userguide/inference-profiles-support.html
var awsModelCanCrossRegionMap = map[string]map[string]bool{
	"anthropic.claude-3-sonnet-20240229-v1:0": {
		"us": true,
		"eu": true,
		"ap": true,
	},
	"anthropic.claude-3-opus-20240229-v1:0": {
		"us": true,
	},
	"anthropic.claude-3-haiku-20240307-v1:0": {
		"us": true,
		"eu": true,
		"ap": true,
	},
	"anthropic.claude-3-5-sonnet-20240620-v1:0": {
		"us": true,
		"eu": true,
		"ap": true,
	},
	"anthropic.claude-3-5-sonnet-20241022-v2:0": {
		"us": true,
		"ap": true,
	},
	"anthropic.claude-3-5-haiku-20241022-v1:0": {
		"us": true,
	},
	"anthropic.claude-3-7-sonnet-20250219-v1:0": {
		"us": true,
		"ap": true,
		"eu": true,
	},
	"anthropic.claude-sonnet-4-20250514-v1:0": {
		"us": true,
		"ap": true,
		"eu": true,
	},
	"anthropic.claude-opus-4-20250514-v1:0": {
		"us": true,
	},
	"anthropic.claude-opus-4-1-20250805-v1:0": {
		"us": true,
	},
	"anthropic.claude-haiku-4-5-20251001-v1:0": {
		"us": true,
		"ap": true,
		"eu": true,
	},
	"anthropic.claude-sonnet-4-5-20250929-v1:0": {
		"us": true,
		"ap": true,
		"eu": true,
	},
	"anthropic.claude-opus-4-5-20251101-v1:0": {
		"us": true,
		"ap": true,
		"eu": true,
	},
	"anthropic.claude-opus-4-6-v1": {
		"us": true,
		"ap": true,
		"eu": true,
	},
}

var awsRegionCrossModelPrefixMap = map[string]string{
	"us": "us",
	"eu": "eu",
	"ap": "apac",
}

func awsRegionPrefix(awsRegionID string) string {
	parts := strings.Split(awsRegionID, "-")

	regionPrefix := ""
	if len(parts) > 0 {
		regionPrefix = parts[0]
	}

	return regionPrefix
}

func awsModelCanCrossRegion(awsModelID, awsRegionPrefix string) bool {
	regionSet, exists := awsModelCanCrossRegionMap[awsModelID]
	return exists && regionSet[awsRegionPrefix]
}

func awsModelCrossRegion(awsModelID, awsRegionPrefix string) string {
	modelPrefix, find := awsRegionCrossModelPrefixMap[awsRegionPrefix]
	if !find {
		return awsModelID
	}

	return fmt.Sprintf("%s.%s", modelPrefix, awsModelID)
}

func awsModelID(requestModel, region string) string {
	item, ok := AwsModelIDMap[requestModel]
	if ok {
		requestModel = item.ID
	}

	regionPrefix := awsRegionPrefix(region)

	if awsModelCanCrossRegion(requestModel, regionPrefix) {
		return awsModelCrossRegion(requestModel, regionPrefix)
	}

	return requestModel
}
