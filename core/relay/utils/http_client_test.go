package utils_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/internal/testutil"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelHTTPProxy(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		proxyTLS  bool
		targetTLS bool
	}{
		{name: "HTTP through HTTP"},
		{name: "HTTPS through HTTP", targetTLS: true},
		{name: "HTTP through HTTPS", proxyTLS: true},
		{name: "HTTPS through HTTPS", proxyTLS: true, targetTLS: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				assert.Equal(t, "/v1/models", req.URL.Path)
				assert.Equal(t, "Bearer upstream-key", req.Header.Get("Authorization"))
				assert.Empty(t, req.Header.Get("Proxy-Authorization"))

				_, _ = io.WriteString(w, "proxied")
			})

			var upstream *httptest.Server
			if tt.targetTLS {
				upstream = httptest.NewTLSServer(handler)
			} else {
				upstream = httptest.NewServer(handler)
			}

			t.Cleanup(upstream.Close)
			proxy, requests := testutil.NewHTTPProxy(t, upstream, tt.proxyTLS)
			proxyURL, err := url.Parse(proxy.URL)
			require.NoError(t, err)

			proxyURL.User = url.UserPassword("user@example", "p:ass")
			m := meta.NewMeta(&model.Channel{
				ProxyURL: proxyURL.String(), SkipTLSVerify: tt.targetTLS || tt.proxyTLS,
			}, mode.Responses, "test-model", model.ModelConfig{})
			m.RequestTimeout = time.Second
			client, err := utils.LoadHTTPClientWithTLSConfigE(
				m.RequestTimeout, m.Channel.ProxyURL, m.Channel.SkipTLSVerify,
			)
			require.NoError(t, err)
			t.Cleanup(client.CloseIdleConnections)

			scheme := "http"
			if tt.targetTLS {
				scheme = "https"
			}

			req, err := http.NewRequestWithContext(
				t.Context(), http.MethodGet, scheme+"://upstream.invalid/v1/models", nil,
			)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer upstream-key")
			resp, err := utils.DoRequestWithMeta(req, m)
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "proxied", string(body))

			select {
			case request := <-requests:
				wantMethod := http.MethodGet
				if tt.targetTLS {
					wantMethod = http.MethodConnect
				}

				assert.Equal(t, wantMethod, request.Method)
				assert.Contains(t, request.Host, "upstream.invalid")
				assert.Equal(
					t,
					"Basic "+base64.StdEncoding.EncodeToString([]byte("user@example:p:ass")),
					request.Authorization,
				)
			case <-time.After(time.Second):
				t.Fatal("proxy did not receive request")
			}
		})
	}
}

func TestSOCKSProxyCancellationClosesConnection(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	connected := make(chan net.Conn, 1)

	closed := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		connected <- conn

		_, _ = io.Copy(io.Discard, conn)

		close(closed)
	}()

	client, err := utils.LoadHTTPClientE(time.Second, "socks5://"+listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	result := make(chan error, 1)
	go func() {
		conn, err := transport.DialContext(ctx, "tcp", "upstream.invalid:80")
		if conn != nil {
			_ = conn.Close()
		}

		result <- err
	}()

	select {
	case conn := <-connected:
		t.Cleanup(func() { _ = conn.Close() })
	case <-time.After(time.Second):
		t.Fatal("SOCKS connection did not start")
	}

	cancel()

	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("SOCKS dial did not honor cancellation")
	}

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("canceled SOCKS handshake leaked its connection")
	}
}

func TestSOCKSProxyRequests(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		scheme string
		auth   bool
	}{
		{scheme: "socks5"},
		{scheme: "socks5", auth: true},
		{scheme: "socks5h"},
		{scheme: "socks5h", auth: true},
	} {
		t.Run(tt.scheme, func(t *testing.T) {
			t.Parallel()

			listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { _ = listener.Close() })

			result := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					result <- err
					return
				}
				defer conn.Close()

				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				result <- serveSOCKSRequest(conn, tt.auth)
			}()

			proxyURL := &url.URL{Scheme: tt.scheme, Host: listener.Addr().String()}
			if tt.auth {
				proxyURL.User = url.UserPassword("user", "password")
			}

			client, err := utils.LoadHTTPClientE(time.Second, proxyURL.String())
			require.NoError(t, err)
			t.Cleanup(client.CloseIdleConnections)
			req, err := http.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"http://upstream.invalid/models",
				nil,
			)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "socks", string(body))
			require.NoError(t, <-result)
		})
	}
}

