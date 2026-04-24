//nolint:testpackage
package baidu

import (
	"net/http"
	"testing"

	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

func TestGetRequestURL_FallsBackToActualModelWhenOriginAliasDoesNotMatchEndpointMap(t *testing.T) {
	adaptor := &Adaptor{}
	m := meta.NewMeta(
		&coremodel.Channel{BaseURL: "https://aip.baidubce.com"},
		mode.ChatCompletions,
		"ernie-custom-alias",
		coremodel.ModelConfig{},
	)
	m.ActualModel = "ERNIE-4.0-8K"

	got, err := adaptor.GetRequestURL(m, nil, nil)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	if got.Method != http.MethodPost {
		t.Fatalf("expected method %s, got %s", http.MethodPost, got.Method)
	}

	wantURL := "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/completions_pro"
	if got.URL != wantURL {
		t.Fatalf("expected URL %s, got %s", wantURL, got.URL)
	}
}
