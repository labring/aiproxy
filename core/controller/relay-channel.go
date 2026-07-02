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
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	relaymeta "github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/plugin/cachefollow"
)

type scopedChannel struct {
	channel *model.Channel
	scope   model.ChannelScope
	groupID string
}

func newGlobalScopedChannel(channel *model.Channel) *scopedChannel {
	return &scopedChannel{channel: channel, scope: model.ChannelScopeGlobal}
}

func newGroupScopedChannel(groupID string, channel *model.Channel) *scopedChannel {
	return &scopedChannel{channel: channel, scope: model.ChannelScopeGroup, groupID: groupID}
}

func (c *scopedChannel) monitorKey() string {
	if c == nil || c.channel == nil {
		return "0"
	}

	if c.scope == model.ChannelScopeGroup {
		return model.GroupChannelMonitorKey(c.groupID, c.channel.ID)
	}

	return strconv.Itoa(c.channel.ID)
}

func (c *scopedChannel) isGroupChannel() bool {
	return c != nil && c.scope == model.ChannelScopeGroup
}

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
	channel *model.Channel,
	modelName string,
	m mode.Mode,
	modelConfig model.ModelConfig,
) *relaymeta.Meta {
	return relaymeta.NewMeta(channel, m, modelName, modelConfig)
}

func supportModeModelConfig(mc *model.ModelCaches, modelName string) model.ModelConfig {
	modelConfig := model.ModelConfig{}
	if mc != nil && mc.ModelConfig != nil {
		modelConfig, _ = mc.ModelConfig.GetModelConfig(modelName)
	}

	return modelConfig
}

func groupChannelSupportModeModelConfig(groupID, modelName string) (model.ModelConfig, bool) {
	return model.ResolveGroupScopeModelConfig(groupID, modelName)
}

func adaptorSupportsMode(
	a adaptor.Adaptor,
	mc *model.ModelCaches,
	channel *model.Channel,
	modelName string,
	m mode.Mode,
) bool {
	return adaptorSupportsModeWithConfig(
		a,
		channel,
		modelName,
		m,
		supportModeModelConfig(mc, modelName),
	)
}

func adaptorSupportsModeWithConfig(
	a adaptor.Adaptor,
	channel *model.Channel,
	modelName string,
	m mode.Mode,
	modelConfig model.ModelConfig,
) bool {
	return a.SupportMode(supportModeMeta(channel, modelName, m, modelConfig))
}

func GetChannelFromHeader(
	header string,
	mc *model.ModelCaches,
	modelName string,
	m mode.Mode,
) (*model.Channel, error) {
	channelID, err := strconv.Atoi(header)
	if err != nil {
		return nil, err
	}

	channel := findGlobalHeaderChannelByID(mc, modelName, channelID)
	if channel == nil {
		return nil, fmt.Errorf("channel %d not found for model `%s`", channelID, modelName)
	}

	if !globalHeaderChannelSupportsRequest(channel, mc, modelName, m) {
		return nil, fmt.Errorf("channel %d not supported for model `%s`", channelID, modelName)
	}

	return channel, nil
}

func findEnabledGlobalChannelByID(
	mc *model.ModelCaches,
	availableSet []string,
	modelName string,
	channelID int,
) *model.Channel {
	if mc == nil || channelID == 0 {
		return nil
	}

	return findGlobalChannelByIDInSets(
		mc.EnabledModel2ChannelsBySet,
		availableSet,
		modelName,
		channelID,
	)
}

func findGlobalHeaderChannelByID(
	mc *model.ModelCaches,
	modelName string,
	channelID int,
) *model.Channel {
	if mc == nil || channelID == 0 {
		return nil
	}

	if channel := findGlobalChannelByIDInSets(
		mc.EnabledModel2ChannelsBySet,
		nil,
		modelName,
		channelID,
	); channel != nil {
		return channel
	}

	return findGlobalChannelByIDInSets(
		mc.DisabledModel2ChannelsBySet,
		nil,
		modelName,
		channelID,
	)
}

