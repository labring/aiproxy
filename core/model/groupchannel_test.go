//nolint:testpackage
package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/stretchr/testify/require"
)

func TestGroupChannelPartialUpdatesPassHooks(t *testing.T) {
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GroupChannel{}, &GroupChannelTest{}))

	DB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled

		require.NoError(t, CacheDeleteGroupChannels("group-1"))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, DB.Create(&GroupChannel{
		ID:              7,
		GroupID:         "group-1",
		Name:            "test group channel",
		Type:            ChannelTypeOpenAI,
		Status:          ChannelStatusEnabled,
		LastTestErrorAt: time.Now(),
	}).Error)

	require.NoError(t, UpdateGroupChannelStatusByID("group-1", 7, ChannelStatusDisabled))
	require.NoError(t, UpdateGroupChannelUsedAmount("group-1", 7, 1.5, 2, 1))
	require.NoError(t, ClearGroupChannelLastTestErrorAt("group-1", 7))

	var got GroupChannel
	require.NoError(t, DB.First(&got, "group_id = ? AND id = ?", "group-1", 7).Error)
	require.Equal(t, ChannelStatusDisabled, got.Status)
	require.Equal(t, 1.5, got.UsedAmount)
	require.Equal(t, 2, got.RequestCount)
	require.Equal(t, 1, got.RetryCount)
	require.True(t, got.LastTestErrorAt.IsZero())
}

func TestGetGroupChannelOrderRejectsTestAt(t *testing.T) {
	require.Equal(t, "id desc", getGroupChannelOrder("test_at"))
	require.Equal(t, "id desc", getGroupChannelOrder("test_at-asc"))
	require.Equal(t, "created_at asc", getGroupChannelOrder("created_at-asc"))
}

func TestGetGroupChannelTestsFiltersGroupChannelAndOrdersByTestAt(t *testing.T) {
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_tests_read_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GroupChannelTest{}))

	DB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	older := time.Now().Add(-time.Hour)
	newer := time.Now()
	require.NoError(t, DB.Create(&[]GroupChannelTest{
		{
			GroupID:        "group-1",
			GroupChannelID: 7,
			Model:          "old-model",
			TestAt:         older,
		},
		{
			GroupID:        "group-1",
			GroupChannelID: 7,
			Model:          "new-model",
			TestAt:         newer,
		},
		{
			GroupID:        "group-1",
			GroupChannelID: 8,
			Model:          "other-channel",
			TestAt:         newer.Add(time.Hour),
		},
		{
			GroupID:        "group-2",
			GroupChannelID: 7,
			Model:          "other-group",
			TestAt:         newer.Add(2 * time.Hour),
		},
	}).Error)

	tests, err := GetGroupChannelTests("group-1", 7)
	require.NoError(t, err)
	require.Len(t, tests, 2)
	require.Equal(t, "new-model", tests[0].Model)
	require.Equal(t, "old-model", tests[1].Model)
}

func TestGroupChannelReadsRequireGroup(t *testing.T) {
	_, err := LoadGroupChannels("")
	require.Error(t, err)

	_, err = LoadEnabledGroupChannels("")
	require.Error(t, err)

	_, err = GetGroupChannelByID("", 7)
	require.Error(t, err)

	_, err = GetGroupChannelTests("", 7)
	require.Error(t, err)
}

func TestGetGroupChannelsBasicInfoByIDsFiltersGroup(t *testing.T) {
	oldDB := DB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_basic_info.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GroupChannel{}))

	DB = db
	t.Cleanup(func() {
		DB = oldDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, DB.Create(&[]GroupChannel{
		{ID: 7, GroupID: "group-1", Name: "one", Type: ChannelTypeOpenAI},
		{ID: 8, GroupID: "group-2", Name: "two", Type: ChannelTypeAnthropic},
	}).Error)

	groupInfos, err := GetGroupChannelsBasicInfoByIDs("group-1", []int{7, 8})
	require.NoError(t, err)
	require.Len(t, groupInfos, 1)
	require.Equal(t, "group-1", groupInfos[0].GroupID)
	require.Equal(t, 7, groupInfos[0].ID)

	globalInfos, err := GetGlobalGroupChannelsBasicInfoByIDs([]int{7, 8})
	require.NoError(t, err)
	require.Len(t, globalInfos, 2)

	_, err = GetGroupChannelsBasicInfoByIDs("", []int{7})
	require.Error(t, err)
}

