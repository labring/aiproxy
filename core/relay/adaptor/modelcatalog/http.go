package modelcatalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/labring/aiproxy/core/model"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
)

const (
	requestTimeout  = 30 * time.Second
	maxResponseSize = 32 << 20
)

var (
	ErrInvalidConfiguration = errors.New("invalid provider configuration")
	ErrInvalidResponse      = errors.New("invalid provider model catalog response")
	ErrUpstreamRequest      = errors.New("provider model catalog request failed")
)

type HTTPStatusError struct {
	Code int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("provider returned status %d", e.Code)
}

func JoinPath(baseURL string, paths ...string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", ErrInvalidConfiguration
	}

	endpoint, err := url.JoinPath(baseURL, paths...)
	if err != nil {
		return "", ErrInvalidConfiguration
	}

	return endpoint, nil
}

func Fetch(
	ctx context.Context,
	channel *model.Channel,
	method, targetURL string,
	headers http.Header,
	body []byte,
) ([]byte, error) {
	if channel == nil {
		return nil, ErrInvalidConfiguration
	}

	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, ErrInvalidConfiguration
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), bytes.NewReader(body))
	if err != nil {
		return nil, ErrInvalidConfiguration
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client, err := relayutils.LoadHTTPClientWithTLSConfigE(
		requestTimeout,
		channel.ProxyURL,
		channel.SkipTLSVerify,
	)
	if err != nil {
		return nil, ErrInvalidConfiguration
	}
	clientCopy := *client
	clientCopy.Timeout = requestTimeout
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || (len(via) > 0 && (req.URL.Host != via[0].URL.Host || req.URL.Scheme != via[0].URL.Scheme)) {
			return http.ErrUseLastResponse
		}
		return nil
	}

	resp, err := clientCopy.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrUpstreamRequest
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &HTTPStatusError{Code: resp.StatusCode}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, ErrUpstreamRequest
	}
	if len(data) > maxResponseSize {
		return nil, ErrInvalidResponse
	}

	return data, nil
}
