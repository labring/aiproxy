//nolint:testpackage
package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	relaycontroller "github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialChannelSelectionPrefersOrdinaryAndAllowsBackupOnlyPool(t *testing.T) {
	t.Parallel()

	primary := &model.Channel{
		ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
	}
	backup := &model.Channel{
		ID: 2, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
		BackupOnly: true, Priority: model.MaxPriority,
	}

	for _, tt := range []struct {
		name       string
		channels   []*model.Channel
		errorRates map[string]float64
		ignored    map[string]struct{}
		wantBackup bool
	}{
		{name: "preferred backup is eligible alongside primary", channels: []*model.Channel{primary, backup}, wantBackup: true},
		{
			name:     "high error primary unlocks backup",
			channels: []*model.Channel{primary, backup}, errorRates: map[string]float64{"1": 1}, wantBackup: true,
		},
		{
			name:     "banned primary unlocks backup",
			channels: []*model.Channel{primary, backup}, ignored: map[string]struct{}{"1": {}}, wantBackup: true,
		},
		{name: "backup alone can start a request", channels: []*model.Channel{backup}, wantBackup: true},
		{
			name: "primary at error threshold remains eligible", channels: []*model.Channel{primary, backup},
			errorRates: map[string]float64{"1": maxRetryErrorRate}, wantBackup: true,
		},
		{
			name: "unhealthy backup does not replace primary fallback", channels: []*model.Channel{primary, backup},
			errorRates: map[string]float64{"1": 1, "2": 1},
		},
		{
			name: "ignored backup does not replace primary fallback", channels: []*model.Channel{primary, backup},
			errorRates: map[string]float64{"1": 1}, ignored: map[string]struct{}{"2": {}},
		},
		{
			name: "primary fallback without backups is preserved", channels: []*model.Channel{primary},
			errorRates: map[string]float64{"1": 1}, ignored: map[string]struct{}{"1": {}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := &model.ModelCaches{
				EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
					model.ChannelDefaultSet: {"backup-test": tt.channels, "other-model": {primary}},
					"other-set":             {"backup-test": {primary}},
				},
			}

			initial, err := getChannelWithFallback(
				mc, []string{model.ChannelDefaultSet}, "backup-test", mode.Responses,
				[]int{backup.ID}, tt.errorRates, tt.ignored,
			)
			require.NoError(t, err)

			if tt.wantBackup {
				assert.Equal(t, backup, initial.channel.channel)
			} else {
				assert.Equal(t, primary, initial.channel.channel)
			}

			assert.ElementsMatch(t, globalScopedChannels(tt.channels), initial.migratedChannels)
		})
	}
}

func TestInitialPreferredBackupKeepsGeneralBackupsLocked(t *testing.T) {
	t.Parallel()

	primary := &model.Channel{
		ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
	}
	preferred := &model.Channel{
		ID: 2, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
		BackupOnly: true, Priority: -1,
	}
	backup := &model.Channel{
		ID: 3, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled, BackupOnly: true,
	}
	generalPrimary := &model.Channel{
		ID: 4, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
	}
	generalBackup := &model.Channel{
		ID: 5, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled, BackupOnly: true,
	}
	channels := []*model.Channel{primary, preferred, backup, generalPrimary, generalBackup}
	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {"backup-test": channels},
		},
	}
	preferIDs := []int{2, 3, 1}
	initial, err := getChannelWithFallback(
		mc, nil, "backup-test", mode.Responses, preferIDs, map[string]float64{"1": 1, "4": 1}, nil,
	)
	require.NoError(t, err)
	require.Equal(t, preferred, initial.channel.channel)

	state := initGlobalRetryState(
		3,
		initial,
		meta.NewMeta(initial.channel.channel, mode.Responses, "backup-test", model.ModelConfig{}),
		&relaycontroller.HandleResult{
			Error: relaymodel.NewOpenAIError(http.StatusTooManyRequests, relaymodel.OpenAIError{
				Message: "rate limited",
			}),
		},
		model.Price{}, time.Now(),
	)
	assert.False(t, state.backupOnlyEnabled)
	assert.ElementsMatch(
		t,
		globalScopedChannels([]*model.Channel{primary, backup, generalPrimary, generalBackup}),
		getRetryCandidates(state, nil),
	)
	assert.Equal(t, channelIDsToKeys(preferIDs), state.preferChannelKeys)

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, backup, channel.channel)

	state.failedChannelIDs["1"] = struct{}{}
	state.failedChannelIDs["3"] = struct{}{}
	assert.ElementsMatch(
		t,
		globalScopedChannels([]*model.Channel{generalPrimary, generalBackup}),
		getRetryCandidates(state, nil),
	)
	assert.False(t, state.backupOnlyEnabled)
	channel, err = getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, generalPrimary, channel.channel)
}

