// Package ppioml is the adaptor for PPIO native multimodal API endpoints.
// Unlike the standard PPIO adaptor (type=54, OpenAI-compatible), this adaptor
// handles PPIO's non-OpenAI image/video/audio endpoints where the model ID is
// embedded in the URL path:
//   - Sync:  POST /v3/{model-id}       (e.g. Seedream 4.0/4.5/5.0-lite)
//   - Async: POST /v3/async/{model-id} (e.g. 即梦, Wan, Kling, Minimax video)
//   - Task:  GET  /v3/async/task-result?task_id=<id>
//
// The request and response bodies are forwarded verbatim (PPIO native format).
// Set channel BaseURL to "https://api.ppinfra.com" — the passthrough adaptor
// appends the request path directly, so /v3/seedream-5.0-lite becomes
// https://api.ppinfra.com/v3/seedream-5.0-lite automatically.
package ppioml

import (
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/passthrough"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/mode"
)

func init() {
	registry.Register(model.ChannelTypePPIOMultimodal, &Adaptor{})
}

var _ adaptor.Adaptor = (*Adaptor)(nil)

// Adaptor handles PPIO native multimodal endpoints via transparent byte forwarding.
type Adaptor struct {
	passthrough.Adaptor
}

const defaultBaseURL = "https://api.ppinfra.com"

func (a *Adaptor) DefaultBaseURL() string {
	return defaultBaseURL
}

// SupportMode returns true only for PPIONative. This ensures the channel
// routing system selects this adaptor exclusively for /v3/* requests, and
// not for any standard OpenAI-protocol modes.
func (a *Adaptor) SupportMode(m mode.Mode) bool {
	return m == mode.PPIONative
}

// NativeMode mirrors SupportMode: PPIONative requests are always native here.
func (a *Adaptor) NativeMode(m mode.Mode) bool {
	return m == mode.PPIONative
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		KeyHelp: "PPIO API Key，从 https://ppinfra.com 控制台获取",
		Readme: "PPIO 派欧云多模态 API\n" +
			"支持图像生成（Seedream 系列、即梦、Qwen-Image 等）、视频生成（Wan、Kling、Minimax、Vidu 等）\n" +
			"请求体使用 PPIO 原生格式（非 OpenAI 格式）\n" +
			"文档：https://ppio.com/docs/models",
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
