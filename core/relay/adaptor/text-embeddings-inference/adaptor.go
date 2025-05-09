package textembeddingsinference

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

// text-embeddings-inference adaptor supports rerank and embeddings models deployed by https://github.com/huggingface/text-embeddings-inference
type Adaptor struct {
	openai.Adaptor
}

// base url for text-embeddings-inference, fake
const baseURL = "https://api.text-embeddings.net/v1"

func (a *Adaptor) GetBaseURL() string {
	return baseURL
}

func (a *Adaptor) GetModelList() []*model.ModelConfig {
	return ModelList
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
	switch meta.Mode {
	case mode.Rerank:
		return meta.Channel.BaseURL + "/rerank", nil
	case mode.Embeddings:
		return meta.Channel.BaseURL + "/embeddings", nil
	default:
		return "", fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

// text-embeddings-inference api see https://huggingface.github.io/text-embeddings-inference/#/Text%20Embeddings%20Inference/rerank

func (a *Adaptor) SetupRequestHeader(meta *meta.Meta, _ *gin.Context, req *http.Request) error {
	switch meta.Mode {
	case mode.Rerank:
		req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)
	case mode.Embeddings:
		req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)
	default:
		return fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
	return nil
}

func (a *Adaptor) ConvertRequest(meta *meta.Meta, req *http.Request) (string, http.Header, io.Reader, error) {
	switch meta.Mode {
	case mode.Rerank:
		// Parse request body into AST node
		node, err := common.UnmarshalBody2Node(req)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to parse request body: %w", err)
		}

		// Set the actual model in the request
		_, err = node.Set("model", ast.NewString(meta.ActualModel))
		if err != nil {
			return "", nil, nil, err
		}

		// Get the documents array and rename it to texts
		documentsNode := node.Get("documents")
		if !documentsNode.Exists() {
			return "", nil, nil, errors.New("documents field not found")
		}

		// Set the texts field with the documents value
		_, err = node.Set("texts", *documentsNode)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to set texts field: %w", err)
		}

		// Remove the documents field
		_, err = node.Unset("documents")
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to remove documents field: %w", err)
		}

		// Convert back to JSON
		jsonData, err := node.MarshalJSON()
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		return http.MethodPost, nil, bytes.NewReader(jsonData), nil
	case mode.Embeddings:
		// Handle embeddings request
		return http.MethodPost, nil, req.Body, nil
	default:
		return "", nil, nil, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoRequest(_ *meta.Meta, _ *gin.Context, req *http.Request) (*http.Response, error) {
	return utils.DoRequest(req)
}

type RerankResponse struct {
	Error     string `json:"error"`
	ErrorType string `json:"error_type"`
}

// handleErrorResponse handle error response
func handleErrorResponse(resp *http.Response, statusCode int) *relaymodel.ErrorWithStatusCode {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	defer resp.Body.Close()
	var errResp RerankResponse
	if err := sonic.Unmarshal(body, &errResp); err != nil {
		return openai.ErrorWrapper(err, "unmarshal_error_response_failed", http.StatusInternalServerError)
	}
	return openai.ErrorWrapperWithMessage(errResp.Error, "upstream_"+errResp.ErrorType, statusCode)
}

func (a *Adaptor) DoResponse(meta *meta.Meta, c *gin.Context, resp *http.Response) (*model.Usage, *relaymodel.ErrorWithStatusCode) {
	usage := &model.Usage{}
	errorWithStatusCode := &relaymodel.ErrorWithStatusCode{}

	switch meta.Mode {
	case mode.Rerank:
		// handle different status codes
		switch resp.StatusCode {

		// calculate usage
		case http.StatusOK:
			usage = &meta.RequestUsage

		// see https://huggingface.github.io/text-embeddings-inference/#/Text%20Embeddings%20Inference/rerank
		case http.StatusRequestEntityTooLarge, // 413
			http.StatusUnprocessableEntity, // 422
			http.StatusFailedDependency,    // 424
			http.StatusTooManyRequests:     // 429

			errorWithStatusCode = handleErrorResponse(resp, resp.StatusCode)
		default:
			errorWithStatusCode = openai.ErrorWrapperWithMessage(
				fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
				"unexpected_status_code",
				resp.StatusCode,
			)
		}
	case mode.Embeddings:
		// OpenAl compatible route. Returns a 424 status code if the moddel is not an embedding model.
		if utils.IsStreamResponse(resp) {
			usage, errorWithStatusCode = openai.StreamHandler(meta, c, resp, nil)
		} else {
			usage, errorWithStatusCode = openai.Handler(meta, c, resp, nil)
		}
	}
	return usage, errorWithStatusCode
}
