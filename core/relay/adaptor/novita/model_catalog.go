package novita

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
	for _, suffix := range []string{"/v3/openai", "/openai/v1", "/openai"} {
		if strings.HasSuffix(basePath, suffix) {
			parsed.Path = strings.TrimSuffix(basePath, suffix)
			parsed.RawPath = ""
			break
		}
	}
	parsed.RawQuery = ""

	catalogChannel := *channel
	catalogChannel.BaseURL = parsed.String()
	return modelcatalog.OpenAICompatibleModels(ctx, &catalogChannel, catalogChannel.BaseURL, "openai", "v1", "models")
}
