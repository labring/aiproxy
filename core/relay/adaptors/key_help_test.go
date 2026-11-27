package adaptors_test

import (
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptors"
)

func TestChannelMetasKeyHelpFormat(t *testing.T) {
	t.Parallel()

	expected := map[model.ChannelType]string{
		model.ChannelTypeTokenDance:  "sk-xxx",
		model.ChannelTypeQwenCloud:   "sk-xxx",
		model.ChannelTypeAIProxyHZH:  "sk-xxx",
		model.ChannelTypeAIProxyUSW1: "sk-xxx",
		model.ChannelTypeVoyage:      "pa-xxx",
	}

	for channelType, want := range expected {
		meta, ok := adaptors.ChannelMetas[channelType]
		if !ok {
			t.Fatalf("missing channel meta for %s (%d)", channelType.String(), channelType)
		}
		if meta.KeyHelp != want {
			t.Errorf("%s keyHelp = %q, want %q", channelType.String(), meta.KeyHelp, want)
		}
	}

	for channelType, meta := range adaptors.ChannelMetas {
		if strings.Contains(meta.KeyHelp, "http://") || strings.Contains(meta.KeyHelp, "https://") {
			t.Errorf("%s keyHelp should describe key format without URLs, got %q", channelType.String(), meta.KeyHelp)
		}
	}
}
