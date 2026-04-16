package utils

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func UnmarshalGeneralThinking(req *http.Request) (relaymodel.GeneralOpenAIThinkingRequest, error) {
	var request relaymodel.GeneralOpenAIThinkingRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return request, err
	}

	return request, nil
}

func UnmarshalGeneralThinkingFromNode(
	node *ast.Node,
) (relaymodel.GeneralOpenAIThinkingRequest, error) {
	var request relaymodel.GeneralOpenAIThinkingRequest

	thinkingNode := node.Get("thinking")
	if thinkingNode == nil || !thinkingNode.Exists() || thinkingNode.TypeSafe() == ast.V_NULL {
		return request, nil
	}

	raw, err := thinkingNode.Raw()
	if err != nil {
		return request, err
	}

	request.Thinking = &relaymodel.ClaudeThinking{}

	err = sonic.UnmarshalString(raw, request.Thinking)
	if err != nil {
		return request, err
	}

	return request, nil
}

func UnmarshalAnthropicMessageRequest(
	req *http.Request,
) (*relaymodel.AnthropicMessageRequest, error) {
	var request relaymodel.AnthropicMessageRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalGeneralOpenAIRequest(req *http.Request) (*relaymodel.GeneralOpenAIRequest, error) {
	var request relaymodel.GeneralOpenAIRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalVideoGenerationJobRequest(
	req *http.Request,
) (*relaymodel.VideoGenerationJobRequest, error) {
	var request relaymodel.VideoGenerationJobRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalImageRequest(req *http.Request) (*relaymodel.ImageRequest, error) {
	var request relaymodel.ImageRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalRerankRequest(req *http.Request) (*relaymodel.RerankRequest, error) {
	var request relaymodel.RerankRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalTTSRequest(req *http.Request) (*relaymodel.TextToSpeechRequest, error) {
	var request relaymodel.TextToSpeechRequest

	err := common.UnmarshalRequestReusable(req, &request)
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func UnmarshalGeminiChatRequest(req *http.Request) (*relaymodel.GeminiChatRequest, error) {
	var request relaymodel.GeminiChatRequest

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

func DoRequest(req *http.Request, timeout time.Duration) (*http.Response, error) {
	client, err := LoadHTTPClientE(timeout, "")
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req) //nolint:gosec // request URL is from caller
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func DoRequestWithMeta(req *http.Request, m *meta.Meta) (*http.Response, error) {
	if m == nil {
		return DoRequest(req, 0)
	}

	client, err := LoadHTTPClientWithTLSConfigE(
		m.RequestTimeout,
		m.Channel.ProxyURL,
		m.Channel.SkipTLSVerify,
	)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req) //nolint:gosec // request URL is from caller
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

const ImageScannerBufferSize = 50 * 1024 * 1024

var imageScannerBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, ImageScannerBufferSize)
		return &buf
	},
}

//nolint:forcetypeassert
func GetImageScannerBuffer() *[]byte {
	v, ok := imageScannerBufferPool.Get().(*[]byte)
	if !ok {
		panic(fmt.Sprintf("image scanner buffer type error: %T, %v", v, v))
	}

	return v
}

func PutImageScannerBuffer(buf *[]byte) {
	if cap(*buf) != ImageScannerBufferSize {
		return
	}

	imageScannerBufferPool.Put(buf)
}

// IsImageModel checks if the model name indicates an image generation model
func IsImageModel(modelName string) bool {
	return strings.Contains(modelName, "image")
}

// NewStreamScanner creates a bufio.Scanner with appropriate buffer size based on model type.
// Returns the scanner and a cleanup function that must be called when done.
func NewStreamScanner(r io.Reader, modelName string) (*bufio.Scanner, func()) {
	scanner := bufio.NewScanner(r)

	if IsImageModel(modelName) {
		buf := GetImageScannerBuffer()
		scanner.Buffer(*buf, cap(*buf))

		return scanner, func() {
			PutImageScannerBuffer(buf)
		}
	}

	buf := GetScannerBuffer()
	scanner.Buffer(*buf, cap(*buf))

	return scanner, func() {
		PutScannerBuffer(buf)
	}
}

// NewScanner creates a bufio.Scanner with standard buffer size.
// Returns the scanner and a cleanup function that must be called when done.
func NewScanner(r io.Reader) (*bufio.Scanner, func()) {
	scanner := bufio.NewScanner(r)
	buf := GetScannerBuffer()
	scanner.Buffer(*buf, cap(*buf))

	return scanner, func() {
		PutScannerBuffer(buf)
	}
}

// IsGeminiStreamRequest checks if the request path ends with :streamGenerateContent
func IsGeminiStreamRequest(path string) bool {
	return strings.HasSuffix(path, ":streamGenerateContent")
}
