package fakeerror

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/fake"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Adaptor = (*Adaptor)(nil)

type Adaptor struct {
	fake.Adaptor
	configCache utils.ChannelConfigCache[Config]
}

type Config struct {
	ErrorStatusCode int    `json:"error_status_code"`
	ErrorMessage    string `json:"error_message"`
	ErrorCode       string `json:"error_code"`
}

func init() {
	registry.Register(model.ChannelTypeFakeError, &Adaptor{})
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	metaInfo := a.Adaptor.Metadata()
	metaInfo.Readme = "Fake error adaptor for protocol debugging and profiling. It reuses fake request conversion and fake upstream responses, but always returns an immediate synthetic error in DoResponse."
	metaInfo.ConfigSchema = configSchema()
	return metaInfo
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	cfg := a.loadConfig(meta)

	return adaptor.DoResponseResult{}, relaymodel.WrapperErrorWithMessage(
		meta.Mode,
		cfg.ErrorStatusCode,
		cfg.ErrorMessage,
		relaymodel.WithType(relaymodel.ErrorTypeUpstream),
		relaymodel.WithCode(cfg.ErrorCode),
	)
}

func (a *Adaptor) loadConfig(meta *meta.Meta) Config {
	cfg := Config{
		ErrorStatusCode: http.StatusBadRequest,
		ErrorMessage:    "fake error",
		ErrorCode:       "fake_error",
	}

	if meta == nil {
		return cfg
	}

	loaded, err := a.configCache.Load(meta, cfg)
	if err == nil {
		cfg = loaded
	}

	if cfg.ErrorStatusCode <= 0 {
		cfg.ErrorStatusCode = http.StatusBadRequest
	}

	if cfg.ErrorMessage == "" {
		cfg.ErrorMessage = "fake error"
	}

	if cfg.ErrorCode == "" {
		cfg.ErrorCode = "fake_error"
	}

	return cfg
}

func configSchema() map[string]any {
	return map[string]any{
		"type":  "object",
		"title": "Fake Error Adaptor Config",
		"properties": map[string]any{
			"error_status_code": map[string]any{
				"type":        "integer",
				"title":       "Error Status Code",
				"description": "HTTP status code returned by the synthetic fake error response.",
			},
			"error_message": map[string]any{
				"type":        "string",
				"title":       "Error Message",
				"description": "Error message returned by the synthetic fake error response.",
			},
			"error_code": map[string]any{
				"type":        "string",
				"title":       "Error Code",
				"description": "Error code returned by the synthetic fake error response.",
			},
		},
	}
}
