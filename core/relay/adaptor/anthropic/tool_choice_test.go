package anthropic_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/adaptor/anthropic"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIConvertRequest_ToolChoiceString(t *testing.T) {
	for choice, want := range map[string]string{
		"auto":     "auto",
		"none":     "none",
		"required": "any",
		"any":      "any",
	} {
		t.Run(choice, func(t *testing.T) {
			body := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}],` +
				`"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}],` +
				`"tool_choice":"` + choice + `"}`
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"http://localhost/v1/chat/completions",
				bytes.NewBufferString(body),
			)
			require.NoError(t, err)

			claudeReq, err := anthropic.OpenAIConvertRequest(&meta.Meta{
				ActualModel: "claude-sonnet-4-20250514",
				OriginModel: "claude-sonnet-4-20250514",
				Mode:        mode.ChatCompletions,
			}, req)
			require.NoError(t, err)

			data, err := sonic.Marshal(claudeReq.ToolChoice)
			require.NoError(t, err)
			assert.JSONEq(t, `{"type":"`+want+`"}`, string(data))
		})
	}
}

func TestConvertGeminiRequestToStruct_ToolChoiceNone(t *testing.T) {
	body := `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],` +
		`"tools":[{"functionDeclarations":[{"name":"f","parameters":{"type":"object"}}]}],` +
		`"toolConfig":{"functionCallingConfig":{"mode":"NONE"}}}`
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"http://localhost/v1beta/models/claude-sonnet-4-20250514:generateContent",
		bytes.NewBufferString(body),
	)
	require.NoError(t, err)

	claudeReq, err := anthropic.ConvertGeminiRequestToStruct(&meta.Meta{
		ActualModel: "claude-sonnet-4-20250514",
		OriginModel: "claude-sonnet-4-20250514",
		Mode:        mode.Gemini,
	}, req)
	require.NoError(t, err)

	data, err := sonic.Marshal(claudeReq.ToolChoice)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"none"}`, string(data))
}
