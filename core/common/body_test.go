package common_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
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

	if req.GetBody != nil {
		t.Fatal("expected GetBody to be cleared")
	}
}

func TestGetRequestBodyReusableDecodesZstd(t *testing.T) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("unexpected encoder error: %v", err)
	}
	defer encoder.Close()

	original := []byte(`{"model":"gpt-5.6-sol","input":[{"type":"compaction_trigger"}]}`)
	compressed := encoder.EncodeAll(original, nil)
	req := httptest.NewRequestWithContext(
		context.Background(),
		"POST",
		"/v1/responses",
		bytes.NewReader(compressed),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")

	body, err := common.GetRequestBodyReusable(req)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if !bytes.Equal(body, original) {
		t.Fatalf("unexpected decoded body: %q", body)
	}

	if got := req.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("expected content encoding to be cleared, got %q", got)
	}

	if req.ContentLength != int64(len(original)) {
		t.Fatalf("unexpected content length: %d", req.ContentLength)
	}
}

func TestGetRequestBodyReusableRejectsOversizedZstdOutput(t *testing.T) {
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("unexpected encoder error: %v", err)
	}
	defer encoder.Close()

	compressed := encoder.EncodeAll(
		bytes.Repeat([]byte{'a'}, int(config.MaxRequestBodySize)+1),
		nil,
	)
	req := httptest.NewRequestWithContext(
		context.Background(),
		"POST",
		"/v1/responses",
		bytes.NewReader(compressed),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")

	_, err = common.GetRequestBodyReusable(req)
	if err == nil || !strings.Contains(err.Error(), "decompressed request body too large") {
		t.Fatalf("expected decompressed size error, got %v", err)
	}
}

func TestGetRequestBodyReusableRejectsCorruptZstd(t *testing.T) {
	req := httptest.NewRequestWithContext(
		context.Background(),
		"POST",
		"/v1/responses",
		strings.NewReader("invalid zstd data"),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "zstd")

	_, err := common.GetRequestBodyReusable(req)
	if err == nil || !strings.Contains(err.Error(), "zstd request body decode failed") {
		t.Fatalf("expected zstd decode error, got %v", err)
	}
}

func TestParseFormWithLimitDecodesZstd(t *testing.T) {
	original := []byte("model=gpt-5&n=2")
	req := newZstdRequest(t, "application/x-www-form-urlencoded", original)

	err := common.ParseFormWithLimit(req)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if got := req.PostForm.Get("model"); got != "gpt-5" {
		t.Fatalf("unexpected model: %q", got)
	}

	if got := req.PostForm.Get("n"); got != "2" {
		t.Fatalf("unexpected n: %q", got)
	}

	assertZstdRequestDecoded(t, req, len(original))
}

func TestParseMultipartFormWithLimitDecodesZstd(t *testing.T) {
	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "gpt-5"); err != nil {
		t.Fatalf("write form field: %v", err)
	}

	part, err := writer.CreateFormFile("image", "image.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	if _, err := part.Write([]byte("image-data")); err != nil {
		t.Fatalf("write form file: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	original := body.Bytes()
	req := newZstdRequest(t, writer.FormDataContentType(), original)

	if err := common.ParseMultipartFormWithLimit(req); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if got := req.FormValue("model"); got != "gpt-5" {
		t.Fatalf("unexpected model: %q", got)
	}

	file, header, err := req.FormFile("image")
	if err != nil {
		t.Fatalf("get form file: %v", err)
	}
	defer file.Close()

	if header.Filename != "image.png" {
		t.Fatalf("unexpected filename: %q", header.Filename)
	}

	fileBody, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read form file: %v", err)
	}

	if string(fileBody) != "image-data" {
		t.Fatalf("unexpected file body: %q", fileBody)
	}

	assertZstdRequestDecoded(t, req, len(original))
}

func newZstdRequest(t *testing.T, contentType string, body []byte) *http.Request {
	t.Helper()

	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("create zstd encoder: %v", err)
	}
	defer encoder.Close()

	compressed := encoder.EncodeAll(body, nil)
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/test",
		bytes.NewReader(compressed),
	)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Encoding", "zstd")

	return req
}

func assertZstdRequestDecoded(t *testing.T, req *http.Request, bodyLength int) {
	t.Helper()

	if got := req.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("expected content encoding to be cleared, got %q", got)
	}

	if req.ContentLength != int64(bodyLength) {
		t.Fatalf("unexpected content length: %d", req.ContentLength)
	}
}

func TestParseFormWithLimitRejectsTooLargeContentLength(t *testing.T) {
	req := httptest.NewRequestWithContext(
		context.Background(),
		"POST",
		"/v1/images/edits",
		strings.NewReader("n=1"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = config.MaxRequestBodySize + 1

	err := common.ParseFormWithLimit(req)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "request body too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetJSONNodeNoCopy(t *testing.T) {
	node, err := common.GetJSONNodeNoCopy(
		[]byte(`{"choices":[{"message":{"content":"hello"}}]}`),
		"choices",
		0,
		"message",
		"content",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	value, err := node.String()
	if err != nil {
		t.Fatalf("unexpected string error: %v", err)
	}

	if value != "hello" {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestGetResponseBodyLimitKnownLengthTooLargeHidesLimit(t *testing.T) {
	resp := &http.Response{
		Body:          io.NopCloser(strings.NewReader("abcd")),
		ContentLength: 4,
	}

	_, err := common.GetResponseBodyLimit(resp, 3)
	if err == nil {
		t.Fatal("expected error")
	}

	if got := err.Error(); got != "response body too large" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestGetResponseBodyLimitUnknownLengthTooLargeHidesLimit(t *testing.T) {
	resp := &http.Response{
		Body:          io.NopCloser(strings.NewReader("abcd")),
		ContentLength: -1,
	}

	_, err := common.GetResponseBodyLimit(resp, 3)
	if err == nil {
		t.Fatal("expected error")
	}

	if got := err.Error(); got != "response body too large" {
		t.Fatalf("unexpected error: %q", got)
	}
}
