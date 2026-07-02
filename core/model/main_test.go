//nolint:testpackage
package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/stretchr/testify/require"
)

func TestPreMigrationCleanupContinuesWhenGroupChannelLogsTableMissing(t *testing.T) {
	oldLogDB := LogDB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "pre_migration_cleanup.db"))
	require.NoError(t, err)
	require.NoError(
		t,
		db.AutoMigrate(&Log{}, &RetryLog{}, &RequestDetail{}, &StoreV2{}, &GroupChannelStoreV2{}),
	)

	LogDB = db

	oldLogStorageHours := config.GetLogStorageHours()
	oldRetryLogStorageHours := config.GetRetryLogStorageHours()
	oldLogDetailStorageHours := config.GetLogDetailStorageHours()
	t.Cleanup(func() {
		LogDB = oldLogDB

		config.SetLogStorageHours(oldLogStorageHours)
		config.SetRetryLogStorageHours(oldRetryLogStorageHours)
		config.SetLogDetailStorageHours(oldLogDetailStorageHours)

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	config.SetLogStorageHours(1)
	config.SetRetryLogStorageHours(1)
	config.SetLogDetailStorageHours(1)

	require.NoError(t, db.Create(&StoreV2{
		ID:        ResponseStoreID("expired_pre_migration_store"),
		GroupID:   "group-1",
		TokenID:   1,
		ChannelID: 1,
		Model:     "gpt-5",
		ExpiresAt: time.Now().Add(-time.Hour),
	}).Error)

	require.NoError(t, preMigrationCleanup(100))

	var count int64
	require.NoError(t, db.Model(&StoreV2{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestPreMigrationCleanupRetryLogsIncludesGroupChannelRetryLogs(t *testing.T) {
	oldLogDB := LogDB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "pre_migration_retry_cleanup.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&RetryLog{}, &GroupChannelRetryLog{}))

	LogDB = db

	oldLogStorageHours := config.GetLogStorageHours()
	oldRetryLogStorageHours := config.GetRetryLogStorageHours()
	t.Cleanup(func() {
		LogDB = oldLogDB

		config.SetLogStorageHours(oldLogStorageHours)
		config.SetRetryLogStorageHours(oldRetryLogStorageHours)

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	config.SetLogStorageHours(24)
	config.SetRetryLogStorageHours(1)

	oldCreatedAt := time.Now().Add(-2 * time.Hour)
	newCreatedAt := time.Now()
	require.NoError(t, db.Create(&[]RetryLog{
		{CreatedAt: oldCreatedAt, Model: "old-global"},
		{CreatedAt: newCreatedAt, Model: "new-global"},
	}).Error)
	require.NoError(t, db.Create(&[]GroupChannelRetryLog{
		{CreatedAt: oldCreatedAt, GroupID: "group-1", Model: "old-group-channel"},
		{CreatedAt: newCreatedAt, GroupID: "group-1", Model: "new-group-channel"},
	}).Error)

	require.NoError(t, preMigrationCleanupRetryLogs(1))

	var globalRetryLogs []RetryLog
	require.NoError(t, db.Order("model asc").Find(&globalRetryLogs).Error)
	require.Len(t, globalRetryLogs, 1)
	require.Equal(t, "new-global", globalRetryLogs[0].Model)

	var groupChannelRetryLogs []GroupChannelRetryLog
	require.NoError(t, db.Order("model asc").Find(&groupChannelRetryLogs).Error)
	require.Len(t, groupChannelRetryLogs, 1)
	require.Equal(t, "new-group-channel", groupChannelRetryLogs[0].Model)
}
