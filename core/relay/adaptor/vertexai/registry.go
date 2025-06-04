package vertexai

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	vertexclaude "github.com/labring/aiproxy/core/relay/adaptor/vertexai/claude"
	vertexgemini "github.com/labring/aiproxy/core/relay/adaptor/vertexai/gemini"
	"github.com/labring/aiproxy/core/relay/meta"
)

type ModelType int

const (
	VerterAIClaude ModelType = iota + 1
	VerterAIGemini
)

var modelList = []model.ModelConfig{}

func init() {
	modelList = append(modelList, vertexclaude.ModelList...)

	modelList = append(modelList, gemini.ModelList...)
}

type innerAIAdapter interface {
	ConvertRequest(
		meta *meta.Meta,
		store adaptor.Store,
		request *http.Request,
	) (adaptor.ConvertResult, error)
	DoResponse(
		meta *meta.Meta,
		store adaptor.Store,
		c *gin.Context,
		resp *http.Response,
	) (usage model.Usage, err adaptor.Error)
}

func GetAdaptor(model string) innerAIAdapter {
	switch {
	case strings.Contains(model, "claude"):
		return &vertexclaude.Adaptor{}
	case strings.Contains(model, "gemini"):
		return &vertexgemini.Adaptor{}
	default:
		return nil
	}
}