func TestGetGroupChannelLastRequestTimesMinute(t *testing.T) {
	oldLogDB := LogDB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_summary_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GroupChannelSummaryMinute{}))

	LogDB = db

	t.Cleanup(func() {
		LogDB = oldLogDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	oldMinute := time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	newMinute := time.Now().Add(-time.Minute).Truncate(time.Minute)
	require.NoError(t, db.Create(&[]GroupChannelSummaryMinute{
		{
			Unique: GroupChannelSummaryMinuteUnique{
				GroupID:         "group-1",
				GroupChannelID:  7,
				Model:           "gpt-5",
				MinuteTimestamp: oldMinute.Unix(),
			},
		},
		{
			Unique: GroupChannelSummaryMinuteUnique{
				GroupID:         "group-1",
				GroupChannelID:  7,
				Model:           "gpt-5",
				MinuteTimestamp: newMinute.Unix(),
			},
		},
		{
			Unique: GroupChannelSummaryMinuteUnique{
				GroupID:         "group-1",
				GroupChannelID:  8,
				Model:           "gpt-5",
				MinuteTimestamp: oldMinute.Unix(),
			},
		},
	}).Error)

	lastRequestTimes, err := GetGroupChannelLastRequestTimesMinute("group-1", []int{7, 8, 9})
	require.NoError(t, err)
	require.Equal(t, newMinute.Unix(), lastRequestTimes[7].Unix())
	require.Equal(t, oldMinute.Unix(), lastRequestTimes[8].Unix())
	require.NotContains(t, lastRequestTimes, 9)
}

func TestUpdateGroupChannelUsedAmountClearsGroupChannelCache(t *testing.T) {
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_cache_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GroupChannel{}))

	DB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled

		require.NoError(t, CacheDeleteGroupChannels("group-1"))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, DB.Create(&GroupChannel{
		ID:      7,
		GroupID: "group-1",
		Name:    "cached",
		Type:    ChannelTypeOpenAI,
		Status:  ChannelStatusEnabled,
	}).Error)

	cached, err := CacheGetGroupChannels("group-1")
	require.NoError(t, err)
	require.Len(t, cached.Channels, 1)
	require.Zero(t, cached.Channels[0].UsedAmount)

	require.NoError(t, UpdateGroupChannelUsedAmount("group-1", 7, 1.5, 2, 1))

	refreshed, err := CacheGetGroupChannels("group-1")
	require.NoError(t, err)
	require.Len(t, refreshed.Channels, 1)
	require.Equal(t, 1.5, refreshed.Channels[0].UsedAmount)
	require.Equal(t, 2, refreshed.Channels[0].RequestCount)
	require.Equal(t, 1, refreshed.Channels[0].RetryCount)
}

func TestDeleteGroupChannelsByIDsDeletesTests(t *testing.T) {
	oldDB := DB
	oldRedisEnabled := common.RedisEnabled

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_delete_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GroupChannel{}, &GroupChannelTest{}))

	DB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		DB = oldDB
		common.RedisEnabled = oldRedisEnabled

		require.NoError(t, CacheDeleteGroupChannels("group-1"))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, DB.Create(&[]GroupChannel{
		{
			ID:      7,
			GroupID: "group-1",
			Name:    "one",
			Type:    ChannelTypeOpenAI,
			Status:  ChannelStatusEnabled,
		},
		{
			ID:      8,
			GroupID: "group-1",
			Name:    "two",
			Type:    ChannelTypeOpenAI,
			Status:  ChannelStatusEnabled,
		},
	}).Error)
	require.NoError(t, DB.Create(&[]GroupChannelTest{
		{GroupID: "group-1", GroupChannelID: 7, Model: "gpt-5", Mode: 1},
		{GroupID: "group-1", GroupChannelID: 8, Model: "gpt-5", Mode: 1},
		{GroupID: "group-2", GroupChannelID: 7, Model: "gpt-5", Mode: 1},
	}).Error)

	require.NoError(t, DeleteGroupChannelsByIDs("group-1", []int{7, 8}))

	var remainingTests []GroupChannelTest
	require.NoError(t, DB.Order("group_id").Find(&remainingTests).Error)
	require.Len(t, remainingTests, 1)
	require.Equal(t, "group-2", remainingTests[0].GroupID)
}

