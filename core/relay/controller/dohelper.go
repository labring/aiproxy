package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	log "github.com/sirupsen/logrus"
)

const (
	// 0.5MB
	maxBufferSize = 512 * 1024
)

type responseWriter struct {
	gin.ResponseWriter
	body        *bytes.Buffer
	bodyLimit   int
	firstByteAt time.Time
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.firstByteAt.IsZero() {
		rw.firstByteAt = time.Now()
	}

	if rw.body != nil && rw.bodyLimit > rw.body.Len() {
		remain := min(rw.bodyLimit-rw.body.Len(), len(b))

		if remain > 0 {
			rw.body.Write(b[:remain])
		}
	}

	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) WriteString(s string) (int, error) {
	return rw.Write(conv.StringToBytes(s))
}

var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, maxBufferSize))
	},
}

func getBuffer() *bytes.Buffer {
	v, ok := bufferPool.Get().(*bytes.Buffer)
	if !ok {
		panic(fmt.Sprintf("buffer type error: %T, %v", v, v))
	}

	return v
}

func putBuffer(buf *bytes.Buffer) {
	buf.Reset()

	if buf.Cap() > maxBufferSize {
		return
	}

	bufferPool.Put(buf)
}

type BodyDetail struct {
	RequestBody  string
	ResponseBody string
	FirstByteAt  time.Time
}

type BodyDetailOption struct {
	IncludeRequestBody  bool
	IncludeResponseBody bool
	MaxRequestBodySize  int64
	MaxResponseBodySize int64
}

func DoHelper(
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
	opts ...BodyDetailOption,
) (
	adaptor.DoResponseResult,
	*BodyDetail,
	adaptor.Error,
) {
	detail := BodyDetail{}
	detailOption := mergeBodyDetailOptions(opts...)

	if requestBody, err := requestBodyDetail(c, detailOption); err != nil {
		common.GetLogger(c).Warnf("get request body detail failed: %v", err)
	} else {
		detail.RequestBody = requestBody
	}

	// donot use c.Request.Context() because it will be canceled by the client
	ctx := context.Background()

	resp, err := prepareAndDoRequest(ctx, a, c, meta, store)
	if err != nil {
		return adaptor.DoResponseResult{}, &detail, err
	}

	if resp == nil {
		relayErr := relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"response is nil",
		)
		respBody, _ := relayErr.MarshalJSON()
		detail.ResponseBody = conv.BytesToString(respBody)

		return adaptor.DoResponseResult{}, &detail, relayErr
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	result, relayErr := handleResponse(a, c, meta, store, resp, &detail, detailOption)
	if relayErr != nil {
		return adaptor.DoResponseResult{}, &detail, relayErr
	}

	log := common.GetLogger(c)
	updateUsageMetrics(result.Usage, log)

	if result.UpstreamID != "" {
		log.Data["upstream_id"] = result.UpstreamID
	}

	if !detail.FirstByteAt.IsZero() {
		ttfb := detail.FirstByteAt.Sub(meta.RequestAt)
		log.Data["ttfb"] = common.TruncateDuration(ttfb).String()
	}

	return result, &detail, nil
}

func prepareAndDoRequest(
	ctx context.Context,
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
) (*http.Response, adaptor.Error) {
	log := common.GetLogger(c)

	convertResult, err := a.ConvertRequest(meta, store, c.Request)
	if err != nil {
		return nil, mapRequestError(meta, err, http.StatusBadRequest, "convert request failed")
	}

	if closer, ok := convertResult.Body.(io.Closer); ok {
		defer closer.Close()
	}

	if meta.Channel.BaseURL == "" {
		meta.Channel.BaseURL = a.DefaultBaseURL()
	}

	fullRequestURL, err := a.GetRequestURL(meta, store, c)
	if err != nil {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusBadRequest,
			"get request url failed: "+err.Error(),
		)
	}

	log.Debugf("request url: %s %s", fullRequestURL.Method, fullRequestURL.URL)

	req, err := http.NewRequestWithContext(
		ctx,
		fullRequestURL.Method,
		fullRequestURL.URL,
		convertResult.Body,
	)
	if err != nil {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusBadRequest,
			"new request failed: "+err.Error(),
		)
	}

	if err := setupRequestHeader(a, c, meta, store, req, convertResult.Header); err != nil {
		return nil, err
	}

	return doRequest(a, c, meta, store, req)
}

func mapRequestError(
	meta *meta.Meta,
	err error,
	fallbackStatusCode int,
	fallbackMessage string,
) adaptor.Error {
	if adaptorErr, ok := errors.AsType[adaptor.Error](err); ok {
		return adaptorErr
	}

	if errors.Is(err, context.Canceled) {
		return relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusBadRequest,
			"request canceled by client: "+err.Error(),
		)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusRequestTimeout,
			"timeout with deadline exceeded: "+err.Error(),
		)
	}

	if errors.Is(err, io.EOF) {
		return relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusServiceUnavailable,
			"request eof: "+err.Error(),
		)
	}

	if errors.Is(err, io.ErrUnexpectedEOF) {
		return relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"request unexpected eof: "+err.Error(),
		)
	}

	if strings.Contains(err.Error(), "timeout awaiting response headers") {
		return relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusRequestTimeout,
			"request timeout: "+err.Error(),
		)
	}

	return relaymodel.WrapperErrorWithMessage(
		meta.Mode,
		fallbackStatusCode,
		fallbackMessage+": "+err.Error(),
	)
}