func TestPreferencesIncludeBackupsBeforeRestartingRound(t *testing.T) {
	t.Parallel()

	channels := []*model.Channel{
		{ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled},
		{
			ID:         2,
			Type:       model.ChannelTypeOpenAI,
			Status:     model.ChannelStatusEnabled,
			BackupOnly: true,
		},
		{ID: 3, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled},
		{
			ID:         4,
			Type:       model.ChannelTypeOpenAI,
			Status:     model.ChannelStatusEnabled,
			BackupOnly: true,
		},
	}
	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {"backup-test": channels},
		},
	}
	initial, err := getChannelWithFallback(
		mc, nil, "backup-test", mode.Responses, []int{2, 1, 3, 4}, nil, nil,
	)
	require.NoError(t, err)
	require.Equal(t, 2, initial.channel.channel.ID)
	assert.False(t, initial.backupOnlyEnabled)

	state := initGlobalRetryState(
		10,
		initial,
		meta.NewMeta(initial.channel.channel, mode.Responses, "backup-test", model.ModelConfig{}),
		&relaycontroller.HandleResult{
			Error: relaymodel.NewOpenAIError(http.StatusTooManyRequests, relaymodel.OpenAIError{
				Message: "rate limited",
			}),
		},
		model.Price{},
		time.Now(),
	)

	for _, id := range []int{1, 3, 4} {
		channel, err := getRetryChannel(context.Background(), state)
		require.NoError(t, err)
		require.Equal(t, id, channel.channel.ID)
		assert.False(t, state.backupOnlyEnabled)

		state.failedChannelIDs[strconv.Itoa(id)] = struct{}{}
	}

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Contains(t, channels, channel.channel)
	assert.Empty(t, state.preferChannelKeys)
	assert.Empty(t, state.failedChannelIDs)
	assert.True(t, state.backupOnlyEnabled)
	assert.ElementsMatch(t, globalScopedChannels(channels), getRetryCandidates(state, nil))
}

func TestBackupUnlockRestartsRoundWithOrdinaryChannels(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		preferred  []int
		priorities [2]int32
		wantIDs    []int
	}{
		{
			name:       "weighted primary retries before backup",
			priorities: [2]int32{10, -1}, wantIDs: []int{1, 1, 2},
		},
		{
			name:       "weighted backup retries before primary",
			priorities: [2]int32{-1, 10}, wantIDs: []int{1, 2, 1},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			channels := []*model.Channel{
				{
					ID:       1,
					Type:     model.ChannelTypeOpenAI,
					Status:   model.ChannelStatusEnabled,
					Priority: tt.priorities[0],
				},
				{
					ID:         2,
					Type:       model.ChannelTypeOpenAI,
					Status:     model.ChannelStatusEnabled,
					Priority:   tt.priorities[1],
					BackupOnly: true,
				},
			}
			mc := &model.ModelCaches{
				EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
					model.ChannelDefaultSet: {"backup-test": channels},
				},
			}
			initial, err := getChannelWithFallback(
				mc, nil, "backup-test", mode.Responses, tt.preferred, nil, nil,
			)
			require.NoError(t, err)
			require.Equal(t, 1, initial.channel.channel.ID)
			require.False(t, initial.backupOnlyEnabled)

			state := initGlobalRetryState(
				2,
				initial,
				meta.NewMeta(
					initial.channel.channel,
					mode.Responses,
					"backup-test",
					model.ModelConfig{},
				),
				&relaycontroller.HandleResult{
					Error: relaymodel.NewOpenAIError(
						http.StatusTooManyRequests,
						relaymodel.OpenAIError{
							Message: "rate limited",
						},
					),
				},
				model.Price{},
				time.Now(),
			)

			ids := make([]int, 1, len(tt.wantIDs))

			ids[0] = initial.channel.channel.ID
			for i := range 2 {
				channel, err := getRetryChannel(context.Background(), state)
				require.NoError(t, err)

				ids = append(ids, channel.channel.ID)

				assert.True(t, state.backupOnlyEnabled)
				assert.Equal(t, channelIDsToKeys(tt.preferred), state.preferChannelKeys)

				if i == 0 {
					assert.Empty(t, state.failedChannelIDs)
					assert.ElementsMatch(
						t,
						globalScopedChannels(channels),
						getRetryCandidates(state, nil),
					)
					assert.Equal(t, 1, state.channelRetryInfo["1"].failures)
				} else {
					assert.Contains(t, state.failedChannelIDs, strconv.Itoa(ids[1]))
				}

				state.failedChannelIDs[channel.monitorKey()] = struct{}{}
			}

			assert.Equal(t, tt.wantIDs, ids)
		})
	}
}

