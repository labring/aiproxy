package model_test

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestModelRetryLimits(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name         string
		model        model.ModelConfig
		group        model.GroupModelConfig
		globalTimes  int64
		globalBudget int64
		wantTimes    int64
		wantBudget   time.Duration
	}{
		{name: "no retry configuration"},
		{name: "global count", globalTimes: 3, wantTimes: 3},
		{name: "global budget only", globalBudget: 60, wantTimes: -1, wantBudget: time.Minute},
		{name: "both global limits", globalTimes: 3, globalBudget: 60, wantTimes: 3, wantBudget: time.Minute},
		{name: "model count", model: model.ModelConfig{RetryTimes: 2}, globalTimes: 3, wantTimes: 2},
		{name: "model budget only ignores global count", model: model.ModelConfig{RetryBudget: new(int64(60))}, globalTimes: 3, wantTimes: -1, wantBudget: time.Minute},
		{name: "both model limits", model: model.ModelConfig{RetryTimes: 2, RetryBudget: new(int64(60))}, globalTimes: 3, wantTimes: 2, wantBudget: time.Minute},
		{name: "model count with inherited budget", model: model.ModelConfig{RetryTimes: 2}, globalTimes: 3, globalBudget: 30, wantTimes: 2, wantBudget: 30 * time.Second},
		{name: "model disables inherited budget", model: model.ModelConfig{RetryBudget: new(int64(0))}, globalTimes: 3, globalBudget: 30, wantTimes: 3},
		{name: "group overrides budget", model: model.ModelConfig{RetryBudget: new(int64(60))}, group: model.GroupModelConfig{OverrideRetryBudget: true, RetryBudget: 20}, globalTimes: 3, wantTimes: -1, wantBudget: 20 * time.Second},
		{name: "group disables budget", model: model.ModelConfig{RetryTimes: 2, RetryBudget: new(int64(60))}, group: model.GroupModelConfig{OverrideRetryBudget: true}, globalBudget: 90, wantTimes: 2},
		{name: "group inherits model count", model: model.ModelConfig{RetryTimes: 2}, group: model.GroupModelConfig{OverrideRetryBudget: true, RetryBudget: 20}, wantTimes: 2, wantBudget: 20 * time.Second},
		{name: "group clears count for budget only", model: model.ModelConfig{RetryTimes: 2}, group: model.GroupModelConfig{OverrideRetryTimes: true}, globalTimes: 3, globalBudget: 30, wantTimes: -1, wantBudget: 30 * time.Second},
		{name: "group sets both limits", group: model.GroupModelConfig{OverrideRetryTimes: true, RetryTimes: 4, OverrideRetryBudget: true, RetryBudget: 40}, wantTimes: 4, wantBudget: 40 * time.Second},
		{name: "disabled group overrides are ignored", model: model.ModelConfig{RetryTimes: 2, RetryBudget: new(int64(60))}, group: model.GroupModelConfig{RetryTimes: 4, RetryBudget: 40}, wantTimes: 2, wantBudget: time.Minute},
		{name: "runtime cap avoids duration overflow", model: model.ModelConfig{RetryBudget: new(int64(math.MaxInt64))}, wantTimes: -1, wantBudget: 3 * time.Minute},
		{name: "global runtime cap", globalBudget: math.MaxInt64, wantTimes: -1, wantBudget: 3 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			effective := tt.model.LoadFromGroupModelConfig(tt.group)
			times, budget := effective.RetryLimits(tt.globalTimes, tt.globalBudget)
			require.Equal(t, tt.wantTimes, times)
			require.Equal(t, tt.wantBudget, budget)
		})
	}
}

func TestRetryBudgetSerializationAndValidation(t *testing.T) {
	t.Parallel()

	for _, budget := range []int64{-1, 0, 1, 180, 181} {
		mc := model.ModelConfig{Model: "test", RetryBudget: new(budget)}

		gc := model.GroupModelConfig{Model: "test", RetryBudget: budget}
		if budget < 0 || budget > 180 {
			require.Error(t, mc.BeforeSave(nil))
			require.Error(t, gc.BeforeSave(nil))
		} else {
			require.NoError(t, mc.BeforeSave(nil))
			require.NoError(t, gc.BeforeSave(nil))
		}
	}

	for _, budget := range []*int64{nil, new(int64(0)), new(int64(60))} {
		original := model.ModelConfig{Model: "test", RetryBudget: budget}
		jsonData, err := sonic.Marshal(original)
		require.NoError(t, err)

		var decoded model.ModelConfig
		require.NoError(t, sonic.Unmarshal(jsonData, &decoded))
		require.Equal(t, budget, decoded.RetryBudget)

		//nolint:musttag // ModelConfig also carries runtime state.
		yamlData, err := yaml.Marshal(original)
		require.NoError(t, err)

		decoded = model.ModelConfig{}
		//nolint:musttag // Verify the actual model configuration round trip.
		require.NoError(t, yaml.Unmarshal(yamlData, &decoded))
		require.Equal(t, budget, decoded.RetryBudget)
	}
}

func TestRetryBudgetPersistence(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "retry.db"))
	require.NoError(t, err)

	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
	require.NoError(t, db.AutoMigrate(&model.ModelConfig{}, &model.GroupModelConfig{}))

	for _, budget := range []*int64{new(int64(60)), new(int64(0)), nil} {
		mc := model.ModelConfig{Model: "test", RetryBudget: budget}
		require.NoError(t, model.SaveModelConfig(mc))

		loaded, err := model.GetModelConfig("test")
		require.NoError(t, err)
		require.Equal(t, budget, loaded.RetryBudget)
	}

	for _, batch := range []bool{false, true} {
		initial := model.GroupModelConfig{
			GroupID: "test", Model: "test", OverrideRetryTimes: true, RetryTimes: 3,
			OverrideRetryBudget: true, RetryBudget: 60,
		}
		require.NoError(t, model.SaveGroupModelConfig(initial))

		for _, override := range []bool{true, false} {
			update := model.GroupModelConfig{
				GroupID:             "test",
				Model:               "test",
				OverrideRetryTimes:  override,
				OverrideRetryBudget: override,
			}
			if batch {
				require.NoError(
					t,
					model.UpdateGroupModelConfigs("test", []model.GroupModelConfig{update}),
				)
			} else {
				require.NoError(t, model.UpdateGroupModelConfig(update))
			}

			loaded, err := model.GetGroupModelConfig("test", "test")
			require.NoError(t, err)
			require.Equal(t, override, loaded.OverrideRetryTimes)
			require.Equal(t, override, loaded.OverrideRetryBudget)
			require.Zero(t, loaded.RetryTimes)
			require.Zero(t, loaded.RetryBudget)
		}
	}
}
