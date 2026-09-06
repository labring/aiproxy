package testutil

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ProxyRequest struct {
	Method        string
	Host          string
	Authorization string
}

// NewHTTPProxy routes all requests to a local upstream, including CONNECT tunnels.
func NewHTTPProxy(
	tb testing.TB,
	upstream *httptest.Server,
	useTLS bool,
) (*httptest.Server, <-chan ProxyRequest) {
	tb.Helper()

	target, err := url.Parse(upstream.URL)
	require.NoError(tb, err)

	requests := make(chan ProxyRequest, 32)
	forward := &httputil.ReverseProxy{
		Transport: upstream.Client().Transport,
		Rewrite: func(req *httputil.ProxyRequest) {
			req.SetURL(target)
			req.Out.Header.Del("Proxy-Authorization")
		},
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests <- ProxyRequest{
			Method: req.Method, Host: req.Host,
			Authorization: req.Header.Get("Proxy-Authorization"),
		}

		if req.Method != http.MethodConnect {
			forward.ServeHTTP(w, req)
			return
		}

		dialer := net.Dialer{Timeout: time.Second}

		remote, err := dialer.DialContext(req.Context(), "tcp", target.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer remote.Close()

		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
			return
		}

		client, buffered, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer client.Close()

		if _, err := buffered.WriteString(
			"HTTP/1.1 200 Connection Established\r\n\r\n",
		); err != nil {
			return
		}

		if err := buffered.Flush(); err != nil {
			return
		}

		done := make(chan struct{})
		go func() {
			defer close(done)

			_, _ = io.Copy(remote, buffered)
			_ = remote.Close()
		}()

		_, _ = io.Copy(client, remote)
		_ = client.Close()

		<-done
	})

	var server *httptest.Server
	if useTLS {
		server = httptest.NewTLSServer(handler)
	} else {
		server = httptest.NewServer(handler)
	}

	tb.Cleanup(server.Close)

	return server, requests
}
