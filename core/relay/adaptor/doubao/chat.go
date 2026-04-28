package doubao

import (
	"net/http"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/utils"
)

func ConvertChatCompletionsRequest(
	meta *meta.Meta,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	callbacks := []func(node *ast.Node) error{
		func(node *ast.Node) error {
			reasoning, err := utils.ParseOpenAIReasoningFromNode(node)
			if err != nil {
				return err
			}

			return utils.ApplyReasoningToDoubaoNode(node, reasoning)
		},
	}

	if utils.FirstMatchingModelName(
		meta.OriginModel,
		meta.ActualModel,
		func(modelName string) bool {
			return strings.HasPrefix(strings.ToLower(modelName), "deepseek-reasoner")
		},
	) != "" {
		callbacks = append(callbacks, patchDeepseekReasonerSystemPrompt)
	}

	return openai.ConvertChatCompletionsRequest(
		meta,
		req,
		false,
		callbacks...,
	)
}

func patchDeepseekReasonerSystemPrompt(node *ast.Node) error {
	messagesNode := node.Get("messages")
	if messagesNode.Check() != nil {
		return nil
	}

	sysMessage := ast.NewObject([]ast.Pair{
		ast.NewPair("role", ast.NewString("system")),
		ast.NewPair("content", ast.NewString("回答前，都先用 <think></think> 输出你的思考过程。")),
	})

	nodes, err := messagesNode.ArrayUseNode()
	if err != nil {
		return err
	}

	newMessages := make([]ast.Node, 0, len(nodes)+1)
	newMessages = append(newMessages, sysMessage)
	newMessages = append(newMessages, nodes...)

	*messagesNode = ast.NewArray(newMessages)

	return nil
}

func newHandlerPreHandler(websearchCount *int64) func(_ *meta.Meta, node *ast.Node) error {
	return func(meta *meta.Meta, node *ast.Node) error {
		return handlerPreHandler(meta, node, websearchCount)
	}
}

// copy bot_usage.model_usage to usage
func handlerPreHandler(meta *meta.Meta, node *ast.Node, websearchCount *int64) error {
	if !strings.HasPrefix(strings.ToLower(featureModel(meta)), "bot-") {
		return nil
	}

	botUsageNode := node.Get("bot_usage")
	if botUsageNode.Check() != nil {
		return nil
	}

	modelUsageNode := botUsageNode.Get("model_usage").Index(0)
	if modelUsageNode.Check() != nil {
		return nil
	}

	_, err := node.SetAny("usage", modelUsageNode)
	if err != nil {
		return err
	}

	actionUsageNodes := botUsageNode.Get("action_usage")
	if actionUsageNodes.Check() != nil {
		return nil
	}

	return actionUsageNodes.ForEach(func(_ ast.Sequence, node *ast.Node) bool {
		if node.Check() != nil {
			return true
		}

		count, err := node.Get("count").Int64()
		if err != nil {
			return true
		}

		*websearchCount += count

		return true
	})
}