func TestPreferredBackupEligibility(t *testing.T) {
	t.Parallel()

	channels := []*model.Channel{
		{ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled},
		{
			ID:         2,
			Type:       model.ChannelTypeOpenAI,
			Status:     model.ChannelStatusEnabled,
			BackupOnly: true,
		},
		{ID: 3, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled},
		{
			ID:         4,
			Type:       model.ChannelTypeOpenAI,
			Status:     model.ChannelStatusEnabled,
			BackupOnly: true,
		},
	}
	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {"backup-test": channels},
		},
	}

	for _, tt := range []struct {
		name         string
		preferred    []int
		errorRates   map[string]float64
		ignored      map[string]struct{}
		wantIDs      []int
		wantUnlocked bool
	}{
		{name: "preferred backup precedes ordinary channels", preferred: []int{2}, wantIDs: []int{2}},
		{
			name: "preferred primary unhealthy", preferred: []int{1, 2},
			errorRates: map[string]float64{"1": 1}, wantIDs: []int{2},
		},
		{
			name: "preferred primary banned", preferred: []int{1, 2},
			ignored: map[string]struct{}{"1": {}}, wantIDs: []int{2},
		},
		{
			name: "preferred backup unhealthy", preferred: []int{1, 2},
			errorRates: map[string]float64{"1": 1, "2": 0.9}, wantIDs: []int{3},
		},
		{
			name: "preferred backup banned", preferred: []int{1, 2},
			ignored: map[string]struct{}{"1": {}, "2": {}}, wantIDs: []int{3},
		},
		{
			name: "preferred backup at error threshold", preferred: []int{1, 2},
			errorRates: map[string]float64{"1": 1, "2": maxRetryErrorRate, "3": 1}, wantIDs: []int{2},
		},
		{
			name: "all primaries unhealthy restores preferred backup", preferred: []int{2},
			errorRates: map[string]float64{"1": 1, "3": 1}, wantIDs: []int{2},
		},
		{
			name: "all primaries banned restores preferred backup", preferred: []int{2},
			ignored: map[string]struct{}{"1": {}, "3": {}}, wantIDs: []int{2},
		},
		{
			name: "preferred backup banned after unlock", preferred: []int{2},
			ignored: map[string]struct{}{"1": {}, "2": {}, "3": {}}, wantIDs: []int{4}, wantUnlocked: true,
		},
		{
			name: "general backup after other channels filtered", preferred: []int{1, 2},
			errorRates: map[string]float64{"1": 1, "2": 1, "3": 1}, wantIDs: []int{4}, wantUnlocked: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			initial, err := getChannelWithFallback(
				mc, nil, "backup-test", mode.Responses, tt.preferred, tt.errorRates, tt.ignored,
			)
			require.NoError(t, err)
			assert.Contains(t, tt.wantIDs, initial.channel.channel.ID)
			assert.Equal(t, tt.wantUnlocked, initial.backupOnlyEnabled)

			state := &retryState{
				migratedChannels:  globalScopedChannels(channels),
				preferChannelKeys: channelIDsToKeys(tt.preferred),
				ignoreChannelIDs:  tt.ignored,
			}
			channel, err := state.selectChannel(
				getRetryCandidates(state, tt.errorRates),
				channelIDsToKeys(tt.preferred),
				tt.errorRates,
			)
			require.NoError(t, err)
			assert.Contains(t, tt.wantIDs, channel.channel.ID)
			assert.Equal(t, tt.wantUnlocked, state.backupOnlyEnabled)
		})
	}
}

