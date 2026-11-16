//nolint:bodyclose // synthetic in-memory responses; the handlers under test close the body
package voyage_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/voyage"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func newRequest(t *testing.T, body string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/embeddings",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	return req
}

func readBody(t *testing.T, body io.Reader) map[string]any {
	t.Helper()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var got map[string]any
	if err := sonic.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}

	return got
}

// obj returns v as a JSON object, failing the test otherwise.
func obj(t *testing.T, v any) map[string]any {
	t.Helper()

	out, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected a JSON object, got %T", v)
	}

	return out
}

// arr returns v as a JSON array, failing the test otherwise.
func arr(t *testing.T, v any) []any {
	t.Helper()

	out, ok := v.([]any)
	if !ok {
		t.Fatalf("expected a JSON array, got %T", v)
	}

	return out
}

func newResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newMeta(m mode.Mode, name string) *meta.Meta {
	return meta.NewMeta(nil, m, name, coremodel.ModelConfig{})
}

// newContextualMeta returns the meta that converting a request with n inputs
// leaves behind; the response handler checks the returned documents against it.
func newContextualMeta(t *testing.T, name string, n int) *meta.Meta {
	t.Helper()

	items := make([]string, n)
	for i := range items {
		items[i] = `"x"`
	}

	m := newMeta(mode.Embeddings, name)

	_, err := voyage.ConvertContextualizedEmbeddingsRequest(
		m,
		newRequest(t, `{"input":[`+strings.Join(items, ",")+`],"input_type":"document"}`),
	)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	return m
}

func newGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)

	return c, rec
}

