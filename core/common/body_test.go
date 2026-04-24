package common_test

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/common"
)

func TestGetRequestBodyReusableJSONWithContentLength(t *testing.T) {
	req := httptest.NewRequestWithContext(
		context.Background(),
		"POST",
		"/v1/chat/completions",
		strings.NewReader(`{"a":1}`),
	)
	req.Header.Set("Content-Type", "application/json")

	body, err := common.GetRequestBodyReusable(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(body) != `{"a":1}` {
		t.Fatalf("unexpected body: %q", string(body))
	}

	cached, ok := common.GetCachedRequestBody(req)
	if !ok {
		t.Fatal("expected cached request body")
	}

	if string(cached) != `{"a":1}` {
		t.Fatalf("unexpected cached body: %q", string(cached))
	}
}

func TestSetRequestBodySyncsBodyAndContentLength(t *testing.T) {
	req := httptest.NewRequestWithContext(
		context.Background(),
		"POST",
		"/v1/chat/completions",
		strings.NewReader(`{"a":1}`),
	)

	common.SetRequestBody(req, []byte(`{"b":2}`))

	if req.ContentLength != int64(len(`{"b":2}`)) {
		t.Fatalf("unexpected content length: %d", req.ContentLength)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if string(body) != `{"b":2}` {
		t.Fatalf("unexpected body: %q", string(body))
	}

	clone, err := req.GetBody()
	if err != nil {
		t.Fatalf("unexpected get body error: %v", err)
	}
	defer clone.Close()

	body, err = io.ReadAll(clone)
	if err != nil {
		t.Fatalf("unexpected clone read error: %v", err)
	}

	if string(body) != `{"b":2}` {
		t.Fatalf("unexpected cloned body: %q", string(body))
	}
}