func findGlobalChannelByIDInSets(
	channelsBySet map[string]map[string][]*model.Channel,
	availableSet []string,
	modelName string,
	channelID int,
) *model.Channel {
	if availableSet != nil && len(availableSet) == 0 {
		return nil
	}

	findInSet := func(set string) *model.Channel {
		for _, channel := range channelsBySet[set][modelName] {
			if channel.ID == channelID {
				return channel
			}
		}

		return nil
	}

	for _, set := range availableSet {
		if channel := findInSet(set); channel != nil {
			return channel
		}
	}

	if len(availableSet) > 0 {
		return nil
	}

	for set := range channelsBySet {
		if channel := findInSet(set); channel != nil {
			return channel
		}
	}

	return nil
}

func globalHeaderChannelSupportsRequest(
	channel *model.Channel,
	mc *model.ModelCaches,
	modelName string,
	m mode.Mode,
) bool {
	if channel == nil {
		return false
	}

	if !model.ChannelSupportsModel(channel, modelName) {
		return false
	}

	a, ok := adaptors.GetAdaptor(channel.Type)
	if !ok {
		return false
	}

	return adaptorSupportsMode(a, mc, channel, modelName, m)
}

func GetGroupChannelFromHeader(
	header string,
	group model.GroupCache,
	availableSet []string,
	ignoreSetLimit bool,
	modelName string,
	m mode.Mode,
) (*scopedChannel, error) {
	channelID, err := strconv.Atoi(header)
	if err != nil {
		return nil, err
	}

	groupChannel, err := model.LoadGroupChannelByID(group.ID, channelID)
	if err != nil {
		return nil, fmt.Errorf("group channel %d not found for model `%s`", channelID, modelName)
	}

	if !groupChannelSupportsRequest(
		groupChannel,
		availableSet,
		ignoreSetLimit,
		modelName,
		m,
	) {
		return nil, fmt.Errorf(
			"group channel %d not supported for model `%s`",
			channelID,
			modelName,
		)
	}

	return newGroupScopedChannel(group.ID, groupChannel.ToChannel()), nil
}

