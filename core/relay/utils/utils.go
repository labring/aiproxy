package utils

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/labring/aiproxy/core/common"
	model "github.com/labring/aiproxy/core/relay/model"
	"github.com/patrickmn/go-cache"
)

func UnmarshalGeneralThinking(req *http.Request) (model.GeneralOpenAIThinkingRequest, error) {
	var request model.GeneralOpenAIThinkingRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return request, err
	}

	return request, nil
}

func UnmarshalGeneralThinkingFromNode(node *ast.Node) (model.GeneralOpenAIThinkingRequest, error) {
	var request model.GeneralOpenAIThinkingRequest

	thinkingNode := node.Get("thinking")
	if !thinkingNode.Exists() || thinkingNode.TypeSafe() == ast.V_NULL {
		return request, nil
	}

	raw, err := thinkingNode.Raw()
	if err != nil {
		return request, err
	}

	request.Thinking = &model.ClaudeThinking{}

	err = sonic.UnmarshalString(raw, request.Thinking)
	if err != nil {
		return request, err
	}

	return request, nil
}

func UnmarshalAnthropicMessageRequest(req *http.Request) (*model.AnthropicMessageRequest, error) {
	var request model.AnthropicMessageRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalGeneralOpenAIRequest(req *http.Request) (*model.GeneralOpenAIRequest, error) {
	var request model.GeneralOpenAIRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalVideoGenerationJobRequest(
	req *http.Request,
) (*model.VideoGenerationJobRequest, error) {
	var request model.VideoGenerationJobRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalImageRequest(req *http.Request) (*model.ImageRequest, error) {
	var request model.ImageRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalRerankRequest(req *http.Request) (*model.RerankRequest, error) {
	var request model.RerankRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalTTSRequest(req *http.Request) (*model.TextToSpeechRequest, error) {
	var request model.TextToSpeechRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalMap(req *http.Request) (map[string]any, error) {
	var request map[string]any

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return request, nil
}

const (
	defaultHeaderTimeout = time.Minute * 15
	tlsHandshakeTimeout  = time.Second * 5
)

var (
	defaultTransport *http.Transport
	defaultClient    *http.Client
	defaultDialer    = &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	clientCache = cache.New(time.Minute, time.Minute)
)

func init() {
	defaultTransport, _ = http.DefaultTransport.(*http.Transport)
	if defaultTransport == nil {
		panic("http default transport is not http.Transport type")
	}

	defaultTransport = defaultTransport.Clone()
	defaultTransport.DialContext = defaultDialer.DialContext
	defaultTransport.ResponseHeaderTimeout = defaultHeaderTimeout
	defaultTransport.TLSHandshakeTimeout = tlsHandshakeTimeout

	defaultClient = &http.Client{
		Transport: defaultTransport,
	}
}

func loadHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 || timeout == defaultHeaderTimeout {
		return defaultClient
	}

	key := strconv.Itoa(int(timeout))

	clientI, ok := clientCache.Get(key)
	if ok {
		client, ok := clientI.(*http.Client)
		if !ok {
			panic("unknow http client type")
		}

		return client
	}

	transport := defaultTransport.Clone()
	transport.ResponseHeaderTimeout = timeout

	client := &http.Client{
		Transport: transport,
	}
	clientCache.SetDefault(key, client)

	return client
}

func DoRequest(req *http.Request, timeout time.Duration) (*http.Response, error) {
	resp, err := loadHTTPClient(timeout).Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func IsStreamResponse(resp *http.Response) bool {
	return IsStreamResponseWithHeader(resp.Header)
}

func IsStreamResponseWithHeader(header http.Header) bool {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return false
	}

	return strings.Contains(contentType, "event-stream") ||
		strings.Contains(contentType, "x-ndjson")
}

const scannerBufferSize = 256 * 1024

var scannerBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, scannerBufferSize)
		return &buf
	},
}

//nolint:forcetypeassert
func GetScannerBuffer() *[]byte {
	v, ok := scannerBufferPool.Get().(*[]byte)
	if !ok {
		panic(fmt.Sprintf("scanner buffer type error: %T, %v", v, v))
	}

	return v
}

func PutScannerBuffer(buf *[]byte) {
	if cap(*buf) != scannerBufferSize {
		return
	}

	scannerBufferPool.Put(buf)
}
