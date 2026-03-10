package model_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/labring/aiproxy/core/relay/model"
	"github.com/smartystreets/goconvey/convey"
)

func TestChatUsage(t *testing.T) {
	convey.Convey("ChatUsage", t, func() {
		convey.Convey("ToModelUsage", func() {
			u := model.ChatUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
				WebSearchCount:   5,
				PromptTokensDetails: &model.PromptTokensDetails{
					CachedTokens:        5,
					CacheCreationTokens: 2,
				},
				CompletionTokensDetails: &model.CompletionTokensDetails{
					ReasoningTokens: 10,
				},
			}

			modelUsage := u.ToModelUsage()
			convey.So(int64(modelUsage.InputTokens), convey.ShouldEqual, 10)
			convey.So(int64(modelUsage.OutputTokens), convey.ShouldEqual, 20)
			convey.So(int64(modelUsage.TotalTokens), convey.ShouldEqual, 30)
			convey.So(int64(modelUsage.WebSearchCount), convey.ShouldEqual, 5)
			convey.So(int64(modelUsage.CachedTokens), convey.ShouldEqual, 5)
			convey.So(int64(modelUsage.CacheCreationTokens), convey.ShouldEqual, 2)
			convey.So(int64(modelUsage.ReasoningTokens), convey.ShouldEqual, 10)
		})

		convey.Convey("Add", func() {
			u1 := model.ChatUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
				PromptTokensDetails: &model.PromptTokensDetails{
					CachedTokens: 5,
				},
			}
			u2 := model.ChatUsage{
				PromptTokens:     5,
				CompletionTokens: 5,
				TotalTokens:      10,
				PromptTokensDetails: &model.PromptTokensDetails{
					CachedTokens: 2,
				},
			}

			u1.Add(&u2)
			convey.So(u1.PromptTokens, convey.ShouldEqual, 15)
			convey.So(u1.CompletionTokens, convey.ShouldEqual, 25)
			convey.So(u1.TotalTokens, convey.ShouldEqual, 40)
			convey.So(u1.PromptTokensDetails.CachedTokens, convey.ShouldEqual, 7)

			// Add nil
			u1.Add(nil)
			convey.So(u1.TotalTokens, convey.ShouldEqual, 40)
		})

		convey.Convey("ToClaudeUsage", func() {
			u := model.ChatUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				PromptTokensDetails: &model.PromptTokensDetails{
					CachedTokens:        5,
					CacheCreationTokens: 2,
				},
			}

			cu := u.ToClaudeUsage()
			convey.So(cu.InputTokens, convey.ShouldEqual, 10)
			convey.So(cu.OutputTokens, convey.ShouldEqual, 20)
			convey.So(cu.CacheReadInputTokens, convey.ShouldEqual, 5)
			convey.So(cu.CacheCreationInputTokens, convey.ShouldEqual, 2)
		})
	})
}

func TestOpenAIError(t *testing.T) {
	convey.Convey("OpenAIError", t, func() {
		convey.Convey("NewOpenAIError", func() {
			err := model.OpenAIError{
				Message: "test error",
				Type:    "test_type",
				Code:    "test_code",
			}
			resp := model.NewOpenAIError(http.StatusBadRequest, err)
			convey.So(resp.StatusCode(), convey.ShouldEqual, http.StatusBadRequest)
			// The Error field is unexported or nested, but NewOpenAIError returns adaptor.Error interface (or struct?)
			// Let's check what adaptor.Error exposes.
			// It usually exposes Error() string.
		})

		convey.Convey("WrapperOpenAIError", func() {
			err := errors.New("base error")
			resp := model.WrapperOpenAIError(err, "code_123", http.StatusInternalServerError)
			convey.So(resp.StatusCode(), convey.ShouldEqual, http.StatusInternalServerError)
		})
	})
}
