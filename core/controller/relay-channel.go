package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	relaymeta "github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/plugin/cachefollow"
)

const (
	AIProxyChannelHeader = "Aiproxy-Channel"
	// maxRetryErrorRate is the maximum error rate threshold for channel retry selection
	// Channels with error rate higher than this will be filtered out during retry
	maxRetryErrorRate = 0.85
	// errorRatePenaltyBase smooths low-error channels so tiny differences near zero
	// do not create outsized weight gaps.
	errorRatePenaltyBase = 0.10
	// errorRatePenalty controls how aggressively unhealthy channels are down-weighted.
	// With base=0.10 and alpha=2, low-error channels remain relatively close while
	// medium/high-error channels are penalized much more strongly.
	errorRatePenalty = 2.0
)

func supportModeMeta(
	mc *model.ModelCaches,
	channel *model.Channel,
	modelName string,
	m mode.Mode,
) *relaymeta.Meta {
	modelConfig := model.ModelConfig{}
	if mc != nil && mc.ModelConfig != nil {
		modelConfig, _ = mc.ModelConfig.GetModelConfig(modelName)
	}

	return relaymeta.NewMeta(channel, m, modelName, modelConfig)
}

func adaptorSupportsMode(
	a adaptor.Adaptor,
	mc *model.ModelCaches,
	channel *model.Channel,
	modelName string,
	m mode.Mode,
) bool {
	return a.SupportMode(supportModeMeta(mc, channel, modelName, m))
}

func GetChannelFromHeader(
	header string,
	mc *model.ModelCaches,
	modelName string,
	m mode.Mode,
) (*model.Channel, error) {
	channelIDInt, err := strconv.Atoi(header)
	if err != nil {
		return nil, err
	}

	channel, ok := mc.ChannelsByID[channelIDInt]
	if !ok {
		return nil, fmt.Errorf("channel %d not found", channelIDInt)
	}

	if !config.EnableAdminBypassChannelModelCheck && !slices.Contains(channel.Models, modelName) {
		return nil, fmt.Errorf("channel %d not found for model `%s`", channelIDInt, modelName)
	}

	a, ok := adaptors.GetAdaptor(channel.Type)
	if !ok {
		return nil, fmt.Errorf("adaptor not found for channel %d", channel.ID)
	}

	if !adaptorSupportsMode(a, mc, channel, modelName, m) {
		return nil, fmt.Errorf("channel %d not supported by adaptor", channel.ID)
	}

	return channel, nil
}

func needPinChannel(m mode.Mode) bool {
	switch m {
	case mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.VideosGet,
		mode.VideosContent,
		mode.VideosDelete,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		return true
	default:
		return false
	}
}

