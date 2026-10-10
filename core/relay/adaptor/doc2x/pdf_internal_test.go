package doc2x

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/model"
	relaycontroller "github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

func TestIsSuccessfulResponseCode(t *testing.T) {
	t.Parallel()

	for _, code := range []string{"success", "ok"} {
		if !isSuccessfulResponseCode(code) {
			t.Errorf("expected %q to be accepted", code)
		}
	}

	if isSuccessfulResponseCode("failed") {
		t.Fatal("failed response code was accepted")
	}
}

func TestJoinMarkdownPagesPreservesPageBoundary(t *testing.T) {
	t.Parallel()

	pages := []string{"第一张页面", "第二张页面"}

	result := joinMarkdownPages(pages)
	if result != "第一张页面\n\n第二张页面" {
		t.Fatalf("expected explicit page boundary, got %q", result)
	}
}

func TestHandleParsePdfResponsePollsReadyUntilSuccess(t *testing.T) {
	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		w.Header().Set("Content-Type", "application/json")

		if calls == 1 {
			_, _ = io.WriteString(w, `{"code":"success","data":{"status":"ready","progress":1}}`)
			return
		}

		_, _ = io.WriteString(
			w,
			`{"code":"success","data":{"status":"success","result":{"pages":[{"md":"正文"}]}}}`,
		)
	}))
	t.Cleanup(server.Close)

	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)

	m := &meta.Meta{
		Channel:        meta.ChannelMeta{BaseURL: server.URL},
		RequestTimeout: time.Second,
	}

	response := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"code":"success","data":{"uid":"test-uid"}}`)),
	}
	if _, err := HandleParsePdfResponse(m, ctx, response); err != nil {
		t.Fatalf("expected ready status to continue polling: %v", err)
	}

	if calls != 2 {
		t.Fatalf("expected two status requests, got %d", calls)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected successful JSON response, got %d", recorder.Code)
	}
}

func TestDisconnectedPDFJobRetainsBillableUsage(t *testing.T) {
	for _, phase := range []string{"submission", "polling"} {
		t.Run(phase, func(t *testing.T) {
			clientCtx, disconnect := context.WithCancel(t.Context())
			defer disconnect()

			var (
				statusCalls atomic.Int32
				imageCalls  atomic.Int32
			)

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/api/v2/parse/pdf":
						_, err := io.Copy(io.Discard, r.Body)
						require.NoError(t, err)

						if phase == "submission" {
							disconnect()
						}

						_, _ = io.WriteString(w, `{"code":"success","data":{"uid":"billable-job"}}`)
					case "/api/v2/parse/status":
						require.Equal(t, "billable-job", r.URL.Query().Get("uid"))
						require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

						if statusCalls.Add(1) == 1 {
							if phase == "polling" {
								disconnect()
							}

							_, _ = io.WriteString(
								w,
								`{"code":"success","data":{"status":"processing"}}`,
							)

							return
						}

						err := json.NewEncoder(w).Encode(StatusResponse{
							Code: "success",
							Data: &StatusResponseData{
								Status: "success",
								Result: &StatusResponseDataResult{
									Pages: []StatusResponseDataResultPage{
										{MD: "first page"},
										{MD: "![image](http://" + r.Host + "/image)"},
									},
								},
							},
						})
						require.NoError(t, err)
					case "/image":
						imageCalls.Add(1)
					default:
						t.Errorf("unexpected path: %s", r.URL.Path)
						w.WriteHeader(http.StatusNotFound)
					}
				}),
			)
			defer server.Close()

			var body bytes.Buffer

			writer := multipart.NewWriter(&body)
			file, err := writer.CreateFormFile("file", "test.pdf")
			require.NoError(t, err)
			_, err = io.WriteString(file, "%PDF-1.4 test")
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequestWithContext(
				clientCtx,
				http.MethodPost,
				"/v1/parse/pdf",
				&body,
			)
			c.Request.Header.Set("Content-Type", writer.FormDataContentType())

			m := meta.NewMeta(
				&model.Channel{Type: model.ChannelTypeDoc2x, BaseURL: server.URL, Key: "test-key"},
				mode.ParsePdf,
				"parse-pdf",
				model.ModelConfig{},
			)
			m.RequestTimeout = time.Second
			result := relaycontroller.Handle(&Adaptor{}, c, m, nil)
			require.Nil(t, result.Error)
			require.ErrorIs(t, clientCtx.Err(), context.Canceled)
			require.EqualValues(t, 2, statusCalls.Load())
			require.Zero(t, imageCalls.Load())
			require.EqualValues(t, 2, result.Usage.InputTokens)
			require.EqualValues(t, 2, result.Usage.TotalTokens)
			amount := consume.CalculateAmount(
				http.StatusOK,
				result.Usage,
				result.UsageContext,
				model.Price{InputPrice: 0.1, InputPriceUnit: 1},
			)
			require.InDelta(t, 0.2, amount, 0.000001)
			require.Empty(t, recorder.Body.String())
		})
	}
}

func TestStatusPollingHasIndependentTimeout(t *testing.T) {
	for _, stalledRequest := range []bool{false, true} {
		t.Run(strconv.FormatBool(stalledRequest), func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if stalledRequest {
						<-r.Context().Done()
						return
					}

					_, _ = io.WriteString(w, `{"code":"success","data":{"status":"processing"}}`)
				}),
			)
			defer server.Close()

			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
			defer cancel()

			m := &meta.Meta{
				Channel:        meta.ChannelMeta{BaseURL: server.URL},
				RequestTimeout: time.Second,
			}
			result, relayErr := waitForParsePdf(ctx, m, "timeout-job")
			require.NotNil(t, relayErr)
			require.Equal(t, http.StatusGatewayTimeout, relayErr.StatusCode())
			require.Nil(t, result)
		})
	}
}