func TestDeleteGroupByIDDeletesGroupScopedChannelState(t *testing.T) {
	oldDB := DB
	oldLogDB := LogDB
	oldRedisEnabled := common.RedisEnabled

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "group_delete_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&Group{},
		&Token{},
		&PublicMCPReusingParam{},
		&GroupMCP{},
		&GroupModelConfig{},
		&GroupChannel{},
		&GroupChannelTest{},
		&GroupScopeModelConfig{},
		&Log{},
		&GroupChannelLog{},
	))

	DB = db
	LogDB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		DB = oldDB
		LogDB = oldLogDB
		common.RedisEnabled = oldRedisEnabled

		require.NoError(t, CacheDeleteGroup("group-delete"))
		require.NoError(t, CacheDeleteGroupChannels("group-delete"))
		require.NoError(t, CacheDeleteGroupScopeModelConfig("group-delete"))

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, DB.Create(&Group{ID: "group-delete"}).Error)
	require.NoError(t, DB.Create(&GroupChannel{
		ID:      7,
		GroupID: "group-delete",
		Name:    "owned",
		Type:    ChannelTypeOpenAI,
		Status:  ChannelStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&GroupChannelTest{
		GroupID:        "group-delete",
		GroupChannelID: 7,
		Model:          "gpt-5",
		Mode:           1,
	}).Error)
	require.NoError(t, DB.Create(&GroupScopeModelConfig{
		GroupID:     "group-delete",
		ModelConfig: NewDefaultModelConfig("gpt-5"),
	}).Error)

	require.NoError(t, DeleteGroupByID("group-delete"))

	var channelCount int64
	require.NoError(t, DB.Unscoped().
		Model(&GroupChannel{}).
		Where("group_id = ?", "group-delete").
		Count(&channelCount).Error)
	require.Zero(t, channelCount)

	var testCount int64
	require.NoError(t, DB.Model(&GroupChannelTest{}).
		Where("group_id = ?", "group-delete").
		Count(&testCount).Error)
	require.Zero(t, testCount)

	var configCount int64
	require.NoError(t, DB.Model(&GroupScopeModelConfig{}).
		Where("group_id = ?", "group-delete").
		Count(&configCount).Error)
	require.Zero(t, configCount)
}

