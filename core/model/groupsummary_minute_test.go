//nolint:testpackage
package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetGroupTokenLastRequestTimesMinute(t *testing.T) {
	oldLogDB := LogDB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupsummary_minute_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GroupSummaryMinute{}))

	LogDB = db

	t.Cleanup(func() {
		LogDB = oldLogDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	oldMinute := time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	newMinute := time.Now().Add(-time.Minute).Truncate(time.Minute)
	require.NoError(t, db.Create(&[]GroupSummaryMinute{
		{
			Unique: GroupSummaryMinuteUnique{
				GroupID:         "group-1",
				TokenName:       "token-a",
				Model:           "gpt-5",
				MinuteTimestamp: oldMinute.Unix(),
			},
		},
		{
			Unique: GroupSummaryMinuteUnique{
				GroupID:         "group-1",
				TokenName:       "token-a",
				Model:           "gpt-5-mini",
				MinuteTimestamp: newMinute.Unix(),
			},
		},
		{
			Unique: GroupSummaryMinuteUnique{
				GroupID:         "group-1",
				TokenName:       "token-b",
				Model:           "gpt-5",
				MinuteTimestamp: oldMinute.Unix(),
			},
		},
		{
			Unique: GroupSummaryMinuteUnique{
				GroupID:         "group-2",
				TokenName:       "token-a",
				Model:           "gpt-5",
				MinuteTimestamp: newMinute.Unix(),
			},
		},
	}).Error)

	lastRequestTimes, err := GetGroupTokenLastRequestTimesMinute(
		"group-1",
		[]string{"token-a", "token-b", "token-c"},
	)
	require.NoError(t, err)
	require.Equal(t, newMinute.Unix(), lastRequestTimes["token-a"].Unix())
	require.Equal(t, oldMinute.Unix(), lastRequestTimes["token-b"].Unix())
	require.NotContains(t, lastRequestTimes, "token-c")

	empty, err := GetGroupTokenLastRequestTimesMinute("group-1", nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}
