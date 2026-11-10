package openai_test

import (
	"bytes"
	"io"
	"maps"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

// TestParsePdfRelayEndToEnd drives the adaptor chain against a fake upstream
// AIProxy: the multipart request must be rebuilt with the model name and file,
// and the upstream JSON body must reach the client unchanged.
//
//nolint:gocyclo
func TestParsePdfRelayEndToEnd(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	var gotAuthorization, gotContentType, gotModel, gotResponseFormat, gotFile string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/parse/pdf" {
			t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.Path)

			w.WriteHeader(http.StatusNotFound)

			return
		}

		gotAuthorization = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		mediaType, params, err := mime.ParseMediaType(gotContentType)
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("invalid upstream content type: %s", gotContentType)

			w.WriteHeader(http.StatusBadRequest)

			return
		}

		form, err := multipart.NewReader(r.Body, params["boundary"]).ReadForm(1024 * 1024)
		if err != nil {
			t.Errorf("parse upstream form failed: %v", err)

			w.WriteHeader(http.StatusBadRequest)

			return
		}
		defer func() { _ = form.RemoveAll() }()

		gotModel = form.Value["model"][0]
		gotResponseFormat = strings.Join(form.Value["response_format"], ",")

		file, err := form.File["file"][0].Open()
		if err != nil {
			t.Errorf("open upstream file failed: %v", err)

			w.WriteHeader(http.StatusBadRequest)

			return
		}
		defer file.Close()

		content, _ := io.ReadAll(file)
		gotFile = string(content)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pages":2,"markdown":"# relayed"}`))
	}))
	defer upstream.Close()

	a, ok := registry.Get(model.ChannelTypeOpenAI)
	if !ok {
		t.Fatal("openai adaptor not registered")
	}

	m := meta.NewMeta(
		&model.Channel{
			BaseURL: upstream.URL + "/v1",
			Key:     "sk-upstream-key",
		},
		mode.ParsePdf,
		"pdf",
		model.ModelConfig{},
	)
	m.ActualModel = "pdf"

	var body bytes.Buffer

	formsWriter := multipart.NewWriter(&body)
	if err := formsWriter.WriteField("model", "pdf"); err != nil {
		t.Fatalf("write model field failed: %v", err)
	}

	if err := formsWriter.WriteField("response_format", "list"); err != nil {
		t.Fatalf("write response_format field failed: %v", err)
	}

	part, err := formsWriter.CreateFormFile("file", "doc.pdf")
	if err != nil {
		t.Fatalf("create file field failed: %v", err)
	}

	if _, err := part.Write([]byte("%PDF-1.4 relay-me")); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	if err := formsWriter.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/parse/pdf",
		&body,
	)
	req.Header.Set("Content-Type", formsWriter.FormDataContentType())

	requestURL, err := a.GetRequestURL(m, nil, nil)
	if err != nil {
		t.Fatalf("GetRequestURL failed: %v", err)
	}

	if requestURL.Method != http.MethodPost || requestURL.URL != upstream.URL+"/v1/parse/pdf" {
		t.Fatalf("unexpected request url: %+v", requestURL)
	}

	convertResult, err := a.ConvertRequest(m, nil, req)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	upstreamReq, err := http.NewRequestWithContext(
		t.Context(),
		requestURL.Method,
		requestURL.URL,
		convertResult.Body,
	)
	if err != nil {
		t.Fatalf("create upstream request failed: %v", err)
	}

	maps.Copy(upstreamReq.Header, convertResult.Header)

	if err := a.SetupRequestHeader(m, nil, nil, upstreamReq); err != nil {
		t.Fatalf("SetupRequestHeader failed: %v", err)
	}

	resp, err := a.DoRequest(m, nil, nil, upstreamReq)
	if err != nil {
		t.Fatalf("DoRequest failed: %v", err)
	}

	// Close is idempotent; the response handlers close the body themselves.
	defer func() { _ = resp.Body.Close() }()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/parse/pdf", nil)

	result, relayErr := a.DoResponse(m, nil, c, resp)
	if relayErr != nil {
		t.Fatalf("DoResponse failed: %v", relayErr)
	}

	if gotAuthorization != "Bearer sk-upstream-key" {
		t.Fatalf("upstream authorization = %q", gotAuthorization)
	}

	if !strings.HasPrefix(gotContentType, "multipart/form-data; boundary=") {
		t.Fatalf("upstream content type = %q", gotContentType)
	}

	if gotModel != "pdf" {
		t.Fatalf("upstream model = %q", gotModel)
	}

	if gotResponseFormat != "list" {
		t.Fatalf("upstream response_format = %q", gotResponseFormat)
	}

	if gotFile != "%PDF-1.4 relay-me" {
		t.Fatalf("upstream file = %q", gotFile)
	}

	if result.Usage.InputTokens != 2 || result.Usage.TotalTokens != 2 {
		t.Fatalf("usage = %+v, want 2 pages", result.Usage)
	}

	var relayed map[string]any
	if err := sonic.Unmarshal(recorder.Body.Bytes(), &relayed); err != nil {
		t.Fatalf("relayed body is not json: %v", err)
	}

	if relayed["markdown"] != "# relayed" {
		t.Fatalf("relayed markdown = %v", relayed["markdown"])
	}

	if recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("relayed content type = %q", recorder.Header().Get("Content-Type"))
	}
}