func TestDeleteGroupsByIDsDeletesGroupScopedChannelState(t *testing.T) {
	oldDB := DB
	oldLogDB := LogDB
	oldRedisEnabled := common.RedisEnabled

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groups_delete_test.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&Group{},
		&Token{},
		&PublicMCPReusingParam{},
		&GroupMCP{},
		&GroupModelConfig{},
		&GroupChannel{},
		&GroupChannelTest{},
		&GroupScopeModelConfig{},
		&Log{},
		&GroupChannelLog{},
	))

	DB = db
	LogDB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		DB = oldDB
		LogDB = oldLogDB
		common.RedisEnabled = oldRedisEnabled

		for _, groupID := range []string{"group-bulk-1", "group-bulk-2", "group-keep"} {
			require.NoError(t, CacheDeleteGroup(groupID))
			require.NoError(t, CacheDeleteGroupChannels(groupID))
			require.NoError(t, CacheDeleteGroupScopeModelConfig(groupID))
		}

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, DB.Create(&[]Group{
		{ID: "group-bulk-1"},
		{ID: "group-bulk-2"},
		{ID: "group-keep"},
	}).Error)
	require.NoError(t, DB.Create(&[]GroupChannel{
		{
			ID:      7,
			GroupID: "group-bulk-1",
			Name:    "owned-1",
			Type:    ChannelTypeOpenAI,
			Status:  ChannelStatusEnabled,
		},
		{
			ID:      8,
			GroupID: "group-bulk-2",
			Name:    "owned-2",
			Type:    ChannelTypeOpenAI,
			Status:  ChannelStatusEnabled,
		},
		{
			ID:      9,
			GroupID: "group-keep",
			Name:    "keep",
			Type:    ChannelTypeOpenAI,
			Status:  ChannelStatusEnabled,
		},
	}).Error)
	require.NoError(t, DB.Create(&[]GroupChannelTest{
		{GroupID: "group-bulk-1", GroupChannelID: 7, Model: "gpt-5", Mode: 1},
		{GroupID: "group-bulk-2", GroupChannelID: 8, Model: "gpt-5", Mode: 1},
		{GroupID: "group-keep", GroupChannelID: 9, Model: "gpt-5", Mode: 1},
	}).Error)
	require.NoError(t, DB.Create(&[]GroupScopeModelConfig{
		{GroupID: "group-bulk-1", ModelConfig: NewDefaultModelConfig("gpt-5")},
		{GroupID: "group-bulk-2", ModelConfig: NewDefaultModelConfig("gpt-5")},
		{GroupID: "group-keep", ModelConfig: NewDefaultModelConfig("gpt-5")},
	}).Error)

	require.NoError(t, DeleteGroupsByIDs([]string{"group-bulk-1", "group-bulk-2"}))

	var deletedChannelCount int64
	require.NoError(t, DB.Unscoped().
		Model(&GroupChannel{}).
		Where("group_id IN ?", []string{"group-bulk-1", "group-bulk-2"}).
		Count(&deletedChannelCount).Error)
	require.Zero(t, deletedChannelCount)

	var deletedTestCount int64
	require.NoError(t, DB.Model(&GroupChannelTest{}).
		Where("group_id IN ?", []string{"group-bulk-1", "group-bulk-2"}).
		Count(&deletedTestCount).Error)
	require.Zero(t, deletedTestCount)

	var deletedConfigCount int64
	require.NoError(t, DB.Model(&GroupScopeModelConfig{}).
		Where("group_id IN ?", []string{"group-bulk-1", "group-bulk-2"}).
		Count(&deletedConfigCount).Error)
	require.Zero(t, deletedConfigCount)

	var keptChannelCount int64
	require.NoError(t, DB.Model(&GroupChannel{}).
		Where("group_id = ?", "group-keep").
		Count(&keptChannelCount).Error)
	require.EqualValues(t, 1, keptChannelCount)
}

func TestPrepareGroupChannelKeepsExplicitModelsOnly(t *testing.T) {
	channel := &GroupChannel{
		Type:   ChannelTypeOpenAI,
		Models: []string{"z-model", "a-model", "z-model"},
	}

	prepareGroupChannel(channel)

	require.Equal(t, []string{"a-model", "z-model"}, channel.Models)
	require.Empty(t, channel.ModelMapping)
}

func TestPrepareGroupChannelKeepsEmptyModelsAndMapping(t *testing.T) {
	channel := &GroupChannel{
		Type: ChannelTypeOpenAI,
	}

	prepareGroupChannel(channel)

	require.Empty(t, channel.Models)
	require.Empty(t, channel.ModelMapping)
}

