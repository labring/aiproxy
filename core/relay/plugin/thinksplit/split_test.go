//nolint:testpackage
package thinksplit

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThinkResponseWriterWriteReturnsOriginalLengthAfterTransform(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	rw := &thinkResponseWriter{
		ResponseWriter: c.Writer,
	}

	input := []byte(`{"choices":[{"message":{"content":"<think>
reasoning
</think>
answer"}}]}`)

	n, err := rw.Write(input)
	require.NoError(t, err)
	assert.Equal(t, len(input), n)
	assert.Contains(t, recorder.Body.String(), `"reasoning_content":"`)
	assert.Contains(t, recorder.Body.String(), `reasoning`)
	assert.Contains(t, recorder.Body.String(), `"content":"answer"`)
}
