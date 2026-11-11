package model_test

import (
	"encoding/json"
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSTTUsageZeroSecondsJSON(t *testing.T) {
	for _, usageType := range []string{model.SttUsageTypeTokens, model.SttUsageTypeDuration} {
		t.Run(usageType, func(t *testing.T) {
			usage := model.SttUsage{Type: usageType, TotalTokens: 77}
			body, err := json.Marshal(usage)
			require.NoError(t, err)
			assert.JSONEq(t, `{"type":"`+usageType+`","seconds":0,"total_tokens":77}`, string(body))

			// Adding response seconds must not change the existing duration-to-token conversion.
			if usageType == model.SttUsageTypeDuration {
				assert.Equal(t, coremodel.Usage{}, usage.ToModelUsage())
			} else {
				assert.Equal(t, coremodel.ZeroNullInt64(77), usage.ToModelUsage().TotalTokens)
			}
		})
	}
}