func TestGroupChannelDashboardChannelsRespectSelectedChannel(t *testing.T) {
	oldLogDB := LogDB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_dashboard.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&GroupChannelSummary{}, &GroupChannelSummaryMinute{}))

	LogDB = db
	t.Cleanup(func() {
		LogDB = oldLogDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	hour := time.Now().Truncate(time.Hour).Unix()

	minute := time.Now().Truncate(time.Minute).Unix()
	for _, channelID := range []int{7, 8} {
		require.NoError(t, db.Create(&GroupChannelSummary{
			Unique: GroupChannelSummaryUnique{
				GroupID:        "group-1",
				GroupChannelID: channelID,
				Model:          "gpt-5",
				HourTimestamp:  hour,
			},
			Data: SummaryData{
				SummaryDataSet: SummaryDataSet{
					Count:  Count{RequestCount: 1},
					Amount: Amount{UsedAmount: float64(channelID)},
				},
			},
		}).Error)
		require.NoError(t, db.Create(&GroupChannelSummaryMinute{
			Unique: GroupChannelSummaryMinuteUnique{
				GroupID:         "group-1",
				GroupChannelID:  channelID,
				Model:           "gpt-5",
				MinuteTimestamp: minute,
			},
			Data: SummaryData{
				SummaryDataSet: SummaryDataSet{
					Count:  Count{RequestCount: 1},
					Amount: Amount{UsedAmount: float64(channelID)},
				},
			},
		}).Error)
	}

	start := time.Unix(hour, 0).Add(-time.Hour)
	end := time.Unix(hour, 0).Add(time.Hour)
	hourly, err := GetGroupChannelDashboardData(
		"group-1",
		7,
		"",
		start,
		end,
		TimeSpanHour,
		time.Local,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, []int{7}, hourly.Channels)

	minutely, err := GetGroupChannelDashboardData(
		"group-1",
		7,
		"",
		start,
		end,
		TimeSpanMinute,
		time.Local,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, []int{7}, minutely.Channels)

	v2, err := GetGroupChannelDashboardV2Data(
		"group-1",
		7,
		"",
		start,
		end,
		TimeSpanHour,
		time.Local,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, []int{7}, v2.Channels)
}

func TestGroupChannelSummaryReadsRequireGroup(t *testing.T) {
	now := time.Now()

	_, err := GetGroupChannelUsedChannels("", 0, time.Time{}, now)
	require.Error(t, err)

	_, err = GetGroupChannelUsedModels("", 0, time.Time{}, now)
	require.Error(t, err)

	_, err = GetGroupChannelUsedChannelsMinute("", 0, time.Time{}, now)
	require.Error(t, err)

	_, err = GetGroupChannelUsedModelsMinute("", 0, time.Time{}, now)
	require.Error(t, err)

	_, err = GetGroupChannelDashboardData(
		"",
		0,
		"",
		time.Time{},
		now,
		TimeSpanMinute,
		time.Local,
		nil,
	)
	require.Error(t, err)

	_, err = GetGroupChannelTimeSeriesModelData(
		"",
		0,
		"",
		time.Time{},
		now,
		TimeSpanMinute,
		time.Local,
		nil,
	)
	require.Error(t, err)

	_, err = GetGroupChannelDashboardV2Data(
		"",
		0,
		"",
		time.Time{},
		now,
		TimeSpanHour,
		time.Local,
		nil,
	)
	require.Error(t, err)
}

func TestBatchUpdateSummaryRecordsGroupChannelTokenSummaryForToken(t *testing.T) {
	oldLogDB := LogDB
	oldDB := DB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_token_summary.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&Group{},
		&Token{},
		&GroupChannel{},
		&GroupSummary{},
		&GroupSummaryMinute{},
		&GroupChannelSummary{},
		&GroupChannelSummaryMinute{},
		&GroupChannelTokenSummary{},
		&GroupChannelTokenSummaryMinute{},
	))

	LogDB = db
	DB = db
	t.Cleanup(func() {
		ProcessBatchUpdatesSummary()

		LogDB = oldLogDB
		DB = oldDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, db.Create(&Group{ID: "group-1"}).Error)
	require.NoError(t, db.Create(&Token{ID: 11, Name: "token-11", GroupID: "group-1"}).Error)
	require.NoError(t, db.Create(&GroupChannel{
		ID:      7,
		GroupID: "group-1",
		Type:    ChannelTypeOpenAI,
	}).Error)

	now := time.Now().Truncate(time.Minute)
	BatchUpdateSummary(
		now,
		now,
		now,
		"group-1",
		200,
		ChannelScopeGroup,
		7,
		"gpt-5",
		11,
		"token-11",
		true,
		Usage{InputTokens: 2, OutputTokens: 3, TotalTokens: 5},
		Amount{UsedAmount: 1.5},
		"",
		false,
	)
	ProcessBatchUpdatesSummary()

	var oldGroupSummaryCount int64
	require.NoError(t, db.Model(&GroupSummary{}).
		Where("group_id = ?", "group-1").
		Count(&oldGroupSummaryCount).Error)
	require.Zero(t, oldGroupSummaryCount)

	var tokenSummaryCount int64
	require.NoError(t, db.Model(&GroupChannelTokenSummary{}).Count(&tokenSummaryCount).Error)
	require.EqualValues(t, 1, tokenSummaryCount)

	var gotTokenSummary GroupChannelTokenSummary
	require.NoError(t, db.Where(
		"group_id = ? AND token_name = ? AND model = ?",
		"group-1",
		"token-11",
		"gpt-5",
	).First(&gotTokenSummary).Error)
	require.Equal(t, int64(1), gotTokenSummary.Data.RequestCount)
	require.Equal(t, ZeroNullInt64(5), gotTokenSummary.Data.TotalTokens)
	require.Equal(t, 1.5, gotTokenSummary.Data.UsedAmount)

	var gotChannelSummary GroupChannelSummary
	require.NoError(t, db.Where(
		"group_id = ? AND group_channel_id = ? AND model = ?",
		"group-1",
		7,
		"gpt-5",
	).First(&gotChannelSummary).Error)
	require.Equal(t, int64(1), gotChannelSummary.Data.RequestCount)
	require.Equal(t, ZeroNullInt64(5), gotChannelSummary.Data.TotalTokens)
	require.Equal(t, 1.5, gotChannelSummary.Data.UsedAmount)

	var gotGroup Group
	require.NoError(t, db.First(&gotGroup, "id = ?", "group-1").Error)
	require.Zero(t, gotGroup.UsedAmount)
	require.Zero(t, gotGroup.RequestCount)
	require.Equal(t, 1.5, gotGroup.GroupChannelUsedAmount)
	require.Equal(t, 1, gotGroup.GroupChannelRequestCount)

	var gotToken Token
	require.NoError(t, db.First(&gotToken, "id = ?", 11).Error)
	require.Zero(t, gotToken.UsedAmount)
	require.Zero(t, gotToken.RequestCount)
	require.Equal(t, 1.5, gotToken.GroupChannelUsedAmount)
	require.Equal(t, 1, gotToken.GroupChannelRequestCount)

	var gotGroupChannel GroupChannel
	require.NoError(t, db.First(&gotGroupChannel, "group_id = ? AND id = ?", "group-1", 7).Error)
	require.Equal(t, 1.5, gotGroupChannel.UsedAmount)
	require.Equal(t, 1, gotGroupChannel.RequestCount)
}

