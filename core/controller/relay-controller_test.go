//nolint:testpackage
package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	relaycontroller "github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type recordingAdaptorStore struct {
	saved     []adaptor.StoreCache
	saveScope model.ChannelScope
	getScope  model.ChannelScope
}

func (s *recordingAdaptorStore) GetStoreByScope(
	_ string,
	_ int,
	_ string,
	scope model.ChannelScope,
) (adaptor.StoreCache, error) {
	s.getScope = scope
	return adaptor.StoreCache{}, errors.New("unused")
}

func (s *recordingAdaptorStore) SaveStore(
	store adaptor.StoreCache,
	scope model.ChannelScope,
) error {
	s.saveScope = scope
	s.saved = append(s.saved, store)
	return nil
}

func (s *recordingAdaptorStore) SaveStoreWithOption(
	store adaptor.StoreCache,
	scope model.ChannelScope,
	_ adaptor.SaveStoreOption,
) error {
	s.saveScope = scope
	s.saved = append(s.saved, store)
	return nil
}

func (s *recordingAdaptorStore) SaveIfNotExistStore(
	store adaptor.StoreCache,
	scope model.ChannelScope,
) error {
	s.saveScope = scope
	s.saved = append(s.saved, store)
	return nil
}

func TestRetryStateRemainingRelayDelay(t *testing.T) {
	t.Parallel()

	t.Run("uses per-channel failure count plus jitter", func(t *testing.T) {
		t.Parallel()

		state := &retryState{}
		base := time.Unix(100, 0)
		jitter := 400 * time.Millisecond

		state.recordChannelFailure("7", base)

		assert.Equal(t, 1400*time.Millisecond, state.remainingRelayDelay("7", base, jitter))
		assert.Equal(
			t,
			900*time.Millisecond,
			state.remainingRelayDelay("7", base.Add(500*time.Millisecond), jitter),
		)

		state.recordChannelFailure("7", base.Add(2*time.Second))

		assert.Equal(
			t,
			2400*time.Millisecond,
			state.remainingRelayDelay("7", base.Add(2*time.Second), jitter),
		)
		assert.Equal(
			t,
			1150*time.Millisecond,
			state.remainingRelayDelay("7", base.Add(3250*time.Millisecond), jitter),
		)
	})

	t.Run("returns zero after required wait has already elapsed", func(t *testing.T) {
		t.Parallel()

		state := &retryState{}
		base := time.Unix(200, 0)

		state.recordChannelFailure("9", base)

		assert.Zero(
			t,
			state.remainingRelayDelay("9", base.Add(1500*time.Millisecond), 400*time.Millisecond),
		)
		assert.Zero(
			t,
			state.remainingRelayDelay("9", base.Add(2*time.Second), 400*time.Millisecond),
		)
	})

	t.Run("tracks each channel independently", func(t *testing.T) {
		t.Parallel()

		state := &retryState{}
		base := time.Unix(300, 0)

		state.recordChannelFailure("1", base)
		state.recordChannelFailure("2", base)
		state.recordChannelFailure("2", base.Add(100*time.Millisecond))

		assert.Equal(
			t,
			500*time.Millisecond,
			state.remainingRelayDelay("1", base.Add(800*time.Millisecond), 300*time.Millisecond),
		)
		assert.Equal(
			t,
			1500*time.Millisecond,
			state.remainingRelayDelay("2", base.Add(900*time.Millisecond), 300*time.Millisecond),
		)
		assert.Zero(t, state.remainingRelayDelay("3", base, 300*time.Millisecond))
	})

	t.Run("caps backoff at five seconds", func(t *testing.T) {
		t.Parallel()

		base := time.Unix(400, 0)
		state := &retryState{}

		for range 20 {
			state.recordChannelFailure("5", base)
		}

		assert.Equal(t, 5*time.Second, state.remainingRelayDelay("5", base, time.Second))
		assert.Zero(t, state.remainingRelayDelay("5", base.Add(5*time.Second), time.Second))
	})
}

func TestCalculateRelayBackoffDelay(t *testing.T) {
	t.Parallel()

	assert.Zero(t, calculateRelayBackoffDelay(0, 500*time.Millisecond))
	assert.Equal(t, 1500*time.Millisecond, calculateRelayBackoffDelay(1, 500*time.Millisecond))
	assert.Equal(t, 2500*time.Millisecond, calculateRelayBackoffDelay(2, 500*time.Millisecond))
	assert.Equal(t, 5*time.Second, calculateRelayBackoffDelay(20, time.Second))
	assert.Equal(t, 2*time.Second, calculateRelayBackoffDelay(1, time.Second))
}

func TestRelayControllerVideoModesValidateRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode mode.Mode
		want ValidateRequest
	}{
		{
			name: "video generation jobs",
			mode: mode.VideoGenerationsJobs,
			want: relaycontroller.ValidateVideoGenerationJobRequest,
		},
		{
			name: "videos",
			mode: mode.Videos,
			want: relaycontroller.ValidateVideosRequest,
		},
		{
			name: "videos remix",
			mode: mode.VideosRemix,
			want: relaycontroller.ValidateVideosRequest,
		},
		{
			name: "gemini video",
			mode: mode.GeminiVideo,
			want: relaycontroller.ValidateGeminiVideoRequest,
		},
		{
			name: "ali native video",
			mode: mode.AliVideo,
			want: relaycontroller.ValidateAliVideoRequest,
		},
		{
			name: "doubao native video",
			mode: mode.DoubaoVideo,
			want: relaycontroller.ValidateDoubaoVideoRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rc := relayController(tt.mode)
			require.NotNil(t, rc.ValidateRequest)
			require.Equal(
				t,
				reflect.ValueOf(tt.want).Pointer(),
				reflect.ValueOf(rc.ValidateRequest).Pointer(),
			)
		})
	}
}

func TestRelayChecksModeBeforeSelectingChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)
	c.Set(middleware.RequestModel, "pdf-only")
	c.Set(middleware.ModelConfig, model.ModelConfig{
		Model: "pdf-only",
		Type:  mode.ParsePdf,
	})
	c.Set(middleware.ModelCaches, &model.ModelCaches{
		ModelConfig: testModelConfigCache{
			"pdf-only": {
				Model: "pdf-only",
				Type:  mode.ParsePdf,
			},
		},
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{
			model.ChannelDefaultSet: {
				"pdf-only": {
					{
						ID:     1,
						Type:   model.ChannelTypeOpenAI,
						Status: model.ChannelStatusEnabled,
						Models: []string{"pdf-only"},
					},
				},
			},
		},
	})
	c.Set(middleware.Group, model.GroupCache{
		ID:            "group-1",
		Status:        model.GroupStatusEnabled,
		AvailableSets: []string{model.ChannelDefaultSet},
	})
	c.Set(middleware.Token, model.TokenCache{})
	c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
	c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeGlobal)

	relay(c, mode.ChatCompletions, RelayController{})

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "does not exist on this endpoint")
}

func TestRelayOwnModeUsesGroupScopeConfigForModeCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := "group-scope-mode-check"
	setTestGroupScopeModelConfigs(t, groupID, model.ModelConfig{
		Model: "public-model",
		Type:  mode.ChatCompletions,
	})
	require.NoError(t, model.CacheSetGroupChannels(&model.GroupChannelsCache{
		GroupID: groupID,
		Channels: []*model.GroupChannel{
			{
				ID:      7,
				GroupID: groupID,
				Type:    model.ChannelTypeOpenAI,
				Status:  model.ChannelStatusEnabled,
				Models:  []string{"public-model"},
			},
		},
	}))
	t.Cleanup(func() {
		require.NoError(t, model.CacheDeleteGroupChannels(groupID))
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)
	c.Set(middleware.RequestModel, "public-model")
	c.Set(middleware.ModelConfig, model.ModelConfig{
		Model: "public-model",
		Type:  mode.ParsePdf,
	})
	c.Set(middleware.ModelCaches, &model.ModelCaches{
		ModelConfig: testModelConfigCache{
			"public-model": {
				Model: "public-model",
				Type:  mode.ParsePdf,
			},
		},
		EnabledModel2ChannelsBySet: map[string]map[string][]*model.Channel{},
	})
	c.Set(middleware.Group, model.GroupCache{
		ID:     groupID,
		Status: model.GroupStatusEnabled,
	})
	c.Set(middleware.Token, model.TokenCache{ID: 1, Name: "token-1"})
	c.Set(middleware.AvailableSets, []string{model.ChannelDefaultSet})
	c.Set(middleware.GroupChannelMode, middleware.GroupChannelModeOwn)
	c.Set(middleware.RequestAt, time.Now())
	c.Set(middleware.RequestServiceTier, "")
	c.Set(middleware.RequestMetadata, map[string]string{})
	c.Set(middleware.PromptCacheKey, "")
	c.Set(middleware.RequestUser, "")
	c.Set(middleware.GroupBalance, &middleware.GroupBalanceConsumer{
		Group:        groupID,
		CheckBalance: func(float64) bool { return true },
	})

	called := false
	relay(c, mode.ChatCompletions, RelayController{
		Handler: func(_ *gin.Context, meta *meta.Meta) *relaycontroller.HandleResult {
			called = true

			require.Equal(t, model.ChannelScopeGroup, meta.Channel.Scope)
			require.Equal(t, mode.ChatCompletions, meta.ModelConfig.Type)

			return &relaycontroller.HandleResult{}
		},
	})

	require.True(t, called)
	require.False(t, c.IsAborted())
}

