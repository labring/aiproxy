package model_test

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAvailableSetsDefaultsToDefault(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{model.ChannelDefaultSet}, model.NormalizeAvailableSets(nil))
	require.Equal(
		t,
		[]string{model.ChannelDefaultSet},
		model.NormalizeAvailableSets([]string{"", " "}),
	)
}

func TestIntersectAvailableSetsUsesGroupOrderAndInheritsEmptyTokenSets(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		[]string{model.ChannelDefaultSet},
		model.IntersectAvailableSets(nil, nil),
	)
	require.Equal(
		t,
		[]string{"beta", "default"},
		model.IntersectAvailableSets(
			[]string{"beta", "default", "beta", "internal"},
			[]string{"default", "beta"},
		),
	)
	require.Equal(t, []string{"beta"}, model.IntersectAvailableSets([]string{"beta"}, nil))
}

func TestResolveTokenAvailableSetsUsesGroupDefault(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		[]string{model.ChannelDefaultSet},
		model.ResolveTokenAvailableSets(nil, nil),
	)
	require.Equal(
		t,
		[]string{},
		model.ResolveTokenAvailableSets(nil, []string{"beta"}),
	)
	require.Equal(
		t,
		[]string{"beta"},
		model.ResolveTokenAvailableSets(
			[]string{"beta", model.ChannelDefaultSet},
			[]string{"beta"},
		),
	)
}

func TestResolveTokenGroupChannelAvailableSetsUsesDedicatedSets(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		[]string{"group-only"},
		model.ResolveTokenGroupChannelAvailableSets(
			[]string{"group-only"},
			nil,
		),
	)
	require.Equal(
		t,
		[]string{"group-only"},
		model.ResolveTokenGroupChannelAvailableSets(
			[]string{"group-only", model.ChannelDefaultSet},
			[]string{"group-only"},
		),
	)
	require.Equal(
		t,
		[]string{model.ChannelDefaultSet, "group-only"},
		model.ResolveTokenGroupChannelAvailableSets(
			[]string{model.ChannelDefaultSet, "group-only"},
			nil,
		),
	)
}

func TestSetsFromModelMapReturnsStableSets(t *testing.T) {
	t.Parallel()

	require.Equal(
		t,
		[]string{"beta", model.ChannelDefaultSet},
		model.SetsFromModelMap(map[string][]string{
			model.ChannelDefaultSet: {"default-model"},
			"beta":                  {"beta-model"},
		}),
	)
}

func TestFilterModelsBySet(t *testing.T) {
	t.Parallel()

	modelsBySet := map[string][]string{
		model.ChannelDefaultSet: {"default-model"},
		"beta":                  {"beta-model"},
	}

	require.Empty(t, model.FilterModelsBySet(modelsBySet, nil))
	require.Equal(
		t,
		map[string][]string{"beta": {"beta-model"}},
		model.FilterModelsBySet(modelsBySet, []string{"beta"}),
	)
}

func TestTokenModelAccessCanUseEmptyEffectiveSets(t *testing.T) {
	t.Parallel()

	modelsBySet := map[string][]string{
		model.ChannelDefaultSet: {"default-model"},
	}

	require.Empty(t, model.FindTokenModel(model.TokenCache{}, "default-model", nil, modelsBySet))
}