func GetChannelFromRequest(
	c *gin.Context,
	mc *model.ModelCaches,
	availableSet []string,
	modelName string,
	m mode.Mode,
) (*model.Channel, error) {
	channelID := middleware.GetChannelID(c)
	if channelID == 0 {
		if needPinChannel(m) {
			return nil, fmt.Errorf("%s need pinned channel", m)
		}
		return nil, nil
	}

	for _, set := range availableSet {
		enabledChannels := mc.EnabledModel2ChannelsBySet[set][modelName]
		if len(enabledChannels) > 0 {
			for _, channel := range enabledChannels {
				if channel.ID == channelID {
					a, ok := adaptors.GetAdaptor(channel.Type)
					if !ok {
						return nil, fmt.Errorf(
							"adaptor not found for pinned channel %d",
							channel.ID,
						)
					}

					if !adaptorSupportsMode(a, mc, channel, modelName, m) {
						return nil, fmt.Errorf(
							"pinned channel %d not supported by adaptor",
							channel.ID,
						)
					}

					return channel, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("pinned channel %d not found for model `%s`", channelID, modelName)
}

var (
	ErrChannelsNotFound  = errors.New("channels not found")
	ErrChannelsExhausted = errors.New("channels exhausted")
)

func getAvailableChannels(
	mc *model.ModelCaches,
	availableSet []string,
	modelName string,
	mode mode.Mode,
) ([]*model.Channel, error) {
	channelMap := make(map[int]*model.Channel)
	if len(availableSet) != 0 {
		for _, set := range availableSet {
			channels := mc.EnabledModel2ChannelsBySet[set][modelName]
			for _, channel := range channels {
				a, ok := adaptors.GetAdaptor(channel.Type)
				if !ok {
					continue
				}

				if !adaptorSupportsMode(a, mc, channel, modelName, mode) {
					continue
				}

				channelMap[channel.ID] = channel
			}
		}
	} else {
		for _, sets := range mc.EnabledModel2ChannelsBySet {
			for _, channel := range sets[modelName] {
				a, ok := adaptors.GetAdaptor(channel.Type)
				if !ok {
					continue
				}

				if !adaptorSupportsMode(a, mc, channel, modelName, mode) {
					continue
				}

				channelMap[channel.ID] = channel
			}
		}
	}

	if len(channelMap) == 0 {
		return nil, ErrChannelsNotFound
	}

	migratedChannels := make([]*model.Channel, 0, len(channelMap))
	for _, channel := range channelMap {
		migratedChannels = append(migratedChannels, channel)
	}

	return migratedChannels, nil
}

func getPriorityWeight(channel *model.Channel, errorRate float64) float64 {
	priority := float64(channel.GetPriority())
	if priority <= 0 {
		return 0
	}

	if errorRate > 1 {
		errorRate = 1
	} else if errorRate < 0 {
		errorRate = 0
	}

	// Weight starts from configured priority and is then reduced by a smoothed
	// error-rate penalty, which keeps low-error channels stable while still
	// strongly penalizing unhealthy channels.
	return priority / math.Pow(errorRate+errorRatePenaltyBase, errorRatePenalty)
}

func getChannelErrorRate(errorRates map[int64]float64, channelID int64) float64 {
	if errorRates == nil {
		return 0
	}

	return errorRates[channelID]
}

func pickChannel(
	channels []*model.Channel,
	errorRates map[int64]float64,
) (*model.Channel, error) {
	if len(channels) == 0 {
		return nil, ErrChannelsExhausted
	}

	if len(channels) == 1 {
		return channels[0], nil
	}

	var totalWeight float64

	cachedWeights := make([]float64, len(channels))
	for i, ch := range channels {
		weight := getPriorityWeight(ch, getChannelErrorRate(errorRates, int64(ch.ID)))
		totalWeight += weight
		cachedWeights[i] = weight
	}

	if totalWeight == 0 {
		return channels[rand.IntN(len(channels))], nil
	}

	r := rand.Float64() * totalWeight
	for i, ch := range channels {
		r -= cachedWeights[i]
		if r < 0 {
			return ch, nil
		}
	}

	return channels[rand.IntN(len(channels))], nil
}

func getChannelWithFallback(
	cache *model.ModelCaches,
	availableSet []string,
	modelName string,
	mode mode.Mode,
	preferChannelIDs []int,
	errorRates map[int64]float64,
	ignoreChannelIDs map[int64]struct{},
) (*initialChannel, error) {
	migratedChannels, err := getAvailableChannels(
		cache,
		availableSet,
		modelName,
		mode,
	)
	if err != nil {
		return nil, err
	}

	initial := &initialChannel{
		preferChannelIDs: preferChannelIDs,
		ignoreChannelIDs: ignoreChannelIDs,
		migratedChannels: migratedChannels,
	}

	filtered := filterChannels(migratedChannels, errorRates, maxRetryErrorRate, ignoreChannelIDs)

	channel, err := initial.selectChannel(filtered, preferChannelIDs, errorRates)
	if err == nil {
		initial.channel = channel
		return initial, nil
	}

	primaryChannels := nonBackupChannels(migratedChannels)
	backupChannels := backupOnlyChannels(migratedChannels)

	// Preserve the initial request's last-resort fallbacks after healthy backups.
	pipeline := []func() []*model.Channel{
		func() []*model.Channel {
			return filterChannels(primaryChannels, errorRates, 0, ignoreChannelIDs)
		},
		func() []*model.Channel {
			return filterChannels(backupChannels, errorRates, 0, ignoreChannelIDs)
		},
		func() []*model.Channel {
			return filterChannels(primaryChannels, errorRates, 0)
		},
		func() []*model.Channel {
			return filterChannels(backupChannels, errorRates, 0)
		},
	}
	for _, step := range pipeline {
		channel, err := pickChannel(step(), errorRates)
		if err == nil {
			initial.channel = channel
			return initial, nil
		}
	}

	return nil, ErrChannelsExhausted
}

type channelSelectionState struct {
	backupOnlyEnabled bool // Remains enabled across retry rounds for this request.
}

// Preferences bypass backup-only gating, while health and failure filters still apply.
func (s *channelSelectionState) selectChannel(
	channels []*model.Channel,
	preferChannelIDs []int,
	errorRates map[int64]float64,
) (*model.Channel, error) {
	if channel := pickPreferredChannel(channels, preferChannelIDs); channel != nil {
		return channel, nil
	}

	for {
		candidates := channels
		if !s.backupOnlyEnabled {
			candidates = nonBackupChannels(channels)
		}

		channel, err := pickChannel(candidates, errorRates)
		if err == nil || s.backupOnlyEnabled {
			return channel, err
		}

		s.backupOnlyEnabled = true
	}
}

func nonBackupChannels(channels []*model.Channel) []*model.Channel {
	primary := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel != nil && !channel.BackupOnly {
			primary = append(primary, channel)
		}
	}

	return primary
}

func backupOnlyChannels(channels []*model.Channel) []*model.Channel {
	backups := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel != nil && channel.BackupOnly {
			backups = append(backups, channel)
		}
	}

	return backups
}

func pickPreferredChannel(
	channels []*model.Channel,
	preferChannelIDs []int,
) *model.Channel {
	if len(channels) == 0 || len(preferChannelIDs) == 0 {
		return nil
	}

	channelMap := make(map[int]*model.Channel, len(channels))
	for _, channel := range channels {
		channelMap[channel.ID] = channel
	}

	seen := make(map[int]struct{}, len(preferChannelIDs))
	for _, channelID := range preferChannelIDs {
		if _, ok := seen[channelID]; ok {
			continue
		}

		seen[channelID] = struct{}{}
		if channel, ok := channelMap[channelID]; ok {
			return channel
		}
	}

	return nil
}

type initialChannel struct {
	channelSelectionState

	channel           *model.Channel
	designatedChannel bool
	preferChannelIDs  []int
	ignoreChannelIDs  map[int64]struct{}
	migratedChannels  []*model.Channel
}

func getInitialChannel(c *gin.Context, modelName string, m mode.Mode) (*initialChannel, error) {
	log := common.GetLogger(c)

	group := middleware.GetGroup(c)
	availableSet := group.GetAvailableSets()

	if channelHeader := c.Request.Header.Get(AIProxyChannelHeader); channelHeader != "" {
		if group.Status != model.GroupStatusInternal {
			return nil, errors.New("channel header is not allowed in non-internal group")
		}

		channel, err := GetChannelFromHeader(
			channelHeader,
			middleware.GetModelCaches(c),
			modelName,
			m,
		)
		if err != nil {
			return nil, err
		}

		log.Data["designated_channel"] = "true"

		return &initialChannel{channel: channel, designatedChannel: true}, nil
	}

	channel, err := GetChannelFromRequest(
		c,
		middleware.GetModelCaches(c),
		availableSet,
		modelName,
		m,
	)
	if err != nil {
		return nil, err
	}

	if channel != nil {
		return &initialChannel{channel: channel, designatedChannel: true}, nil
	}

	mc := middleware.GetModelCaches(c)

	ignoreChannelIDs, err := monitor.GetBannedChannelsMapWithModel(c.Request.Context(), modelName)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		log.Errorf("get %s auto banned channels failed: %+v", modelName, err)
	}

	log.Debugf("%s model banned channels: %+v", modelName, ignoreChannelIDs)

	errorRates, err := monitor.GetModelChannelErrorRate(c.Request.Context(), modelName)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		log.Errorf("get channel model error rates failed: %+v", err)
	}

	preferChannelIDs := getPreferChannelIDs(c, modelName, m)

	if len(preferChannelIDs) > 0 {
		log.Data["prefer_channels"] = fmt.Sprintf("%v", preferChannelIDs)
	}

	return getChannelWithFallback(
		mc,
		availableSet,
		modelName,
		m,
		preferChannelIDs,
		errorRates,
		ignoreChannelIDs,
	)
}

