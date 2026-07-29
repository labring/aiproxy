//nolint:testpackage
package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchInsertGroupChannelsPostgresConcurrentlyEnsuresOneGroup(t *testing.T) {
	withTestPostgresStoreDB(t, func() {
		require.NoError(t, DB.AutoMigrate(&Group{}, &GroupModelConfig{}, &GroupChannel{}))

		const (
			groupID = "group-postgres-concurrent"
			workers = 8
		)

		t.Cleanup(func() {
			require.NoError(t, CacheDeleteGroup(groupID))
			require.NoError(t, CacheDeleteGroupChannels(groupID))
		})

		start := make(chan struct{})
		errs := make(chan error, workers)

		var wg sync.WaitGroup
		wg.Add(workers)

		for range workers {
			go func() {
				defer wg.Done()

				<-start

				errs <- BatchInsertGroupChannels([]*GroupChannel{
					{
						GroupID: groupID,
						Name:    "concurrent",
						Type:    ChannelTypeOpenAI,
						Status:  ChannelStatusEnabled,
					},
				})
			}()
		}

		close(start)
		wg.Wait()
		close(errs)

		for err := range errs {
			require.NoError(t, err)
		}

		var groupCount int64
		require.NoError(t, DB.Model(&Group{}).Where("id = ?", groupID).Count(&groupCount).Error)
		require.EqualValues(t, 1, groupCount)

		var channelCount int64
		require.NoError(t, DB.Model(&GroupChannel{}).
			Where("group_id = ?", groupID).
			Count(&channelCount).Error)
		require.EqualValues(t, workers, channelCount)
	})
}
