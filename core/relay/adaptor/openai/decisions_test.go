package openai_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	relaycontroller "github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

func TestDecisionsRelayPreservesNativeContract(t *testing.T) {
	t.Parallel()

	const response = `{
		"model":"gpt-6-luna",
		"answers":[
			{"type":"predicate","name":null,"probability":0},
			{"type":"choice","name":"confirmed","choice":false,"confidence":0,
			 "probabilities":[{"value":false,"probability":1},{"value":"false","probability":0}]},
			{"type":"score","name":"severity","score":0,"confidence":0,
			 "probabilities":[{"value":0,"label":"low","probability":1},{"value":1,"label":"high","probability":0}]},
			{"type":"refusal","name":"restricted"}
		],
		"usage":{"input_tokens":1200,"input_tokens_details":{"cached_tokens":1000,"cache_write_tokens":100},
		 "output_tokens":0,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":1200},
		"future_field":{"preserved":true}
	}`

	for name, input := range map[string]string{
		"text": `"The package arrived damaged."`,
		"conversation and tool history": `[
			{"role":"system","content":"Use the conversation as evidence."},
			{"role":"developer","content":[{"type":"input_text","text":"Evaluate the order."}]},
			{"role":"user","content":[
				{"type":"input_text","text":"Inspect this order.","prompt_cache_breakpoint":{"mode":"explicit"}},
				{"type":"input_image","image_url":"https://example.com/image.png","detail":"original"},
				{"type":"input_image","image_url":"data:image/png;base64,aGVsbG8="},
				{"type":"input_json","data":{"order_id":9007199254740993,"confirmed":false,"note":null}}
			]},
			{"role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"Checking."}]},
			{"type":"function_call","call_id":"fn1","name":"get_order","arguments":"{}"},
			{"type":"function_call_output","call_id":"fn1","output":"damaged"},
			{"type":"custom_tool_call","call_id":"custom1","name":"inspect","input":"photo"},
			{"type":"custom_tool_call_output","call_id":"custom1","output":[{"type":"input_text","text":"broken"}]},
			{"role":"assistant","content":[{"type":"refusal","refusal":"Cannot process that request."}]}
		]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body := `{"model":"public-luna","input":` + input + `,"questions":[
				{"type":"predicate","instructions":[
					{"type":"input_text","text":"Is the item damaged?"},
					{"type":"input_image","image_url":"https://example.com/reference.png"}
				]},
				{"type":"choice","name":"confirmed","instructions":"Was the action confirmed?",
				 "choices":[{"value":false,"description":"Not confirmed"},{"value":"false"}]},
				{"type":"score","name":"severity","instructions":"Rate the damage.",
				 "levels":[{"label":"low"},{"label":"high","description":"Unusable"}]}
				,{"type":"predicate","name":"restricted","instructions":"Evaluate this restricted request."}
			],"prompt_cache_key":"orders","prompt_cache_options":{"mode":"explicit","ttl":"30m"},
			"safety_identifier":null,"future_field":{"preserved":true}}`

			upstreamBodies := make(chan []byte, 1)

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method != http.MethodPost || req.URL.Path != "/proxy/v1/decisions" {
						t.Errorf("unexpected upstream route: %s %s", req.Method, req.URL.Path)
					}

					if req.Header.Get("Authorization") != "Bearer upstream-key" ||
						req.Header.Get("Accept") != "application/json" {
						t.Errorf("unexpected upstream headers: %v", req.Header)
					}

					upstreamBody, err := io.ReadAll(req.Body)
					if err != nil {
						t.Errorf("read upstream request: %v", err)
					}

					upstreamBodies <- upstreamBody

					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Request-ID", "req_decisions")
					_, _ = io.WriteString(w, response)
				}),
			)
			t.Cleanup(server.Close)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/decisions",
				strings.NewReader(body),
			)
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("Authorization", "Bearer client-key")

			requestMeta := &meta.Meta{
				Mode:        mode.Decisions,
				OriginModel: "public-luna",
				ActualModel: "gpt-6-luna",
				Channel: meta.ChannelMeta{
					Type:    model.ChannelTypeOpenAI,
					BaseURL: server.URL + "/proxy/v1",
					Key:     "upstream-key",
				},
			}

			result, _, relayErr := relaycontroller.DoHelper(&openai.Adaptor{}, c, requestMeta, nil)
			require.Nil(t, relayErr)
			require.Equal(t, http.StatusOK, recorder.Code)

			upstreamBody := <-upstreamBodies
			require.JSONEq(
				t,
				strings.Replace(body, `"public-luna"`, `"gpt-6-luna"`, 1),
				string(upstreamBody),
			)

			if strings.Contains(input, "order_id") {
				require.Contains(t, string(upstreamBody), "9007199254740993")
			}

			require.JSONEq(
				t,
				strings.Replace(response, `"gpt-6-luna"`, `"public-luna"`, 1),
				recorder.Body.String(),
			)
			require.EqualValues(t, 1200, result.Usage.InputTokens)
			require.EqualValues(t, 1000, result.Usage.CachedTokens)
			require.EqualValues(t, 100, result.Usage.CacheCreationTokens)
			require.EqualValues(t, 0, result.Usage.OutputTokens)
			require.EqualValues(t, 1200, result.Usage.TotalTokens)
			require.False(t, result.AsyncUsage)
			require.Equal(t, "req_decisions", result.UpstreamID)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			require.Equal(
				t,
				strconv.Itoa(recorder.Body.Len()),
				recorder.Header().Get("Content-Length"),
			)
		})
	}
}

func TestDecisionsErrorsPreserveOpenAIEnvelope(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		status int
		body   string
		code   any
	}{
		{"bad request", http.StatusBadRequest, `{"error":{"message":"Missing questions","type":"invalid_request_error","param":"questions","code":null}}`, nil},
		{"rate limit", http.StatusTooManyRequests, `{"error":{"message":"Slow down","type":"rate_limit_error","param":null,"code":"rate_limit_exceeded","future_field":true},"request_id":"req_123"}`, "rate_limit_exceeded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			_, relayErr := openai.DecisionsHandler(&meta.Meta{}, c, &http.Response{
				StatusCode: tt.status,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			})
			require.NotNil(t, relayErr)
			require.Equal(t, tt.status, relayErr.StatusCode())
			encoded, err := json.Marshal(relayErr)
			require.NoError(t, err)
			require.JSONEq(t, tt.body, string(encoded))

			provider, ok := relayErr.(adaptor.ErrorCodeProvider)
			require.True(t, ok)
			require.Equal(t, tt.code, provider.ErrorCode())
			require.Contains(t, relayErr.Error(), tt.body)
		})
	}
}

func TestConvertDecisionsRequestRejectsUnreadableOrNonObjectBodies(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"malformed JSON", `{"input":}`},
		{"invalid escape", `{"input":"\x"}`},
		{"array", `[]`},
		{"string", `"hello"`},
		{"number", `42`},
		{"null", `null`},
		{"trailing garbage", `{"model":"gpt-6-luna"} garbage`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/decisions",
				strings.NewReader(tt.body),
			)
			result, err := openai.ConvertDecisionsRequest(
				&meta.Meta{ActualModel: "gpt-6-luna"},
				req,
			)
			require.Error(t, err)
			require.Nil(t, result.Body)
		})
	}

	t.Run("nil request", func(t *testing.T) {
		t.Parallel()

		_, err := openai.ConvertDecisionsRequest(&meta.Meta{}, nil)
		require.ErrorContains(t, err, "request is nil")
	})

	t.Run("body read failure", func(t *testing.T) {
		t.Parallel()

		body := &decisionsReadCloser{Reader: decisionsErrorReader{}}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/decisions", nil)
		req.Body = body
		_, err := openai.ConvertDecisionsRequest(&meta.Meta{}, req)
		require.ErrorContains(t, err, "simulated read failure")
		require.True(t, body.closed)
	})

	t.Run("request limit", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/decisions",
			strings.NewReader(`{}`),
		)
		req.ContentLength = config.MaxRequestBodySize + 1
		_, err := openai.ConvertDecisionsRequest(&meta.Meta{}, req)
		require.ErrorContains(t, err, "body too large")
	})
}

func TestConvertDecisionsRequestCanRetryWithoutMutatingInput(t *testing.T) {
	t.Parallel()

	const requestBody = `{"model":"public-luna","input":[{"role":"user","content":[{"type":"input_json","data":{"id":9007199254740993}}]}],"questions":[{"type":"predicate","instructions":"Check."}],"prompt_cache_key":null}`

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/decisions",
		strings.NewReader(requestBody),
	)
	for _, upstreamModel := range []string{"gpt-6-luna", "备用模型"} {
		result, err := openai.ConvertDecisionsRequest(&meta.Meta{ActualModel: upstreamModel}, req)
		require.NoError(t, err)

		body, err := io.ReadAll(result.Body)
		require.NoError(t, err)
		require.JSONEq(
			t,
			strings.Replace(requestBody, "public-luna", upstreamModel, 1),
			string(body),
		)
		require.Contains(t, string(body), "9007199254740993")
		require.Equal(t, "application/json", result.Header.Get("Content-Type"))
		require.Equal(t, strconv.Itoa(len(body)), result.Header.Get("Content-Length"))

		originalBody, err := common.GetRequestBodyReusable(req)
		require.NoError(t, err)
		require.Equal(t, requestBody, string(originalBody))
	}
}

func TestDecisionsHandlerUpstreamFailures(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		status        int
		body          string
		contentLength int64
		readerFails   bool
		wantStatus    int
		wantCode      string
	}{
		{"read failure", http.StatusOK, "", 0, true, http.StatusInternalServerError, "read_response_body_failed"},
		{"truncated response", http.StatusOK, `{}`, 10, false, http.StatusInternalServerError, "read_response_body_failed"},
		{"oversized response", http.StatusOK, `{}`, config.MaxResponseBodySize + 1, false, http.StatusInternalServerError, "read_response_body_failed"},
		{"malformed usage", http.StatusOK, `{"usage":{"input_tokens":"invalid"}}`, 0, false, http.StatusInternalServerError, "unmarshal_response_body_failed"},
		{"non-object response", http.StatusOK, `[]`, 0, false, http.StatusInternalServerError, "unmarshal_response_body_failed"},
		{"null response", http.StatusOK, `null`, 0, false, http.StatusInternalServerError, "unmarshal_response_body_failed"},
		{"HTML error", http.StatusBadGateway, `<html>upstream unavailable</html>`, 0, false, http.StatusBadGateway, "bad_response"},
		{"missing error object", http.StatusServiceUnavailable, `{"message":"unavailable"}`, 0, false, http.StatusServiceUnavailable, "bad_response"},
		{"null error object", http.StatusBadRequest, `{"error":null}`, 0, false, http.StatusBadRequest, "bad_response"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &decisionsReadCloser{Reader: strings.NewReader(tt.body)}
			if tt.readerFails {
				body.Reader = decisionsErrorReader{}
			}

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			result, relayErr := openai.DecisionsHandler(&meta.Meta{}, c, &http.Response{
				StatusCode: tt.status, Body: body, ContentLength: tt.contentLength,
			})
			require.NotNil(t, relayErr)
			require.Equal(t, tt.wantStatus, relayErr.StatusCode())
			provider, ok := relayErr.(adaptor.ErrorCodeProvider)
			require.True(t, ok)
			require.Equal(t, tt.wantCode, provider.ErrorCode())
			require.Zero(t, result.Usage.TotalTokens)
			require.Empty(t, recorder.Body.String())
			require.True(t, body.closed)
		})
	}
}

func TestDecisionsHandlerUsageAndModelBoundaries(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		meta      *meta.Meta
		body      string
		wantBody  string
		wantUsage model.Usage
	}{
		{
			name: "all tokens cached and no alias", meta: &meta.Meta{ActualModel: "gpt-6-luna"},
			body:      `{"model":"gpt-6-luna","answers":[],"usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":10},"output_tokens":0,"total_tokens":10}}`,
			wantUsage: model.Usage{InputTokens: 10, CachedTokens: 10, TotalTokens: 10},
		},
		{
			name: "cache write and reasoning", meta: &meta.Meta{OriginModel: "中文别名", ActualModel: "gpt-6-luna"},
			body:      `{"model":"gpt-6-luna","answers":[],"usage":{"input_tokens":10,"input_tokens_details":{"cache_write_tokens":10},"output_tokens":4,"output_tokens_details":{"reasoning_tokens":4},"total_tokens":14}}`,
			wantBody:  `{"model":"中文别名","answers":[],"usage":{"input_tokens":10,"input_tokens_details":{"cache_write_tokens":10},"output_tokens":4,"output_tokens_details":{"reasoning_tokens":4},"total_tokens":14}}`,
			wantUsage: model.Usage{InputTokens: 10, CacheCreationTokens: 10, OutputTokens: 4, ReasoningTokens: 4, TotalTokens: 14},
		},
		{
			name: "no model mapping", meta: nil,
			body: `{"model":"gpt-6-luna","answers":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0},"extra":9007199254740993}`,
		},
		{
			name: "missing model and usage", meta: &meta.Meta{OriginModel: "public-luna"},
			body: `{"answers":[{"type":"refusal","name":null}]}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &decisionsReadCloser{Reader: strings.NewReader(tt.body)}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			result, relayErr := openai.DecisionsHandler(
				tt.meta,
				c,
				&http.Response{StatusCode: http.StatusOK, Body: body},
			)
			require.Nil(t, relayErr)
			require.Equal(t, tt.wantUsage, result.Usage)
			require.False(t, result.AsyncUsage)
			require.True(t, body.closed)

			wantBody := tt.wantBody
			if wantBody == "" {
				wantBody = tt.body
			}

			require.JSONEq(t, wantBody, recorder.Body.String())
			require.Equal(
				t,
				strconv.Itoa(recorder.Body.Len()),
				recorder.Header().Get("Content-Length"),
			)

			if strings.Contains(tt.body, "9007199254740993") {
				require.Contains(t, recorder.Body.String(), "9007199254740993")
			}
		})
	}
}

