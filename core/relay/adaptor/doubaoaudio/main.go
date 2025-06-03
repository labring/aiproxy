package doubaoaudio

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func GetRequestURL(meta *meta.Meta) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL
	switch meta.Mode {
	case mode.AudioSpeech:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api/v1/tts/ws_binary",
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

type Adaptor struct{}

const baseURL = "https://openspeech.bytedance.com"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"https://www.volcengine.com/docs/6561/1257543",
			"TTS support",
		},
		KeyHelp: "app_id|app_token",
		Models:  ModelList,
	}
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	return GetRequestURL(meta)
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.AudioSpeech:
		return ConvertTTSRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) SetupRequestHeader(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	switch meta.Mode {
	case mode.AudioSpeech:
		_, token, err := getAppIDAndToken(meta.Channel.Key)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer;"+token)
		return nil
	default:
		return fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoRequest(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	switch meta.Mode {
	case mode.AudioSpeech:
		return TTSDoRequest(meta, req)
	default:
		return nil, fmt.Errorf("unsupported mode: %s", meta.Mode)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	switch meta.Mode {
	case mode.AudioSpeech:
		return TTSDoResponse(meta, c, resp)
	default:
		return model.Usage{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("unsupported mode: %s", meta.Mode),
			nil,
			http.StatusBadRequest,
		)
	}
}
