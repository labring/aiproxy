package controller

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"github.com/labring/aiproxy/core/relay/mode"
)

const (
	AIProxyChannelHeader = "Aiproxy-Channel"
	// maxRetryErrorRate is the maximum error rate threshold for channel retry selection
	// Channels with error rate higher than this will be filtered out during retry
	maxRetryErrorRate = 0.75
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

func GetRandomChannel(
	mc *model.ModelCaches,
	availableSet []string,
	modelName string,
	mode mode.Mode,
	errorRates map[int64]float64,
	maxErrorRate float64,
	ignoreChannelMap map[int64]struct{},
) (*model.Channel, []*model.Channel, error) {
	channelMap := make(map[int]*model.Channel)
	if len(availableSet) != 0 {
		for _, set := range availableSet {
			for _, channel := range mc.EnabledModel2ChannelsBySet[set][modelName] {
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

	migratedChannels := make([]*model.Channel, 0, len(channelMap))
	for _, channel := range channelMap {
		migratedChannels = append(migratedChannels, channel)
	}

	channel, err := ignoreChannel(
		migratedChannels,
		mode,
		errorRates,
		maxErrorRate,
		ignoreChannelMap,
	)

	return channel, migratedChannels, err
}

func getPriority(channel *model.Channel, errorRate float64) int32 {
	priority := channel.GetPriority()

	if errorRate > 1 {
		errorRate = 1
	} else if errorRate < 0.1 {
		errorRate = 0.1
	}

	return int32(float64(priority) / errorRate)
}

func ignoreChannel(
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

	var totalWeight int32

	cachedPrioritys := make([]int32, len(channels))
	for i, ch := range channels {
		priority := getPriority(ch, errorRates[int64(ch.ID)])
		totalWeight += priority
		cachedPrioritys[i] = priority
	}

	if totalWeight == 0 {
		return channels[rand.IntN(len(channels))], nil
	}

	r := rand.Int32N(totalWeight)
	for i, ch := range channels {
		r -= cachedPrioritys[i]
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
	errorRates map[int64]float64,
	ignoreChannelIDs map[int64]struct{},
) (*model.Channel, []*model.Channel, error) {
	channel, migratedChannels, err := GetRandomChannel(
		cache,
		availableSet,
		modelName,
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

	channel, migratedChannels, err = GetRandomChannel(
		cache,
		availableSet,
		modelName,
		mode,
		errorRates,
		0,
		nil,
	)

	return channel, migratedChannels, err
}

type initialChannel struct {
	channel           *model.Channel
	designatedChannel bool
	ignoreChannelIDs  map[int64]struct{}
	errorRates        map[int64]float64
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
		log.Errorf("get %s auto banned channels failed: %+v", modelName, err)
	}

	log.Debugf("%s model banned channels: %+v", modelName, ignoreChannelIDs)

	errorRates, err := monitor.GetModelChannelErrorRate(c.Request.Context(), modelName)
	if err != nil {
		log.Errorf("get channel model error rates failed: %+v", err)
	}

	channel, migratedChannels, err := getChannelWithFallback(
		mc,
		availableSet,
		modelName,
		m,
		errorRates,
		ignoreChannelIDs,
	)
	if err != nil {
		return nil, err
	}

	return &initialChannel{
		channel:          channel,
		ignoreChannelIDs: ignoreChannelIDs,
		errorRates:       errorRates,
		migratedChannels: migratedChannels,
	}, nil
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
		errorRates,
		ignoreChannelIDs)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

func getRetryChannel(state *retryState, currentRetry, totalRetries int) (*model.Channel, error) {
	if state.exhausted {
		if state.lastHasPermissionChannel == nil {
			return nil, ErrChannelsExhausted
		}

		// Check if lastHasPermissionChannel has high error rate
		// If so, return exhausted to prevent retrying with a bad channel
		channelID := int64(state.lastHasPermissionChannel.ID)
		if errorRate, ok := state.errorRates[channelID]; ok && errorRate > maxRetryErrorRate {
			return nil, ErrChannelsExhausted
		}

		return state.lastHasPermissionChannel, nil
	}

	// For the last retry, filter out all previously failed channels if there are other options
	if currentRetry == totalRetries-1 && len(state.failedChannelIDs) > 0 {
		// Check if there are channels available after filtering out failed channels
		newChannel, err := ignoreChannel(
			state.migratedChannels,
			state.meta.Mode,
			state.errorRates,
			maxRetryErrorRate,
			state.ignoreChannelIDs,
			state.failedChannelIDs,
		)
		if err == nil {
			return newChannel, nil
		}
		// If no channels available after filtering, fall back to not using failed channels filter
	}

	newChannel, err := ignoreChannel(
		state.migratedChannels,
		state.meta.Mode,
		state.errorRates,
		maxRetryErrorRate,
		state.ignoreChannelIDs,
	)
	if err != nil {
		if !errors.Is(err, ErrChannelsExhausted) || state.lastHasPermissionChannel == nil {
			return nil, err
		}

		// Check if lastHasPermissionChannel has high error rate before using it
		channelID := int64(state.lastHasPermissionChannel.ID)
		if errorRate, ok := state.errorRates[channelID]; ok && errorRate > maxRetryErrorRate {
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
