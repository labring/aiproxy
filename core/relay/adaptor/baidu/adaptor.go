package baidu

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

type Adaptor struct{}

const (
	baseURL = "https://aip.baidubce.com"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

// Get model-specific endpoint using map
var modelEndpointMap = map[string]string{
	"ERNIE-4.0-8K":         "completions_pro",
	"ERNIE-4.0":            "completions_pro",
	"ERNIE-Bot-4":          "completions_pro",
	"ERNIE-Bot":            "completions",
	"ERNIE-Bot-turbo":      "eb-instant",
	"ERNIE-Speed":          "ernie_speed",
	"ERNIE-3.5-8K":         "completions",
	"ERNIE-Bot-8K":         "ernie_bot_8k",
	"ERNIE-Speed-8K":       "ernie_speed",
	"ERNIE-Lite-8K-0922":   "eb-instant",
	"ERNIE-Lite-8K-0308":   "ernie-lite-8k",
	"BLOOMZ-7B":            "bloomz_7b1",
	"bge-large-zh":         "bge_large_zh",
	"bge-large-en":         "bge_large_en",
	"tao-8k":               "tao_8k",
	"bce-reranker-base_v1": "bce_reranker_base",
	"Stable-Diffusion-XL":  "sd_xl",
	"Fuyu-8B":              "fuyu_8b",
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	// Get API path suffix based on mode
	var pathSuffix string
	switch meta.Mode {
	case mode.ChatCompletions:
		pathSuffix = "chat"
	case mode.Embeddings:
		pathSuffix = "embeddings"
	case mode.Rerank:
		pathSuffix = "reranker"
	case mode.ImagesGenerations:
		pathSuffix = "text2image"
	}

	modelEndpoint, ok := modelEndpointMap[meta.ActualModel]
	if !ok {
		modelEndpoint = strings.ToLower(meta.ActualModel)
	}

	// Construct full URL
	fullURL := fmt.Sprintf("%s/rpc/2.0/ai_custom/v1/wenxinworkshop/%s/%s",
		meta.Channel.BaseURL, pathSuffix, modelEndpoint)

	return adaptor.RequestURL{
		Method: http.MethodPost,
		URL:    fullURL,
	}, nil
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("Authorization", "Bearer "+meta.Channel.Key)
	accessToken, err := GetAccessToken(context.Background(), meta.Channel.Key)
	if err != nil {
		return err
	}
	req.URL.RawQuery = "access_token=" + accessToken
	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.Embeddings:
		return openai.ConvertEmbeddingsRequest(meta, req, nil, true)
	case mode.Rerank:
		return openai.ConvertRequest(meta, store, req)
	case mode.ImagesGenerations:
		return openai.ConvertRequest(meta, store, req)
	case mode.ChatCompletions:
		return ConvertRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoRequest(
	_ *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequest(req)
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.Embeddings:
		usage, err = EmbeddingsHandler(meta, c, resp)
	case mode.Rerank:
		usage, err = RerankHandler(meta, c, resp)
	case mode.ImagesGenerations:
		usage, err = ImageHandler(meta, c, resp)
	case mode.ChatCompletions:
		if utils.IsStreamResponse(resp) {
			usage, err = StreamHandler(meta, c, resp)
		} else {
			usage, err = Handler(meta, c, resp)
		}
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			nil,
			http.StatusBadRequest,
		)
	}
	return
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "client_id|client_secret",
		Models:  ModelList,
	}
}
