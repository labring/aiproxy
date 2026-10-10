package reqlimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/stretchr/testify/require"
)

func TestInMemoryRecordDropsExpiredMinuteWindowWithoutRealWait(t *testing.T) {
	rl := &InMemoryRecord{}

	e := rl.getEntry([]string{"group1", "model1"})

	nowSecond := time.Now().Unix()

	e.Lock()
	e.windows[nowSecond-61] = &windowCounts{normal: 3}
	e.windows[nowSecond-1] = &windowCounts{normal: 2}
	e.aggregateInitialized = false
	e.Unlock()

	totalCount, secondCount := rl.GetRequest(time.Minute, "group1", "model1")
	require.Equal(t, int64(2), totalCount)
	require.Equal(t, int64(0), secondCount)

	e.Lock()
	_, oldExists := e.windows[nowSecond-61]
	_, recentExists := e.windows[nowSecond-1]
	e.Unlock()
	require.False(t, oldExists)
	require.True(t, recentExists)
}

func TestInMemoryRecordRebuildsAggregateAfterTimeDiscontinuity(t *testing.T) {
	rl := &InMemoryRecord{}
	e := rl.getEntry([]string{"group1", "model1"})
	nowSecond := time.Now().Unix()

	e.Lock()
	e.windows[nowSecond-61] = &windowCounts{normal: 3}
	e.windows[nowSecond] = &windowCounts{normal: 2}
	e.windowSeconds = 60
	e.totalNormal = 5
	e.lastCleanedCutoff = nowSecond + 60
	e.aggregateInitialized = true
	e.Unlock()

	totalCount, secondCount := rl.GetRequest(time.Minute, "group1", "model1")
	require.Equal(t, int64(2), totalCount)
	require.Equal(t, int64(2), secondCount)

	e.Lock()
	_, expiredExists := e.windows[nowSecond-61]
	lastCleanedCutoff := e.lastCleanedCutoff
	e.Unlock()
	require.False(t, expiredExists)
	require.Equal(t, nowSecond-60, lastCleanedCutoff)
}

func TestInMemoryRecordCleanupInactiveEntries(t *testing.T) {
	rl := &InMemoryRecord{}

	e := rl.getEntry([]string{"group1", "model1"})
	e.lastAccess.Store(time.Now().Add(-time.Second))

	go rl.cleanupInactiveEntries(10*time.Millisecond, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := rl.entries.Load("group1:model1")
		return !ok
	}, 300*time.Millisecond, 10*time.Millisecond)
}

func TestChooseRecordSnapshotsKeepsEmptyRedisSnapshot(t *testing.T) {
	memoryCalled := false
	snapshots := chooseRecordSnapshots(true, nil, nil, func() []recordSnapshot {
		memoryCalled = true
		return []recordSnapshot{{Keys: []string{"memory"}, TotalCount: 1}}
	}, "redis snapshot error: ")

	require.Empty(t, snapshots)
	require.False(t, memoryCalled)
}

func TestChooseRecordSnapshotsFallsBackOnRedisError(t *testing.T) {
	snapshots := chooseRecordSnapshots(
		true,
		nil,
		errors.New("boom"),
		func() []recordSnapshot {
			return []recordSnapshot{{Keys: []string{"memory"}, TotalCount: 1}}
		},
		"redis snapshot error: ",
	)

	require.Len(t, snapshots, 1)
	require.Equal(t, []string{"memory"}, snapshots[0].Keys)
	require.Equal(t, int64(1), snapshots[0].TotalCount)
}

