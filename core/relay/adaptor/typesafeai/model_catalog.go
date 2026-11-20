package typesafeai

import (
	"context"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
)

var _ adaptor.ModelCatalogLister = (*Adaptor)(nil)

func (a *Adaptor) ListModels(
	ctx context.Context,
	channel *model.Channel,
	defaultBaseURL string,
) ([]adaptor.ModelCatalogItem, error) {
	return modelcatalog.OpenAIDocumentedModels(ctx, channel, defaultBaseURL)
}
