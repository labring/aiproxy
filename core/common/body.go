package common

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/klauspost/compress/zstd"
	"github.com/labring/aiproxy/core/common/config"
)

type reusableRequestBody struct {
	*bytes.Reader
	body []byte
}

func newReusableRequestBody(body []byte) *reusableRequestBody {
	return &reusableRequestBody{
		Reader: bytes.NewReader(body),
		body:   body,
	}
}

func (b *reusableRequestBody) Close() error {
	return nil
}

func (b *reusableRequestBody) Bytes() []byte {
	return b.body
}

func LimitReader(r io.Reader, n int64) io.Reader { return &LimitedReader{r, n} }

func ParseMultipartFormWithLimit(req *http.Request) error {
	if err := decodeZstdRequest(req); err != nil {
		return err
	}

	if req.ContentLength > 0 && req.ContentLength > config.MaxRequestBodySize {
		return fmt.Errorf(
			"request body too large: %d, max: %d",
			req.ContentLength,
			config.MaxRequestBodySize,
		)
	}

	originalBody := req.Body

	req.Body = http.MaxBytesReader(nil, req.Body, config.MaxRequestBodySize)
	defer func() {
		req.Body = originalBody
	}()

	// #nosec G120 -- ContentLength is checked above and Body is capped by MaxBytesReader.
	return req.ParseMultipartForm(config.MultipartFormMemoryLimit)
}

func ParseFormWithLimit(req *http.Request) error {
	if err := decodeZstdRequest(req); err != nil {
		return err
	}

	if req.ContentLength > 0 && req.ContentLength > config.MaxRequestBodySize {
		return fmt.Errorf(
			"request body too large: %d, max: %d",
			req.ContentLength,
			config.MaxRequestBodySize,
		)
	}

	originalBody := req.Body

	req.Body = http.MaxBytesReader(nil, req.Body, config.MaxRequestBodySize)
	defer func() {
		req.Body = originalBody
	}()

	return req.ParseForm()
}

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
	return GetRequestBodyLimit(req, config.MaxRequestBodySize)
}

func SetRequestBody(req *http.Request, body []byte) {
	req.ContentLength = int64(len(body))
	req.GetBody = nil
	req.Body = newReusableRequestBody(body)
}

func GetCachedRequestBody(req *http.Request) ([]byte, bool) {
	if req == nil {
		return nil, false
	}

	if body, ok := req.Body.(*reusableRequestBody); ok {
		return body.Bytes(), true
	}

	return nil, false
}

func IsJSONContentType(ct string) bool {
	return strings.HasSuffix(ct, "/json") ||
		strings.Contains(ct, "/json;")
}

func GetRequestBodyReusable(req *http.Request) ([]byte, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	if err := decodeZstdRequest(req); err != nil {
		return nil, err
	}

	contentType := req.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") ||
		strings.HasPrefix(contentType, "multipart/form-data") {
		return nil, nil
	}

	if body, ok := GetCachedRequestBody(req); ok {
		return body, nil
	}

	originalBody := req.Body
	defer func() {
		if originalBody != nil {
			_ = originalBody.Close()
		}
	}()

	buf, err := GetBodyLimit(req.Body, req.ContentLength, config.MaxRequestBodySize)
	if err != nil {
		return nil, fmt.Errorf("request body read failed: %w", err)
	}

	SetRequestBody(req, buf)

	return buf, nil
}

func isZstdContentEncoding(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "zstd")
}

func decodeZstdRequest(req *http.Request) error {
	if req == nil {
		return errors.New("request is nil")
	}

	if !isZstdContentEncoding(req.Header.Get("Content-Encoding")) {
		return nil
	}

	originalBody := req.Body
	if originalBody != nil {
		defer originalBody.Close()
	}

	compressed, err := GetBodyLimit(req.Body, req.ContentLength, config.MaxRequestBodySize)
	if err != nil {
		return fmt.Errorf("request body read failed: %w", err)
	}

	decoded, err := decodeZstdRequestBody(compressed)
	if err != nil {
		return err
	}

	SetRequestBody(req, decoded)
	req.Header.Del("Content-Encoding")

	return nil
}

func decodeZstdRequestBody(compressed []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(
		nil,
		zstd.WithDecoderMaxMemory(uint64(max(config.MaxRequestBodySize, 0))),
		zstd.WithDecoderMaxWindow(uint64(max(config.MaxRequestBodySize, 0))),
	)
	if err != nil {
		return nil, fmt.Errorf("zstd decoder initialization failed: %w", err)
	}
	defer decoder.Close()

	decoded, err := decoder.DecodeAll(compressed, nil)
	if err != nil {
		if errors.Is(err, zstd.ErrDecoderSizeExceeded) {
			return nil, fmt.Errorf(
				"decompressed request body too large, max: %d",
				config.MaxRequestBodySize,
			)
		}

		return nil, fmt.Errorf("zstd request body decode failed: %w", err)
	}

	if int64(len(decoded)) > config.MaxRequestBodySize {
		return nil, fmt.Errorf(
			"decompressed request body too large: %d, max: %d",
			len(decoded),
			config.MaxRequestBodySize,
		)
	}

	return decoded, nil
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

	return GetJSONNodeNoCopy(requestBody, path...)
}

func GetJSONNodeNoCopy(data []byte, path ...any) (ast.Node, error) {
	return sonic.GetWithOptions(data, ast.SearchOptions{}, path...)
}

func GetResponseBodyLimit(resp *http.Response, n int64) ([]byte, error) {
	var (
		buf []byte
		err error
	)

	if resp.ContentLength <= 0 {
		buf, err = io.ReadAll(LimitReader(resp.Body, n))
		if err != nil {
			if errors.Is(err, ErrLimitedReaderExceeded) {
				return nil, errors.New("response body too large")
			}
			return nil, fmt.Errorf("response body read failed: %w", err)
		}
	} else {
		if resp.ContentLength > n {
			return nil, errors.New("response body too large")
		}

		buf = make([]byte, resp.ContentLength)
		_, err = io.ReadFull(resp.Body, buf)
	}

	if err != nil {
		return nil, fmt.Errorf("response body read failed: %w", err)
	}

	return buf, nil
}

func GetResponseBody(resp *http.Response) ([]byte, error) {
	return GetResponseBodyLimit(resp, config.MaxResponseBodySize)
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

	return GetJSONNodeNoCopy(responseBody, path...)
}
