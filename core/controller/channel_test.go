//nolint:testpackage
package controller

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/model"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestRunAutoTestBannedModelsHonorsConcurrencyLimit(t *testing.T) {
	const (
		concurrency = 7
		totalJobs   = 41
	)

	channels := map[string][]int64{
		"model-a": make([]int64, totalJobs),
	}
	for i := range totalJobs {
		channels["model-a"][i] = int64(i + 1)
	}

	var (
		currentActive atomic.Int32
		maxActive     atomic.Int32
		processed     atomic.Int32
	)

	deps := autoTestBannedModelsDeps{
		tryTestChannel: func(channelID int, modelName string) bool {
			return true
		},
		loadChannelByID: func(id int) (*model.Channel, error) {
			return &model.Channel{
				ID:     id,
				Name:   "channel",
				Type:   model.ChannelTypeOpenAI,
				Status: model.ChannelStatusEnabled,
				Models: []string{"model-a"},
			}, nil
		},
		testSingleModel: func(mc *model.ModelCaches, channel *model.Channel, modelName string, saveToDB bool) (*model.ChannelTest, error) {
			active := currentActive.Add(1)
			for {
				previous := maxActive.Load()
				if active <= previous || maxActive.CompareAndSwap(previous, active) {
					break
				}
			}

			time.Sleep(15 * time.Millisecond)

			processed.Add(1)
			currentActive.Add(-1)

			return &model.ChannelTest{Success: true}, nil
		},
		clearChannelModelErrors: func(ctx context.Context, modelName string, channelID int) error {
			return nil
		},
		notifyInfo:  func(title, message string) {},
		notifyError: func(title, message string) {},
	}

	runAutoTestBannedModels(log.NewEntry(log.StandardLogger()), channels, nil, concurrency, deps)

	require.Equal(t, int32(totalJobs), processed.Load())
	require.LessOrEqual(t, maxActive.Load(), int32(concurrency))
}

func TestRunAutoTestBannedModelsClearsWhenModelRemovedFromChannel(t *testing.T) {
	var (
		cleared        atomic.Int32
		testInvoked    atomic.Bool
		clearedChannel atomic.Int64
	)

	deps := autoTestBannedModelsDeps{
		tryTestChannel: func(channelID int, modelName string) bool {
			return true
		},
		loadChannelByID: func(id int) (*model.Channel, error) {
			return &model.Channel{
				ID:     id,
				Name:   "channel",
				Type:   model.ChannelTypeOpenAI,
				Status: model.ChannelStatusEnabled,
				Models: []string{"another-model"},
			}, nil
		},
		testSingleModel: func(mc *model.ModelCaches, channel *model.Channel, modelName string, saveToDB bool) (*model.ChannelTest, error) {
			testInvoked.Store(true)
			return &model.ChannelTest{Success: true}, nil
		},
		clearChannelModelErrors: func(ctx context.Context, modelName string, channelID int) error {
			cleared.Add(1)
			clearedChannel.Store(int64(channelID))
			return nil
		},
		notifyInfo:  func(title, message string) {},
		notifyError: func(title, message string) {},
	}

	runAutoTestBannedModels(
		log.NewEntry(log.StandardLogger()),
		map[string][]int64{"removed-model": {123}},
		nil,
		1,
		deps,
	)

	require.False(t, testInvoked.Load())
	require.Equal(t, int32(1), cleared.Load())
	require.Equal(t, int64(123), clearedChannel.Load())
}