func TestConvertEmbeddingsRequest(t *testing.T) {
	t.Parallel()

	result, err := voyage.ConvertEmbeddingsRequest(
		newMeta(mode.Embeddings, "voyage-4-lite"),
		newRequest(
			t,
			`{"model":"alias","input":"hello","encoding_format":"float","dimensions":256,"input_type":"query"}`,
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBody(t, result.Body)

	if _, ok := got["encoding_format"]; ok {
		t.Fatal("encoding_format=float must be dropped, Voyage rejects it")
	}

	if _, ok := got["dimensions"]; ok {
		t.Fatal("dimensions must be renamed")
	}

	if got["output_dimension"] != float64(256) {
		t.Fatalf("output_dimension = %v, want 256", got["output_dimension"])
	}

	if got["input_type"] != "query" || got["model"] != "voyage-4-lite" {
		t.Fatalf("input_type/model not preserved: %#v", got)
	}

	input, ok := got["input"].([]any)
	if !ok || len(input) != 1 || input[0] != "hello" {
		t.Fatalf("string input must become a one-element list, got %#v", got["input"])
	}
}

func TestConvertEmbeddingsRequestExplicitOutputDimensionWins(t *testing.T) {
	t.Parallel()

	result, err := voyage.ConvertEmbeddingsRequest(
		newMeta(mode.Embeddings, "voyage-4"),
		newRequest(t, `{"input":["a"],"dimensions":512,"output_dimension":2048}`),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBody(t, result.Body)
	if got["output_dimension"] != float64(2048) {
		t.Fatalf("output_dimension = %v, want 2048", got["output_dimension"])
	}
}

func TestConvertEmbeddingsRequestRejectsBadInput(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"unknown encoding format": `{"input":["a"],"encoding_format":"int8"}`,
		"non-string encoding":     `{"input":["a"],"encoding_format":1}`,
		"missing input":           `{"model":"voyage-4"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := voyage.ConvertEmbeddingsRequest(
				newMeta(mode.Embeddings, "voyage-4"),
				newRequest(t, body),
			)
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestConvertMultimodalEmbeddingsRequest(t *testing.T) {
	t.Parallel()

	result, err := voyage.ConvertMultimodalEmbeddingsRequest(
		newMeta(mode.Embeddings, "voyage-multimodal-3.5"),
		newRequest(t, `{
			"input": [
				"plain text",
				{"type":"image_url","image_url":{"url":"https://example.com/a.png"}},
				{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}},
				{"content":[{"type":"text","text":"mixed"},{"type":"image_url","image_url":"https://example.com/b.png"}]}
			],
			"encoding_format":"float",
			"dimensions":512,
			"input_type":"document"
		}`),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBody(t, result.Body)

	if _, ok := got["input"]; ok {
		t.Fatal("input must be replaced by inputs")
	}

	if _, ok := got["encoding_format"]; ok || got["output_dimension"] != float64(512) {
		t.Fatalf("encoding_format/dimensions not normalised: %#v", got)
	}

	inputs, ok := got["inputs"].([]any)
	if !ok || len(inputs) != 4 {
		t.Fatalf("want 4 inputs, got %#v", got["inputs"])
	}

	wantFirstTypes := []string{"text", "image_url", "image_base64"}
	for i, want := range wantFirstTypes {
		content := arr(t, obj(t, inputs[i])["content"])
		if len(content) != 1 || obj(t, content[0])["type"] != want {
			t.Fatalf("input %d content = %#v, want type %s", i, content, want)
		}
	}

	base64Item := obj(t, arr(t, obj(t, inputs[2])["content"])[0])
	if base64Item["image_base64"] != "data:image/png;base64,AAAA" {
		t.Fatalf("data URL must be forwarded unchanged, got %#v", base64Item)
	}

	mixed := arr(t, obj(t, inputs[3])["content"])
	if len(mixed) != 2 {
		t.Fatalf("content array must be forwarded as is, got %#v", mixed)
	}
}

func TestConvertMultimodalEmbeddingsRequestRejectsBadInput(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"video not relayed":    `{"input":[{"type":"video_url","video_url":"https://example.com/v.mp4"}]}`,
		"video in content":     `{"input":[{"content":[{"type":"text","text":"x"},{"type":"video_url","video_url":"https://example.com/v.mp4"}]}]}`,
		"content item no type": `{"input":[{"content":[{"text":"x"}]}]}`,
		"object without type":  `{"input":[{"foo":"bar"}]}`,
		"empty input":          `{"input":[]}`,
		"number input":         `{"input":[5]}`,
		"empty image url":      `{"input":[{"type":"image_url","image_url":{"url":""}}]}`,
		"content not an array": `{"input":[{"content":"x"}]}`,
		"missing input":        `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := voyage.ConvertMultimodalEmbeddingsRequest(
				newMeta(mode.Embeddings, "voyage-multimodal-3"),
				newRequest(t, body),
			)
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestConvertRerankRequestMapsTopN(t *testing.T) {
	t.Parallel()

	result, err := voyage.ConvertRerankRequest(
		newMeta(mode.Rerank, "rerank-3"),
		newRequest(t, `{"query":"q","documents":["a","b"],"top_n":1,"return_documents":true}`),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBody(t, result.Body)
	if _, ok := got["top_n"]; ok || got["top_k"] != float64(1) {
		t.Fatalf("top_n must become top_k, got %#v", got)
	}

	if got["return_documents"] != true {
		t.Fatalf("return_documents must be passed through, got %#v", got)
	}
}

func TestEmbeddingsHandler(t *testing.T) {
	t.Parallel()

	m := newMeta(mode.Embeddings, "voyage-4-lite")
	c, rec := newGinContext()

	result, err := voyage.EmbeddingsHandler(m, c, newResponse(
		http.StatusOK,
		`{"object":"list","data":[{"object":"embedding","embedding":[0.1],"index":0}],"model":"voyage-4-lite","usage":{"total_tokens":7}}`,
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Usage.InputTokens != 7 || result.Usage.TotalTokens != 7 {
		t.Fatalf("usage = %+v, want 7 tokens", result.Usage)
	}

	got := readBody(t, rec.Body)
	if obj(t, got["usage"])["prompt_tokens"] != float64(7) {
		t.Fatalf("prompt_tokens must be filled for OpenAI clients: %#v", got["usage"])
	}
}

func TestHandlersFailClosedWithoutUsage(t *testing.T) {
	t.Parallel()

	for name, run := range map[string]func() error{
		"embeddings": func() error {
			c, _ := newGinContext()
			_, err := voyage.EmbeddingsHandler(newMeta(mode.Embeddings, "voyage-4"), c,
				newResponse(http.StatusOK, `{"data":[{"embedding":[0.1],"index":0}]}`))

			return err
		},
		"embeddings negative tokens": func() error {
			c, _ := newGinContext()
			_, err := voyage.EmbeddingsHandler(newMeta(mode.Embeddings, "voyage-4"), c,
				newResponse(http.StatusOK, `{"data":[],"usage":{"total_tokens":-1}}`))

			return err
		},
		"multimodal": func() error {
			c, _ := newGinContext()
			_, err := voyage.MultimodalEmbeddingsHandler(
				newMeta(mode.Embeddings, "voyage-multimodal-3"), c,
				newResponse(http.StatusOK, `{"data":[{"embedding":[0.1],"index":0}],"usage":{}}`),
			)

			return err
		},
		"rerank": func() error {
			c, _ := newGinContext()
			_, err := voyage.RerankHandler(newMeta(mode.Rerank, "rerank-3"), c,
				newResponse(http.StatusOK, `{"data":[{"index":0,"relevance_score":0.5}]}`))

			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if run() == nil {
				t.Fatal("a response without usage must fail instead of billing zero")
			}
		})
	}
}

func TestMultimodalEmbeddingsHandlerSplitsImageUsage(t *testing.T) {
	t.Parallel()

	c, _ := newGinContext()

	result, err := voyage.MultimodalEmbeddingsHandler(
		newMeta(mode.Embeddings, "voyage-multimodal-3"), c,
		newResponse(
			http.StatusOK,
			`{"data":[{"embedding":[0.1],"index":0,"text":null}],"usage":{"text_tokens":1,"image_pixels":50000,"video_pixels":0,"total_tokens":90}}`,
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Usage.InputTokens != 90 || result.Usage.ImageInputTokens != 89 {
		t.Fatalf("usage = %+v, want input 90 / image 89", result.Usage)
	}
}

func TestRerankHandlerConvertsResponse(t *testing.T) {
	t.Parallel()

	c, rec := newGinContext()

	result, err := voyage.RerankHandler(newMeta(mode.Rerank, "rerank-3"), c,
		newResponse(
			http.StatusOK,
			`{"object":"list","data":[{"relevance_score":0.93,"index":1,"document":"Paris"},{"relevance_score":0.1,"index":0}],"model":"rerank-3","usage":{"total_tokens":12}}`,
		))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Usage.InputTokens != 12 {
		t.Fatalf("usage = %+v", result.Usage)
	}

	got := readBody(t, rec.Body)
	if _, ok := got["data"]; ok {
		t.Fatal("data must be converted to results")
	}

	results := arr(t, got["results"])
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}

	first := obj(t, results[0])
	if first["index"] != float64(1) || first["relevance_score"] != 0.93 {
		t.Fatalf("first result = %#v", first)
	}

	if obj(t, first["document"])["text"] != "Paris" {
		t.Fatalf("document must be wrapped as {text}: %#v", first)
	}

	if obj(t, obj(t, got["meta"])["tokens"])["input_tokens"] != float64(12) {
		t.Fatalf("meta.tokens.input_tokens missing: %#v", got["meta"])
	}
}

func TestRerankHandlerRejectsMalformedResults(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"no data":  `{"usage":{"total_tokens":3}}`,
		"no index": `{"data":[{"relevance_score":0.5}],"usage":{"total_tokens":3}}`,
		"no score": `{"data":[{"index":0}],"usage":{"total_tokens":3}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, _ := newGinContext()

			_, err := voyage.RerankHandler(newMeta(mode.Rerank, "rerank-3"), c,
				newResponse(http.StatusOK, body))
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestErrorHandlerExtractsDetail(t *testing.T) {
	t.Parallel()

	c, _ := newGinContext()

	_, err := voyage.EmbeddingsHandler(newMeta(mode.Embeddings, "voyage-4"), c,
		newResponse(http.StatusBadRequest, `{"detail":"Provided model is not supported."}`))
	if err == nil {
		t.Fatal("expected an error")
	}

	data, _ := err.MarshalJSON()
	if !strings.Contains(string(data), "Provided model is not supported.") {
		t.Fatalf("detail not surfaced: %s", data)
	}
}

func TestErrorHandlerKeepsUnrecognisedBody(t *testing.T) {
	t.Parallel()

	c, _ := newGinContext()

	_, err := voyage.RerankHandler(newMeta(mode.Rerank, "rerank-3"), c,
		newResponse(http.StatusBadGateway, `<html>upstream down</html>`))
	if err == nil {
		t.Fatal("expected an error")
	}

	data, _ := err.MarshalJSON()
	if !strings.Contains(string(data), "HTTP 502") ||
		!strings.Contains(string(data), "upstream down") {
		t.Fatalf("status and body must be kept: %s", data)
	}
}

func TestModelListIsExplicit(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for _, m := range voyage.ModelList {
		if seen[m.Model] {
			t.Fatalf("duplicate model %s", m.Model)
		}

		seen[m.Model] = true

		if m.Price.InputPrice <= 0 {
			t.Fatalf("model %s has no input price", m.Model)
		}
	}

	for _, want := range []string{"voyage-4-large", "voyage-multimodal-3.5", "rerank-3", "rerank-3-lite"} {
		if !seen[want] {
			t.Fatalf("model %s missing from the catalog", want)
		}
	}
}

func TestAdaptorOnlySupportsEmbeddingsAndRerank(t *testing.T) {
	t.Parallel()

	a := &voyage.Adaptor{}

	for _, m := range []mode.Mode{mode.Embeddings, mode.Rerank} {
		if !a.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("mode %s must be supported", m)
		}
	}

	for _, m := range []mode.Mode{mode.ChatCompletions, mode.ImagesGenerations, mode.AudioSpeech, mode.Responses} {
		if a.SupportMode(&meta.Meta{Mode: m}) {
			t.Fatalf("mode %s must not be supported", m)
		}
	}
}

// Voyage reports 0 tokens for short query inputs; that is the billed figure, not a missing usage.
func TestEmbeddingsHandlerAcceptsReportedZeroTokens(t *testing.T) {
	t.Parallel()

	c, _ := newGinContext()

	result, err := voyage.EmbeddingsHandler(
		newMeta(mode.Embeddings, "voyage-4"),
		c,
		newResponse(
			http.StatusOK,
			`{"data":[{"embedding":[0.1],"index":0}],"usage":{"total_tokens":0}}`,
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Usage.TotalTokens != 0 {
		t.Fatalf("usage = %+v", result.Usage)
	}
}

func TestConvertContextualizedEmbeddingsRequestDocument(t *testing.T) {
	t.Parallel()

	result, err := voyage.ConvertContextualizedEmbeddingsRequest(
		newMeta(mode.Embeddings, "voyage-context-4"),
		newRequest(
			t,
			`{"model":"alias","input":["a","b"],"input_type":"document","dimensions":256,"encoding_format":"float"}`,
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBody(t, result.Body)

	inputs := arr(t, got["inputs"])
	if len(inputs) != 2 || inputs[0] != "a" || inputs[1] != "b" {
		t.Fatalf("document mode needs a flat list of strings, got %#v", got["inputs"])
	}

	if got["enable_auto_chunking"] != true || got["chunk_size"] != float64(32000) {
		t.Fatalf("auto-chunking must always be on at 32000: %#v", got)
	}

	for _, key := range []string{"input", "dimensions", "encoding_format"} {
		if _, ok := got[key]; ok {
			t.Fatalf("%s must not be sent to Voyage", key)
		}
	}

	if got["output_dimension"] != float64(256) || got["model"] != "voyage-context-4" {
		t.Fatalf("dimension/model wrong: %#v", got)
	}
}

func TestConvertContextualizedEmbeddingsRequestQueryNeverAutoChunks(t *testing.T) {
	t.Parallel()

	result, err := voyage.ConvertContextualizedEmbeddingsRequest(
		newMeta(mode.Embeddings, "voyage-context-3"),
		newRequest(t, `{"input":"what is x","input_type":"query"}`),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBody(t, result.Body)

	inputs := arr(t, got["inputs"])
	if len(inputs) != 1 {
		t.Fatalf("inputs = %#v", got["inputs"])
	}

	inner := arr(t, inputs[0])
	if len(inner) != 1 || inner[0] != "what is x" {
		t.Fatalf("a query must be wrapped in its own inner list, got %#v", inputs[0])
	}

	if _, ok := got["enable_auto_chunking"]; ok {
		t.Fatal("query mode must not auto-chunk, Voyage answers 400")
	}

	if _, ok := got["chunk_size"]; ok {
		t.Fatal("query mode must not send chunk_size")
	}
}

func TestConvertContextualizedEmbeddingsRequestRejectsBadInput(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing input_type":    `{"input":["a"]}`,
		"unknown input_type":    `{"input":["a"],"input_type":"other"}`,
		"missing input":         `{"input_type":"document"}`,
		"empty list":            `{"input":[],"input_type":"document"}`,
		"empty string":          `{"input":[""],"input_type":"document"}`,
		"pre-chunked document":  `{"input":[["a","b"]],"input_type":"document"}`,
		"numeric array":         `{"input":[[1,2,3]],"input_type":"document"}`,
		"non-string item":       `{"input":[1],"input_type":"document"}`,
		"bad encoding_format":   `{"input":["a"],"input_type":"document","encoding_format":"int8"}`,
		"input is an object":    `{"input":{"a":1},"input_type":"document"}`,
		"image object item":     `{"input":[{"type":"image_url"}],"input_type":"document"}`,
		"null input_type value": `{"input":["a"],"input_type":null}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := voyage.ConvertContextualizedEmbeddingsRequest(
				newMeta(mode.Embeddings, "voyage-context-4"),
				newRequest(t, body),
			)
			if err == nil {
				t.Fatalf("%s must be rejected", name)
			}
		})
	}
}

func TestContextualizedEmbeddingsHandlerFlattensAndKeepsChunkText(t *testing.T) {
	t.Parallel()

	c, rec := newGinContext()

	result, err := voyage.ContextualizedEmbeddingsHandler(
		newContextualMeta(t, "voyage-context-4", 2),
		c,
		newResponse(http.StatusOK, `{"object":"list","model":"voyage-context-4","data":[
			{"object":"list","index":0,"data":[{"object":"embedding","embedding":[0.1,0.2],"index":0,"text":"alpha"}]},
			{"object":"list","index":1,"data":[{"object":"embedding","embedding":[0.3,0.4],"index":0,"text":"beta"}]}
		],"usage":{"total_tokens":7},"chunker_version":"1.0.0"}`),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Usage.TotalTokens != 7 || result.Usage.InputTokens != 7 {
		t.Fatalf("usage = %+v", result.Usage)
	}

	var got map[string]any
	if err := sonic.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data := arr(t, got["data"])
	if len(data) != 2 {
		t.Fatalf("data = %#v", got["data"])
	}

	second := obj(t, data[1])
	if second["object"] != "embedding" || second["index"] != float64(1) ||
		second["text"] != "beta" {
		t.Fatalf("second item = %#v", second)
	}

	if len(arr(t, second["embedding"])) != 2 {
		t.Fatalf("embedding must be a flat vector: %#v", second["embedding"])
	}
}

func TestContextualizedEmbeddingsHandlerQueryHasNoText(t *testing.T) {
	t.Parallel()

	c, rec := newGinContext()

	_, err := voyage.ContextualizedEmbeddingsHandler(
		newContextualMeta(t, "voyage-context-3", 1),
		c,
		newResponse(
			http.StatusOK,
			`{"data":[{"index":0,"data":[{"embedding":[0.1],"index":0}]}],"usage":{"total_tokens":0}}`,
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(rec.Body.String(), `"text"`) {
		t.Fatalf("no text field when Voyage did not chunk: %s", rec.Body.String())
	}
}

func TestContextualizedEmbeddingsHandlerFailures(t *testing.T) {
	t.Parallel()

	doc := `{"index":0,"data":[{"embedding":[0.1],"text":"a"}]}`

	cases := map[string]string{
		"missing usage":          `{"data":[` + doc + `]}`,
		"null usage":             `{"data":[` + doc + `],"usage":{"total_tokens":null}}`,
		"false usage":            `{"data":[` + doc + `],"usage":{"total_tokens":false}}`,
		"true usage":             `{"data":[` + doc + `],"usage":{"total_tokens":true}}`,
		"string usage":           `{"data":[` + doc + `],"usage":{"total_tokens":"12"}}`,
		"fractional usage":       `{"data":[` + doc + `],"usage":{"total_tokens":7.9}}`,
		"negative usage":         `{"data":[` + doc + `],"usage":{"total_tokens":-1}}`,
		"split into chunks":      `{"data":[{"index":0,"data":[{"embedding":[0.1]},{"embedding":[0.2]}]}],"usage":{"total_tokens":5}}`,
		"data not a list":        `{"data":{},"usage":{"total_tokens":5}}`,
		"empty data":             `{"data":[],"usage":{"total_tokens":5}}`,
		"missing document":       `{"data":[` + doc + `,` + `],"usage":{"total_tokens":5}}`,
		"document without list":  `{"data":[{"index":0}],"usage":{"total_tokens":5}}`,
		"chunk without vector":   `{"data":[{"index":0,"data":[{"text":"a"}]}],"usage":{"total_tokens":5}}`,
		"null vector":            `{"data":[{"index":0,"data":[{"embedding":null}]}],"usage":{"total_tokens":5}}`,
		"object vector":          `{"data":[{"index":0,"data":[{"embedding":{"x":1}}]}],"usage":{"total_tokens":5}}`,
		"document without index": `{"data":[{"data":[{"embedding":[0.1]}]}],"usage":{"total_tokens":5}}`,
		"wrong document index":   `{"data":[{"index":3,"data":[{"embedding":[0.1]}]}],"usage":{"total_tokens":5}}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, rec := newGinContext()

			_, err := voyage.ContextualizedEmbeddingsHandler(
				newContextualMeta(t, "voyage-context-4", 1), c, newResponse(http.StatusOK, body),
			)
			if err == nil {
				t.Fatalf("%s must fail", name)
			}

			if rec.Body.Len() != 0 {
				t.Fatalf("nothing may be written on failure: %s", rec.Body.String())
			}
		})
	}
}

// Fewer documents than inputs would leave the client unable to tell which input is missing.
func TestContextualizedEmbeddingsHandlerChecksDocumentCount(t *testing.T) {
	t.Parallel()

	c, _ := newGinContext()

	_, err := voyage.ContextualizedEmbeddingsHandler(
		newContextualMeta(t, "voyage-context-4", 3),
		c,
		newResponse(
			http.StatusOK,
			`{"data":[{"index":0,"data":[{"embedding":[0.1]}]},{"index":1,"data":[{"embedding":[0.2]}]}],"usage":{"total_tokens":5}}`,
		),
	)
	if err == nil {
		t.Fatal("2 documents for 3 inputs must fail")
	}
}

func TestConvertContextualizedEmbeddingsRequestChunkingFields(t *testing.T) {
	t.Parallel()

	rejected := map[string]string{
		"chunking disabled": `{"input":["a"],"input_type":"document","enable_auto_chunking":false}`,
		"smaller chunks":    `{"input":["a"],"input_type":"document","chunk_size":500}`,
		"string chunk size": `{"input":["a"],"input_type":"document","chunk_size":"32000"}`,
	}

	for name, body := range rejected {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := voyage.ConvertContextualizedEmbeddingsRequest(
				newMeta(mode.Embeddings, "voyage-context-4"), newRequest(t, body),
			)
			if err == nil {
				t.Fatalf("%s must be rejected, not overridden", name)
			}
		})
	}

	// Query mode drops client-sent chunking fields, which Voyage answers with 400.
	result, err := voyage.ConvertContextualizedEmbeddingsRequest(
		newMeta(mode.Embeddings, "voyage-context-4"),
		newRequest(
			t,
			`{"input":"q","input_type":"query","enable_auto_chunking":true,"chunk_size":10,"chunk_fn":"x"}`,
		),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBody(t, result.Body)
	for _, key := range []string{"enable_auto_chunking", "chunk_size", "chunk_fn"} {
		if _, ok := got[key]; ok {
			t.Fatalf("%s must be removed in query mode", key)
		}
	}

	// An explicit, matching setting is accepted in document mode.
	if _, err := voyage.ConvertContextualizedEmbeddingsRequest(
		newMeta(mode.Embeddings, "voyage-context-4"),
		newRequest(
			t,
			`{"input":["a"],"input_type":"document","enable_auto_chunking":true,"chunk_size":32000}`,
		),
	); err != nil {
		t.Fatalf("matching settings must pass: %v", err)
	}
}

// Billing must stay fail-closed for every non-integer usage value, in all three embeddings handlers.
func TestEmbeddingsHandlersRejectNonIntegerUsage(t *testing.T) {
	t.Parallel()

	for _, total := range []string{`null`, `false`, `true`, `"12"`, `7.9`} {
		t.Run(total, func(t *testing.T) {
			t.Parallel()

			body := `{"data":[{"embedding":[0.1],"index":0}],"usage":{"total_tokens":` + total + `}}`

			c, _ := newGinContext()
			if _, err := voyage.EmbeddingsHandler(
				newMeta(mode.Embeddings, "voyage-4"), c, newResponse(http.StatusOK, body),
			); err == nil {
				t.Fatalf("embeddings must reject total_tokens=%s", total)
			}

			c, _ = newGinContext()
			if _, err := voyage.MultimodalEmbeddingsHandler(
				newMeta(
					mode.Embeddings,
					"voyage-multimodal-3.5",
				),
				c,
				newResponse(http.StatusOK, body),
			); err == nil {
				t.Fatalf("multimodal must reject total_tokens=%s", total)
			}
		})
	}
}

// A split document was already charged by Voyage, so the failed request still reports usage.
func TestContextualizedEmbeddingsHandlerBillsSplitDocuments(t *testing.T) {
	t.Parallel()

	c, _ := newGinContext()

	result, err := voyage.ContextualizedEmbeddingsHandler(
		newMeta(mode.Embeddings, "voyage-context-4"),
		c,
		newResponse(
			http.StatusOK,
			`{"data":[{"data":[{"embedding":[0.1]},{"embedding":[0.2]}]}],"usage":{"total_tokens":9}}`,
		),
	)
	if err == nil {
		t.Fatal("a split document must fail")
	}

	if result.Usage.TotalTokens != 9 {
		t.Fatalf("usage = %+v, want 9 billed tokens", result.Usage)
	}
}

func TestContextualModelsAreListed(t *testing.T) {
	t.Parallel()

	want := map[string]float64{"voyage-context-4": 0.00012, "voyage-context-3": 0.00018}

	for _, m := range voyage.ModelList {
		if price, ok := want[m.Model]; ok {
			if float64(m.Price.InputPrice) != price || m.Type != mode.Embeddings {
				t.Fatalf("%s: price %v type %v", m.Model, m.Price.InputPrice, m.Type)
			}

			delete(want, m.Model)
		}
	}

	if len(want) != 0 {
		t.Fatalf("contextual models missing: %v", want)
	}
}
