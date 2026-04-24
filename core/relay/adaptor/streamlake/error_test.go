//nolint:testpackage
package streamlake

import (
	"io"
	"net/http"
	"strings"
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestErrorHandlerSystemUnsafe(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body: io.NopCloser(
			strings.NewReader(
				`{"error":{"code":"system_unsafe","message":"the content of system field is invalid","type":"Forbidden"}}`,
			),
		),
	}

	err := ErrorHandler(resp)
	if err.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.StatusCode())
	}
}

func TestAdaptorDoResponseUsesStreamlakeErrorHandler(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://wanqing.streamlakeapi.com/api/gateway/v1/endpoints"},
		mode.ChatCompletions,
		"deepseek-v3.1",
		coremodel.ModelConfig{},
	)
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body: io.NopCloser(
			strings.NewReader(
				`{"error":{"code":"system_unsafe","message":"the content of system field is invalid","type":"Forbidden"}}`,
			),
		),
	}

	_, err := adaptor.DoResponse(m, nil, nil, resp)
	if err == nil {
		t.Fatal("expected streamlake error")
	}

	if err.StatusCode() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.StatusCode())
	}
}