func TestInitialGeneralBackupStaysUnlockedAfterPrimaryRecovery(t *testing.T) {
	t.Parallel()

	primary := &model.Channel{
		ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
	}
	backup := &model.Channel{
		ID: 2, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled, BackupOnly: true,
	}
	otherBackup := &model.Channel{
		ID: 3, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled, BackupOnly: true,
	}
	channels := []*model.Channel{primary, backup, otherBackup}
	mc := &model.ModelCaches{
		ChannelsByID: map[int]*model.Channel{1: backup},
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {"backup-test": channels},
		},
	}
	initial, err := getChannelWithFallback(
		mc, nil, "backup-test", mode.Responses, nil, map[string]float64{"1": 1, "3": 1}, nil,
	)
	require.NoError(t, err)
	require.Equal(t, backup, initial.channel.channel)

	state := initGlobalRetryState(
		3, initial, meta.NewMeta(backup, mode.Responses, "backup-test", model.ModelConfig{}),
		&relaycontroller.HandleResult{
			Error: relaymodel.NewOpenAIError(http.StatusTooManyRequests, relaymodel.OpenAIError{
				Message: "rate limited",
			}),
		}, model.Price{}, time.Now(),
	)
	assert.True(t, state.backupOnlyEnabled)
	assert.ElementsMatch(
		t,
		globalScopedChannels([]*model.Channel{primary, otherBackup}),
		getRetryCandidates(state, nil),
	)
}

func TestBackupOnlyPoolUsesPreferredChannelsAndRetries(t *testing.T) {
	t.Parallel()

	channels := []*model.Channel{
		{
			ID:         1,
			Type:       model.ChannelTypeOpenAI,
			Status:     model.ChannelStatusEnabled,
			BackupOnly: true,
		},
		{
			ID:         2,
			Type:       model.ChannelTypeOpenAI,
			Status:     model.ChannelStatusEnabled,
			BackupOnly: true,
		},
	}
	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {"backup-test": channels},
		},
	}
	initial, err := getChannelWithFallback(
		mc, nil, "backup-test", mode.Responses, []int{2, 1}, nil, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, 2, initial.channel.channel.ID)

	state := &retryState{
		channelSelectionState: initial.channelSelectionState,
		meta: meta.NewMeta(
			initial.channel.channel,
			mode.Responses,
			"backup-test",
			model.ModelConfig{},
		),
		migratedChannels:  initial.migratedChannels,
		preferChannelKeys: channelIDsToKeys([]int{2, 1}),
		failedChannelIDs:  map[string]struct{}{"2": {}},
	}
	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, 1, channel.channel.ID)

	state.failedChannelIDs["1"] = struct{}{}
	channel, err = getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Contains(t, channels, channel.channel)
	assert.True(t, state.backupOnlyEnabled)

	assert.ElementsMatch(
		t,
		globalScopedChannels(channels),
		getRetryCandidates(&retryState{migratedChannels: globalScopedChannels(channels)}, nil),
	)
}

func TestUnsupportedPrimaryDoesNotExcludeBackup(t *testing.T) {
	t.Parallel()

	primary := &model.Channel{
		ID:     1,
		Type:   model.ChannelTypeFake,
		Status: model.ChannelStatusEnabled,
	}
	backup := &model.Channel{
		ID: 2, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled, BackupOnly: true,
	}
	mc := &model.ModelCaches{
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {"backup-test": {primary, backup}},
		},
	}
	initial, err := getChannelWithFallback(
		mc, nil, "backup-test", mode.ResponsesCompact, []int{1, 2}, nil, nil,
	)
	require.NoError(t, err)
	assert.Equal(t, backup, initial.channel.channel)
	assert.Equal(t, globalScopedChannels([]*model.Channel{backup}), initial.migratedChannels)
}