func TestResolveScopedModelConfigForGroupChannelUsesGroupScopeConfig(t *testing.T) {
	t.Parallel()

	groupID := "group-scope-resolve"
	setTestGroupScopeModelConfigs(t, groupID, model.ModelConfig{
		Model:      "public-model",
		Type:       mode.ChatCompletions,
		RetryTimes: 9,
		RPM:        100,
		TPM:        200,
		Price: model.Price{
			PerRequestPrice: 2,
		},
		Plugin: map[string]map[string]any{
			"cachefollow": {"enable": true},
		},
	})

	group := model.GroupCache{
		ID:       groupID,
		RPMRatio: 2,
		TPMRatio: 3,
		ModelConfigs: map[string]model.GroupModelConfig{
			"public-model": {
				Model:              "public-model",
				OverrideLimit:      true,
				RPM:                10,
				TPM:                20,
				OverrideRetryTimes: true,
				RetryTimes:         4,
				OverridePrice:      true,
				Price: model.Price{
					PerRequestPrice: 1,
				},
			},
		},
	}
	modelCaches := &model.ModelCaches{
		ModelConfig: testModelConfigCache{
			"public-model": {
				Model:      "public-model",
				Type:       mode.GeminiImage,
				RetryTimes: 9,
				RPM:        100,
				TPM:        200,
				Plugin: map[string]map[string]any{
					"cachefollow": {"enable": true},
				},
			},
		},
	}

	got, ok := resolveScopedModelConfig(
		group,
		modelCaches,
		newGroupScopedChannel(groupID, &model.Channel{ID: 7}),
		"public-model",
	)

	require.True(t, ok)
	require.Equal(t, "public-model", got.Model)
	require.Equal(t, mode.ChatCompletions, got.Type)
	require.Equal(t, int64(200), got.RPM)
	require.Equal(t, int64(600), got.TPM)
	require.Equal(t, int64(9), got.RetryTimes)
	require.Equal(t, float64(2), float64(got.Price.PerRequestPrice))
	require.Equal(t, map[string]map[string]any{"cachefollow": {"enable": true}}, got.Plugin)
}

func TestResolveScopedModelConfigForGroupChannelUsesDefaultWhenModelConfigDisabled(t *testing.T) {
	oldDisableModelConfig := config.DisableModelConfig
	config.DisableModelConfig = true
	t.Cleanup(func() {
		config.DisableModelConfig = oldDisableModelConfig
	})

	got, ok := resolveScopedModelConfig(
		model.GroupCache{ID: "group-disabled-model-config"},
		&model.ModelCaches{ModelConfig: testModelConfigCache{}},
		newGroupScopedChannel("group-disabled-model-config", &model.Channel{ID: 7}),
		"dynamic-model",
	)

	require.True(t, ok)
	require.Equal(t, model.NewDefaultModelConfig("dynamic-model"), got)
}

