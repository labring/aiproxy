package minimax

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://api.minimax.chat/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"Chat、Embeddings、TTS(need group id) Support",
		},
		KeyHelp: "api_key|group_id",
		Models:  ModelList,
	}
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	apiKey, _, err := GetAPIKeyAndGroupID(meta.Channel.Key)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return nil
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, store adaptor.Store) (adaptor.RequestURL, error) {
	_, groupID, err := GetAPIKeyAndGroupID(meta.Channel.Key)
	if err != nil {
		return adaptor.RequestURL{}, err
	}
	switch meta.Mode {
	case mode.ChatCompletions:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    meta.Channel.BaseURL + "/text/chatcompletion_v2",
		}, nil
	case mode.Embeddings:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s/embeddings?GroupId=%s", meta.Channel.BaseURL, groupID),
		}, nil
	case mode.AudioSpeech:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    fmt.Sprintf("%s/t2a_v2?GroupId=%s", meta.Channel.BaseURL, groupID),
		}, nil
	default:
		return a.Adaptor.GetRequestURL(meta, store)
	}
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		return openai.ConvertChatCompletionsRequest(meta, req, nil, true)
	case mode.AudioSpeech:
		return ConvertTTSRequest(meta, req)
	default:
		return a.Adaptor.ConvertRequest(meta, store, req)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.AudioSpeech:
		return TTSHandler(meta, c, resp)
	default:
		return a.Adaptor.DoResponse(meta, store, c, resp)
	}
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}
