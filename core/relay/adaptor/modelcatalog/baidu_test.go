package modelcatalog_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/modelcatalog"
	"github.com/stretchr/testify/require"
)

func TestBaiduSystemModelSetsFollowsMarker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v2/model", r.URL.Path)
		require.Equal(t, "DescribeSystemModelSets", r.URL.Query().Get("Action"))
		require.Contains(t, r.Header.Get("Authorization"), "bce-auth-v1/access-key/")
		require.NotEmpty(t, r.Header.Get("X-Bce-Date"))

		var request struct {
			Marker  string `json:"marker"`
			MaxKeys int    `json:"maxKeys"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, 100, request.MaxKeys)
		w.Header().Set("Content-Type", "application/json")

		if request.Marker == "" {
			_, err := w.Write(
				[]byte(
					`{"result":{"systemModelSetList":[{"modelSetId":"set-1","modelSetName":"reranker","description":"reranking models","modelNames":["reranker-v1","reranker-v2"]}],"pageInfo":{"isTruncated":true,"nextMarker":"next"}}}`,
				),
			)
			require.NoError(t, err)

			return
		}

		require.Equal(t, "next", request.Marker)

		_, err := w.Write(
			[]byte(
				`{"result":{"systemModelSetList":[{"modelSetId":"set-2","modelSetName":"embedder","modelNames":["embedder-v1"]}],"pageInfo":{"isTruncated":false}}}`,
			),
		)
		require.NoError(t, err)
	}))
	defer server.Close()

	models, err := modelcatalog.BaiduSystemModelSets(
		context.Background(),
		&model.Channel{},
		server.URL+"/v2",
		"access-key",
		"secret-key",
	)
	require.NoError(t, err)
	require.Len(t, models, 3)
	require.Equal(t, "reranker-v1", models[0]["id"])
	require.NotContains(t, models[0], "name")
	require.NotContains(t, models[0], "description")
	require.Equal(t, "embedder-v1", models[2]["id"])
}

func TestBaiduSystemModelSetsHTTPStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := modelcatalog.BaiduSystemModelSets(
		context.Background(),
		&model.Channel{ID: 14},
		server.URL,
		"access-key",
		"secret-key",
	)

	var statusErr *modelcatalog.HTTPStatusError
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, http.StatusForbidden, statusErr.Code)
	require.Equal(t, "provider returned status 403", err.Error())
}
