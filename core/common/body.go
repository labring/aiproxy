package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
)

type requestBodyKey struct{}

const (
	MaxRequestBodySize  = 1024 * 1024 * 50 // 50MB
	MaxResponseBodySize = 1024 * 1024 * 50 // 50MB
)

func LimitReader(r io.Reader, n int64) io.Reader { return &LimitedReader{r, n} }

type LimitedReader struct {
	R io.Reader
	N int64
}

var ErrLimitedReaderExceeded = errors.New("limited reader exceeded")

func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, ErrLimitedReaderExceeded
	}

	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}

	n, err = l.R.Read(p)
	l.N -= int64(n)

	return n, err
}

func GetBodyLimit(body io.Reader, contentLength, n int64) ([]byte, error) {
	var (
		buf []byte
		err error
	)

	if contentLength <= 0 {
		buf, err = io.ReadAll(LimitReader(body, n))
		if err != nil {
			if errors.Is(err, ErrLimitedReaderExceeded) {
				return nil, fmt.Errorf("body too large, max: %d", n)
			}
			return nil, fmt.Errorf("body read failed: %w", err)
		}
	} else {
		if contentLength > n {
			return nil, fmt.Errorf("body too large: %d, max: %d", contentLength, n)
		}

		buf = make([]byte, contentLength)
		_, err = io.ReadFull(body, buf)
	}

	if err != nil {
		return nil, fmt.Errorf("body read failed: %w", err)
	}

	return buf, nil
}

func GetRequestBodyLimit(req *http.Request, n int64) ([]byte, error) {
	return GetBodyLimit(req.Body, req.ContentLength, n)
}

func GetRequestBody(req *http.Request) ([]byte, error) {
	return GetRequestBodyLimit(req, MaxRequestBodySize)
}

func SetRequestBody(req *http.Request, body []byte) {
	ctx := req.Context()
	bufCtx := context.WithValue(ctx, requestBodyKey{}, body)
	*req = *req.WithContext(bufCtx)
}

func IsJSONContentType(ct string) bool {
	return strings.HasSuffix(ct, "/json") ||
		strings.Contains(ct, "/json;")
}

func GetRequestBodyReusable(req *http.Request) ([]byte, error) {
	contentType := req.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") ||
		strings.HasPrefix(contentType, "multipart/form-data") {
		return nil, nil
	}

	requestBody := req.Context().Value(requestBodyKey{})
	if requestBody != nil {
		b, _ := requestBody.([]byte)
		return b, nil
	}

	var (
		buf []byte
		err error
	)

	defer func() {
		req.Body.Close()

		if err == nil {
			req.Body = io.NopCloser(bytes.NewBuffer(buf))
		}
	}()

	if req.ContentLength <= 0 ||
		IsJSONContentType(contentType) {
		buf, err = io.ReadAll(LimitReader(req.Body, MaxRequestBodySize))
		if err != nil {
			if errors.Is(err, ErrLimitedReaderExceeded) {
				return nil, fmt.Errorf("request body too large, max: %d", MaxRequestBodySize)
			}
			return nil, fmt.Errorf("request body read failed: %w", err)
		}
	} else {
		if req.ContentLength > MaxRequestBodySize {
			return nil, fmt.Errorf("request body too large: %d, max: %d", req.ContentLength, MaxRequestBodySize)
		}

		buf = make([]byte, req.ContentLength)
		_, err = io.ReadFull(req.Body, buf)
	}

	if err != nil {
		return nil, fmt.Errorf("request body read failed: %w", err)
	}

	SetRequestBody(req, buf)

	return buf, nil
}

func UnmarshalRequestReusable(req *http.Request, v any) error {
	requestBody, err := GetRequestBodyReusable(req)
	if err != nil {
		return err
	}

	return sonic.Unmarshal(requestBody, &v)
}

func UnmarshalRequest2NodeReusable(req *http.Request, path ...any) (ast.Node, error) {
	requestBody, err := GetRequestBodyReusable(req)
	if err != nil {
		return ast.Node{}, err
	}

	return sonic.Get(requestBody, path...)
}

func GetResponseBodyLimit(resp *http.Response, n int64) ([]byte, error) {
	return GetBodyLimit(resp.Body, resp.ContentLength, n)
}

func GetResponseBody(resp *http.Response) ([]byte, error) {
	return GetResponseBodyLimit(resp, MaxResponseBodySize)
}

func UnmarshalResponse(resp *http.Response, v any) error {
	responseBody, err := GetResponseBody(resp)
	if err != nil {
		return err
	}

	return sonic.Unmarshal(responseBody, &v)
}

func UnmarshalResponse2Node(resp *http.Response, path ...any) (ast.Node, error) {
	responseBody, err := GetResponseBody(resp)
	if err != nil {
		return ast.Node{}, err
	}

	return sonic.Get(responseBody, path...)
}
