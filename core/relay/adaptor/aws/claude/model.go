package aws

import (
	"fmt"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/pkg/errors"
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
}

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

func awsModelID(requestModel, region string) (string, error) {
	awsModelID, ok := AwsModelIDMap[requestModel]
	if !ok {
		return "", errors.Errorf("model %s not found", requestModel)
	}

	regionPrefix := awsRegionPrefix(region)

	if awsModelCanCrossRegion(awsModelID.ID, regionPrefix) {
		return awsModelCrossRegion(awsModelID.ID, regionPrefix), nil
	}

	return awsModelID.ID, nil
}
