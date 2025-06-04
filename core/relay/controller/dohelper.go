package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
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
	firstByteAt time.Time
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.firstByteAt.IsZero() {
		rw.firstByteAt = time.Now()
	}
	if rw.body.Len()+len(b) <= maxBufferSize {
		rw.body.Write(b)
	} else {
		rw.body.Write(b[:maxBufferSize-rw.body.Len()])
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

type RequestDetail struct {
	RequestBody  string
	ResponseBody string
	FirstByteAt  time.Time
}

func DoHelper(
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
) (
	model.Usage,
	*RequestDetail,
	adaptor.Error,
) {
	detail := RequestDetail{}

	// 1. Get request body
	if err := getRequestBody(meta, c, &detail); err != nil {
		return model.Usage{}, nil, err
	}

	// donot use c.Request.Context() because it will be canceled by the client
	ctx := context.Background()

	timeout := meta.ModelConfig.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// 2. Convert and prepare request
	resp, err := prepareAndDoRequest(ctx, a, c, meta, store)
	if err != nil {
		return model.Usage{}, &detail, err
	}

	// 3. Handle error response
	if resp == nil {
		relayErr := relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"response is nil",
			relaymodel.ErrorCodeBadResponse,
		)
		respBody, _ := relayErr.MarshalJSON()
		detail.ResponseBody = conv.BytesToString(respBody)
		return model.Usage{}, &detail, relayErr
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	// 4. Handle success response
	usage, relayErr := handleResponse(a, c, meta, store, resp, &detail)
	if relayErr != nil {
		return model.Usage{}, &detail, relayErr
	}

	// 5. Update usage metrics
	updateUsageMetrics(usage, middleware.GetLogger(c))

	return usage, &detail, nil
}

func getRequestBody(meta *meta.Meta, c *gin.Context, detail *RequestDetail) adaptor.Error {
	switch {
	case meta.Mode == mode.AudioTranscription,
		meta.Mode == mode.AudioTranslation,
		meta.Mode == mode.ImagesEdits:
		return nil
	case !strings.Contains(c.GetHeader("Content-Type"), "/json"):
		return nil
	default:
		reqBody, err := common.GetRequestBody(c.Request)
		if err != nil {
			return relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusBadRequest,
				"get request body failed: "+err.Error(),
				"get_request_body_failed",
			)
		}
		detail.RequestBody = conv.BytesToString(reqBody)
		return nil
	}
}

const defaultTimeout = 60 * 30 // 30 minutes

func prepareAndDoRequest(
	ctx context.Context,
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
) (*http.Response, adaptor.Error) {
	log := middleware.GetLogger(c)

	convertResult, err := a.ConvertRequest(meta, store, c.Request)
	if err != nil {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusBadRequest,
			"convert request failed: "+err.Error(),
			"convert_request_failed",
		)
	}
	if closer, ok := convertResult.Body.(io.Closer); ok {
		defer closer.Close()
	}

	if meta.Channel.BaseURL == "" {
		meta.Channel.BaseURL = a.DefaultBaseURL()
	}

	fullRequestURL, err := a.GetRequestURL(meta, store)
	if err != nil {
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusBadRequest,
			"get request url failed: "+err.Error(),
			"get_request_url_failed",
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
			"new_request_failed",
		)
	}

	if err := setupRequestHeader(a, c, meta, store, req, convertResult.Header); err != nil {
		return nil, err
	}

	return doRequest(a, c, meta, store, req)
}

func setupRequestHeader(
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
	header http.Header,
) adaptor.Error {
	for key, value := range header {
		req.Header[key] = value
	}
	if err := a.SetupRequestHeader(meta, store, c, req); err != nil {
		return relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"setup request header failed: "+err.Error(),
			"setup_request_header_failed",
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
		if errors.Is(err, context.Canceled) {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusBadRequest,
				"do request failed: request canceled by client",
				"request_canceled",
			)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusGatewayTimeout,
				"do request failed: request timeout",
				"request_timeout",
			)
		}
		if errors.Is(err, io.EOF) {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusServiceUnavailable,
				"do request failed: "+err.Error(),
				"request_failed",
			)
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusInternalServerError,
				"do request failed: "+err.Error(),
				"request_failed",
			)
		}
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusBadRequest,
			"do request failed: "+err.Error(),
			"request_failed",
		)
	}
	return resp, nil
}

func handleResponse(
	a adaptor.Adaptor,
	c *gin.Context,
	meta *meta.Meta,
	store adaptor.Store,
	resp *http.Response,
	detail *RequestDetail,
) (model.Usage, adaptor.Error) {
	buf := getBuffer()
	defer putBuffer(buf)

	rw := &responseWriter{
		ResponseWriter: c.Writer,
		body:           buf,
	}
	rawWriter := c.Writer
	defer func() {
		c.Writer = rawWriter
		detail.FirstByteAt = rw.firstByteAt
	}()
	c.Writer = rw

	usage, relayErr := a.DoResponse(meta, store, c, resp)
	if relayErr != nil {
		respBody, _ := relayErr.MarshalJSON()
		detail.ResponseBody = conv.BytesToString(respBody)
	} else {
		// copy body buffer
		// do not use bytes conv
		detail.ResponseBody = rw.body.String()
	}

	return usage, relayErr
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
	if usage.OutputTokens > 0 {
		log.Data["t_output"] = usage.OutputTokens
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
