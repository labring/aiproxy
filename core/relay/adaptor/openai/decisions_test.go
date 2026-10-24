package openai_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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

	for _, input := range []string{
		`"The package arrived damaged."`,
		`[
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
		t.Run(input[:1], func(t *testing.T) {
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
		})
	}
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