func TestProxyURLValidation(t *testing.T) {
	t.Parallel()

	for _, proxyURL := range []string{
		"", "  ", "http://proxy.example", "https://proxy.example:8443/",
		"socks5://proxy.example", "socks5h://[::1]:1080",
		" http://user:p%40ss@proxy.example:8080 ",
	} {
		t.Run("valid "+proxyURL, func(t *testing.T) {
			t.Parallel()

			client, err := utils.LoadHTTPClientE(time.Second, proxyURL)
			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}

	for _, proxyURL := range []string{
		"localhost:8080", "http://", "https:///proxy", "http://:8080", "socks5://",
		"ftp://proxy.example", "http://proxy.example:0", "http://proxy.example:65536",
		"http://proxy.example:", "http://proxy.example/path", "http://proxy.example?token=secret",
		"http://proxy.example#fragment", "http://user:secret@proxy.example:bad",
	} {
		t.Run("invalid "+proxyURL, func(t *testing.T) {
			t.Parallel()

			client, err := utils.LoadHTTPClientE(time.Second, proxyURL)
			require.Error(t, err)
			require.Nil(t, client)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

func serveSOCKSRequest(conn net.Conn, authenticate bool) error {
	reader := bufio.NewReader(conn)

	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}

	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(reader, methods); err != nil {
		return err
	}

	if authenticate {
		if err := authenticateSOCKSRequest(conn, reader); err != nil {
			return err
		}
	} else if _, err := conn.Write([]byte{5, 0}); err != nil {
		return err
	}

	command := make([]byte, 5)
	if _, err := io.ReadFull(reader, command); err != nil {
		return err
	}

	if command[0] != 5 || command[1] != 1 || command[3] != 3 {
		return &net.AddrError{Err: "expected SOCKS domain CONNECT", Addr: string(command)}
	}

	address := make([]byte, int(command[4])+2)
	if _, err := io.ReadFull(reader, address); err != nil {
		return err
	}

	if string(address[:len(address)-2]) != "upstream.invalid" {
		return &net.AddrError{Err: "unexpected SOCKS destination", Addr: string(address)}
	}

	if _, err := conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 80}); err != nil {
		return err
	}

	req, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	defer req.Body.Close()

	response := &http.Response{
		StatusCode: http.StatusOK, ProtoMajor: 1, ProtoMinor: 1,
		Body: io.NopCloser(strings.NewReader("socks")), ContentLength: 5,
		Header: make(http.Header), Close: true,
	}

	return response.Write(conn)
}

func authenticateSOCKSRequest(conn net.Conn, reader *bufio.Reader) error {
	if _, err := conn.Write([]byte{5, 2}); err != nil {
		return err
	}

	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}

	user := make([]byte, int(header[1]))
	if _, err := io.ReadFull(reader, user); err != nil {
		return err
	}

	length, err := reader.ReadByte()
	if err != nil {
		return err
	}

	password := make([]byte, int(length))
	if _, err := io.ReadFull(reader, password); err != nil {
		return err
	}

	if string(user) != "user" || string(password) != "password" {
		_, err := conn.Write([]byte{1, 1})
		return err
	}

	_, err = conn.Write([]byte{1, 0})

	return err
}

func TestChannelTLSVerificationIsIsolated(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	t.Cleanup(upstream.Close)

	for _, skipTLSVerify := range []bool{true, false, true} {
		client, err := utils.LoadHTTPClientWithTLSConfigE(time.Second, "", skipTLSVerify)
		require.NoError(t, err)
		t.Cleanup(client.CloseIdleConnections)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, upstream.URL, nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}

		if skipTLSVerify {
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		} else {
			require.ErrorContains(t, err, "x509")
		}
	}
}
