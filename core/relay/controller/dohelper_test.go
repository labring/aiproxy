//nolint:testpackage
package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/require"
)

type testAdaptor struct {
	convertRequest func(
		meta *meta.Meta,
		store adaptor.Store,
		req *http.Request,
	) (adaptor.ConvertResult, error)
}

func (a testAdaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{}
}

func (a testAdaptor) SupportMode(mode.Mode) bool {
	return true
}

func (a testAdaptor) DefaultBaseURL() string {
	return "https://example.com"
}

func (a testAdaptor) GetRequestURL(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
) (adaptor.RequestURL, error) {
	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    "https://example.com/v1/test",
	}, nil
}

func (a testAdaptor) SetupRequestHeader(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	_ *http.Request,
) error {
	return nil
}

func (a testAdaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	return a.convertRequest(meta, store, req)
}

func (a testAdaptor) DoRequest(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	_ *http.Request,
) (*http.Response, error) {
	panic("unexpected DoRequest call")
}

func (a testAdaptor) DoResponse(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	_ *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	panic("unexpected DoResponse call")
}

func newTestRelayContext() (*gin.Context, *meta.Meta) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader("{}"),
	)

	return c, meta.NewMeta(nil, mode.ChatCompletions, "gpt-4o-mini", model.ModelConfig{})
}

func TestPrepareAndDoRequestConvertRequestReturnsAdaptorError(t *testing.T) {
	c, relayMeta := newTestRelayContext()
	expectedErr := relaymodel.WrapperErrorWithMessage(
		relayMeta.Mode,
		http.StatusTooManyRequests,
		"limited",
	)

	resp, err := prepareAndDoRequest(
		context.Background(),
		testAdaptor{
			convertRequest: func(
				_ *meta.Meta,
				_ adaptor.Store,
				_ *http.Request,
			) (adaptor.ConvertResult, error) {
				return adaptor.ConvertResult{}, expectedErr
			},
		},
		c,
		relayMeta,
		nil,
	)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, http.StatusTooManyRequests, err.StatusCode())
}

func TestPrepareAndDoRequestConvertRequestCanceled(t *testing.T) {
	c, relayMeta := newTestRelayContext()

	resp, err := prepareAndDoRequest(
		context.Background(),
		testAdaptor{
			convertRequest: func(
				_ *meta.Meta,
				_ adaptor.Store,
				_ *http.Request,
			) (adaptor.ConvertResult, error) {
				return adaptor.ConvertResult{}, context.Canceled
			},
		},
		c,
		relayMeta,
		nil,
	)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	require.Equal(t, http.StatusBadRequest, err.StatusCode())
	require.Contains(t, err.Error(), "request canceled by client")
}

func TestPrepareAndDoRequestConvertRequestGenericError(t *testing.T) {
	c, relayMeta := newTestRelayContext()

	resp, err := prepareAndDoRequest(
		context.Background(),
		testAdaptor{
			convertRequest: func(
				_ *meta.Meta,
				_ adaptor.Store,
				_ *http.Request,
			) (adaptor.ConvertResult, error) {
				return adaptor.ConvertResult{}, errors.New("invalid payload")
			},
		},
		c,
		relayMeta,
		nil,
	)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	require.Equal(t, http.StatusBadRequest, err.StatusCode())
	require.Contains(t, err.Error(), "convert request failed: invalid payload")
}

func TestPrepareAndDoRequestConvertRequestEOF(t *testing.T) {
	c, relayMeta := newTestRelayContext()

	resp, err := prepareAndDoRequest(
		context.Background(),
		testAdaptor{
			convertRequest: func(
				_ *meta.Meta,
				_ adaptor.Store,
				_ *http.Request,
			) (adaptor.ConvertResult, error) {
				return adaptor.ConvertResult{}, io.EOF
			},
		},
		c,
		relayMeta,
		nil,
	)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	require.Equal(t, http.StatusServiceUnavailable, err.StatusCode())
	require.Contains(t, err.Error(), "request eof")
}