func TestBatchUpdateSummaryRecordsGroupChannelAggregatesWithoutTokenName(t *testing.T) {
	oldLogDB := LogDB
	oldDB := DB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_admin_token_summary.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&Group{},
		&Token{},
		&GroupChannel{},
		&GroupChannelSummary{},
		&GroupChannelSummaryMinute{},
		&GroupChannelTokenSummary{},
		&GroupChannelTokenSummaryMinute{},
	))

	LogDB = db
	DB = db
	t.Cleanup(func() {
		ProcessBatchUpdatesSummary()

		LogDB = oldLogDB
		DB = oldDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, db.Create(&Group{ID: "group-1"}).Error)
	require.NoError(t, db.Create(&Token{ID: 11, Name: "token-11", GroupID: "group-1"}).Error)
	require.NoError(t, db.Create(&GroupChannel{
		ID:      7,
		GroupID: "group-1",
		Type:    ChannelTypeOpenAI,
	}).Error)

	now := time.Now().Truncate(time.Minute)
	BatchUpdateSummary(
		now,
		now,
		now,
		"group-1",
		200,
		ChannelScopeGroup,
		7,
		"gpt-5",
		11,
		"",
		true,
		Usage{InputTokens: 2, OutputTokens: 3, TotalTokens: 5},
		Amount{UsedAmount: 1.5},
		"",
		false,
	)
	ProcessBatchUpdatesSummary()

	var channelSummaryCount int64
	require.NoError(t, db.Model(&GroupChannelSummary{}).Count(&channelSummaryCount).Error)
	require.EqualValues(t, 1, channelSummaryCount)

	var tokenSummaryCount int64
	require.NoError(t, db.Model(&GroupChannelTokenSummary{}).Count(&tokenSummaryCount).Error)
	require.Zero(t, tokenSummaryCount)

	var tokenSummaryMinuteCount int64
	require.NoError(t, db.Model(&GroupChannelTokenSummaryMinute{}).
		Count(&tokenSummaryMinuteCount).Error)
	require.Zero(t, tokenSummaryMinuteCount)

	var gotGroup Group
	require.NoError(t, db.First(&gotGroup, "id = ?", "group-1").Error)
	require.Zero(t, gotGroup.UsedAmount)
	require.Zero(t, gotGroup.RequestCount)
	require.Equal(t, 1.5, gotGroup.GroupChannelUsedAmount)
	require.Equal(t, 1, gotGroup.GroupChannelRequestCount)

	var gotToken Token
	require.NoError(t, db.First(&gotToken, "id = ?", 11).Error)
	require.Zero(t, gotToken.UsedAmount)
	require.Zero(t, gotToken.RequestCount)
	require.Equal(t, 1.5, gotToken.GroupChannelUsedAmount)
	require.Equal(t, 1, gotToken.GroupChannelRequestCount)

	var gotGroupChannel GroupChannel
	require.NoError(t, db.First(&gotGroupChannel, "group_id = ? AND id = ?", "group-1", 7).Error)
	require.Equal(t, 1.5, gotGroupChannel.UsedAmount)
	require.Equal(t, 1, gotGroupChannel.RequestCount)
}