func TestResolveScopedModelConfigForGlobalChannelUsesGlobalWithGroupOverride(t *testing.T) {
	t.Parallel()

	group := model.GroupCache{
		ID:       "group-1",
		RPMRatio: 2,
		TPMRatio: 3,
		ModelConfigs: map[string]model.GroupModelConfig{
			"public-model": {
				Model:              "public-model",
				OverrideRetryTimes: true,
				RetryTimes:         4,
			},
		},
	}
	modelCaches := &model.ModelCaches{
		ModelConfig: testModelConfigCache{
			"public-model": {
				Model:      "public-model",
				Type:       mode.GeminiImage,
				RetryTimes: 3,
				RPM:        60,
				TPM:        600,
				Plugin: map[string]map[string]any{
					"cachefollow": {"enable": true},
				},
			},
		},
	}

	got, ok := resolveScopedModelConfig(
		group,
		modelCaches,
		newGlobalScopedChannel(&model.Channel{ID: 7}),
		"public-model",
	)

	require.True(t, ok)
	require.Equal(t, mode.GeminiImage, got.Type)
	require.Equal(t, int64(120), got.RPM)
	require.Equal(t, int64(1800), got.TPM)
	require.Equal(t, int64(4), got.RetryTimes)
	require.Equal(t, map[string]map[string]any{"cachefollow": {"enable": true}}, got.Plugin)
}

func TestResolveScopedModelConfigForGlobalChannelRequiresGlobalConfig(t *testing.T) {
	t.Parallel()

	got, ok := resolveScopedModelConfig(
		model.GroupCache{ID: "group-1"},
		&model.ModelCaches{ModelConfig: testModelConfigCache{}},
		newGlobalScopedChannel(&model.Channel{ID: 7}),
		"public-model",
	)

	require.False(t, ok)
	require.Empty(t, got)
}

func TestNewMetaByScopedChannelCanOverrideModelConfig(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		nil,
	)
	c.Set(middleware.RequestModel, "public-model")
	c.Set(middleware.ModelConfig, model.ModelConfig{
		Model:      "public-model",
		RetryTimes: 9,
		RPM:        90,
	})
	c.Set(middleware.Group, model.GroupCache{ID: "group-1"})
	c.Set(middleware.Token, model.TokenCache{ID: 1, Name: "token-1"})

	defaultConfig := model.NewDefaultModelConfig("public-model")
	meta := NewMetaByScopedChannel(
		c,
		newGroupScopedChannel("group-1", &model.Channel{ID: 7}),
		mode.ChatCompletions,
		meta.WithModelConfig(defaultConfig),
	)

	require.Equal(t, defaultConfig, meta.ModelConfig)
	require.Equal(t, model.ChannelScopeGroup, meta.Channel.Scope)
	require.Equal(t, "group-1", meta.Channel.GroupID)
}

func TestSaveAsyncUsageInfoDoesNotStoreInitialUsage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.AsyncUsageInfo{},
		&model.GroupChannel{},
		&model.GroupChannelLog{},
		&model.GroupChannelSummary{},
		&model.GroupChannelSummaryMinute{},
		&model.GroupChannelTokenSummary{},
		&model.GroupChannelTokenSummaryMinute{},
	))

	oldLogDB := model.LogDB
	oldDB := model.DB
	model.LogDB = db
	model.DB = db
	t.Cleanup(func() {
		consume.Wait()
		model.ProcessBatchUpdatesSummary()

		model.LogDB = oldLogDB
		model.DB = oldDB
	})

	m := meta.NewMeta(
		&model.Channel{ID: 11, BaseURL: "https://example.com"},
		mode.Videos,
		"test-video-model",
		model.ModelConfig{},
		meta.WithRequestID("request-async-1"),
		meta.WithRequestUsage(model.Usage{
			OutputTokens: 9,
			TotalTokens:  9,
		}),
		meta.WithRequestUsageContext(model.UsageContext{
			ServiceTier: "priority",
		}),
		meta.WithGroup(model.GroupCache{ID: "group-1"}),
		meta.WithToken(model.TokenCache{ID: 22, Name: "token-1"}),
	)

	saveAsyncUsageInfo(m, model.Price{}, &relaycontroller.HandleResult{
		UpstreamID: "video-123",
		Usage: model.Usage{
			OutputTokens: 99,
			TotalTokens:  99,
		},
	})

	var captured model.AsyncUsageInfo
	require.NoError(t, db.Where("upstream_id = ?", "video-123").First(&captured).Error)
	require.Zero(t, captured.Usage.OutputTokens)
	require.Zero(t, captured.Usage.TotalTokens)
	require.Equal(t, "priority", captured.UsageContext.ServiceTier)
}