func TestBackupOnlyUnlocksAfterPrimaryFailuresAndSurvivesRounds(t *testing.T) {
	t.Parallel()

	channels := []*model.Channel{
		{ID: 1, Status: model.ChannelStatusEnabled},
		{ID: 2, Status: model.ChannelStatusEnabled},
		{ID: 3, Status: model.ChannelStatusEnabled, BackupOnly: true},
		{ID: 4, Status: model.ChannelStatusEnabled, BackupOnly: true},
	}
	initial := &initialChannel{
		channel: newGlobalScopedChannel(
			channels[0],
		),
		migratedChannels:  globalScopedChannels(channels),
		preferChannelKeys: channelIDsToKeys([]int{2}),
	}
	state := initGlobalRetryState(
		10, initial, meta.NewMeta(channels[0], mode.Responses, "backup-test", model.ModelConfig{}),
		&relaycontroller.HandleResult{
			Error: relaymodel.NewOpenAIError(http.StatusTooManyRequests, relaymodel.OpenAIError{
				Message: "rate limited",
			}),
		}, model.Price{}, time.Now(),
	)

	channel, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, 2, channel.channel.ID)
	assert.False(t, state.backupOnlyEnabled)
	assert.Equal(t, channelIDsToKeys([]int{2}), state.preferChannelKeys)
	state.failedChannelIDs["2"] = struct{}{}

	ids := make([]int, 0, 4)
	for i := range 4 {
		channel, err = getRetryChannel(context.Background(), state)
		require.NoError(t, err)

		if i == 0 {
			assert.Equal(t, 2, channel.channel.ID)
		}

		ids = append(ids, channel.channel.ID)

		assert.True(t, state.backupOnlyEnabled)
		assert.Equal(t, channelIDsToKeys([]int{2}), state.preferChannelKeys)
		state.failedChannelIDs[channel.monitorKey()] = struct{}{}
	}

	assert.ElementsMatch(t, []int{1, 2, 3, 4}, ids)

	channel, err = getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Contains(t, channels, channel.channel)
	assert.Empty(t, state.failedChannelIDs)
	assert.True(t, state.backupOnlyEnabled)
	assert.ElementsMatch(t, globalScopedChannels(channels), getRetryCandidates(state, nil))

	// A new request starts with its own locked backup state.
	fresh := &retryState{migratedChannels: globalScopedChannels(channels)}
	channel, err = fresh.selectChannel(getRetryCandidates(fresh, nil), nil, nil)
	require.NoError(t, err)
	assert.Contains(t, globalScopedChannels(channels[:2]), channel)
	assert.False(t, fresh.backupOnlyEnabled)
}

func TestBackupOnlyRetryEligibility(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		failed      map[string]struct{}
		ignored     map[string]struct{}
		errorRates  map[string]float64
		wantIDs     []int
		wantEnabled bool
	}{
		{
			name: "one primary remains", failed: map[string]struct{}{"1": {}}, wantIDs: []int{2},
		},
		{
			name: "ignored primary does not prevent backup", failed: map[string]struct{}{"1": {}},
			ignored: map[string]struct{}{"2": {}}, wantIDs: []int{1, 3}, wantEnabled: true,
		},
		{
			name: "high error primary does not prevent backup", failed: map[string]struct{}{"1": {}},
			errorRates: map[string]float64{"2": 0.9}, wantIDs: []int{1, 3}, wantEnabled: true,
		},
		{
			name: "permission failure still unlocks backup", failed: map[string]struct{}{"1": {}, "2": {}},
			ignored: map[string]struct{}{"1": {}, "2": {}}, wantIDs: []int{3}, wantEnabled: true,
		},
		{
			name: "backup respects ignored channels", failed: map[string]struct{}{"1": {}, "2": {}},
			ignored: map[string]struct{}{"3": {}}, wantIDs: []int{1, 2}, wantEnabled: true,
		},
		{
			name: "backup respects error threshold", failed: map[string]struct{}{"1": {}, "2": {}},
			errorRates: map[string]float64{"3": 0.9}, wantIDs: []int{1, 2}, wantEnabled: true,
		},
		{
			name: "all primaries ignored unlocks backup without an attempt", ignored: map[string]struct{}{"1": {}, "2": {}},
			wantIDs: []int{3}, wantEnabled: true,
		},
		{
			name: "all primaries unhealthy unlocks backup without an attempt", errorRates: map[string]float64{"1": 1, "2": 1},
			wantIDs: []int{3}, wantEnabled: true,
		},
		{
			name: "all channels unhealthy stay exhausted after reset", failed: map[string]struct{}{"1": {}, "2": {}},
			errorRates: map[string]float64{"1": 1, "2": 1, "3": 1}, wantEnabled: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := &retryState{
				migratedChannels: globalScopedChannels([]*model.Channel{
					{ID: 1, Status: model.ChannelStatusEnabled},
					{ID: 2, Status: model.ChannelStatusEnabled},
					{ID: 3, Status: model.ChannelStatusEnabled, BackupOnly: true},
					{ID: 4, Status: model.ChannelStatusDisabled, BackupOnly: true},
				}),
				failedChannelIDs: tt.failed, ignoreChannelIDs: tt.ignored,
			}

			channel, err := state.selectRetryChannel(tt.errorRates)
			if len(tt.wantIDs) == 0 {
				require.ErrorIs(t, err, ErrChannelsExhausted)
				assert.Nil(t, channel)
			} else {
				require.NoError(t, err)
				assert.Contains(t, tt.wantIDs, channel.channel.ID)
			}

			assert.Equal(t, tt.wantEnabled, state.backupOnlyEnabled)

			if tt.wantEnabled {
				assert.Empty(t, state.failedChannelIDs)

				candidates := getRetryCandidates(state, tt.errorRates)

				ids := make([]int, 0, len(candidates))
				for _, candidate := range candidates {
					ids = append(ids, candidate.channel.ID)
				}

				assert.ElementsMatch(t, tt.wantIDs, ids)
			} else {
				assert.Equal(t, tt.failed, state.failedChannelIDs)
			}

			assert.Equal(t, tt.ignored, state.ignoreChannelIDs)
		})
	}
}

