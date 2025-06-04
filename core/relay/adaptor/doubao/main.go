package doubao

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

func GetRequestURL(meta *meta.Meta) (adaptor.RequestURL, error) {
	u := meta.Channel.BaseURL
	switch meta.Mode {
	case mode.ChatCompletions:
		if strings.HasPrefix(meta.ActualModel, "bot-") {
			return adaptor.RequestURL{
				Method: http.MethodPost,
				URL:    u + "/api/v3/bots/chat/completions",
			}, nil
		}
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api/v3/chat/completions",
		}, nil
	case mode.Embeddings:
		return adaptor.RequestURL{
			Method: http.MethodPost,
			URL:    u + "/api/v3/embeddings",
		}, nil
	default:
		return adaptor.RequestURL{}, fmt.Errorf("unsupported relay mode %d for doubao", meta.Mode)
	}
}

type Adaptor struct {
	openai.Adaptor
}

const baseURL = "https://ark.cn-beijing.volces.com"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Features: []string{
			"Bot support",
			"Network search metering support",
		},
		Models: ModelList,
	}
}

func (a *Adaptor) GetRequestURL(meta *meta.Meta, _ adaptor.Store) (adaptor.RequestURL, error) {
	return GetRequestURL(meta)
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	result, err := a.Adaptor.ConvertRequest(meta, store, req)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}
	if meta.Mode != mode.ChatCompletions || meta.OriginModel != "deepseek-reasoner" {
		return result, nil
	}

	m := make(map[string]any)
	err = sonic.ConfigDefault.NewDecoder(result.Body).Decode(&m)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}
	messages, _ := m["messages"].([]any)
	if len(messages) == 0 {
		return adaptor.ConvertResult{}, errors.New("messages is empty")
	}
	sysMessage := relaymodel.Message{
		Role:    "system",
		Content: "回答前，都先用 <think></think> 输出你的思考过程。",
	}
	messages = append([]any{sysMessage}, messages...)
	m["messages"] = messages
	newBody, err := sonic.Marshal(m)
	if err != nil {
		return adaptor.ConvertResult{}, err
	}

	header := result.Header
	header.Set("Content-Length", strconv.Itoa(len(newBody)))

	return adaptor.ConvertResult{
		Header: header,
		Body:   bytes.NewReader(newBody),
	}, nil
}

func newHandlerPreHandler(websearchCount *int64) func(_ *meta.Meta, node *ast.Node) error {
	return func(meta *meta.Meta, node *ast.Node) error {
		return handlerPreHandler(meta, node, websearchCount)
	}
}

// copy bot_usage.model_usage to usage
func handlerPreHandler(meta *meta.Meta, node *ast.Node, websearchCount *int64) error {
	if !strings.HasPrefix(meta.ActualModel, "bot-") {
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

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (usage model.Usage, err adaptor.Error) {
	switch meta.Mode {
	case mode.ChatCompletions:
		websearchCount := int64(0)
		if utils.IsStreamResponse(resp) {
			usage, err = openai.StreamHandler(meta, c, resp, newHandlerPreHandler(&websearchCount))
		} else {
			usage, err = openai.Handler(meta, c, resp, newHandlerPreHandler(&websearchCount))
		}
		usage.WebSearchCount += model.ZeroNullInt64(websearchCount)
	default:
		return openai.DoResponse(meta, store, c, resp)
	}
	return usage, err
}

func (a *Adaptor) GetBalance(_ *model.Channel) (float64, error) {
	return 0, adaptor.ErrGetBalanceNotImplemented
}