func TestSaveAsyncUsageInfoSkipsGroupChannel(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AsyncUsageInfo{}))

	oldLogDB := model.LogDB
	model.LogDB = db
	t.Cleanup(func() {
		model.LogDB = oldLogDB
	})

	m := meta.NewMeta(
		&model.Channel{ID: 12, BaseURL: "https://example.com"},
		mode.Videos,
		"test-video-model",
		model.ModelConfig{},
		meta.WithRequestID("request-async-2"),
		meta.WithGroup(model.GroupCache{ID: "group-1"}),
		meta.WithToken(model.TokenCache{ID: 22, Name: "token-1"}),
		meta.WithChannelScope(model.ChannelScopeGroup, "group-1"),
	)

	saveAsyncUsageInfo(m, model.Price{}, &relaycontroller.HandleResult{
		UpstreamID: "video-456",
	})

	var count int64
	require.NoError(t, db.Model(&model.AsyncUsageInfo{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestShouldRecordAsyncUsageSkipsGroupChannel(t *testing.T) {
	m := meta.NewMeta(
		&model.Channel{ID: 12, BaseURL: "https://example.com"},
		mode.Videos,
		"test-video-model",
		model.ModelConfig{},
		meta.WithRequestID("request-group-async"),
		meta.WithGroup(model.GroupCache{ID: "group-1"}),
		meta.WithToken(model.TokenCache{ID: 22, Name: "token-1"}),
		meta.WithChannelScope(model.ChannelScopeGroup, "group-1"),
	)

	require.False(t, shouldRecordAsyncUsage(m, &relaycontroller.HandleResult{
		UpstreamID: "video-789",
		AsyncUsage: true,
		Usage:      model.Usage{OutputTokens: 10, TotalTokens: 10},
	}, true))
}

func TestRecordResultGroupChannelForcesSynchronousUsage(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "record_result_group_async.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.GroupChannel{},
		&model.GroupChannelLog{},
		&model.GroupChannelSummary{},
		&model.GroupChannelSummaryMinute{},
		&model.GroupChannelTokenSummary{},
		&model.GroupChannelTokenSummaryMinute{},
	))
	require.NoError(t, db.Create(&model.GroupChannel{
		ID:      7,
		GroupID: "group-1",
		Name:    "group-channel-7",
	}).Error)

	oldLogDB := model.LogDB
	oldDB := model.DB
	model.LogDB = db
	model.DB = db
	t.Cleanup(func() {
		consume.Wait()
		model.ProcessBatchUpdatesSummary()

		model.LogDB = oldLogDB
		model.DB = oldDB
	})

	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/v1/videos",
		nil,
	)
	c.Set(middleware.GroupBalance, &middleware.GroupBalanceConsumer{
		Group:        "group-1",
		CheckBalance: func(float64) bool { return true },
	})

	requestMeta := meta.NewMeta(
		&model.Channel{ID: 7, BaseURL: "https://example.com"},
		mode.Videos,
		"video-model",
		model.ModelConfig{},
		meta.WithRequestID("group-record-result-async"),
		meta.WithGroup(model.GroupCache{ID: "group-1"}),
		meta.WithToken(model.TokenCache{ID: 22, Name: "token-1"}),
		meta.WithChannelScope(model.ChannelScopeGroup, "group-1"),
	)

	recordResult(
		c,
		requestMeta,
		model.Price{OutputPrice: 1, OutputPriceUnit: 1},
		&relaycontroller.HandleResult{
			Usage: model.Usage{
				OutputTokens: 5,
				TotalTokens:  5,
			},
			UpstreamID: "video-789",
			AsyncUsage: true,
		},
		0,
		true,
		nil,
	)
	consume.Wait()
	model.ProcessBatchUpdatesSummary()

	var logEntry model.GroupChannelLog
	require.NoError(t, db.Where("request_id = ?", requestMeta.RequestID).First(&logEntry).Error)
	require.Equal(t, model.AsyncUsageStatusNone, logEntry.AsyncUsageStatus)
	require.Equal(t, model.ZeroNullInt64(5), logEntry.Usage.OutputTokens)
	require.Equal(t, 5.0, logEntry.Amount.UsedAmount)
}

