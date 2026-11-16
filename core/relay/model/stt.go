package model

import "github.com/labring/aiproxy/core/model"

type SttJSONResponse struct {
	Text string `json:"text,omitempty"`
}

type SttVerboseJSONResponse struct {
	Task     string     `json:"task,omitempty"`
	Language string     `json:"language,omitempty"`
	Text     string     `json:"text,omitempty"`
	Segments []*Segment `json:"segments,omitempty"`
	Duration float64    `json:"duration,omitempty"`
	Usage    *SttUsage  `json:"usage,omitempty"`
}

type SttSSEResponseType = string

const (
	SttSSEResponseTypeTranscriptTextDelta SttSSEResponseType = "transcript.text.delta"
	SttSSEResponseTypeTranscriptTextDone  SttSSEResponseType = "transcript.text.done"
)

type SttSSEResponse struct {
	Type  SttSSEResponseType `json:"type,omitempty"`
	Delta string             `json:"delta,omitempty"`
	Text  string             `json:"text,omitempty"`
	Usage *SttUsage          `json:"usage,omitempty"`
}

type Segment struct {
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

type SttUsageType = string

const (
	SttUsageTypeTokens   SttUsageType = "tokens"
	SttUsageTypeDuration SttUsageType = "duration"
)

type SttUsage struct {
	Type              SttUsageType               `json:"type,omitempty"`
	Seconds           int64                      `json:"seconds,omitempty"`
	InputTokens       int64                      `json:"input_tokens,omitempty"`
	OutputTokens      int64                      `json:"output_tokens,omitempty"`
	TotalTokens       int64                      `json:"total_tokens,omitempty"`
	InputTokenDetails *SttUsageInputTokenDetails `json:"input_token_details,omitempty"`
}

type SttUsageInputTokenDetails struct {
	TextTokens  int64 `json:"text_tokens"`
	AudioTokens int64 `json:"audio_tokens"`
}

func (u *SttUsage) ToModelUsage() model.Usage {
	switch u.Type {
	case SttUsageTypeDuration:
		return model.Usage{
			InputTokens: model.ZeroNullInt64(u.Seconds),
			TotalTokens: model.ZeroNullInt64(u.Seconds),
		}
	default:
		modelUsage := model.Usage{
			InputTokens:  model.ZeroNullInt64(u.InputTokens),
			OutputTokens: model.ZeroNullInt64(u.OutputTokens),
			TotalTokens:  model.ZeroNullInt64(u.TotalTokens),
		}
		if u.InputTokenDetails != nil {
			modelUsage.AudioInputTokens = model.ZeroNullInt64(u.InputTokenDetails.AudioTokens)
		}

		return modelUsage
	}
}
