package adaptors_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/labring/aiproxy/core/internal/testutil"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	awsutils "github.com/labring/aiproxy/core/relay/adaptor/aws/utils"
	"github.com/labring/aiproxy/core/relay/adaptor/baidu"
	"github.com/labring/aiproxy/core/relay/adaptor/baiduv2"
	"github.com/labring/aiproxy/core/relay/adaptor/deepseek"
	"github.com/labring/aiproxy/core/relay/adaptor/doc2x"
	"github.com/labring/aiproxy/core/relay/adaptor/moonshot"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/siliconflow"
	"github.com/labring/aiproxy/core/relay/adaptor/vertexai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelBalanceUsesProxy(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		balancer adaptor.Balancer
		paths    map[string]string
		want     float64
	}{
		{
			name: "openai", balancer: &openai.Adaptor{}, want: 98,
			paths: map[string]string{
				"/v1/dashboard/billing/subscription": `{"hard_limit_usd":100,"has_payment_method":true}`,
				"/v1/dashboard/billing/usage":        `{"total_usage":200}`,
			},
		},
		{
			name: "deepseek", balancer: &deepseek.Adaptor{}, want: 8.25,
			paths: map[string]string{
				"/user/balance": `{"balance_infos":[{"currency":"CNY","total_balance":"8.25"}]}`,
			},
		},
		{
			name: "moonshot", balancer: &moonshot.Adaptor{}, want: 8.25,
			paths: map[string]string{
				"/users/me/balance": `{"data":{"available_balance":8.25}}`,
			},
		},
		{
			name: "siliconflow", balancer: &siliconflow.Adaptor{}, want: 8.25,
			paths: map[string]string{
				"/user/info": `{"data":{"balance":"8.25"}}`,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			upstream := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					body, ok := tt.paths[req.URL.Path]
					assert.True(t, ok, "unexpected upstream path: %s", req.URL.Path)
					assert.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))

					_, _ = io.WriteString(w, body)
				}),
			)
			t.Cleanup(upstream.Close)
			proxy, requests := testutil.NewHTTPProxy(t, upstream, true)
			balance, err := tt.balancer.GetBalance(&model.Channel{
				BaseURL: "http://balance.invalid", Key: "test-key",
				ProxyURL: proxy.URL, SkipTLSVerify: true,
			})
			require.NoError(t, err)
			assert.InDelta(t, tt.want, balance, 0.00001)
			assert.Len(t, requests, len(tt.paths))
		})
	}
}

func TestChannelTokenRequestsUseProxy(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		adaptor adaptor.Adaptor
		path    string
		body    string
		status  int
	}{
		{
			name: "baidu", adaptor: &baidu.Adaptor{}, path: "/oauth/2.0/token",
			body: `{"access_token":"test-token","expires_in":3600}`, status: http.StatusOK,
		},
		{
			name: "baiduv2", adaptor: &baiduv2.Adaptor{}, path: "/v1/BCE-BEARER/token",
			body: `{"token":"test-token","expireTime":"2099-01-01T00:00:00Z"}`, status: http.StatusCreated,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			upstream := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					assert.Equal(t, tt.path, req.URL.Path)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tt.status)
					_, _ = io.WriteString(w, tt.body)
				}),
			)
			t.Cleanup(upstream.Close)
			proxy, requests := testutil.NewHTTPProxy(t, upstream, false)
			m := meta.NewMeta(&model.Channel{
				Key: proxy.URL + "|secret", ProxyURL: proxy.URL, SkipTLSVerify: true,
			}, mode.ChatCompletions, "test-model", model.ModelConfig{})
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"https://upstream.invalid/chat",
				nil,
			)
			require.NoError(t, err)
			require.NoError(t, tt.adaptor.SetupRequestHeader(m, nil, nil, req))

			if tt.name == "baidu" {
				assert.Equal(t, "test-token", req.URL.Query().Get("access_token"))
			} else {
				assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
			}

			assert.Len(t, requests, 1)
		})
	}
}

