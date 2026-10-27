package geminiopenai

import (
	"context"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/gemini"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
)

func (a *Adaptor) ListModels(
	ctx context.Context,
	channel *model.Channel,
	defaultBaseURL string,
) ([]adaptor.ModelCatalogItem, error) {
	if channel == nil {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	baseURL := strings.TrimRight(strings.TrimSpace(channel.BaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(defaultBaseURL), "/")
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1beta/openai")
	if baseURL == "" {
		return nil, modelcatalog.ErrInvalidConfiguration
	}

	catalogChannel := *channel
	catalogChannel.BaseURL = baseURL
	return (&gemini.Adaptor{}).ListModels(ctx, &catalogChannel, baseURL)
}
