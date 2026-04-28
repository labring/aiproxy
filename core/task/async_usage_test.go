//nolint:testpackage
package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type failingAsyncUsageBalance struct {
	err error
}

func (b failingAsyncUsageBalance) GetGroupRemainBalance(
	context.Context,
	model.GroupCache,
) (float64, balance.PostGroupConsumer, error) {
	return 100, failingAsyncUsageConsumer(b), nil
}

func (b failingAsyncUsageBalance) GetGroupQuota(
	context.Context,
	model.GroupCache,
) (*balance.GroupQuota, error) {
	return &balance.GroupQuota{Total: 100, Remain: 100}, nil
}

type failingAsyncUsageConsumer struct {
	err error
}

func (c failingAsyncUsageConsumer) PostGroupConsume(
	context.Context,
	string,
	float64,
) (float64, error) {
	return 0, c.err
}

func TestCompleteAsyncUsageIgnoresMissingLog(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.AsyncUsageInfo{}))

	oldLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = oldLogDB
	})

	info := &model.AsyncUsageInfo{
		RequestID: "missing_log",
		RequestAt: time.Now(),
		Status:    model.AsyncUsageStatusPending,
		Model:     "gpt-5.4",
	}
	require.NoError(t, model.CreateAsyncUsageInfo(info))

	usage := model.Usage{
		InputTokens: 10,
		TotalTokens: 10,
	}

	require.NoError(t, completeAsyncUsage(context.Background(), info, usage))
	require.Equal(t, model.AsyncUsageStatusCompleted, info.Status)
	require.Equal(t, usage.InputTokens, info.Usage.InputTokens)
}

func TestCompleteAsyncUsageReturnsLogUpdateError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.AsyncUsageInfo{}))

	oldLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = oldLogDB
	})

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	info := &model.AsyncUsageInfo{
		RequestID: "log_update_error",
		RequestAt: time.Now(),
		Status:    model.AsyncUsageStatusPending,
		Model:     "gpt-5.4",
	}
	usage := model.Usage{
		InputTokens: 10,
		TotalTokens: 10,
	}

	require.Error(t, completeAsyncUsage(context.Background(), info, usage))
	require.Equal(t, model.AsyncUsageStatusPending, info.Status)
	require.Equal(t, model.ZeroNullInt64(0), info.Usage.InputTokens)
}

func TestCompleteAsyncUsageReturnsBalanceConsumeError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.AsyncUsageInfo{},
		&model.ConsumeError{},
	))

	oldLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = oldLogDB
	})

	oldBalance := balance.Default
	balance.Default = failingAsyncUsageBalance{err: errors.New("balance unavailable")}
	t.Cleanup(func() {
		balance.Default = oldBalance
	})

	require.NoError(t, model.CacheSetGroup(&model.GroupCache{
		ID:     "group-async-balance",
		Status: model.GroupStatusEnabled,
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroup("group-async-balance"))
	})

	info := &model.AsyncUsageInfo{
		RequestID:  "balance_error",
		RequestAt:  time.Now(),
		Status:     model.AsyncUsageStatusPending,
		Model:      "gpt-5.4",
		GroupID:    "group-async-balance",
		TokenID:    1,
		TokenName:  "token-1",
		Price:      model.Price{InputPrice: 1, InputPriceUnit: 1},
		UpstreamID: "resp_balance_error",
	}
	require.NoError(t, model.CreateAsyncUsageInfo(info))

	err = completeAsyncUsage(context.Background(), info, model.Usage{
		InputTokens: 10,
		TotalTokens: 10,
	})
	require.ErrorContains(t, err, "consume async usage balance")
	require.Equal(t, model.AsyncUsageStatusPending, info.Status)
	require.False(t, info.BalanceConsumed)

	var consumeErrors []model.ConsumeError
	require.NoError(t, db.Find(&consumeErrors).Error)
	require.Len(t, consumeErrors, 1)
	require.Equal(t, "balance_error", consumeErrors[0].RequestID)
	require.Equal(t, float64(10), consumeErrors[0].UsedAmount)
}

func TestTouchAsyncUsagePollCursorAdvancesUpdatedAtAndNextPollAt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AsyncUsageInfo{}))

	oldLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = oldLogDB
	})

	oldUpdatedAt := time.Now().Add(-time.Hour)
	oldNextPollAt := time.Now().Add(-time.Minute)
	info := &model.AsyncUsageInfo{
		RequestID:  "pending",
		Status:     model.AsyncUsageStatusPending,
		UpdatedAt:  oldUpdatedAt,
		NextPollAt: oldNextPollAt,
		Error:      "previous error",
	}
	require.NoError(t, db.Create(info).Error)

	beforeTouch := time.Now()

	touchAsyncUsagePollCursor(info)

	var got model.AsyncUsageInfo
	require.NoError(t, db.First(&got, info.ID).Error)
	require.True(t, got.UpdatedAt.After(oldUpdatedAt))
	require.True(t, got.NextPollAt.After(beforeTouch))
	require.Empty(t, got.Error)
}