func TestChannelModelScopedAggregateRequests(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldRequestRecord := memoryChannelModelRecord
	oldTokensRecord := memoryChannelModelTokensRecord
	oldGroupChannelRequestRecord := memoryGroupChannelModelRecord
	oldGroupChannelTokensRecord := memoryGroupChannelModelTokensRecord
	common.RedisEnabled = false
	memoryChannelModelRecord = &InMemoryRecord{}
	memoryChannelModelTokensRecord = &InMemoryRecord{}
	memoryGroupChannelModelRecord = &InMemoryRecord{}
	memoryGroupChannelModelTokensRecord = &InMemoryRecord{}
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		memoryChannelModelRecord = oldRequestRecord
		memoryChannelModelTokensRecord = oldTokensRecord
		memoryGroupChannelModelRecord = oldGroupChannelRequestRecord
		memoryGroupChannelModelTokensRecord = oldGroupChannelTokensRecord
	})

	ctx := context.Background()
	PushChannelModelRequest(ctx, "1", "gpt-5")
	PushGroupChannelModelRequest(ctx, "group-a", "1", "gpt-5")
	PushGroupChannelModelRequest(ctx, "group-a", "2", "gpt-5")
	PushGroupChannelModelRequest(ctx, "group-b", "1", "gpt-5")
	PushChannelModelTokensRequest(ctx, "1", "gpt-5", 10)
	PushGroupChannelModelTokensRequest(ctx, "group-a", "1", "gpt-5", 20)
	PushGroupChannelModelTokensRequest(ctx, "group-a", "2", "gpt-5", 30)
	PushGroupChannelModelTokensRequest(ctx, "group-b", "1", "gpt-5", 40)

	rpm, _ := GetChannelModelRequest(ctx, "*", "gpt-5")
	require.Equal(t, int64(1), rpm)

	tpm, _ := GetChannelModelTokensRequest(ctx, "*", "gpt-5")
	require.Equal(t, int64(10), tpm)

	PushChannelModelRequest(ctx, "2", "gpt-5-mini")
	PushChannelModelTokensRequest(ctx, "2", "gpt-5-mini", 15)
	rpm, _ = GetChannelModelRequest(ctx, "*", "*")
	require.Equal(t, int64(2), rpm)

	tpm, _ = GetChannelModelTokensRequest(ctx, "*", "*")
	require.Equal(t, int64(25), tpm)

	rpm, _ = GetGroupChannelModelRequest(ctx, "group-a", "", "gpt-5")
	require.Equal(t, int64(2), rpm)

	tpm, _ = GetGroupChannelModelTokensRequest(ctx, "group-a", "", "gpt-5")
	require.Equal(t, int64(50), tpm)

	PushGroupChannelModelRequest(ctx, "group-a", "3", "gpt-5-mini")
	PushGroupChannelModelTokensRequest(ctx, "group-a", "3", "gpt-5-mini", 15)
	rpm, _ = GetGroupChannelModelRequest(ctx, "group-a", "", "*")
	require.Equal(t, int64(3), rpm)

	tpm, _ = GetGroupChannelModelTokensRequest(ctx, "group-a", "", "*")
	require.Equal(t, int64(65), tpm)

	rpm, _ = GetGroupChannelModelRequest(ctx, "group-a", "1", "gpt-5")
	require.Equal(t, int64(1), rpm)

	tpm, _ = GetGroupChannelModelTokensRequest(ctx, "group-a", "1", "gpt-5")
	require.Equal(t, int64(20), tpm)
}

func TestGroupChannelScopedAggregateRequestsMatchExactGroup(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldGroupChannelRequestRecord := memoryGroupChannelModelRecord
	oldGroupChannelTokensRecord := memoryGroupChannelModelTokensRecord
	common.RedisEnabled = false
	memoryGroupChannelModelRecord = &InMemoryRecord{}
	memoryGroupChannelModelTokensRecord = &InMemoryRecord{}
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		memoryGroupChannelModelRecord = oldGroupChannelRequestRecord
		memoryGroupChannelModelTokensRecord = oldGroupChannelTokensRecord
	})

	ctx := context.Background()
	PushGroupChannelModelRequest(ctx, "group-a", "1", "gpt-5")
	PushGroupChannelModelRequest(ctx, "group-a-child", "1", "gpt-5")
	PushGroupChannelModelTokensRequest(ctx, "group-a", "1", "gpt-5", 20)
	PushGroupChannelModelTokensRequest(ctx, "group-a-child", "1", "gpt-5", 30)

	rpm, _ := GetGroupChannelModelRequest(ctx, "group-a", "", "gpt-5")
	require.Equal(t, int64(1), rpm)

	tpm, _ := GetGroupChannelModelTokensRequest(ctx, "group-a", "", "gpt-5")
	require.Equal(t, int64(20), tpm)

	rpm, _ = GetGroupChannelModelRequest(ctx, "group-a-child", "", "gpt-5")
	require.Equal(t, int64(1), rpm)

	tpm, _ = GetGroupChannelModelTokensRequest(ctx, "group-a-child", "", "gpt-5")
	require.Equal(t, int64(30), tpm)
}