// The mock simulates native boundary outcomes. AIProxy must forward the
// complete payload and preserve upstream validation rather than impose a schema.
func TestDecisionsRelayQuestionBoundaries(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		kind  string
		count int
		valid bool
	}{
		{"questions", 0, false},
		{"questions", 1, true},
		{"questions", 400, true},
		{"questions", 401, false},
		{"choices", 1, false},
		{"choices", 2, true},
		{"choices", 255, true},
		{"choices", 256, false},
		{"levels", 1, false},
		{"levels", 2, true},
		{"levels", 10, true},
		{"levels", 11, false},
	} {
		t.Run(fmt.Sprintf("%s/%d", tt.kind, tt.count), func(t *testing.T) {
			t.Parallel()

			question := map[string]any{"type": "predicate", "instructions": "Evaluate the input."}
			questions := []map[string]any{}

			switch tt.kind {
			case "questions":
				for range tt.count {
					questions = append(questions, question)
				}
			case "choices":
				question["type"] = "choice"
				choices := make([]map[string]any, tt.count)

				for i := range tt.count {
					choices[i] = map[string]any{"value": fmt.Sprintf("option-%d", i)}
				}

				question["choices"] = choices
				questions = append(questions, question)
			case "levels":
				question["type"] = "score"
				levels := make([]map[string]any, tt.count)

				for i := range tt.count {
					levels[i] = map[string]any{"label": fmt.Sprintf("level-%d", i)}
				}

				question["levels"] = levels
				questions = append(questions, question)
			}

			requestBody, err := json.Marshal(map[string]any{
				"model": "gpt-6-luna", "input": "Inspect this order.", "questions": questions,
			})
			require.NoError(t, err)

			status := http.StatusOK
			answer := `{"type":"predicate","name":null,"probability":0.5}`

			if tt.kind == "choices" || tt.kind == "levels" {
				probabilities := make([]string, tt.count)
				for i := range tt.count {
					probability := 0
					if i == 0 {
						probability = 1
					}

					if tt.kind == "choices" {
						probabilities[i] = fmt.Sprintf(
							`{"value":"option-%d","probability":%d}`,
							i,
							probability,
						)
					} else {
						probabilities[i] = fmt.Sprintf(
							`{"value":%d,"label":"level-%d","probability":%d}`,
							i,
							i,
							probability,
						)
					}
				}

				answer = fmt.Sprintf(
					`{"type":"choice","name":null,"choice":"option-0","confidence":1,"probabilities":[%s]}`,
					strings.Join(probabilities, ","),
				)
				if tt.kind == "levels" {
					answer = fmt.Sprintf(
						`{"type":"score","name":null,"score":0,"confidence":1,"probabilities":[%s]}`,
						strings.Join(probabilities, ","),
					)
				}
			}

			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = answer
			}

			responseBody := fmt.Sprintf(
				`{"model":"gpt-6-luna","answers":[%s],"usage":{"input_tokens":5,"output_tokens":0,"total_tokens":5}}`,
				strings.Join(answers, ","),
			)

			if !tt.valid {
				status = http.StatusBadRequest
				responseBody = fmt.Sprintf(
					`{"error":{"message":"Invalid %s count","type":"invalid_request_error","param":"questions","code":null}}`,
					tt.kind,
				)
			}

			upstreamBodies := make(chan []byte, 1)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					body, readErr := io.ReadAll(req.Body)
					if readErr != nil {
						t.Errorf("read upstream body: %v", readErr)
					}

					upstreamBodies <- body

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, responseBody)
				}),
			)
			t.Cleanup(server.Close)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/decisions",
				strings.NewReader(string(requestBody)),
			)
			c.Request.Header.Set("Content-Type", "application/json")

			requestMeta := &meta.Meta{
				Mode:        mode.Decisions,
				OriginModel: "gpt-6-luna",
				ActualModel: "gpt-6-luna",
				Channel: meta.ChannelMeta{
					Type:    model.ChannelTypeOpenAI,
					BaseURL: server.URL + "/v1",
					Key:     "test-key",
				},
			}
			_, _, relayErr := relaycontroller.DoHelper(&openai.Adaptor{}, c, requestMeta, nil)

			require.Len(t, upstreamBodies, 1, "the native boundary payload must reach the upstream")
			require.JSONEq(t, string(requestBody), string(<-upstreamBodies))

			if tt.valid {
				require.Nil(t, relayErr)
				require.Equal(t, status, recorder.Code)
				require.JSONEq(t, responseBody, recorder.Body.String())
			} else {
				require.NotNil(t, relayErr)
				require.Equal(t, status, relayErr.StatusCode())
				data, marshalErr := relayErr.MarshalJSON()
				require.NoError(t, marshalErr)
				require.JSONEq(t, responseBody, string(data))
				require.Empty(t, recorder.Body.String())
			}
		})
	}
}

type decisionsReadCloser struct {
	io.Reader
	closed bool
}

func (r *decisionsReadCloser) Close() error {
	r.closed = true
	return nil
}

type decisionsErrorReader struct{}

func (decisionsErrorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read failure")
}

func TestDecisionsHandlerRejectsMalformedUpstreamJSON(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	_, relayErr := openai.DecisionsHandler(&meta.Meta{}, c, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"answers":`)),
	})
	require.NotNil(t, relayErr)
	require.Equal(t, http.StatusInternalServerError, relayErr.StatusCode())
	require.Empty(t, recorder.Body.String())
}