func TestVertexTokenExchangeAndIAMUseProxy(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch req.URL.Path {
			case "/token":
				assert.NoError(t, req.ParseForm())
				assert.Equal(
					t,
					"urn:ietf:params:oauth:grant-type:jwt-bearer",
					req.Form.Get("grant_type"),
				)
				assert.NotEmpty(t, req.Form.Get("assertion"))

				_, _ = io.WriteString(
					w,
					`{"access_token":"oauth-token","token_type":"Bearer","expires_in":3600}`,
				)
			case "/v1/projects/-/serviceAccounts/service@example.invalid:generateAccessToken":
				assert.Equal(t, "Bearer oauth-token", req.Header.Get("Authorization"))

				_, _ = io.WriteString(
					w,
					`{"accessToken":"iam-token","expireTime":"2099-01-01T00:00:00Z"}`,
				)
			default:
				t.Errorf("unexpected token request: %s", req.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}),
	)
	t.Cleanup(upstream.Close)
	proxy, requests := testutil.NewHTTPProxy(t, upstream, false)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKey := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	//nolint:gosec // Credentials use an ephemeral test key generated above.
	adc, err := json.Marshal(vertexai.ApplicationDefaultCredentials{
		Type: "service_account", ProjectID: "test-project",
		ClientEmail: "service@example.invalid", PrivateKey: string(privateKey),
		TokenURI: "https://oauth.invalid/token",
	})
	require.NoError(t, err)

	m := meta.NewMeta(&model.Channel{
		Key: "us-central1|" + string(adc), ProxyURL: proxy.URL, SkipTLSVerify: true,
	}, mode.ChatCompletions, "gemini-test", model.ModelConfig{Type: mode.Gemini})
	m.RequestTimeout = time.Second
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"https://upstream.invalid/chat",
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, (&vertexai.Adaptor{}).SetupRequestHeader(m, nil, nil, req))
	assert.Equal(t, "Bearer iam-token", req.Header.Get("Authorization"))
	assert.Len(t, requests, 2)
}

func TestAWSRequestUsesChannelProxyAndTLS(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, "Bearer test-api-key", req.Header.Get("Authorization"))
			assert.Equal(t, "/model/test-model/invoke", req.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"result":"proxied"}`)
		}),
	)
	t.Cleanup(upstream.Close)
	proxy, requests := testutil.NewHTTPProxy(t, upstream, false)
	m := meta.NewMeta(&model.Channel{
		Key: "us-east-1|test-api-key", ProxyURL: proxy.URL, SkipTLSVerify: true,
	}, mode.ChatCompletions, "test-model", model.ModelConfig{})
	client, err := awsutils.AwsClientFromMeta(m)
	require.NoError(t, err)
	output, err := client.InvokeModel(t.Context(), &bedrockruntime.InvokeModelInput{
		ModelId: new("test-model"), ContentType: new("application/json"), Body: []byte(`{}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"result":"proxied"}`, string(output.Body))
	assert.Len(t, requests, 1)
}

func TestDoc2xImageUsesChannelProxyAndTLS(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/page.png", req.URL.Path)
		w.Header().Set("Content-Type", "image/png")
		_, _ = io.WriteString(w, "image-data")
	}))
	t.Cleanup(upstream.Close)
	proxy, requests := testutil.NewHTTPProxy(t, upstream, true)
	m := meta.NewMeta(&model.Channel{
		ProxyURL: proxy.URL, SkipTLSVerify: true,
	}, mode.ChatCompletions, "test-model", model.ModelConfig{})
	result := doc2x.InlineMdImage(t.Context(), m, "![page](http://image.invalid/page.png)")
	assert.Equal(t, "![page](data:image/png;base64,aW1hZ2UtZGF0YQ==)", result)
	assert.Len(t, requests, 1)
}