func setupRequestHeader(
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
	header http.Header,
) adaptor.Error {
	maps.Copy(req.Header, header)

	if err := a.SetupRequestHeader(meta, store, c, req); err != nil {
		return relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"setup request header failed: "+err.Error(),
		)
	}

	return nil
}

func doRequest(
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (*http.Response, adaptor.Error) {
	resp, err := a.DoRequest(meta, store, c, req)
	if err != nil {
		return nil, mapRequestError(meta, err, http.StatusInternalServerError, "request error")
	}

	return resp, nil
}

func handleResponse(
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
	resp *http.Response,
	detail *BodyDetail,
	opt BodyDetailOption,
) (adaptor.DoResponseResult, adaptor.Error) {
	var (
		buf       *bytes.Buffer
		bodyLimit int
	)

	bodyLimit = responseBodyCaptureLimit(opt)

	if bodyLimit > 0 {
		buf = getBuffer()
		defer putBuffer(buf)
	}

	rw := &responseWriter{
		ResponseWriter: c.Writer,
		body:           buf,
		bodyLimit:      bodyLimit,
	}

	rawWriter := c.Writer
	defer func() {
		c.Writer = rawWriter
		detail.FirstByteAt = rw.firstByteAt
	}()

	c.Writer = rw

	result, relayErr := a.DoResponse(meta, store, c, resp)
	if relayErr != nil && opt.IncludeResponseBody && opt.MaxResponseBodySize >= 0 {
		respBody, _ := relayErr.MarshalJSON()
		detail.ResponseBody = limitBodyDetail(conv.BytesToString(respBody), opt.MaxResponseBodySize)
	} else if rw.body != nil {
		// copy body buffer
		// do not use bytes conv
		detail.ResponseBody = limitBodyDetail(rw.body.String(), opt.MaxResponseBodySize)
	}

	if result.UpstreamID == "" && resp != nil && resp.Header != nil &&
		resp.Header.Get("x-request-id") != "" {
		result.UpstreamID = resp.Header.Get("x-request-id")
	}

	return result, relayErr
}

func mergeBodyDetailOptions(opts ...BodyDetailOption) BodyDetailOption {
	if len(opts) == 0 {
		return BodyDetailOption{}
	}

	return opts[0]
}

func requestBodyDetail(c *gin.Context, opt BodyDetailOption) (string, error) {
	if !opt.IncludeRequestBody ||
		opt.MaxRequestBodySize < 0 ||
		c == nil ||
		c.Request == nil ||
		!common.IsJSONContentType(c.GetHeader("Content-Type")) {
		return "", nil
	}

	body, err := common.GetRequestBodyReusable(c.Request)
	if err != nil {
		return "", err
	}

	return limitBodyDetail(conv.BytesToString(body), opt.MaxRequestBodySize), nil
}

func limitBodyDetail(body string, maxSize int64) string {
	if maxSize != 0 && int64(len(body)) > maxSize {
		body = body[:min(len(body), int(maxSize)+1)]
	}

	for len(body) > 0 && !utf8.ValidString(body) {
		body = body[:len(body)-1]
	}

	return body
}

func responseBodyCaptureLimit(opt BodyDetailOption) int {
	if !opt.IncludeResponseBody || opt.MaxResponseBodySize < 0 {
		return 0
	}

	if opt.MaxResponseBodySize == 0 {
		return maxBufferSize
	}

	if opt.MaxResponseBodySize >= int64(maxBufferSize) {
		return maxBufferSize
	}

	return int(opt.MaxResponseBodySize + 1)
}

func updateUsageMetrics(usage model.Usage, log *log.Entry) {
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	if usage.InputTokens > 0 {
		log.Data["t_input"] = usage.InputTokens
	}

	if usage.ImageInputTokens > 0 {
		log.Data["t_image_input"] = usage.ImageInputTokens
	}

	if usage.AudioInputTokens > 0 {
		log.Data["t_audio_input"] = usage.AudioInputTokens
	}

	if usage.OutputTokens > 0 {
		log.Data["t_output"] = usage.OutputTokens
	}

	if usage.ImageOutputTokens > 0 {
		log.Data["t_image_output"] = usage.ImageOutputTokens
	}

	if usage.TotalTokens > 0 {
		log.Data["t_total"] = usage.TotalTokens
	}

	if usage.CachedTokens > 0 {
		log.Data["t_cached"] = usage.CachedTokens
	}

	if usage.CacheCreationTokens > 0 {
		log.Data["t_cache_creation"] = usage.CacheCreationTokens
	}

	if usage.ReasoningTokens > 0 {
		log.Data["t_reason"] = usage.ReasoningTokens
	}

	if usage.WebSearchCount > 0 {
		log.Data["t_websearch"] = usage.WebSearchCount
	}
}
