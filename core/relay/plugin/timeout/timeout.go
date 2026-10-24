package timeout

import (
	"net/http"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/noop"
)

var _ plugin.Plugin = (*Timeout)(nil)

type Timeout struct {
	noop.Noop
}

func NewTimeoutPlugin() plugin.Plugin {
	return &Timeout{}
}

func (t *Timeout) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
	do adaptor.ConvertRequest,
) (adaptor.ConvertResult, error) {
	var stream bool
	switch meta.Mode {
	case mode.Embeddings:
		meta.RequestTimeout = time.Second * 30
	case mode.Moderations:
		meta.RequestTimeout = time.Minute * 5
	case mode.ImagesGenerations,
		mode.ImagesEdits:
		meta.RequestTimeout = time.Minute * 10
	case mode.AudioTranscription,
		mode.AudioTranslation:
		meta.RequestTimeout = time.Minute * 5
	case mode.Rerank, mode.Decisions:
		meta.RequestTimeout = time.Minute * 3
	case mode.ParsePdf:
		meta.RequestTimeout = time.Minute * 3
	case mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.Videos,
		mode.VideosGet,
		mode.VideosContent,
		mode.VideosDelete,
		mode.VideosRemix,
		mode.VideosEdits,
		mode.VideosExtensions:
		meta.RequestTimeout = time.Second * 30
	case mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		meta.RequestTimeout = time.Second * 30
	case mode.ChatCompletions,
		mode.Completions,
		mode.Responses,
		mode.Anthropic:
		stream, _ = isStream(req)

		inputTokens := meta.RequestUsage.InputTokens
		if stream {
			switch {
			case inputTokens > 100*1024:
				meta.RequestTimeout = time.Minute * 10
			case inputTokens > 10*1024:
				meta.RequestTimeout = time.Minute * 5
			default:
				meta.RequestTimeout = time.Minute * 3
			}
		} else {
			switch {
			case inputTokens > 100*1024:
				meta.RequestTimeout = time.Minute * 15
			case inputTokens > 10*1024:
				meta.RequestTimeout = time.Minute * 10
			default:
				meta.RequestTimeout = time.Minute * 5
			}
		}
	default:
		if common.IsJSONContentType(req.Header.Get("Content-Type")) {
			stream, _ = isStream(req)
			if stream {
				meta.RequestTimeout = time.Minute * 3
			} else {
				meta.RequestTimeout = time.Minute * 15
			}
		}
	}

	if timeout := meta.ModelConfig.RequestTimeout(); timeout != 0 {
		meta.RequestTimeout = timeout
	}

	if stream {
		if timeout := meta.ModelConfig.StreamRequestTimeout(); timeout != 0 {
			meta.RequestTimeout = timeout
		}
	}

	if meta.RequestTimeout != 0 {
		log := common.GetLoggerFromReq(req)
		log.Data["req_timeout"] = common.TruncateDuration(meta.RequestTimeout).String()
	}

	return do.ConvertRequest(meta, store, req)
}

func isStream(req *http.Request) (bool, error) {
	body, err := common.GetRequestBodyReusable(req)
	if err != nil {
		return false, nil
	}

	node, err := common.GetJSONNodeNoCopy(body, "stream")
	if err != nil {
		return false, err
	}

	return node.Bool()
}
