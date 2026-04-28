package adaptors

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestResponsesDeleteNoContentSupportedAdaptors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for channelType, adaptor := range ChannelAdaptor {
		if channelType == model.ChannelTypeFakeError {
			continue
		}

		if !adaptor.SupportMode(mode.ResponsesDelete) {
			continue
		}

		t.Run(channelType.String(), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			resp := &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     make(http.Header),
				Body:       io.NopCloser(http.NoBody),
			}

			_, err := adaptor.DoResponse(
				&meta.Meta{Mode: mode.ResponsesDelete},
				nil,
				ctx,
				resp,
			)
			if err != nil {
				t.Fatalf("DoResponse returned error for 204 ResponsesDelete: %v", err)
			}
		})
	}
}
