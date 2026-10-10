//nolint:testpackage
package controller

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestValidateTokenRequiresName(t *testing.T) {
	err := validateToken(AddTokenRequest{Name: "  "})
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")

	require.NoError(t, validateToken(AddTokenRequest{Name: "token"}))
}

func TestAddTokenRequestToTokenIncludesGroupChannelModels(t *testing.T) {
	request := AddTokenRequest{
		Name:               "token",
		Models:             []string{"global-model"},
		GroupChannelModels: []string{"group-model"},
		GroupChannelSets:   []string{"group-set"},
	}
	token := request.ToToken()

	require.Equal(t, model.EmptyNullString("token"), token.Name)
	require.Equal(t, []string{"global-model"}, token.Models)
	require.Equal(t, []string{"group-model"}, token.GroupChannelModels)
	require.Equal(t, []string{"group-set"}, token.GroupChannelSets)
}