func groupChannelSupportsRequest(
	groupChannel *model.GroupChannel,
	availableSet []string,
	ignoreSetLimit bool,
	modelName string,
	m mode.Mode,
) bool {
	if groupChannel == nil || groupChannel.Status != model.ChannelStatusEnabled {
		return false
	}

	if !model.GroupChannelSupportsModel(groupChannel, modelName) {
		return false
	}

	if !ignoreSetLimit && len(availableSet) > 0 &&
		!slices.ContainsFunc(groupChannel.GetSets(), func(set string) bool {
			return slices.Contains(availableSet, set)
		}) {
		return false
	}

	if !ignoreSetLimit && availableSet != nil && len(availableSet) == 0 {
		return false
	}

	channel := groupChannel.ToChannel()

	a, ok := adaptors.GetAdaptor(channel.Type)
	if !ok {
		return false
	}

	modelConfig, ok := groupChannelSupportModeModelConfig(groupChannel.GroupID, modelName)
	if !ok {
		return false
	}

	return adaptorSupportsModeWithConfig(
		a,
		channel,
		modelName,
		m,
		modelConfig,
	)
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

	channel := findEnabledGlobalChannelByID(mc, availableSet, modelName, channelID)
	if channel != nil {
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

	return nil, fmt.Errorf("pinned channel %d not found for model `%s`", channelID, modelName)
}

func GetScopedChannelFromRequest(
	c *gin.Context,
	mc *model.ModelCaches,
	availableSet []string,
	ignoreGroupChannelSetLimit bool,
	modelName string,
	m mode.Mode,
) (*scopedChannel, error) {
	channelID := middleware.GetChannelID(c)
	if channelID == 0 {
		if needPinChannel(m) {
			return nil, fmt.Errorf("%s need pinned channel", m)
		}
		return nil, nil
	}

	switch middleware.GetChannelScope(c) {
	case model.ChannelScopeGroup:
		group := middleware.GetGroup(c)

		groupChannel, err := model.LoadGroupChannelByID(group.ID, channelID)
		if err != nil {
			return nil, fmt.Errorf(
				"pinned group channel %d not found for model `%s`",
				channelID,
				modelName,
			)
		}

		if groupChannel.Status != model.ChannelStatusEnabled {
			return nil, fmt.Errorf("pinned group channel %d is disabled", channelID)
		}

		if !groupChannelSupportsRequest(
			groupChannel,
			availableSet,
			ignoreGroupChannelSetLimit,
			modelName,
			m,
		) {
			return nil, fmt.Errorf(
				"pinned group channel %d not supported for model `%s`",
				channelID,
				modelName,
			)
		}

		return newGroupScopedChannel(group.ID, groupChannel.ToChannel()), nil
	default:
		channel, err := GetChannelFromRequest(c, mc, availableSet, modelName, m)
		if err != nil || channel == nil {
			return nil, err
		}

		return newGlobalScopedChannel(channel), nil
	}
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
) ([]*scopedChannel, error) {
	channelMap := make(map[int]*model.Channel)

	if availableSet != nil && len(availableSet) == 0 {
		return nil, ErrChannelsNotFound
	}

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

	migratedChannels := make([]*scopedChannel, 0, len(channelMap))
	for _, channel := range channelMap {
		migratedChannels = append(migratedChannels, newGlobalScopedChannel(channel))
	}

	return migratedChannels, nil
}

func mergeGroupChannels(
	channels []*scopedChannel,
	groupChannels []*model.GroupChannel,
	availableSet []string,
	modelName string,
	m mode.Mode,
) []*scopedChannel {
	if len(groupChannels) == 0 {
		return channels
	}

	if availableSet != nil && len(availableSet) == 0 {
		return channels
	}

	setAllowed := make(map[string]struct{}, len(availableSet))
	for _, set := range availableSet {
		setAllowed[set] = struct{}{}
	}

	for _, groupChannel := range groupChannels {
		if groupChannel.Status != model.ChannelStatusEnabled {
			continue
		}

		if !model.GroupChannelSupportsModel(groupChannel, modelName) {
			continue
		}

		if len(setAllowed) > 0 &&
			!slices.ContainsFunc(groupChannel.GetSets(), func(set string) bool {
				_, ok := setAllowed[set]
				return ok
			}) {
			continue
		}

		channel := groupChannel.ToChannel()

		a, ok := adaptors.GetAdaptor(channel.Type)
		if !ok {
			continue
		}

		modelConfig, ok := groupChannelSupportModeModelConfig(groupChannel.GroupID, modelName)
		if !ok {
			continue
		}

		if !adaptorSupportsModeWithConfig(
			a,
			channel,
			modelName,
			m,
			modelConfig,
		) {
			continue
		}

		channels = append(channels, newGroupScopedChannel(groupChannel.GroupID, channel))
	}

	return channels
}

func getPriorityWeight(channel *scopedChannel, errorRate float64) float64 {
	priority := float64(channel.channel.GetPriority())
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

func getChannelErrorRate(errorRates map[string]float64, channelID string) float64 {
	if errorRates == nil {
		return 0
	}

	return errorRates[channelID]
}

func int64SetToStringSet(values map[int64]struct{}) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]struct{}, len(values))
	for value := range values {
		result[strconv.FormatInt(value, 10)] = struct{}{}
	}

	return result
}

func pickMinErrorRateHasPermissionChannel(
	current *scopedChannel,
	currentErrorRate float64,
	candidate *scopedChannel,
	candidateErrorRate float64,
) *scopedChannel {
	if candidate == nil {
		return current
	}

	if current == nil {
		return candidate
	}

	if candidateErrorRate < currentErrorRate {
		return candidate
	}

	return current
}

