package model_test

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/model"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeRequestSystemPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		want    []model.ClaudeContent
		wantErr bool
	}{
		{
			name: "string",
			body: `{"system":"You are a helpful assistant."}`,
			want: []model.ClaudeContent{{
				Type: model.ClaudeContentTypeText,
				Text: "You are a helpful assistant.",
			}},
		},
		{
			name: "escaped string with whitespace",
			body: "{\"system\": \" 你好\\n\\\"assistant\\\" \\u263a \" }",
			want: []model.ClaudeContent{{
				Type: model.ClaudeContentTypeText,
				Text: " 你好\n\"assistant\" ☺ ",
			}},
		},
		{
			name: "content blocks preserve cache control",
			body: `{"system":[
				{"type":"text","text":"First instruction","cache_control":{"type":"ephemeral","ttl":"1h"}},
				{"type":"text","text":"Second instruction"}
			]}`,
			want: []model.ClaudeContent{
				{
					Type: model.ClaudeContentTypeText,
					Text: "First instruction",
					CacheControl: &model.ClaudeCacheControl{
						Type: "ephemeral",
						TTL:  "1h",
					},
				},
				{Type: model.ClaudeContentTypeText, Text: "Second instruction"},
			},
		},
		{name: "empty string", body: `{"system":""}`},
		{name: "null", body: `{"system":null}`},
		{name: "omitted", body: `{}`},
		{name: "empty array", body: `{"system":[]}`, want: []model.ClaudeContent{}},
		{name: "number", body: `{"system":42}`, wantErr: true},
		{name: "boolean", body: `{"system":true}`, wantErr: true},
		{name: "object", body: `{"system":{"type":"text","text":"Invalid"}}`, wantErr: true},
		{name: "invalid content block", body: `{"system":[true]}`, wantErr: true},
		{name: "malformed string", body: `{"system":"unterminated}`, wantErr: true},
	}
	decoders := map[string]func([]byte, any) error{
		"encoding/json": json.Unmarshal,
		"sonic":         sonic.Unmarshal,
	}

	for decoderName, decode := range decoders {
		t.Run(decoderName, func(t *testing.T) {
			t.Parallel()

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					t.Run("ClaudeRequest", func(t *testing.T) {
						t.Parallel()

						var request model.ClaudeRequest

						err := decode([]byte(tt.body), &request)
						if tt.wantErr {
							require.Error(t, err)
							return
						}

						require.NoError(t, err)
						assert.Equal(t, tt.want, []model.ClaudeContent(request.System))
					})
					t.Run("ClaudeAnyContentRequest", func(t *testing.T) {
						t.Parallel()

						var request model.ClaudeAnyContentRequest

						err := decode([]byte(tt.body), &request)
						if tt.wantErr {
							require.Error(t, err)
							return
						}

						require.NoError(t, err)
						assert.Equal(t, tt.want, []model.ClaudeContent(request.System))
					})
				})
			}
		})
	}
}

func TestClaudeUsage(t *testing.T) {
	convey.Convey("ClaudeUsage", t, func() {
		convey.Convey("ToOpenAIUsage", func() {
			u := model.ClaudeUsage{
				InputTokens:              10,
				OutputTokens:             20,
				CacheCreationInputTokens: 5,
				CacheReadInputTokens:     3,
				ServerToolUse: &model.ClaudeServerToolUse{
					WebSearchRequests: 2,
				},
			}

			usage := u.ToOpenAIUsage()
			// PromptTokens = Input + Read + Creation = 10 + 3 + 5 = 18
			convey.So(usage.PromptTokens, convey.ShouldEqual, 18)
			convey.So(usage.CompletionTokens, convey.ShouldEqual, 20)
			convey.So(usage.TotalTokens, convey.ShouldEqual, 38)
			convey.So(usage.WebSearchCount, convey.ShouldEqual, 2)
			convey.So(usage.PromptTokensDetails.CachedTokens, convey.ShouldEqual, 3)
			convey.So(usage.PromptTokensDetails.CacheCreationTokens, convey.ShouldEqual, 5)
		})

		convey.Convey("ToOpenAIUsage without details", func() {
			u := model.ClaudeUsage{
				InputTokens:  10,
				OutputTokens: 20,
			}
			usage := u.ToOpenAIUsage()
			convey.So(usage.PromptTokens, convey.ShouldEqual, 10)
			convey.So(usage.CompletionTokens, convey.ShouldEqual, 20)
			convey.So(usage.TotalTokens, convey.ShouldEqual, 30)
			convey.So(usage.PromptTokensDetails.CachedTokens, convey.ShouldEqual, 0)
		})
	})
}

func TestClaudeCacheControl(t *testing.T) {
	convey.Convey("ClaudeCacheControl", t, func() {
		convey.Convey("ResetTTL", func() {
			cc := &model.ClaudeCacheControl{
				Type: "ephemeral",
				TTL:  "5m",
			}
			cc.ResetTTL()
			convey.So(cc.TTL, convey.ShouldEqual, "")
			convey.So(cc.Type, convey.ShouldEqual, "ephemeral")
		})

		convey.Convey("ResetTTL nil", func() {
			var cc *model.ClaudeCacheControl

			res := cc.ResetTTL()
			convey.So(res, convey.ShouldBeNil)
		})
	})
}
