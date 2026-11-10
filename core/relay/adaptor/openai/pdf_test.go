//nolint:testpackage
package openai

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func newParsePdfTestMeta() *meta.Meta {
	m := meta.NewMeta(nil, mode.ParsePdf, "pdf", model.ModelConfig{})
	m.ActualModel = "pdf"
	return m
}

func newParsePdfTestRequest(t *testing.T, responseFormat string) *http.Request {
	t.Helper()

	return newParsePdfTestRequestWithQueryModel(t, responseFormat, false)
}

func newParsePdfTestRequestWithQueryModel(
	t *testing.T,
	responseFormat string,
	queryModelOnly bool,
) *http.Request {
	t.Helper()

	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)
	if !queryModelOnly {
		if err := writer.WriteField("model", "pdf"); err != nil {
			t.Fatalf("write model field failed: %v", err)
		}
	}

	if responseFormat != "" {
		if err := writer.WriteField("response_format", responseFormat); err != nil {
			t.Fatalf("write response_format field failed: %v", err)
		}
	}

	part, err := writer.CreateFormFile("file", "test.pdf")
	if err != nil {
		t.Fatalf("create file field failed: %v", err)
	}

	if _, err := part.Write([]byte("%PDF-1.4 test")); err != nil {
		t.Fatalf("write file field failed: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}

	requestURL := "/parse/pdf"
	if queryModelOnly {
		requestURL += "?model=pdf"
	}

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		requestURL,
		&buf,
	)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req
}

func TestConvertParsePdfRequestModelInQuery(t *testing.T) {
	t.Parallel()

	// The request model can arrive via ?model= instead of a form field; the
	// rebuilt form must still carry the model for the upstream.
	req := newParsePdfTestRequestWithQueryModel(t, "", true)

	result, err := ConvertParsePdfRequest(newParsePdfTestMeta(), req)
	if err != nil {
		t.Fatalf("ConvertParsePdfRequest returned error: %v", err)
	}

	contentType := result.Header.Get("Content-Type")

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type failed: %v", err)
	}

	if mediaType != "multipart/form-data" {
		t.Fatalf("unexpected content type: %s", contentType)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}

	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])

	form, err := reader.ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("parse multipart body failed: %v", err)
	}
	defer func() { _ = form.RemoveAll() }()

	if got := form.Value["model"]; len(got) != 1 || got[0] != "pdf" {
		t.Fatalf("model field missing or wrong: %#v", got)
	}
}

func TestConvertParsePdfRequest(t *testing.T) {
	t.Parallel()

	for _, responseFormat := range []string{"", "list"} {
		req := newParsePdfTestRequest(t, responseFormat)

		result, err := ConvertParsePdfRequest(newParsePdfTestMeta(), req)
		if err != nil {
			t.Fatalf("ConvertParsePdfRequest returned error: %v", err)
		}

		contentType := result.Header.Get("Content-Type")

		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			t.Fatalf("parse content type failed: %v", err)
		}

		if mediaType != "multipart/form-data" {
			t.Fatalf("unexpected content type: %s", contentType)
		}

		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}

		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])

		form, err := reader.ReadForm(1024 * 1024)
		if err != nil {
			t.Fatalf("parse multipart body failed: %v", err)
		}

		if form.Value["model"][0] != "pdf" {
			t.Fatalf("model field missing or wrong: %#v", form.Value["model"])
		}

		if responseFormat == "" && len(form.Value["response_format"]) != 0 {
			t.Fatalf("response_format should not be forwarded: %#v", form.Value)
		}

		if responseFormat != "" && form.Value["response_format"][0] != responseFormat {
			t.Fatalf("response_format not forwarded: %#v", form.Value)
		}

		file, err := form.File["file"][0].Open()
		if err != nil {
			t.Fatalf("open file part failed: %v", err)
		}
		defer file.Close()

		fileContent, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read file part failed: %v", err)
		}

		if string(fileContent) != "%PDF-1.4 test" {
			t.Fatalf("file content mismatch: %s", fileContent)
		}
	}
}

func TestParsePdfHandlerRelaysBodyAndUsage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	for name, tc := range map[string]struct {
		body         string
		wantPages    model.ZeroNullInt64
		wantMarkdown string
	}{
		"markdown response": {
			body:         `{"pages":3,"markdown":"# title"}`,
			wantPages:    3,
			wantMarkdown: "# title",
		},
		"list response": {
			body:         `{"markdowns":["a","b"]}`,
			wantPages:    2,
			wantMarkdown: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/parse/pdf",
				nil,
			)

			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(tc.body)),
			}

			result, relayErr := ParsePdfHandler(newParsePdfTestMeta(), c, resp)
			if relayErr != nil {
				t.Fatalf("ParsePdfHandler returned error: %v", relayErr)
			}

			if result.Usage.InputTokens != tc.wantPages {
				t.Fatalf("input tokens = %d, want %d", result.Usage.InputTokens, tc.wantPages)
			}

			if result.Usage.TotalTokens != tc.wantPages {
				t.Fatalf("total tokens = %d, want %d", result.Usage.TotalTokens, tc.wantPages)
			}

			var response map[string]any
			if err := sonic.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode relayed body failed: %v", err)
			}

			if !strings.Contains(w.Body.String(), tc.wantMarkdown) && tc.wantMarkdown != "" {
				t.Fatalf("markdown missing from relayed body: %s", w.Body.String())
			}

			if w.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("unexpected content type: %s", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestParsePdfHandlerUpstreamError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/parse/pdf",
		nil,
	)

	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"message":"upstream unavailable","type":"upstream_error","code":"bad_gateway"}}`,
		)),
	}

	_, relayErr := ParsePdfHandler(newParsePdfTestMeta(), c, resp)
	if relayErr == nil {
		t.Fatal("ParsePdfHandler should return an error for non-200 responses")
	}

	if relayErr.StatusCode() != http.StatusBadGateway {
		t.Fatalf("unexpected status code: %d", relayErr.StatusCode())
	}
}
