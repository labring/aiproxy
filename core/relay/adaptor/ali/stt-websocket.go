package ali

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/utils"
)

// dialSTTWebSocket reuses channel proxy/TLS policy without changing shared HTTP transports.
func dialSTTWebSocket(
	ctx context.Context,
	meta *meta.Meta,
	req *http.Request,
) (*websocket.Conn, *http.Response, error) {
	wsURL := *req.URL
	switch wsURL.Scheme {
	case "https":
		wsURL.Scheme = "wss"
	case "http":
		wsURL.Scheme = "ws"
	}

	client, err := utils.LoadHTTPClientWithTLSConfigE(
		meta.RequestTimeout,
		meta.Channel.ProxyURL,
		meta.Channel.SkipTLSVerify,
	)
	if err != nil {
		return nil, nil, err
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		return nil, nil, errors.New("unsupported STT HTTP transport")
	}
	// Clone synchronizes the transport's lazy HTTP/2 initialization before
	// inspecting TLS settings shared with concurrent HTTP requests.
	transport = transport.Clone()
	// HTTP transports may advertise h2. Gorilla's upgrade handshake requires
	// HTTP/1.1; clone the config so this does not alter the shared HTTP client.
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if transport.TLSClientConfig != nil {
		tlsConfig = transport.TLSClientConfig.Clone()
	}

	tlsConfig.NextProtos = []string{"http/1.1"}
	dialer := websocket.Dialer{
		Proxy: transport.Proxy, NetDialContext: transport.DialContext,
		TLSClientConfig: tlsConfig, HandshakeTimeout: 10 * time.Second,
	}

	if transport.Proxy != nil {
		proxyURL, proxyErr := transport.Proxy(req)
		if proxyErr != nil {
			return nil, nil, proxyErr
		}

		if proxyURL != nil && strings.EqualFold(proxyURL.Scheme, "https") {
			// Gorilla implements HTTP CONNECT but does not register HTTPS
			// proxies. Encrypt the proxy hop first, then reuse its CONNECT
			// implementation and the separate upstream TLS handshake.
			connectProxy := *proxyURL

			connectProxy.Scheme = "http"
			if connectProxy.Port() == "" {
				connectProxy.Host = net.JoinHostPort(proxyURL.Hostname(), "443")
			}

			dialer.Proxy = http.ProxyURL(&connectProxy)
			proxyTLS := tlsConfig.Clone()
			proxyTLS.ServerName = proxyURL.Hostname()
			dialer.NetDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				conn, err := transport.DialContext(ctx, network, address)
				if err != nil {
					return nil, err
				}

				encrypted := tls.Client(conn, proxyTLS)
				if err := encrypted.HandshakeContext(ctx); err != nil {
					conn.Close()
					return nil, err
				}

				return encrypted, nil
			}
		}
	}

	return dialer.DialContext(ctx, wsURL.String(), req.Header)
}