func supportsPromptCacheKeyMode(m mode.Mode) bool {
	switch m {
	case mode.Responses, mode.ResponsesCompact, mode.ChatCompletions:
		return true
	default:
		return false
	}
}

func supportsCacheFollowMode(m mode.Mode) bool {
	switch m {
	case mode.Responses,
		mode.ResponsesCompact,
		mode.ChatCompletions,
		mode.Gemini,
		mode.GeminiVideo,
		mode.GeminiTTS,
		mode.GeminiImage,
		mode.Anthropic:
		return true
	default:
		return false
	}
}

func getCacheFollowConfig(c *gin.Context) (cachefollow.Config, bool) {
	v, ok := c.Get(middleware.ModelConfig)
	if !ok {
		return cachefollow.Config{}, false
	}

	modelConfig, ok := v.(model.ModelConfig)
	if !ok {
		panic(fmt.Sprintf("model config type error: %T, %v", v, v))
	}

	pluginConfig := cachefollow.Config{}
	if err := modelConfig.LoadPluginConfig(cachefollow.PluginName, &pluginConfig); err != nil {
		return cachefollow.Config{}, false
	}

	if !pluginConfig.Enable {
		return cachefollow.Config{}, false
	}

	return pluginConfig, true
}

func getPreferChannelIDs(c *gin.Context, modelName string, m mode.Mode) []int {
	pluginConfig, ok := getCacheFollowConfig(c)
	if !supportsCacheFollowMode(m) || !ok {
		return nil
	}

	group := middleware.GetGroup(c)
	token := middleware.GetToken(c)
	user := middleware.GetRequestUser(c)
	preferChannelIDs := make([]int, 0, 6)
	seen := make(map[int]struct{}, 6)

	appendChannelID := func(storeID string) {
		if storeID == "" {
			return
		}

		store, err := model.CacheGetStore(group.ID, token.ID, storeID)
		if err != nil || store.ChannelID == 0 {
			return
		}

		if _, ok := seen[store.ChannelID]; ok {
			return
		}

		seen[store.ChannelID] = struct{}{}
		preferChannelIDs = append(preferChannelIDs, store.ChannelID)
	}

	if supportsPromptCacheKeyMode(m) {
		if promptCacheKey := middleware.GetPromptCacheKey(c); promptCacheKey != "" {
			appendChannelID(
				model.PromptCacheStoreID(
					modelName,
					promptCacheKey,
					model.CacheKeyTypeStable,
				),
			)
			appendChannelID(
				model.PromptCacheStoreID(
					modelName,
					promptCacheKey,
					model.CacheKeyTypeRecent,
				),
			)
		}
	}

	if user != "" {
		appendChannelID(model.CacheFollowUserStoreID(modelName, user, model.CacheKeyTypeStable))
		appendChannelID(model.CacheFollowUserStoreID(modelName, user, model.CacheKeyTypeRecent))
	}

	if pluginConfig.EnableGenericFollow {
		appendChannelID(model.CacheFollowStoreID(modelName, model.CacheKeyTypeStable))
		appendChannelID(model.CacheFollowStoreID(modelName, model.CacheKeyTypeRecent))
	}

	if len(preferChannelIDs) == 0 {
		return nil
	}

	return preferChannelIDs
}