func TestDesignatedBackupOnlyChannelRemainsPinned(t *testing.T) {
	t.Parallel()

	backup := &model.Channel{
		ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled, BackupOnly: true,
		Models: []string{"backup-test"},
	}
	mc := &model.ModelCaches{
		ChannelsByID: map[int]*model.Channel{1: backup},
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {"backup-test": {backup}},
		},
	}
	channel, err := GetChannelFromHeader(
		"1", mc, "backup-test", mode.Responses,
	)
	require.NoError(t, err)
	assert.Equal(t, backup, channel)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(middleware.ChannelID, backup.ID)
	channel, err = GetChannelFromRequest(
		c,
		mc,
		[]string{model.ChannelDefaultSet},
		"backup-test",
		mode.Responses,
	)
	require.NoError(t, err)
	assert.Equal(t, backup, channel)

	state := &retryState{
		designatedChannel: newGlobalScopedChannel(backup),
		meta:              meta.NewMeta(backup, mode.Responses, "backup-test", model.ModelConfig{}),
	}
	scoped, err := getRetryChannel(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, backup, scoped.channel)
}

func TestBackupOnlyModelRemainsListedAndRetrievable(t *testing.T) {
	t.Parallel()

	const name = "backup-test"

	backup := &model.Channel{
		ID: 1, Type: model.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled, BackupOnly: true,
	}
	mc := &model.ModelCaches{
		EnabledModelsBySet:     map[string][]string{model.ChannelDefaultSet: {name}},
		EnabledModelConfigsMap: map[string]model.ModelConfig{name: {Model: name}},
		ModelConfig:            testModelConfigCache{name: {Model: name, Type: mode.Responses}},
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {name: {backup}},
		},
	}
	token := model.TokenCache{}

	for _, handler := range []gin.HandlerFunc{ListModels, RetrieveModel} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(middleware.ModelCaches, mc)
		c.Set(middleware.Token, token)
		c.Set(middleware.Group, model.GroupCache{})
		c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
		c.Set(middleware.AvailableModels, mc.EnabledModelsBySet)
		c.Params = gin.Params{{Key: "model", Value: name}}
		handler(c)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"id":"backup-test"`)
	}

	response := newEnabledModelChannel(backup)
	data, err := sonic.Marshal(response)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"backup_only":true`)
}

func globalScopedChannels(channels []*model.Channel) []*scopedChannel {
	out := make([]*scopedChannel, len(channels))
	for i, ch := range channels {
		out[i] = newGlobalScopedChannel(ch)
	}

	return out
}

//nolint:unparam // Keep the helper signature aligned with production selector tests.
func getChannelWithFallback(cache *model.ModelCaches, sets []string, modelName string, m mode.Mode,
	preferred []int, rates map[string]float64, ignored map[string]struct{},
) (*initialChannel, error) {
	return getScopedChannelWithFallback(
		cache,
		sets,
		nil,
		false,
		modelName,
		m,
		channelIDsToKeys(preferred),
		rates,
		ignored,
	)
}

//nolint:unparam // Price is part of the shared retry-state constructor contract.
func initGlobalRetryState(retryTimes int, channel *initialChannel, mt *meta.Meta,
	result *relaycontroller.HandleResult, price model.Price, endAt time.Time,
) *retryState {
	return initRetryState(
		retryTimes,
		channel,
		mt,
		result,
		price,
		endAt,
		&model.ModelCaches{ModelConfig: testModelConfigCache{
			mt.OriginModel: {Model: mt.OriginModel, Type: mt.Mode},
		}},
		mt.OriginModel,
		RelayController{},
		false,
	)
}