func TestGetGroupChannelTokenDashboardUsesScopedMinuteCounters(t *testing.T) {
	oldLogDB := LogDB

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "groupchannel_token_dashboard.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&GroupChannelTokenSummary{},
		&GroupChannelTokenSummaryMinute{},
	))

	LogDB = db
	t.Cleanup(func() {
		LogDB = oldLogDB

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	now := time.Now().Truncate(time.Minute)
	require.NoError(t, db.Create(&GroupChannelTokenSummaryMinute{
		Unique: GroupChannelTokenSummaryMinuteUnique{
			GroupID:         "group-1",
			TokenName:       "token-1",
			Model:           "gpt-5",
			MinuteTimestamp: now.Unix(),
		},
		Data: SummaryData{
			SummaryDataSet: SummaryDataSet{
				Count: Count{RequestCount: 3},
				Usage: Usage{TotalTokens: 17},
			},
		},
	}).Error)

	result, err := GetGroupChannelTokenDashboardData(
		"group-1",
		now.Add(-time.Hour),
		now.Add(time.Hour),
		"token-1",
		"gpt-5",
		TimeSpanMinute,
		time.Local,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(3), result.RPM)
	require.Equal(t, int64(17), result.TPM)
}

func TestGroupChannelTokenSummaryReadsRequireGroup(t *testing.T) {
	now := time.Now()

	_, err := GetGroupChannelTokenUsedModels("", "", time.Time{}, now)
	require.Error(t, err)

	_, err = GetGroupChannelTokenUsedTokenNames("", time.Time{}, now)
	require.Error(t, err)

	_, err = GetGroupChannelTokenUsedModelsMinute("", "", time.Time{}, now)
	require.Error(t, err)

	_, err = GetGroupChannelTokenUsedTokenNamesMinute("", time.Time{}, now)
	require.Error(t, err)

	_, err = GetGroupChannelTokenDashboardData(
		"",
		time.Time{},
		now,
		"",
		"",
		TimeSpanMinute,
		time.Local,
		nil,
	)
	require.Error(t, err)

	_, err = GetGroupChannelTokenTimeSeriesModelData(
		"",
		"",
		"",
		time.Time{},
		now,
		TimeSpanMinute,
		time.Local,
		nil,
	)
	require.Error(t, err)

	_, err = GetGroupChannelTokenDashboardV2Data(
		"",
		"",
		"",
		time.Time{},
		now,
		TimeSpanHour,
		time.Local,
		nil,
	)
	require.Error(t, err)
}