func getWebSearchChannel(
	ctx context.Context,
	mc *model.ModelCaches,
	modelName string,
) (*model.Channel, error) {
	ignoreChannelIDs, _ := monitor.GetBannedChannelsMapWithModel(ctx, modelName)
	errorRates, _ := monitor.GetModelChannelErrorRate(ctx, modelName)

	initial, err := getChannelWithFallback(
		mc,
		nil,
		modelName,
		mode.ChatCompletions,
		nil,
		errorRates,
		ignoreChannelIDs,
	)
	if err != nil {
		return nil, err
	}

	return initial.channel, nil
}

func getRetryChannel(
	ctx context.Context,
	state *retryState,
) (*model.Channel, error) {
	errorRates, err := monitor.GetModelChannelErrorRate(ctx, state.meta.OriginModel)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}

	if state.designatedChannel != nil {
		// Explicitly selected channels stay pinned for the request.
		channelID := int64(state.designatedChannel.ID)
		if _, ignored := state.ignoreChannelIDs[channelID]; ignored {
			return nil, ErrChannelsExhausted
		}

		if errorRate := getChannelErrorRate(errorRates, channelID); errorRate > maxRetryErrorRate {
			return nil, ErrChannelsExhausted
		}

		return state.designatedChannel, nil
	}

	return state.selectRetryChannel(errorRates)
}

