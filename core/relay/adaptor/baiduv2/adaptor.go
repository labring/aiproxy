package baiduv2

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
	baseURL = "https://qianfan.baidubce.com/v2"
)

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Fm2vrveyu
var v2ModelMap = map[string]string{
	"ERNIE-Character-8K":         "ernie-char-8k",
	"ERNIE-Character-Fiction-8K": "ernie-char-fiction-8k",
}

func toV2ModelName(modelName string) string {
	if v2Model, ok := v2ModelMap[modelName]; ok {
		return v2Model
	}
	return strings.ToLower(modelName)
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    meta.Channel.BaseURL + "/chat/completions",
		}, nil
	case mode.Rerank:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    meta.Channel.BaseURL + "/rerankers",
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	token, err := GetBearerToken(context.Background(), meta.Channel.Key)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.ChatCompletions, mode.Rerank:
		actModel := meta.ActualModel
		v2Model := toV2ModelName(actModel)
		if v2Model != actModel {
			meta.ActualModel = v2Model
			defer func() { meta.ActualModel = actModel }()
		}
		return openai.ConvertRequest(meta, store, req)
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
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.ChatCompletions, mode.Rerank:
		return openai.DoResponse(meta, store, c, resp)
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			nil,
			http.StatusBadRequest,
		)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "ak|sk",
		Models:  ModelList,
	}
}
