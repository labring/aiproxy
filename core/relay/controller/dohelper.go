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

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
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

	if err := storeRequestBody(meta, c, &detail); err != nil {
		return model.Usage{}, nil, err
	}

	// donot use c.Request.Context() because it will be canceled by the client
	ctx := context.Background()

	resp, err := prepareAndDoRequest(ctx, a, c, meta, store)
	if err != nil {
		return model.Usage{}, &detail, err
	}

	if resp == nil {
		relayErr := relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"response is nil",
		)
		respBody, _ := relayErr.MarshalJSON()
		detail.ResponseBody = conv.BytesToString(respBody)

		return model.Usage{}, &detail, relayErr
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	usage, relayErr := handleResponse(a, c, meta, store, resp, &detail)
	if relayErr != nil {
		return model.Usage{}, &detail, relayErr
	}

	log := common.GetLogger(c)
	updateUsageMetrics(usage, log)

	if !detail.FirstByteAt.IsZero() {
		ttfb := detail.FirstByteAt.Sub(meta.RequestAt)
		log.Data["ttfb"] = common.TruncateDuration(ttfb).String()
	}

	return usage, &detail, nil
}

func storeRequestBody(meta *meta.Meta, c *gin.Context, detail *RequestDetail) adaptor.Error {
	switch {
	case meta.Mode == mode.AudioTranscription,
		meta.Mode == mode.AudioTranslation,
		meta.Mode == mode.ImagesEdits:
		return nil
	case !common.IsJSONContentType(c.GetHeader("Content-Type")):
		return nil
	default:
		reqBody, err := common.GetRequestBodyReusable(c.Request)
		if err != nil {
			return relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusBadRequest,
				"get request body failed: "+err.Error(),
			)
		}

		detail.RequestBody = conv.BytesToString(reqBody)

		return nil
	}
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
		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusBadRequest,
			"convert request failed: "+err.Error(),
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
		var adaptorErr adaptor.Error

		ok := errors.As(err, &adaptorErr)
		if ok {
			return nil, adaptorErr
		}

		if errors.Is(err, context.Canceled) {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusBadRequest,
				"request canceled by client: "+err.Error(),
			)
		}

		if errors.Is(err, context.DeadlineExceeded) {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusRequestTimeout,
				"request timeout: "+err.Error(),
			)
		}

		if errors.Is(err, io.EOF) {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusServiceUnavailable,
				"request eof: "+err.Error(),
			)
		}

		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusInternalServerError,
				"request unexpected eof: "+err.Error(),
			)
		}

		if strings.Contains(err.Error(), "timeout awaiting response headers") {
			return nil, relaymodel.WrapperErrorWithMessage(
				meta.Mode,
				http.StatusRequestTimeout,
				"request timeout: "+err.Error(),
			)
		}

		return nil, relaymodel.WrapperErrorWithMessage(
			meta.Mode,
			http.StatusInternalServerError,
			"request error: "+err.Error(),
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

	if usage.AudioInputTokens > 0 {
		log.Data["t_audio_input"] = usage.AudioInputTokens
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