func (s *retryState) selectRetryChannel(
	errorRates map[int64]float64,
) (*model.Channel, error) {
	candidates := getRetryCandidates(s, errorRates)
	if channel := pickPreferredChannel(candidates, s.preferChannelIDs); channel != nil {
		return channel, nil
	}

	if !s.backupOnlyEnabled && len(candidates) > 0 && len(nonBackupChannels(candidates)) == 0 {
		// Adding eligible backups starts a fresh round while preserving cache preferences.
		s.backupOnlyEnabled = true
		s.failedChannelIDs = make(map[int64]struct{})
		candidates = getRetryCandidates(s, errorRates)
	}

	newChannel, err := s.selectChannel(
		candidates,
		s.preferChannelIDs,
		errorRates,
	)
	if err != nil {
		if !errors.Is(err, ErrChannelsExhausted) || len(s.failedChannelIDs) == 0 {
			return nil, err
		}

		// Start a new round so every currently eligible channel gets another attempt.
		s.failedChannelIDs = make(map[int64]struct{})
		s.preferChannelIDs = nil

		return s.selectChannel(
			getRetryCandidates(s, errorRates),
			s.preferChannelIDs,
			errorRates,
		)
	}

	return newChannel, nil
}

func getRetryCandidates(
	state *retryState,
	errorRates map[int64]float64,
) []*model.Channel {
	return filterChannels(
		state.migratedChannels,
		errorRates,
		maxRetryErrorRate,
		state.ignoreChannelIDs,
		state.failedChannelIDs,
	)
}

func filterChannels(
	channels []*model.Channel,
	errorRates map[int64]float64,
	maxErrorRate float64,
	ignoreChannel ...map[int64]struct{},
) []*model.Channel {
	filtered := make([]*model.Channel, 0)
	for _, channel := range channels {
		if !isChannelSelectable(channel, errorRates, maxErrorRate, ignoreChannel...) {
			continue
		}

		filtered = append(filtered, channel)
	}

	return filtered
}

func isChannelSelectable(
	channel *model.Channel,
	errorRates map[int64]float64,
	maxErrorRate float64,
	ignoreChannel ...map[int64]struct{},
) bool {
	if channel == nil || channel.Status != model.ChannelStatusEnabled {
		return false
	}

	chid := int64(channel.ID)

	if maxErrorRate != 0 {
		// Filter out channels with error rate higher than threshold
		// This avoids amplifying attacks and retrying with bad channels.
		if errorRate, ok := errorRates[chid]; ok && errorRate > maxErrorRate {
			return false
		}
	}

	for _, ignores := range ignoreChannel {
		if ignores == nil {
			continue
		}

		if _, needIgnore := ignores[chid]; needIgnore {
			return false
		}
	}

	return true
}
