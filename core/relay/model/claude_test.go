package model_test

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/model"
	"github.com/smartystreets/goconvey/convey"
)

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
