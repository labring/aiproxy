package utils

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	xproxy "golang.org/x/net/proxy"
)

const (
	defaultHeaderTimeout = time.Minute * 15
	tlsHandshakeTimeout  = time.Second * 5
)

var (
	defaultDialer = &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	httpClientCache = cache.New(time.Minute*10, time.Minute)
)

type cachedHTTPClient struct {
	client    *http.Client
	transport *http.Transport
}

func init() {
	httpClientCache.OnEvicted(func(_ string, value any) {
		cached, ok := value.(*cachedHTTPClient)
		if !ok || cached == nil || cached.transport == nil {
			return
		}

		cached.transport.CloseIdleConnections()
	})
}

func defaultTransportTemplate() *http.Transport {
	transport, _ := http.DefaultTransport.(*http.Transport)
	if transport == nil {
		panic("http default transport is not http.Transport type")
	}

	transport = transport.Clone()
	transport.DialContext = defaultDialer.DialContext
	transport.ResponseHeaderTimeout = defaultHeaderTimeout
	transport.TLSHandshakeTimeout = tlsHandshakeTimeout

	return transport
}

func normalizeTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultHeaderTimeout
	}

	return timeout
}

func normalizeProxyURL(proxyURL string) string {
	return strings.TrimSpace(proxyURL)
}

func httpClientCacheKey(timeout time.Duration, proxyURL string, skipTLSVerify bool) string {
	return fmt.Sprintf(
		"%d|%s|%t",
		normalizeTimeout(timeout),
		normalizeProxyURL(proxyURL),
		skipTLSVerify,
	)
}

func createTransport(
	timeout time.Duration,
	proxyURL string,
	skipTLSVerify bool,
) (*http.Transport, error) {
	transport := defaultTransportTemplate()

	transport.ResponseHeaderTimeout = normalizeTimeout(timeout)
	if skipTLSVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		}
	}

	proxyURL = normalizeProxyURL(proxyURL)
	if proxyURL == "" {
		return transport, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsed)
	case "socks5", "socks5h":
		dialer, err := socks5Dialer(parsed)
		if err != nil {
			return nil, err
		}

		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			type dialResult struct {
				conn net.Conn
				err  error
			}

			resultCh := make(chan dialResult, 1)
			go func() {
				defer close(resultCh)

				conn, err := dialer.Dial(network, addr)
				resultCh <- dialResult{conn: conn, err: err}
			}()

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case result := <-resultCh:
				return result.conn, result.err
			}
		}
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", parsed.Scheme)
	}

	return transport, nil
}

func socks5Dialer(proxyURL *url.URL) (xproxy.Dialer, error) {
	address := proxyURL.Host
	if address == "" {
		return nil, errors.New("invalid proxy url: host is required")
	}

	var auth *xproxy.Auth
	if proxyURL.User != nil {
		auth = &xproxy.Auth{
			User: proxyURL.User.Username(),
		}

		if password, ok := proxyURL.User.Password(); ok {
			auth.Password = password
		}
	}

	dialer, err := xproxy.SOCKS5("tcp", address, auth, defaultDialer)
	if err != nil {
		return nil, fmt.Errorf("create socks5 proxy dialer failed: %w", err)
	}

	return dialer, nil
}

func LoadHTTPClient(timeout time.Duration, proxyURL string) *http.Client {
	client, err := LoadHTTPClientE(timeout, proxyURL)
	if err != nil {
		panic(err)
	}

	return client
}

func LoadHTTPClientE(timeout time.Duration, proxyURL string) (*http.Client, error) {
	return LoadHTTPClientWithTLSConfigE(timeout, proxyURL, false)
}

func LoadHTTPClientWithTLSConfigE(
	timeout time.Duration,
	proxyURL string,
	skipTLSVerify bool,
) (*http.Client, error) {
	key := httpClientCacheKey(timeout, proxyURL, skipTLSVerify)
	if value, ok := httpClientCache.Get(key); ok {
		cached, ok := value.(*cachedHTTPClient)
		if !ok {
			return nil, fmt.Errorf("invalid http client cache type: %T", value)
		}

		return cached.client, nil
	}

	transport, err := createTransport(timeout, proxyURL, skipTLSVerify)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: transport,
	}

	httpClientCache.SetDefault(key, &cachedHTTPClient{
		client:    client,
		transport: transport,
	})

	return client, nil
}
