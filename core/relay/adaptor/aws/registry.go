package aws

import (
	"strings"

	"github.com/labring/aiproxy/core/model"
	claude "github.com/labring/aiproxy/core/relay/adaptor/aws/claude"
	"github.com/labring/aiproxy/core/relay/adaptor/aws/utils"
)

type ModelType int

const (
	AwsClaude ModelType = iota + 1
)

type Model struct {
	config    model.ModelConfig
	modelType ModelType
}

var adaptors = map[string]Model{}

func init() {
	for name, model := range claude.AwsModelIDMap {
		model.Model = name
		adaptors[model.Model] = Model{config: model.ModelConfig, modelType: AwsClaude}
	}
}

func GetAdaptor(model string) utils.AwsAdapter {
	adaptorType := adaptors[model]
	switch {
	case adaptorType.modelType == AwsClaude,
		strings.Contains(model, "claude-"):
		return &claude.Adaptor{}
	default:
		return nil
	}
}
