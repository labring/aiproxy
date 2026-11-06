package gemini_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/require"
)

func TestStreamHandlerToolCallIndexes(t *testing.T) {
	stream := `data: {"candidates":[{"content":{"parts":[{"text":"Calling tools"},{"functionCall":{"name":"weather","args":{"city":"Paris"}},"thoughtSignature":"signature"},{"functionCall":{"name":"weather","args":{"city":"London"}}}]}},{"content":{"parts":[{"functionCall":{"name":"clock","args":{}}}]}}]}

data: {"candidates":[{"content":{"parts":[{"text":"More tools"}]}}]}

data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"clock","args":{}}}]}},{"content":{"parts":[{"functionCall":{"name":"weather","args":{"city":"Rome"}}}]}}]}

data: [DONE]

`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	_, err := gemini.StreamHandler(
		&meta.Meta{OriginModel: "gemini-2.5-pro"},
		c,
		&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(stream)),
		},
	)
	require.Nil(t, err)

	calls := make(map[int][]relaymodel.ToolCall)
	for line := range bytes.SplitSeq(recorder.Body.Bytes(), []byte("\n")) {
		if !bytes.HasPrefix(line, []byte("data: {")) {
			continue
		}

		var chunk relaymodel.ChatCompletionsStreamResponse

		require.NoError(t, json.Unmarshal(bytes.TrimPrefix(line, []byte("data: ")), &chunk))

		for _, choice := range chunk.Choices {
			calls[choice.Index] = append(calls[choice.Index], choice.Delta.ToolCalls...)
		}
	}

	require.Len(t, calls[0], 3)
	require.Len(t, calls[1], 2)

	for choiceIndex, toolCalls := range calls {
		for index, call := range toolCalls {
			require.Equal(t, index, call.Index, "choice %d, call %d", choiceIndex, index)
			require.NotEmpty(t, call.ID)
		}
	}

	require.Equal(t, `{"city":"Paris"}`, calls[0][0].Function.Arguments)
	require.Equal(t, "signature", calls[0][0].ExtraContent.Google.ThoughtSignature)
}