func TestScopedAdaptorStoreFillsEmptyStoreFields(t *testing.T) {
	base := &recordingAdaptorStore{}
	m := meta.NewMeta(
		&model.Channel{ID: 12},
		mode.Responses,
		"gpt-5",
		model.ModelConfig{},
		meta.WithGroup(model.GroupCache{ID: "group-1"}),
		meta.WithToken(model.TokenCache{ID: 22}),
		meta.WithChannelScope(model.ChannelScopeGroup, "group-1"),
	)
	store := newScopedAdaptorStore(base, m)

	require.NoError(t, store.SaveStore(adaptor.StoreCache{
		ID: model.ResponseStoreID("resp_scope_fields"),
	}, model.ChannelScopeGroup))

	require.Len(t, base.saved, 1)
	require.Equal(t, "group-1", base.saved[0].GroupID)
	require.Equal(t, 22, base.saved[0].TokenID)
	require.Equal(t, model.ChannelScopeGroup, base.saveScope)
	require.Equal(t, 12, base.saved[0].ChannelID)
	require.Equal(t, "gpt-5", base.saved[0].Model)
}

func TestScopedAdaptorStorePreservesExplicitStoreFieldsAndOverridesScope(t *testing.T) {
	base := &recordingAdaptorStore{}
	m := meta.NewMeta(
		&model.Channel{ID: 12},
		mode.Responses,
		"gpt-5",
		model.ModelConfig{},
		meta.WithGroup(model.GroupCache{ID: "group-1"}),
		meta.WithToken(model.TokenCache{ID: 22}),
		meta.WithChannelScope(model.ChannelScopeGroup, "group-1"),
	)
	store := newScopedAdaptorStore(base, m)

	require.NoError(t, store.SaveStore(adaptor.StoreCache{
		ID:        model.ResponseStoreID("resp_explicit_scope_fields"),
		GroupID:   "other-group",
		TokenID:   33,
		ChannelID: 44,
		Model:     "other-model",
	}, model.ChannelScopeGroup))

	require.Len(t, base.saved, 1)
	require.Equal(t, "other-group", base.saved[0].GroupID)
	require.Equal(t, 33, base.saved[0].TokenID)
	require.Equal(t, model.ChannelScopeGroup, base.saveScope)
	require.Equal(t, 44, base.saved[0].ChannelID)
	require.Equal(t, "other-model", base.saved[0].Model)
}

func TestBuildRequestDetailForLogSkipsRequestBodyForUpstreamOnlyStatuses(t *testing.T) {
	t.Parallel()

	bodyDetail := &relaycontroller.BodyDetail{
		RequestBody:  `{"prompt":"secret"}`,
		ResponseBody: `{"error":"upstream"}`,
	}

	for _, statusCode := range []int{
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusTooManyRequests,
	} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			t.Parallel()

			detail := buildRequestDetailForLog(bodyDetail, model.ModelConfig{}, statusCode, false)

			require.NotNil(t, detail)
			assert.Empty(t, detail.RequestBody)
			assert.Equal(t, `{"error":"upstream"}`, detail.ResponseBody)
		})
	}
}

func TestBuildRequestDetailForLogKeepsRequestBodyWhenForced(t *testing.T) {
	t.Parallel()

	detail := buildRequestDetailForLog(
		&relaycontroller.BodyDetail{
			RequestBody:  `{"prompt":"secret"}`,
			ResponseBody: `{"error":"limited"}`,
		},
		model.ModelConfig{},
		http.StatusTooManyRequests,
		true,
	)

	require.NotNil(t, detail)
	assert.Equal(t, `{"prompt":"secret"}`, detail.RequestBody)
	assert.Equal(t, `{"error":"limited"}`, detail.ResponseBody)
}

func TestBuildRequestDetailForLogKeepsRequestBodyForClientPayloadErrors(t *testing.T) {
	t.Parallel()

	detail := buildRequestDetailForLog(
		&relaycontroller.BodyDetail{
			RequestBody:  `{"prompt":"secret"}`,
			ResponseBody: `{"error":"bad request"}`,
		},
		model.ModelConfig{},
		http.StatusBadRequest,
		false,
	)

	require.NotNil(t, detail)
	assert.Equal(t, `{"prompt":"secret"}`, detail.RequestBody)
	assert.Equal(t, `{"error":"bad request"}`, detail.ResponseBody)
}

func TestBuildRequestDetailForLogDropsInvalidUTF8Bodies(t *testing.T) {
	t.Parallel()

	detail := buildRequestDetailForLog(
		&relaycontroller.BodyDetail{
			RequestBody:  string([]byte{0xff, 0xfe}),
			ResponseBody: string([]byte{'o', 'k', 0xff}),
		},
		model.ModelConfig{},
		http.StatusBadRequest,
		false,
	)

	require.NotNil(t, detail)
	assert.Empty(t, detail.RequestBody)
	assert.Empty(t, detail.ResponseBody)
}
