package deepseek

import (
	"context"
	"net/url"
	"strings"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
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

	baseURL := strings.TrimSpace(channel.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, modelcatalog.ErrInvalidConfiguration
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(basePath, "/v1") {
		parsed.Path = strings.TrimSuffix(basePath, "/v1")
		parsed.RawPath = ""
	}
	parsed.RawQuery = ""

	catalogChannel := *channel
	catalogChannel.BaseURL = parsed.String()
	return modelcatalog.OpenAICompatibleModels(ctx, &catalogChannel, catalogChannel.BaseURL, "models")
}