func pickChannel(
	channels []*scopedChannel,
	errorRates map[string]float64,
) (*scopedChannel, error) {
	if len(channels) == 0 {
		return nil, ErrChannelsExhausted
	}

	if len(channels) == 1 {
		return channels[0], nil
	}

	var totalWeight float64

	cachedWeights := make([]float64, len(channels))
	for i, ch := range channels {
		weight := getPriorityWeight(ch, getChannelErrorRate(errorRates, ch.monitorKey()))
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

func getInitialChannelScope(groupMode string) model.ChannelScope {
	if groupMode == middleware.GroupChannelModeOwn {
		return model.ChannelScopeGroup
	}

	return model.ChannelScopeGlobal
}

func getScopedBannedChannelKeysMap(
	ctx context.Context,
	scope model.ChannelScope,
	modelName string,
) (map[string]struct{}, error) {
	if scope == model.ChannelScopeGroup {
		return monitor.GetGroupChannelBannedChannelKeysMapWithModel(ctx, modelName)
	}

	return monitor.GetBannedChannelKeysMapWithModel(ctx, modelName)
}

func getScopedModelChannelErrorRateByKey(
	ctx context.Context,
	scope model.ChannelScope,
	modelName string,
) (map[string]float64, error) {
	if scope == model.ChannelScopeGroup {
		return monitor.GetGroupChannelModelErrorRateByKey(ctx, modelName)
	}

	return monitor.GetModelChannelErrorRateByKey(ctx, modelName)
}

func getScopedChannelModelErrorRateByKey(
	ctx context.Context,
	scope model.ChannelScope,
	modelName string,
	channelKey string,
) (float64, error) {
	if scope == model.ChannelScopeGroup {
		return monitor.GetGroupChannelChannelModelErrorRateByKey(ctx, modelName, channelKey)
	}

	return monitor.GetChannelModelErrorRateByKey(ctx, modelName, channelKey)
}

func getScopedChannelWithFallback(
	cache *model.ModelCaches,
	availableSet []string,
	groupChannels []*model.GroupChannel,
	groupOnly bool,
	modelName string,
	mode mode.Mode,
	preferChannelKeys []string,
	errorRates map[string]float64,
	ignoreChannelIDs map[string]struct{},
) (*scopedChannel, []*scopedChannel, error) {
	var migratedChannels []*scopedChannel
	if !groupOnly {
		var err error

		migratedChannels, err = getAvailableChannels(
			cache,
			availableSet,
			modelName,
			mode,
		)
		if err != nil && len(groupChannels) == 0 {
			return nil, nil, err
		}
	}

	migratedChannels = mergeGroupChannels(
		migratedChannels,
		groupChannels,
		availableSet,
		modelName,
		mode,
	)

	filteredChannels := filterChannels(
		migratedChannels,
		errorRates,
		maxRetryErrorRate,
		ignoreChannelIDs,
	)

	if len(preferChannelKeys) > 0 {
		channel := pickPreferredChannel(
			filteredChannels,
			preferChannelKeys,
		)
		if channel != nil {
			return channel, migratedChannels, nil
		}
	}

	pipeline := []func() []*scopedChannel{
		func() []*scopedChannel {
			return filteredChannels
		},
		func() []*scopedChannel {
			return filterChannels(
				migratedChannels,
				errorRates,
				0,
				ignoreChannelIDs,
			)
		},
		func() []*scopedChannel {
			return filterChannels(
				migratedChannels,
				errorRates,
				0,
			)
		},
	}

	for _, step := range pipeline {
		channel, err := pickChannel(step(), errorRates)
		if err == nil {
			return channel, migratedChannels, nil
		}
	}

	return nil, nil, ErrChannelsExhausted
}

func pickPreferredChannel(
	channels []*scopedChannel,
	preferChannelKeys []string,
) *scopedChannel {
	if len(channels) == 0 || len(preferChannelKeys) == 0 {
		return nil
	}

	channelMap := make(map[string]*scopedChannel, len(channels))
	for _, channel := range channels {
		channelMap[channel.monitorKey()] = channel
	}

	seen := make(map[string]struct{}, len(preferChannelKeys))
	for _, channelKey := range preferChannelKeys {
		if _, ok := seen[channelKey]; ok {
			continue
		}

		seen[channelKey] = struct{}{}
		if channel, ok := channelMap[channelKey]; ok {
			return channel
		}
	}

	return nil
}

func channelIDsToKeys(ids []int) []string {
	if len(ids) == 0 {
		return nil
	}

	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}

		keys = append(keys, strconv.Itoa(id))
	}

	return keys
}

