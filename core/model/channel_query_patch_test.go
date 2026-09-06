//nolint:testpackage
package model

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelQueriesAndPatches(t *testing.T) {
	t.Run("SQLite", func(t *testing.T) {
		db, err := OpenSQLite(filepath.Join(t.TempDir(), "channels.db"))
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)

		previousDB, previousSQLite := DB, common.UsingSQLite
		DB, common.UsingSQLite = db, true
		t.Cleanup(func() {
			DB, common.UsingSQLite = previousDB, previousSQLite

			require.NoError(t, sqlDB.Close())
		})
		testChannelQueriesAndPatches(t)
	})
	t.Run("PostgreSQL", func(t *testing.T) {
		withTestPostgresStoreDB(t, func() {
			testChannelQueriesAndPatches(t)
		})
	})
}

func testChannelQueriesAndPatches(t *testing.T) {
	t.Helper()

	previousCaches := LoadModelCaches()
	t.Cleanup(func() { modelCaches.Store(previousCaches) })
	require.NoError(t, DB.AutoMigrate(&Channel{}, &ChannelTest{}, &ModelConfig{}))
	require.True(t, DB.Migrator().HasIndex(&Channel{}, "idx_channels_remark"))
	indexes, err := DB.Migrator().GetIndexes(&Channel{})
	require.NoError(t, err)

	var backupIndexColumns []string
	for _, index := range indexes {
		if index.Name() == "idx_channels_backup_active_id" {
			backupIndexColumns = index.Columns()
		}
	}

	if DB.Name() == "postgres" {
		require.NoError(t, DB.Raw(
			"SELECT pg_get_indexdef(?::regclass, position, true) FROM generate_series(1, 3) AS position ORDER BY position",
			"idx_channels_backup_active_id",
		).Scan(&backupIndexColumns).Error)
	}

	require.Equal(t, []string{"backup_only", "deleted_at", "id"}, backupIndexColumns)

	require.NoError(t, DB.Create(&ModelConfig{Model: "search-test", Type: mode.Responses}).Error)

	remark := "Production \u534e\u4e1c primary-region"
	channels := []*Channel{
		{
			Name:     "primary",
			Remark:   remark,
			Type:     ChannelTypeOpenAI,
			Key:      "test-key",
			BaseURL:  "https://example.invalid",
			ProxyURL: "http://proxy.invalid",
			Models: []string{
				"search-test",
			},
			ModelMapping:            map[string]string{"search-test": "upstream"},
			Configs:                 ChannelConfigs{"region": "primary"},
			Sets:                    []string{"production"},
			Priority:                42,
			EnabledAutoBalanceCheck: true,
			SkipTLSVerify:           true,
			EnabledNoPermissionBan:  true,
			WarnErrorRate:           0.1,
			MaxErrorRate:            0.2,
			BalanceThreshold:        3,
		},
		{Name: "backup", Remark: remark, Type: ChannelTypeOpenAI, BackupOnly: true},
		{Name: "empty", Type: ChannelTypeOpenAI},
		{Name: "legacy", Type: ChannelTypeOpenAI},
		{
			Name: "deleted", Remark: remark, Type: ChannelTypeOpenAI, BackupOnly: true,
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
	}
	require.NoError(t, DB.Create(&channels).Error)
	require.NoError(
		t,
		DB.Model(&Channel{}).Where("id = ?", channels[3].ID).Update("remark", nil).Error,
	)

	backupOnly, regularOnly, emptyRemark := true, false, ""

	t.Run("filters", func(t *testing.T) {
		for _, test := range []struct {
			name    string
			keyword string
			filter  ChannelFilter
			want    []int
		}{
			{"all", "", ChannelFilter{}, []int{channels[0].ID, channels[1].ID, channels[2].ID, channels[3].ID}},
			{"remark keyword", "primary-region", ChannelFilter{}, []int{channels[0].ID, channels[1].ID}},
			{"unicode remark", "\u534e\u4e1c", ChannelFilter{}, []int{channels[0].ID, channels[1].ID}},
			{"case insensitive", "PRODUCTION", ChannelFilter{}, []int{channels[0].ID, channels[1].ID}},
			{"backup only", "", ChannelFilter{BackupOnly: &backupOnly}, []int{channels[1].ID}},
			{"regular only", "", ChannelFilter{BackupOnly: &regularOnly}, []int{channels[0].ID, channels[2].ID, channels[3].ID}},
			{"combined", "primary-region", ChannelFilter{BackupOnly: &backupOnly}, []int{channels[1].ID}},
			{"exact remark", "", ChannelFilter{Remark: &remark, BackupOnly: &regularOnly}, []int{channels[0].ID}},
			{"empty remark", "", ChannelFilter{Remark: &emptyRemark}, []int{channels[2].ID, channels[3].ID}},
			{"existing exact filters", "primary-region", ChannelFilter{
				ID: channels[0].ID, Name: "primary", Type: int(ChannelTypeOpenAI),
				Key: "test-key", BaseURL: "https://example.invalid",
			}, []int{channels[0].ID}},
			{"boolean is not keyword content", "true", ChannelFilter{}, nil},
		} {
			t.Run(test.name, func(t *testing.T) {
				found, total, err := SearchChannels(test.keyword, 1, 20, test.filter, "id-asc")
				require.NoError(t, err)
				require.EqualValues(t, len(test.want), total)

				var ids []int
				for _, channel := range found {
					ids = append(ids, channel.ID)
				}

				require.Equal(t, test.want, ids)

				if test.keyword == "" {
					listed, count, err := GetChannels(1, 20, test.filter, "id-asc")
					require.NoError(t, err)
					require.Equal(t, total, count)
					require.Equal(t, found, listed)
				}
			})
		}

		page, total, err := SearchChannels("primary-region", 2, 1, ChannelFilter{}, "id-asc")
		require.NoError(t, err)
		require.EqualValues(t, 2, total)
		require.Len(t, page, 1)
		require.Equal(t, channels[1].ID, page[0].ID)
	})

	t.Run("optional updates", func(t *testing.T) {
		first, err := GetChannelByID(channels[0].ID)
		require.NoError(t, err)

		stale := *first
		newRemark := "changed concurrently"
		require.NoError(t, UpdateChannel(first, &ChannelPatch{Remark: &newRemark}))
		require.NoError(t, UpdateChannel(&stale, &ChannelPatch{BackupOnly: &backupOnly}))

		loaded, err := GetChannelByID(first.ID)
		require.NoError(t, err)
		require.Equal(t, newRemark, loaded.Remark)
		require.True(t, loaded.BackupOnly)
		require.Equal(t, loaded, &stale)

		var unchanged ChannelPatch
		require.NoError(
			t,
			sonic.Unmarshal(
				[]byte(`{"remark":null,"backup_only":null,"configs":null}`),
				&unchanged,
			),
		)
		require.NoError(t, UpdateChannel(&stale, &unchanged))
		require.Equal(t, loaded, &stale)

		var clearPatch ChannelPatch
		require.NoError(t, sonic.Unmarshal([]byte(`{
			"name":"","remark":"","base_url":"","proxy_url":"","priority":0,
			"backup_only":false,"enabled_auto_balance_check":false,"skip_tls_verify":false,
			"enabled_no_permission_ban":false,"warn_error_rate":0,"max_error_rate":0,
			"balance_threshold":0,"models":[],"model_mapping":{},"configs":{},"sets":[]
		}`), &clearPatch))
		require.NoError(t, UpdateChannel(&stale, &clearPatch))

		loaded, err = GetChannelByID(first.ID)
		require.NoError(t, err)
		require.Equal(t, loaded, &stale)
		require.Empty(t, loaded.Name)
		require.Empty(t, loaded.Remark)
		require.Empty(t, loaded.BaseURL)
		require.Empty(t, loaded.ProxyURL)
		require.Zero(t, loaded.Priority)
		require.False(t, loaded.BackupOnly)
		require.False(t, loaded.EnabledAutoBalanceCheck)
		require.False(t, loaded.SkipTLSVerify)
		require.False(t, loaded.EnabledNoPermissionBan)
		require.Zero(t, loaded.WarnErrorRate)
		require.Zero(t, loaded.MaxErrorRate)
		require.Zero(t, loaded.BalanceThreshold)
		require.Empty(t, loaded.Models)
		require.Empty(t, loaded.ModelMapping)
		require.Empty(t, loaded.Configs)
		require.Empty(t, loaded.Sets)
		require.Equal(t, "test-key", loaded.Key)
		require.Equal(t, ChannelStatusEnabled, loaded.Status)

		missingModels := []string{"missing-model"}
		err = UpdateChannel(&stale, &ChannelPatch{Models: &missingModels})
		require.ErrorContains(t, err, "model config not found")
		require.Error(t, UpdateChannel(&Channel{ID: -1}, &ChannelPatch{}))
	})
}
