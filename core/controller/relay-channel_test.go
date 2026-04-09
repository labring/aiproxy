package controller

import (
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetChannelWithFallbackPreferred(t *testing.T) {
	t.Parallel()

	ch1 := &model.Channel{
		ID:       1,
		Type:     model.ChannelTypeOpenAI,
		Status:   model.ChannelStatusEnabled,
		Priority: 10,
	}
	ch2 := &model.Channel{
		ID:       2,
		Type:     model.ChannelTypeOpenAI,
		Status:   model.ChannelStatusEnabled,
		Priority: 10,
	}

	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {
				"gpt-5": {ch1, ch2},
			},
		},
	}

	t.Run("uses preferred channel when healthy", func(t *testing.T) {
		channel, migratedChannels, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, migratedChannels, 2)
		assert.Equal(t, 2, channel.ID)
	})

	t.Run("uses prefer id order instead of priority", func(t *testing.T) {
		ch1.Priority = 100
		ch2.Priority = 1

		channel, _, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2, 1},
			map[int64]float64{},
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.ID)
	})

	t.Run("falls back from preferred when preferred exceeds max error rate", func(t *testing.T) {
		channel, _, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{2: 0.9, 1: 0.1},
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 1, channel.ID)
	})

	t.Run("preferred path shares fallback semantics with default path", func(t *testing.T) {
		channel, _, err := getChannelWithFallback(
			mc,
			[]string{model.ChannelDefaultSet},
			"gpt-5",
			mode.Responses,
			[]int{2},
			map[int64]float64{2: 0.9},
			map[int64]struct{}{1: {}},
		)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.ID)
	})
}

func TestGetRetryChannelPrefersPreferredChannels(t *testing.T) {
	t.Parallel()

	ch1 := &model.Channel{
		ID:       1,
		Type:     model.ChannelTypeOpenAI,
		Status:   model.ChannelStatusEnabled,
		Priority: 10,
	}
	ch2 := &model.Channel{
		ID:       2,
		Type:     model.ChannelTypeOpenAI,
		Status:   model.ChannelStatusEnabled,
		Priority: 10,
	}

	state := &retryState{
		preferChannelIDs: []int{2},
		errorRates:       map[int64]float64{},
		meta: meta.NewMeta(
			ch1,
			mode.Responses,
			"gpt-5",
			model.ModelConfig{},
		),
		migratedChannels: []*model.Channel{ch1, ch2},
		failedChannelIDs: map[int64]struct{}{},
	}

	t.Run("retry prefers preferred channel when available", func(t *testing.T) {
		channel, err := getRetryChannel(state, 0, 2)
		require.NoError(t, err)
		assert.Equal(t, 2, channel.ID)
	})

	t.Run("retry skips preferred channel after it failed", func(t *testing.T) {
		state.failedChannelIDs = map[int64]struct{}{2: {}}
		channel, err := getRetryChannel(state, 0, 2)
		require.NoError(t, err)
		assert.Equal(t, 1, channel.ID)
	})

	t.Run("last retry can fall back to failed channel when no other choice exists", func(t *testing.T) {
		state.preferChannelIDs = nil
		state.failedChannelIDs = map[int64]struct{}{1: {}, 2: {}}
		state.ignoreChannelIDs = nil
		state.errorRates = map[int64]float64{}
		state.meta = meta.NewMeta(
			ch1,
			mode.Responses,
			"gpt-5",
			model.ModelConfig{},
		)

		channel, err := getRetryChannel(state, 1, 2)
		require.NoError(t, err)
		assert.Contains(t, []int{1, 2}, channel.ID)
	})
}