type initialChannel struct {
	channel           *scopedChannel
	designatedChannel bool
	groupRetryOnly    bool
	preferChannelKeys []string
	ignoreChannelIDs  map[string]struct{}
	migratedChannels  []*scopedChannel
}

func getInitialChannel(c *gin.Context, modelName string, m mode.Mode) (*initialChannel, error) {
	log := common.GetLogger(c)

	group := middleware.GetGroup(c)
	groupMode := middleware.GetGroupChannelMode(c)
	availableSet := middleware.GetActiveAvailableSets(c)

	if channelHeader := c.Request.Header.Get(AIProxyChannelHeader); channelHeader != "" {
		if groupMode == middleware.GroupChannelModeOwn {
			channel, err := GetGroupChannelFromHeader(
				channelHeader,
				group,
				availableSet,
				group.Status == model.GroupStatusInternal,
				modelName,
				m,
			)
			if err != nil {
				return nil, err
			}

			log.Data["designated_channel"] = "true"

			return &initialChannel{
				channel:           channel,
				designatedChannel: true,
				groupRetryOnly:    true,
			}, nil
		}

		if group.Status != model.GroupStatusInternal {
			return nil, errors.New("global channel header is not allowed in non-internal group")
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

		return &initialChannel{
			channel:           newGlobalScopedChannel(channel),
			designatedChannel: true,
		}, nil
	}

	channel, err := GetScopedChannelFromRequest(
		c,
		middleware.GetModelCaches(c),
		availableSet,
		group.Status == model.GroupStatusInternal,
		modelName,
		m,
	)
	if err != nil {
		return nil, err
	}

	if channel != nil {
		return &initialChannel{
			channel:           channel,
			designatedChannel: true,
			groupRetryOnly:    channel.isGroupChannel(),
		}, nil
	}

	mc := middleware.GetModelCaches(c)

	var groupChannels []*model.GroupChannel
	if groupMode == middleware.GroupChannelModeOwn && group.ID != "" {
		groupChannelsCache, err := model.CacheGetGroupChannels(group.ID)
		if err != nil {
			log.Errorf("get group channels failed: %+v", err)
		} else {
			groupChannels = groupChannelsCache.Channels
		}
	}

	selectionScope := getInitialChannelScope(groupMode)

	ignoreChannelKeyMap, err := getScopedBannedChannelKeysMap(
		c.Request.Context(),
		selectionScope,
		modelName,
	)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		log.Errorf("get %s auto banned channels failed: %+v", modelName, err)
	}

	log.Debugf("%s model banned channels: %+v", modelName, ignoreChannelKeyMap)

	errorRates, err := getScopedModelChannelErrorRateByKey(
		c.Request.Context(),
		selectionScope,
		modelName,
	)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		log.Errorf("get channel model error rates failed: %+v", err)
	}

	preferChannelKeys := getPreferChannelKeys(c, modelName, m)

	if len(preferChannelKeys) > 0 {
		log.Data["prefer_channels"] = fmt.Sprintf("%v", preferChannelKeys)
	}

	selectedChannel, migratedChannels, err := getScopedChannelWithFallback(
		mc,
		availableSet,
		groupChannels,
		groupMode == middleware.GroupChannelModeOwn,
		modelName,
		m,
		preferChannelKeys,
		errorRates,
		ignoreChannelKeyMap,
	)
	if err != nil {
		return nil, err
	}

	return &initialChannel{
		channel:           selectedChannel,
		groupRetryOnly:    groupMode == middleware.GroupChannelModeOwn,
		preferChannelKeys: preferChannelKeys,
		ignoreChannelIDs:  ignoreChannelKeyMap,
		migratedChannels:  migratedChannels,
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
	case mode.Responses,
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

func getCacheFollowConfig(modelConfig model.ModelConfig) (cachefollow.Config, bool) {
	pluginConfig := cachefollow.Config{}
	if err := modelConfig.LoadPluginConfig(cachefollow.PluginName, &pluginConfig); err != nil {
		return cachefollow.Config{}, false
	}

	if !pluginConfig.Enable {
		return cachefollow.Config{}, false
	}

	return pluginConfig, true
}

func getPreferChannelKeys(c *gin.Context, modelName string, m mode.Mode) []string {
	if !supportsCacheFollowMode(m) {
		return nil
	}

	group := middleware.GetGroup(c)
	token := middleware.GetToken(c)
	user := middleware.GetRequestUser(c)
	modelCaches := middleware.GetModelCaches(c)
	groupMode := middleware.GetGroupChannelMode(c)
	preferChannelKeys := make([]string, 0, 6)
	seen := make(map[string]struct{}, 6)

	scopes := []model.ChannelScope{model.ChannelScopeGlobal}
	if groupMode == middleware.GroupChannelModeOwn && group.ID != "" {
		scopes = []model.ChannelScope{model.ChannelScopeGroup}
	}

	appendChannelKey := func(storeID string, scope model.ChannelScope) {
		if storeID == "" {
			return
		}

		store, err := model.CacheGetStoreByScope(group.ID, token.ID, storeID, scope)
		if err != nil {
			return
		}

		channelKey := model.StoreChannelKey(store, scope)
		if channelKey == "" {
			return
		}

		if _, ok := seen[channelKey]; ok {
			return
		}

		seen[channelKey] = struct{}{}
		preferChannelKeys = append(preferChannelKeys, channelKey)
	}

	appendScopeKeys := func(scope model.ChannelScope) {
		modelConfig, ok := resolveScopedModelConfig(
			group,
			modelCaches,
			&scopedChannel{scope: scope},
			modelName,
		)
		if !ok {
			return
		}

		pluginConfig, ok := getCacheFollowConfig(modelConfig)
		if !ok {
			return
		}

		if supportsPromptCacheKeyMode(m) {
			if promptCacheKey := middleware.GetPromptCacheKey(c); promptCacheKey != "" {
				appendChannelKey(
					model.PromptCacheStoreID(
						modelName,
						promptCacheKey,
						model.CacheKeyTypeStable,
					),
					scope,
				)
				appendChannelKey(
					model.PromptCacheStoreID(
						modelName,
						promptCacheKey,
						model.CacheKeyTypeRecent,
					),
					scope,
				)
			}
		}

		if user != "" {
			appendChannelKey(
				model.CacheFollowUserStoreID(modelName, user, model.CacheKeyTypeStable),
				scope,
			)
			appendChannelKey(
				model.CacheFollowUserStoreID(modelName, user, model.CacheKeyTypeRecent),
				scope,
			)
		}

		if pluginConfig.EnableGenericFollow {
			appendChannelKey(model.CacheFollowStoreID(modelName, model.CacheKeyTypeStable), scope)
			appendChannelKey(model.CacheFollowStoreID(modelName, model.CacheKeyTypeRecent), scope)
		}
	}

	for _, scope := range scopes {
		appendScopeKeys(scope)
	}

	if len(preferChannelKeys) == 0 {
		return nil
	}

	return preferChannelKeys
}

func getWebSearchChannel(
	ctx context.Context,
	mc *model.ModelCaches,
	modelName string,
) (*model.Channel, error) {
	ignoreChannelKeyMap, _ := monitor.GetBannedChannelKeysMapWithModel(ctx, modelName)
	errorRates, _ := monitor.GetModelChannelErrorRateByKey(ctx, modelName)

	channel, _, err := getScopedChannelWithFallback(
		mc,
		nil,
		nil,
		false,
		modelName,
		mode.ChatCompletions,
		nil,
		errorRates,
		ignoreChannelKeyMap)
	if err != nil {
		return nil, err
	}

	return channel.channel, nil
}

func getRetryChannel(
	ctx context.Context,
	state *retryState,
) (*scopedChannel, error) {
	retryScope := model.ChannelScopeGlobal
	if state.groupRetryOnly {
		retryScope = model.ChannelScopeGroup
	}

	errorRates, err := getScopedModelChannelErrorRateByKey(
		ctx,
		retryScope,
		state.meta.OriginModel,
	)
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
	}

	candidates := state.migratedChannels
	if state.groupRetryOnly {
		candidates = filterScopedChannelsByScope(candidates, model.ChannelScopeGroup)
	}

	if state.exhausted {
		if state.lastMinErrorRateHasPermissionChannel == nil {
			return nil, ErrChannelsExhausted
		}

		// Check if the lowest-error has-permission channel has high error rate.
		// If so, return exhausted to prevent retrying with a bad channel
		channelID := state.lastMinErrorRateHasPermissionChannel.monitorKey()
		if errorRate := getChannelErrorRate(errorRates, channelID); errorRate > maxRetryErrorRate {
			return nil, ErrChannelsExhausted
		}

		return state.lastMinErrorRateHasPermissionChannel, nil
	}

	filteredChannels := filterChannels(
		candidates,
		errorRates,
		maxRetryErrorRate,
		state.ignoreChannelIDs,
		state.failedChannelIDs,
	)

	if len(state.preferChannelKeys) > 0 {
		newChannel := pickPreferredChannel(
			filteredChannels,
			state.preferChannelKeys,
		)
		if newChannel != nil {
			return newChannel, nil
		}
	}

	newChannel, err := pickChannel(
		filteredChannels,
		errorRates,
	)
	if err != nil {
		if !errors.Is(err, ErrChannelsExhausted) ||
			state.lastMinErrorRateHasPermissionChannel == nil {
			return nil, err
		}

		// Check if the lowest-error has-permission channel has high error rate.
		// If so, return exhausted to prevent retrying with a bad channel
		channelID := state.lastMinErrorRateHasPermissionChannel.monitorKey()
		if errorRate := getChannelErrorRate(errorRates, channelID); errorRate > maxRetryErrorRate {
			return nil, ErrChannelsExhausted
		}

		// Check if the lowest-error has-permission channel is still healthy before using it.
		state.exhausted = true

		return state.lastMinErrorRateHasPermissionChannel, nil
	}

	return newChannel, nil
}

func filterScopedChannelsByScope(
	channels []*scopedChannel,
	scope model.ChannelScope,
) []*scopedChannel {
	filtered := make([]*scopedChannel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.scope != scope {
			continue
		}

		filtered = append(filtered, channel)
	}

	return filtered
}

func filterChannels(
	channels []*scopedChannel,
	errorRates map[string]float64,
	maxErrorRate float64,
	ignoreChannel ...map[string]struct{},
) []*scopedChannel {
	filtered := make([]*scopedChannel, 0)
	for _, channel := range channels {
		if !isChannelSelectable(channel, errorRates, maxErrorRate, ignoreChannel...) {
			continue
		}

		filtered = append(filtered, channel)
	}

	return filtered
}

func isChannelSelectable(
	channel *scopedChannel,
	errorRates map[string]float64,
	maxErrorRate float64,
	ignoreChannel ...map[string]struct{},
) bool {
	if channel == nil || channel.channel == nil ||
		channel.channel.Status != model.ChannelStatusEnabled {
		return false
	}

	chid := channel.monitorKey()

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
