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

func TestNormalizeSTTTokenUsage(t *testing.T) {
	for _, tt := range []struct{ name, input, want string }{
		{"chat aliases", `{"prompt_tokens":125,"completion_tokens":17,"total_tokens":999,"vendor":{"large":9007199254740993}}`, `{"prompt_tokens":125,"completion_tokens":17,"input_tokens":125,"output_tokens":17,"total_tokens":999,"vendor":{"large":9007199254740993}}`},
		{"canonical zero wins", `{"input_tokens":0,"prompt_tokens":125,"output_tokens":0,"completion_tokens":17}`, `{"input_tokens":0,"prompt_tokens":125,"output_tokens":0,"completion_tokens":17}`},
		{"missing output", `{"input_tokens":125,"total_tokens":999}`, `{"input_tokens":125,"total_tokens":999}`},
		{"only total", `{"total_tokens":999}`, `{"total_tokens":999}`},
		{"invalid canonical", `{"input_tokens":"125","prompt_tokens":125,"output_tokens":-1}`, `{"input_tokens":0,"prompt_tokens":125,"output_tokens":0}`},
		{"fraction and unsafe integer", `{"input_tokens":1.5,"output_tokens":9007199254740992}`, `{"input_tokens":0,"output_tokens":0}`},
		{"null", `null`, `null`},
		{"duration compatibility", `{"type":"duration","seconds":5,"total_tokens":5}`, `{"type":"duration","seconds":5,"total_tokens":5}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := model.NormalizeSTTTokenUsage([]byte(tt.input))
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(actual))
		})
	}

	_, err := model.NormalizeSTTTokenUsage([]byte(`invalid`))
	require.Error(t, err)
}
