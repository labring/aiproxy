package adaptors

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
	"github.com/stretchr/testify/require"
)

func TestEveryChannelTypeHasModelCatalogBehavior(t *testing.T) {
	for channelType, channelAdaptor := range ChannelAdaptor {
		if modelcatalog.IsUnsupportedType(channelType) {
			continue
		}

		if _, ok := channelAdaptor.(adaptor.ModelCatalogLister); !ok {
			t.Errorf("channel type %s has no provider model catalog lister", channelType)
		}
	}

	for _, channelType := range modelcatalog.UnsupportedTypes() {
		_, ok := ChannelAdaptor[channelType]
		require.Truef(t, ok, "unsupported channel type %s must be registered", channelType)
	}
}

func TestAIProxyGatewayChannelsUseOpenAICompatibleCatalog(t *testing.T) {
	for _, channelType := range []model.ChannelType{
		model.ChannelTypeAIProxyHZH,
		model.ChannelTypeAIProxyUSW1,
	} {
		channelAdaptor, ok := ChannelAdaptor[channelType]
		require.True(t, ok)
		_, ok = channelAdaptor.(adaptor.ModelCatalogLister)
		require.True(t, ok)
	}
}

func TestRetiredLingyiwanwuChannelIsExplicitlyUnsupported(t *testing.T) {
	require.True(t, modelcatalog.IsUnsupportedType(model.ChannelTypeLingyiwanwu))
}
