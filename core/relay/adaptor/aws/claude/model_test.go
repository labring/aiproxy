//nolint:testpackage
package aws

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/stretchr/testify/assert"
)

func TestAWSModelIDFromMetaUsesActualModel(t *testing.T) {
	m := &meta.Meta{
		OriginModel: "claude-opus-4-7",
		ActualModel: "claude-3-haiku-20240307",
	}

	assert.Equal(
		t,
		"us.anthropic.claude-3-haiku-20240307-v1:0",
		awsModelIDFromMeta(m, "us-east-1"),
	)
}

func TestAWSModelIDFromMetaPreservesActualARN(t *testing.T) {
	const arn = "arn:aws:bedrock:us-east-1:123456789012:provisioned-model/test"

	m := &meta.Meta{
		OriginModel: "claude-opus-4-7",
		ActualModel: arn,
	}

	assert.Equal(t, arn, awsModelIDFromMeta(m, "us-east-1"))
}
