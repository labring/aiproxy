package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/plugin/cachefollow"
)

const (
	AIProxyChannelHeader = "Aiproxy-Channel"
	// maxRetryErrorRate is the maximum error rate threshold for channel retry selection
	// Channels with error rate higher than this will be filtered out during retry
	maxRetryErrorRate = 0.75
	// errorRatePenaltyBase smooths low-error channels so tiny differences near zero
	// do not create outsized weight gaps.
	errorRatePenaltyBase = 0.10
	// errorRatePenalty controls how aggressively unhealthy channels are down-weighted.
	// With base=0.10 and alpha=2, low-error channels remain relatively close while
	// medium/high-error channels are penalized much more strongly.
	errorRatePenalty = 2.0
)

func GetChannelFromHeader(
	header string,
	mc *model.ModelCaches,
	availableSet []string,
	model string,
	m mode.Mode,
) (*model.Channel, error) {
	channelIDInt, err := strconv.ParseInt(header, 10, 64)
	if err != nil {
		return nil, err
	}

	for _, set := range availableSet {
		enabledChannels := mc.EnabledModel2ChannelsBySet[set][model]
		if len(enabledChannels) > 0 {
			for _, channel := range enabledChannels {
				if int64(channel.ID) == channelIDInt {
					a, ok := adaptors.GetAdaptor(channel.Type)
					if !ok {
						return nil, fmt.Errorf("adaptor not found for channel %d", channel.ID)
					}

					if !a.SupportMode(m) {
						return nil, fmt.Errorf("channel %d not supported by adaptor", channel.ID)
					}

					return channel, nil
				}
			}
		}

		disabledChannels := mc.DisabledModel2ChannelsBySet[set][model]
		if len(disabledChannels) > 0 {
			for _, channel := range disabledChannels {
				if int64(channel.ID) == channelIDInt {
					a, ok := adaptors.GetAdaptor(channel.Type)
					if !ok {
						return nil, fmt.Errorf("adaptor not found for channel %d", channel.ID)
					}

					if !a.SupportMode(m) {
						return nil, fmt.Errorf("channel %d not supported by adaptor", channel.ID)
					}

					return channel, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("channel %d not found for model `%s`", channelIDInt, model)
}

func needPinChannel(m mode.Mode) bool {
	switch m {
	case mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
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

					if !a.SupportMode(m) {
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

				if !a.SupportMode(mode) {
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

				if !a.SupportMode(mode) {
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
	mode mode.Mode,
	errorRates map[int64]float64,
	maxErrorRate float64,
	ignoreChannelIDs ...map[int64]struct{},
) (*model.Channel, error) {
	if len(channels) == 0 {
		return nil, ErrChannelsNotFound
	}

	channels = filterChannels(channels, mode, errorRates, maxErrorRate, ignoreChannelIDs...)
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
) (*model.Channel, []*model.Channel, error) {
	migratedChannels, err := getAvailableChannels(
		cache,
		availableSet,
		modelName,
		mode,
	)
	if err != nil {
		return nil, migratedChannels, err
	}

	if len(preferChannelIDs) > 0 {
		channel := pickPreferredChannel(
			migratedChannels,
			mode,
			preferChannelIDs,
			errorRates,
			maxRetryErrorRate,
			ignoreChannelIDs,
		)
		if channel != nil {
			return channel, migratedChannels, nil
		}
	}

	channel, err := pickChannel(
		migratedChannels,
		mode,
		errorRates,
		maxRetryErrorRate,
		ignoreChannelIDs,
	)
	if err == nil {
		return channel, migratedChannels, nil
	}

	if !errors.Is(err, ErrChannelsExhausted) {
		return nil, migratedChannels, err
	}

	channel, err = pickChannel(
		migratedChannels,
		mode,
		errorRates,
		0,
		ignoreChannelIDs,
	)

	return channel, migratedChannels, err
}

func pickPreferredChannel(
	channels []*model.Channel,
	mode mode.Mode,
	preferChannelIDs []int,
	errorRates map[int64]float64,
	maxErrorRate float64,
	ignoreChannelIDs ...map[int64]struct{},
) *model.Channel {
	if len(preferChannelIDs) == 0 {
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
			filtered := filterChannels(
				[]*model.Channel{channel},
				mode,
				errorRates,
				maxErrorRate,
				ignoreChannelIDs...,
			)
			if len(filtered) > 0 {
				return filtered[0]
			}
		}
	}

	return nil
}

type initialChannel struct {
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
			availableSet,
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

	channel, migratedChannels, err := getChannelWithFallback(
		mc,
		availableSet,
		modelName,
		m,
		preferChannelIDs,
		errorRates,
		ignoreChannelIDs,
	)
	if err != nil {
		return nil, err
	}

	return &initialChannel{
		channel:          channel,
		preferChannelIDs: preferChannelIDs,
		ignoreChannelIDs: ignoreChannelIDs,
		migratedChannels: migratedChannels,
	}, nil
}

func supportsPromptCacheKeyMode(m mode.Mode) bool {
	switch m {
	case mode.Responses, mode.ChatCompletions:
		return true
	default:
		return false
	}
}

func supportsCacheFollowMode(m mode.Mode) bool {
	switch m {
	case mode.Responses, mode.ChatCompletions, mode.Gemini, mode.Anthropic:
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

	channel, _, err := getChannelWithFallback(
		mc,
		nil,
		modelName,
		mode.ChatCompletions,
		nil,
		errorRates,
		ignoreChannelIDs)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

func getRetryChannel(
	ctx context.Context,
	state *retryState,
	currentRetry, totalRetries int,
) (*model.Channel, error) {
	errorRates, err := monitor.GetModelChannelErrorRate(ctx, state.meta.OriginModel)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}

	if state.exhausted {
		if state.lastHasPermissionChannel == nil {
			return nil, ErrChannelsExhausted
		}

		// Check if lastHasPermissionChannel has high error rate
		// If so, return exhausted to prevent retrying with a bad channel
		channelID := int64(state.lastHasPermissionChannel.ID)
		if errorRate := getChannelErrorRate(errorRates, channelID); errorRate > maxRetryErrorRate {
			return nil, ErrChannelsExhausted
		}

		return state.lastHasPermissionChannel, nil
	}

	if len(state.preferChannelIDs) > 0 {
		newChannel := pickPreferredChannel(
			state.migratedChannels,
			state.meta.Mode,
			state.preferChannelIDs,
			errorRates,
			maxRetryErrorRate,
			state.ignoreChannelIDs,
			state.failedChannelIDs,
		)
		if newChannel != nil {
			return newChannel, nil
		}
	}

	// For the last retry, filter out all previously failed channels if there are other options
	if currentRetry == totalRetries-1 && len(state.failedChannelIDs) > 0 {
		// Check if there are channels available after filtering out failed channels
		newChannel, err := pickChannel(
			state.migratedChannels,
			state.meta.Mode,
			errorRates,
			maxRetryErrorRate,
			state.ignoreChannelIDs,
			state.failedChannelIDs,
		)
		if err == nil {
			return newChannel, nil
		}
		// If no channels available after filtering, fall back to not using failed channels filter
	}

	ignoreChannelIDs := []map[int64]struct{}{state.ignoreChannelIDs}
	if currentRetry != totalRetries-1 {
		ignoreChannelIDs = append(ignoreChannelIDs, state.failedChannelIDs)
	}

	newChannel, err := pickChannel(
		state.migratedChannels,
		state.meta.Mode,
		errorRates,
		maxRetryErrorRate,
		ignoreChannelIDs...,
	)
	if err != nil {
		if !errors.Is(err, ErrChannelsExhausted) || state.lastHasPermissionChannel == nil {
			return nil, err
		}

		// Check if lastHasPermissionChannel has high error rate before using it
		channelID := int64(state.lastHasPermissionChannel.ID)
		if errorRate := getChannelErrorRate(errorRates, channelID); errorRate > maxRetryErrorRate {
			return nil, ErrChannelsExhausted
		}

		state.exhausted = true

		return state.lastHasPermissionChannel, nil
	}

	return newChannel, nil
}

func filterChannels(
	channels []*model.Channel,
	mode mode.Mode,
	errorRates map[int64]float64,
	maxErrorRate float64,
	ignoreChannel ...map[int64]struct{},
) []*model.Channel {
	filtered := make([]*model.Channel, 0)
	for _, channel := range channels {
		if channel.Status != model.ChannelStatusEnabled {
			continue
		}

		a, ok := adaptors.GetAdaptor(channel.Type)
		if !ok {
			continue
		}

		if !a.SupportMode(mode) {
			continue
		}

		chid := int64(channel.ID)

		if maxErrorRate != 0 {
			// Filter out channels with error rate higher than threshold
			// This avoids amplifying attacks and retrying with bad channels
			if errorRate, ok := errorRates[chid]; ok && errorRate > maxErrorRate {
				continue
			}
		}

		needIgnore := false

		for _, ignores := range ignoreChannel {
			if ignores == nil {
				continue
			}

			_, needIgnore = ignores[chid]
			if needIgnore {
				break
			}
		}

		if needIgnore {
			continue
		}

		filtered = append(filtered, channel)
	}

	return filtered
}
