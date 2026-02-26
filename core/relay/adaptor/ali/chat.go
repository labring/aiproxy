package ali

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

// qwen3 enable_thinking must be set to false for non-streaming calls
func patchQwen3EnableThinking(node *ast.Node) error {
	streamNode := node.Get("stream")
	isStreaming := false

	if streamNode.Exists() {
		streamBool, err := streamNode.Bool()
		if err != nil {
			return errors.New("stream is not a boolean")
		}

		isStreaming = streamBool
	}

	// Set enable_thinking to false for non-streaming requests
	if !isStreaming {
		_, err := node.Set("enable_thinking", ast.NewBool(false))
		return err
	}

	return nil
}

// qwq only support stream mode
func patchQwqOnlySupportStream(node *ast.Node) error {
	_, err := node.Set("stream", ast.NewBool(true))
	return err
}

// https://help.aliyun.com/zh/model-studio/deep-thinking
func patchGeneralThinkingFromNode(node *ast.Node) error {
	request, err := utils.UnmarshalGeneralThinkingFromNode(node)
	if err != nil {
		return err
	}

	if request.Thinking == nil {
		return nil
	}

	switch request.Thinking.Type {
	case relaymodel.ClaudeThinkingTypeEnabled:
		_, err := node.Set("enable_thinking", ast.NewBool(true))
		if err != nil {
			return err
		}

		if request.Thinking.BudgetTokens > 0 {
			_, err = node.Set(
				"thinking_budget",
				ast.NewNumber(strconv.Itoa(request.Thinking.BudgetTokens)),
			)
			if err != nil {
				return err
			}
		}
	case relaymodel.ClaudeThinkingTypeDisabled:
		_, err := node.Set("enable_thinking", ast.NewBool(false))
		if err != nil {
			return err
		}

		_, err = node.Unset("thinking_budget")
		if err != nil {
			return err
		}
	}

	return nil
}

func ConvertCompletionsRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	callbacks := []func(node *ast.Node) error{
		patchGeneralThinkingFromNode,
	}
	if strings.HasPrefix(meta.ActualModel, "qwen3-") {
		callbacks = append(callbacks, patchQwen3EnableThinking)
	}

	if strings.HasPrefix(meta.ActualModel, "qwq-") {
		callbacks = append(callbacks, patchQwqOnlySupportStream)
	}

	return openai.ConvertCompletionsRequest(meta, req, callbacks...)
}

func ConvertChatCompletionsRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	callbacks := []func(node *ast.Node) error{
		patchGeneralThinkingFromNode,
	}
	if strings.HasPrefix(meta.ActualModel, "qwen3-") {
		callbacks = append(callbacks, patchQwen3EnableThinking)
	}

	if strings.HasPrefix(meta.ActualModel, "qwq-") {
		callbacks = append(callbacks, patchQwqOnlySupportStream)
	}

	return openai.ConvertChatCompletionsRequest(meta, req, false, callbacks...)
}

func getEnableSearch(node *ast.Node) bool {
	enableSearch, _ := node.Get("enable_search").Bool()
	return enableSearch
}

func ChatHandler(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, ErrorHanlder(resp)
	}

	node, err := common.UnmarshalRequest2NodeReusable(c.Request)
	if err != nil {
		return adaptor.DoResponseResult{}, relaymodel.WrapperOpenAIErrorWithMessage(
			fmt.Sprintf("get request body failed: %s", err),
			"get_request_body_failed",
			http.StatusInternalServerError,
		)
	}

	result, e := openai.DoResponse(meta, store, c, resp)
	if e != nil {
		return adaptor.DoResponseResult{}, e
	}

	if getEnableSearch(&node) {
		result.Usage.WebSearchCount++
	}

	return result, nil
}
