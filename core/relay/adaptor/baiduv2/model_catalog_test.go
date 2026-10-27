package baiduv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestListModelsUsesChannelAKAndSK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "DescribeSystemModelSets", r.URL.Query().Get("Action"))
		require.Contains(t, r.Header.Get("Authorization"), "bce-auth-v1/channel-ak/")
		_, err := w.Write([]byte(`{"result":{"systemModelSetList":[{"modelSetName":"reranker","modelNames":["reranker-v1"]}],"pageInfo":{"isTruncated":false}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := (&Adaptor{}).ListModels(context.Background(), &model.Channel{
		Type:    model.ChannelTypeBaiduV2,
		Key:     "channel-ak|channel-sk",
		BaseURL: server.URL + "/v2",
	}, "")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "reranker-v1", models[0]["id"])
}
