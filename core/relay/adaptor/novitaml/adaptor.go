// Package novitaml is the adaptor for Novita native multimodal API endpoints.
// It mirrors ppioml (PPIO multimodal) since Novita shares the same API
// structure (domain: api.novita.ai instead of api.ppinfra.com).
//
// Handles non-OpenAI image/video/audio endpoints where the model ID is
// embedded in the URL path:
//   - Sync:  POST /v3/{model-id}
//   - Async: POST /v3/async/{model-id}
//   - Task:  GET  /v3/async/task-result?task_id=<id>
//
// Request and response bodies are forwarded verbatim (native format).
// Set channel BaseURL to "https://api.novita.ai".
package novitaml

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/passthrough"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/mode"
)

func init() {
	registry.Register(model.ChannelTypeNovitaMultimodal, &Adaptor{})
}

var _ adaptor.Adaptor = (*Adaptor)(nil)

// Adaptor handles Novita native multimodal endpoints via transparent byte forwarding.
type Adaptor struct {
	passthrough.Adaptor
}

const defaultBaseURL = "https://api.novita.ai"

func (a *Adaptor) DefaultBaseURL() string {
	return defaultBaseURL
}

// SupportMode returns true only for PPIONative. Novita multimodal endpoints
// use the same /v3/* path structure as PPIO.
func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.PPIONative
}

// NativeMode mirrors SupportMode: PPIONative requests are always native here.
func (a *Adaptor) NativeMode(m mode.Mode) bool {
	return m == mode.PPIONative
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "Novita AI 多模态 API\n" +
			"支持图像生成、视频生成、音频生成等多模态模型\n" +
			"请求体使用 Novita 原生格式（非 OpenAI 格式）\n" +
			"文档：https://novita.ai/docs",
		ConfigSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				model.ChannelConfigAllowPassthroughUnknown: map[string]any{
					"type":        "boolean",
					"title":       "透传未知模型 (allow_passthrough_unknown)",
					"description": "开启后，此渠道可作为兜底路由，将未注册的多模态模型直接转发到上游，首次成功后自动注册模型和价格。",
				},
			},
		},
	}
}
