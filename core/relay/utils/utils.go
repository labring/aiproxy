package utils

import (
	"net/http"
	"strings"

	"github.com/labring/aiproxy/core/common"
	model "github.com/labring/aiproxy/core/relay/model"
)

func UnmarshalAnthropicMessageRequest(req *http.Request) (*model.AnthropicMessageRequest, error) {
	var request model.AnthropicMessageRequest
	err := common.UnmarshalBodyReusable(req, &request)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func UnmarshalGeneralOpenAIRequest(req *http.Request) (*model.GeneralOpenAIRequest, error) {
	var request model.GeneralOpenAIRequest
	err := common.UnmarshalBodyReusable(req, &request)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func UnmarshalVideoGenerationJobRequest(
	req *http.Request,
) (*model.VideoGenerationJobRequest, error) {
	var request model.VideoGenerationJobRequest
	err := common.UnmarshalBodyReusable(req, &request)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func UnmarshalImageRequest(req *http.Request) (*model.ImageRequest, error) {
	var request model.ImageRequest
	err := common.UnmarshalBodyReusable(req, &request)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func UnmarshalRerankRequest(req *http.Request) (*model.RerankRequest, error) {
	var request model.RerankRequest
	err := common.UnmarshalBodyReusable(req, &request)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func UnmarshalTTSRequest(req *http.Request) (*model.TextToSpeechRequest, error) {
	var request model.TextToSpeechRequest
	err := common.UnmarshalBodyReusable(req, &request)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func UnmarshalMap(req *http.Request) (map[string]any, error) {
	var request map[string]any
	err := common.UnmarshalBodyReusable(req, &request)
	if err != nil {
		return nil, err
	}
	return request, nil
}

var defaultClient = &http.Client{}

func DoRequest(req *http.Request) (*http.Response, error) {
	resp, err := defaultClient.Do(req)
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
