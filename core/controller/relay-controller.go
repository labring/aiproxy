package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/reqlimit"
	"github.com/labring/aiproxy/core/common/trylock"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/cache"
	"github.com/labring/aiproxy/core/relay/plugin/thinksplit"
	websearch "github.com/labring/aiproxy/core/relay/plugin/web-search"
	log "github.com/sirupsen/logrus"
)

// https://platform.openai.com/docs/api-reference/chat

type (
	RelayHandler    func(*gin.Context, *meta.Meta) *controller.HandleResult
	GetRequestUsage func(*gin.Context, model.ModelConfig) (model.Usage, error)
	GetRequestPrice func(*gin.Context, model.ModelConfig) (model.Price, error)
)

type RelayController struct {
	GetRequestUsage GetRequestUsage
	GetRequestPrice GetRequestPrice
	Handler         RelayHandler
}

// TODO: convert to plugin
type wrapAdaptor struct {
	adaptor.Adaptor
}

const (
	MetaChannelModelKeyRPM = "channel_model_rpm"
	MetaChannelModelKeyRPS = "channel_model_rps"
	MetaChannelModelKeyTPM = "channel_model_tpm"
	MetaChannelModelKeyTPS = "channel_model_tps"
)

func getChannelModelRequestRate(c *gin.Context, meta *meta.Meta) model.RequestRate {
	rate := model.RequestRate{}

	if rpm, ok := meta.Get(MetaChannelModelKeyRPM); ok {
		rate.RPM, _ = rpm.(int64)
		rate.RPS = meta.GetInt64(MetaChannelModelKeyRPS)
	} else {
		rpm, rps := reqlimit.GetChannelModelRequest(context.Background(), strconv.Itoa(meta.Channel.ID), meta.OriginModel)
		rate.RPM = rpm
		rate.RPS = rps
		updateChannelModelRequestRate(c, meta, rpm, rps)
	}

	if tpm, ok := meta.Get(MetaChannelModelKeyTPM); ok {
		rate.TPM, _ = tpm.(int64)
		rate.TPS = meta.GetInt64(MetaChannelModelKeyTPS)
	} else {
		tpm, tps := reqlimit.GetChannelModelTokensRequest(context.Background(), strconv.Itoa(meta.Channel.ID), meta.OriginModel)
		rate.TPM = tpm
		rate.TPS = tps
		updateChannelModelTokensRequestRate(c, meta, tpm, tps)
	}

	return rate
}

func updateChannelModelRequestRate(c *gin.Context, meta *meta.Meta, rpm, rps int64) {
	meta.Set(MetaChannelModelKeyRPM, rpm)
	meta.Set(MetaChannelModelKeyRPS, rps)
	log := middleware.GetLogger(c)
	log.Data["ch_rpm"] = rpm
	log.Data["ch_rps"] = rps
}

func updateChannelModelTokensRequestRate(c *gin.Context, meta *meta.Meta, tpm, tps int64) {
	meta.Set(MetaChannelModelKeyTPM, tpm)
	meta.Set(MetaChannelModelKeyTPS, tps)
	log := middleware.GetLogger(c)
	log.Data["ch_tpm"] = tpm
	log.Data["ch_tps"] = tps
}

func (w *wrapAdaptor) DoRequest(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	count, overLimitCount, secondCount := reqlimit.PushChannelModelRequest(
		context.Background(),
		strconv.Itoa(meta.Channel.ID),
		meta.OriginModel,
	)
	updateChannelModelRequestRate(c, meta, count+overLimitCount, secondCount)
	return w.Adaptor.DoRequest(meta, store, c, req)
}

func (w *wrapAdaptor) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (model.Usage, adaptor.Error) {
	usage, relayErr := w.Adaptor.DoResponse(meta, store, c, resp)

	if usage.TotalTokens > 0 {
		count, overLimitCount, secondCount := reqlimit.PushChannelModelTokensRequest(
			context.Background(),
			strconv.Itoa(meta.Channel.ID),
			meta.OriginModel,
			int64(usage.TotalTokens),
		)
		updateChannelModelTokensRequestRate(c, meta, count+overLimitCount, secondCount)

		count, overLimitCount, secondCount = reqlimit.PushGroupModelTokensRequest(
			context.Background(),
			meta.Group.ID,
			meta.OriginModel,
			meta.ModelConfig.TPM,
			int64(usage.TotalTokens),
		)
		middleware.UpdateGroupModelTokensRequest(c, meta.Group, count+overLimitCount, secondCount)

		count, overLimitCount, secondCount = reqlimit.PushGroupModelTokennameTokensRequest(
			context.Background(),
			meta.Group.ID,
			meta.OriginModel,
			meta.Token.Name,
			int64(usage.TotalTokens),
		)
		middleware.UpdateGroupModelTokennameTokensRequest(c, count+overLimitCount, secondCount)
	}

	return usage, relayErr
}

var adaptorStore adaptor.Store = &storeImpl{}

type storeImpl struct{}

func (s *storeImpl) GetStore(id string) (adaptor.StoreCache, error) {
	store, err := model.CacheGetStore(id)
	if err != nil {
		return adaptor.StoreCache{}, err
	}
	return adaptor.StoreCache{
		ID:        store.ID,
		GroupID:   store.GroupID,
		TokenID:   store.TokenID,
		ChannelID: store.ChannelID,
		Model:     store.Model,
		ExpiresAt: store.ExpiresAt,
	}, nil
}

func (s *storeImpl) SaveStore(store adaptor.StoreCache) error {
	_, err := model.SaveStore(&model.Store{
		ID:        store.ID,
		GroupID:   store.GroupID,
		TokenID:   store.TokenID,
		ChannelID: store.ChannelID,
		Model:     store.Model,
		ExpiresAt: store.ExpiresAt,
	})
	return err
}

func relayHandler(c *gin.Context, meta *meta.Meta) *controller.HandleResult {
	log := middleware.GetLogger(c)
	middleware.SetLogFieldsFromMeta(meta, log.Data)

	adaptor, ok := adaptors.GetAdaptor(meta.Channel.Type)
	if !ok {
		return &controller.HandleResult{
			Error: relaymodel.WrapperOpenAIErrorWithMessage(
				fmt.Sprintf("invalid channel type: %d", meta.Channel.Type),
				"invalid_channel_type",
				http.StatusInternalServerError,
			),
		}
	}

	a := plugin.WrapperAdaptor(&wrapAdaptor{adaptor},
		cache.NewCachePlugin(common.RDB),
		websearch.NewWebSearchPlugin(func(modelName string) (*model.Channel, error) {
			return getWebSearchChannel(c, modelName)
		}),
		thinksplit.NewThinkPlugin(),
	)

	return controller.Handle(a, c, meta, adaptorStore)
}

func relayController(m mode.Mode) RelayController {
	c := RelayController{
		Handler: relayHandler,
	}
	switch m {
	case mode.ImagesGenerations:
		c.GetRequestPrice = controller.GetImagesRequestPrice
		c.GetRequestUsage = controller.GetImagesRequestUsage
	case mode.ImagesEdits:
		c.GetRequestPrice = controller.GetImagesEditsRequestPrice
		c.GetRequestUsage = controller.GetImagesEditsRequestUsage
	case mode.AudioSpeech:
		c.GetRequestPrice = controller.GetTTSRequestPrice
		c.GetRequestUsage = controller.GetTTSRequestUsage
	case mode.AudioTranslation, mode.AudioTranscription:
		c.GetRequestPrice = controller.GetSTTRequestPrice
		c.GetRequestUsage = controller.GetSTTRequestUsage
	case mode.ParsePdf:
		c.GetRequestPrice = controller.GetPdfRequestPrice
		c.GetRequestUsage = controller.GetPdfRequestUsage
	case mode.Rerank:
		c.GetRequestPrice = controller.GetRerankRequestPrice
		c.GetRequestUsage = controller.GetRerankRequestUsage
	case mode.Anthropic:
		c.GetRequestPrice = controller.GetAnthropicRequestPrice
		c.GetRequestUsage = controller.GetAnthropicRequestUsage
	case mode.ChatCompletions:
		c.GetRequestPrice = controller.GetChatRequestPrice
		c.GetRequestUsage = controller.GetChatRequestUsage
	case mode.Embeddings:
		c.GetRequestPrice = controller.GetEmbedRequestPrice
		c.GetRequestUsage = controller.GetEmbedRequestUsage
	case mode.Completions:
		c.GetRequestPrice = controller.GetCompletionsRequestPrice
		c.GetRequestUsage = controller.GetCompletionsRequestUsage
	case mode.VideoGenerationsJobs:
		c.GetRequestPrice = controller.GetVideoGenerationJobRequestPrice
		c.GetRequestUsage = controller.GetVideoGenerationJobRequestUsage
	}
	return c
}

const (
	AIProxyChannelHeader = "Aiproxy-Channel"
)

func GetChannelFromHeader(
	header string,
	mc *model.ModelCaches,
	availableSet []string,
	model string,
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
					return channel, nil
				}
			}
		}

		disabledChannels := mc.DisabledModel2ChannelsBySet[set][model]
		if len(disabledChannels) > 0 {
			for _, channel := range disabledChannels {
				if int64(channel.ID) == channelIDInt {
					return channel, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("channel %d not found for model `%s`", channelIDInt, model)
}

func GetChannelFromRequest(
	c *gin.Context,
	mc *model.ModelCaches,
	availableSet []string,
	modelName string,
	m mode.Mode,
) (*model.Channel, error) {
	switch m {
	case mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent:
		channelID := middleware.GetChannelID(c)
		if channelID == 0 {
			return nil, errors.New("channel id is required")
		}
		for _, set := range availableSet {
			enabledChannels := mc.EnabledModel2ChannelsBySet[set][modelName]
			if len(enabledChannels) > 0 {
				for _, channel := range enabledChannels {
					if channel.ID == channelID {
						return channel, nil
					}
				}
			}
		}
		return nil, fmt.Errorf("channel %d not found for model `%s`", channelID, modelName)
	default:
		channelID := middleware.GetChannelID(c)
		if channelID == 0 {
			return nil, nil
		}
		for _, set := range availableSet {
			enabledChannels := mc.EnabledModel2ChannelsBySet[set][modelName]
			if len(enabledChannels) > 0 {
				for _, channel := range enabledChannels {
					if channel.ID == channelID {
						return channel, nil
					}
				}
			}
		}
	}
	return nil, nil
}

func RelayHelper(
	c *gin.Context,
	meta *meta.Meta,
	handel RelayHandler,
) (*controller.HandleResult, bool) {
	result := handel(c, meta)
	if result.Error == nil {
		if _, _, err := monitor.AddRequest(
			context.Background(),
			meta.OriginModel,
			int64(meta.Channel.ID),
			false,
			false,
			meta.ModelConfig.MaxErrorRate,
		); err != nil {
			log.Errorf("add request failed: %+v", err)
		}
		return result, false
	}
	shouldRetry := shouldRetry(c, result.Error)
	if shouldRetry {
		hasPermission := channelHasPermission(result.Error)
		beyondThreshold, banExecution, err := monitor.AddRequest(
			context.Background(),
			meta.OriginModel,
			int64(meta.Channel.ID),
			true,
			!hasPermission,
			meta.ModelConfig.MaxErrorRate,
		)
		if err != nil {
			log.Errorf("add request failed: %+v", err)
		}
		switch {
		case banExecution:
			notifyChannelIssue(c, meta, "autoBanned", "Auto Banned", result.Error)
		case beyondThreshold:
			notifyChannelIssue(
				c,
				meta,
				"beyondThreshold",
				"Error Rate Beyond Threshold",
				result.Error,
			)
		case !hasPermission:
			notifyChannelIssue(c, meta, "channelHasPermission", "No Permission", result.Error)
		}
	}
	return result, shouldRetry
}

func notifyChannelIssue(
	c *gin.Context,
	meta *meta.Meta,
	issueType, titleSuffix string,
	err adaptor.Error,
) {
	var notifyFunc func(title, message string)

	lockKey := fmt.Sprintf("%s:%d:%s", issueType, meta.Channel.ID, meta.OriginModel)
	switch issueType {
	case "beyondThreshold":
		notifyFunc = func(title, message string) {
			notify.WarnThrottle(lockKey, time.Minute, title, message)
		}
	default:
		notifyFunc = func(title, message string) {
			notify.ErrorThrottle(lockKey, time.Minute, title, message)
		}
	}

	respBody, _ := err.MarshalJSON()

	message := fmt.Sprintf(
		"channel: %s (type: %d, type name: %s, id: %d)\nmodel: %s\nmode: %s\nstatus code: %d\ndetail: %s\nrequest id: %s",
		meta.Channel.Name,
		meta.Channel.Type,
		meta.Channel.Type.String(),
		meta.Channel.ID,
		meta.OriginModel,
		meta.Mode,
		err.StatusCode(),
		conv.BytesToString(respBody),
		meta.RequestID,
	)

	if err.StatusCode() == http.StatusTooManyRequests {
		if !trylock.Lock(lockKey, time.Minute) {
			return
		}
		switch issueType {
		case "beyondThreshold":
			notifyFunc = notify.Warn
		default:
			notifyFunc = notify.Error
		}

		rate := getChannelModelRequestRate(c, meta)
		message += fmt.Sprintf(
			"\nrpm: %d\nrps: %d\ntpm: %d\ntps: %d",
			rate.RPM,
			rate.RPS,
			rate.TPM,
			rate.TPS,
		)
	}

	notifyFunc(
		fmt.Sprintf("%s `%s` %s", meta.Channel.Name, meta.OriginModel, titleSuffix),
		message,
	)
}

func filterChannels(channels []*model.Channel, ignoreChannel ...int64) []*model.Channel {
	filtered := make([]*model.Channel, 0)
	for _, channel := range channels {
		if channel.Status != model.ChannelStatusEnabled {
			continue
		}
		if slices.Contains(ignoreChannel, int64(channel.ID)) {
			continue
		}
		filtered = append(filtered, channel)
	}
	return filtered
}

var (
	ErrChannelsNotFound  = errors.New("channels not found")
	ErrChannelsExhausted = errors.New("channels exhausted")
)

func GetRandomChannel(
	mc *model.ModelCaches,
	availableSet []string,
	modelName string,
	errorRates map[int64]float64,
	ignoreChannel ...int64,
) (*model.Channel, []*model.Channel, error) {
	channelMap := make(map[int]*model.Channel)
	if len(availableSet) != 0 {
		for _, set := range availableSet {
			for _, channel := range mc.EnabledModel2ChannelsBySet[set][modelName] {
				channelMap[channel.ID] = channel
			}
		}
	} else {
		for _, sets := range mc.EnabledModel2ChannelsBySet {
			for _, channel := range sets[modelName] {
				channelMap[channel.ID] = channel
			}
		}
	}
	migratedChannels := make([]*model.Channel, 0, len(channelMap))
	for _, channel := range channelMap {
		migratedChannels = append(migratedChannels, channel)
	}
	channel, err := getRandomChannel(migratedChannels, errorRates, ignoreChannel...)
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

//

func getRandomChannel(
	channels []*model.Channel,
	errorRates map[int64]float64,
	ignoreChannel ...int64,
) (*model.Channel, error) {
	if len(channels) == 0 {
		return nil, ErrChannelsNotFound
	}

	channels = filterChannels(channels, ignoreChannel...)
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
	errorRates map[int64]float64,
	ignoreChannelIDs ...int64,
) (*model.Channel, []*model.Channel, error) {
	channel, migratedChannels, err := GetRandomChannel(
		cache,
		availableSet,
		modelName,
		errorRates,
		ignoreChannelIDs...)
	if err == nil {
		return channel, migratedChannels, nil
	}
	if !errors.Is(err, ErrChannelsExhausted) {
		return nil, migratedChannels, err
	}
	channel, migratedChannels, err = GetRandomChannel(cache, availableSet, modelName, errorRates)
	return channel, migratedChannels, err
}

func NewRelay(mode mode.Mode) func(c *gin.Context) {
	relayController := relayController(mode)
	return func(c *gin.Context) {
		relay(c, mode, relayController)
	}
}

func NewMetaByContext(
	c *gin.Context,
	channel *model.Channel,
	mode mode.Mode,
	opts ...meta.Option,
) *meta.Meta {
	return middleware.NewMetaByContext(c, channel, mode, opts...)
}

func relay(c *gin.Context, mode mode.Mode, relayController RelayController) {
	requestModel := middleware.GetRequestModel(c)
	mc := middleware.GetModelConfig(c)

	// Get initial channel
	initialChannel, err := getInitialChannel(c, requestModel, mode)
	if err != nil || initialChannel == nil || initialChannel.channel == nil {
		middleware.AbortLogWithMessageWithMode(mode, c,
			http.StatusServiceUnavailable,
			"the upstream load is saturated, please try again later",
		)
		return
	}

	price := model.Price{}
	if relayController.GetRequestPrice != nil {
		price, err = relayController.GetRequestPrice(c, mc)
		if err != nil {
			middleware.AbortLogWithMessageWithMode(mode, c,
				http.StatusInternalServerError,
				"get request price failed: "+err.Error(),
			)
			return
		}
	}

	meta := NewMetaByContext(c, initialChannel.channel, mode)

	if relayController.GetRequestUsage != nil {
		requestUsage, err := relayController.GetRequestUsage(c, mc)
		if err != nil {
			middleware.AbortLogWithMessageWithMode(mode, c,
				http.StatusInternalServerError,
				"get request usage failed: "+err.Error(),
			)
			return
		}
		gbc := middleware.GetGroupBalanceConsumerFromContext(c)
		if !gbc.CheckBalance(consume.CalculateAmount(http.StatusOK, requestUsage, price)) {
			middleware.AbortLogWithMessageWithMode(mode, c,
				http.StatusForbidden,
				fmt.Sprintf("group (%s) balance not enough", gbc.Group),
				middleware.GroupBalanceNotEnough,
			)
			return
		}

		meta.RequestUsage = requestUsage
	}

	// First attempt
	result, retry := RelayHelper(c, meta, relayController.Handler)

	retryTimes := int(config.GetRetryTimes())
	if mc.RetryTimes > 0 {
		retryTimes = int(mc.RetryTimes)
	}
	if handleRelayResult(c, result.Error, retry, retryTimes) {
		recordResult(
			c,
			meta,
			price,
			result,
			0,
			true,
			middleware.GetRequestUser(c),
			middleware.GetRequestMetadata(c),
		)
		return
	}

	// Setup retry state
	retryState := initRetryState(
		retryTimes,
		initialChannel,
		meta,
		result,
		price,
	)

	// Retry loop
	retryLoop(c, mode, retryState, relayController.Handler)
}

// recordResult records the consumption for the final result
func recordResult(
	c *gin.Context,
	meta *meta.Meta,
	price model.Price,
	result *controller.HandleResult,
	retryTimes int,
	downstreamResult bool,
	user string,
	metadata map[string]string,
) {
	code := http.StatusOK
	content := ""
	if result.Error != nil {
		code = result.Error.StatusCode()
		respBody, _ := result.Error.MarshalJSON()
		content = conv.BytesToString(respBody)
	}

	var detail *model.RequestDetail
	firstByteAt := result.Detail.FirstByteAt
	if code == http.StatusOK && !config.GetSaveAllLogDetail() {
		detail = nil
	} else {
		detail = &model.RequestDetail{
			RequestBody:  result.Detail.RequestBody,
			ResponseBody: result.Detail.ResponseBody,
		}
	}

	gbc := middleware.GetGroupBalanceConsumerFromContext(c)

	amount := consume.CalculateAmount(
		code,
		result.Usage,
		price,
	)
	if amount > 0 {
		log := middleware.GetLogger(c)
		log.Data["amount"] = strconv.FormatFloat(amount, 'f', -1, 64)
	}

	consume.AsyncConsume(
		gbc.Consumer,
		code,
		firstByteAt,
		meta,
		result.Usage,
		price,
		content,
		c.ClientIP(),
		retryTimes,
		detail,
		downstreamResult,
		user,
		metadata,
		getChannelModelRequestRate(c, meta),
		middleware.GetGroupModelTokenRequestRate(c),
	)
}

type retryState struct {
	retryTimes               int
	lastHasPermissionChannel *model.Channel
	ignoreChannelIDs         []int64
	errorRates               map[int64]float64
	exhausted                bool

	meta             *meta.Meta
	price            model.Price
	requestUsage     model.Usage
	result           *controller.HandleResult
	migratedChannels []*model.Channel
}

type initialChannel struct {
	channel           *model.Channel
	designatedChannel bool
	ignoreChannelIDs  []int64
	errorRates        map[int64]float64
	migratedChannels  []*model.Channel
}

func getInitialChannel(c *gin.Context, modelName string, m mode.Mode) (*initialChannel, error) {
	log := middleware.GetLogger(c)

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

	ids, err := monitor.GetBannedChannelsWithModel(c.Request.Context(), modelName)
	if err != nil {
		log.Errorf("get %s auto banned channels failed: %+v", modelName, err)
	}
	log.Debugf("%s model banned channels: %+v", modelName, ids)

	errorRates, err := monitor.GetModelChannelErrorRate(c.Request.Context(), modelName)
	if err != nil {
		log.Errorf("get channel model error rates failed: %+v", err)
	}

	channel, migratedChannels, err := getChannelWithFallback(
		mc,
		availableSet,
		modelName,
		errorRates,
		ids...)
	if err != nil {
		return nil, err
	}

	return &initialChannel{
		channel:          channel,
		ignoreChannelIDs: ids,
		errorRates:       errorRates,
		migratedChannels: migratedChannels,
	}, nil
}

func getWebSearchChannel(c *gin.Context, modelName string) (*model.Channel, error) {
	log := middleware.GetLogger(c)
	mc := middleware.GetModelCaches(c)

	ids, err := monitor.GetBannedChannelsWithModel(c.Request.Context(), modelName)
	if err != nil {
		log.Errorf("get %s auto banned channels failed: %+v", modelName, err)
	}
	log.Debugf("%s model banned channels: %+v", modelName, ids)

	errorRates, err := monitor.GetModelChannelErrorRate(c.Request.Context(), modelName)
	if err != nil {
		log.Errorf("get channel model error rates failed: %+v", err)
	}

	channel, _, err := getChannelWithFallback(mc, nil, modelName, errorRates, ids...)
	if err != nil {
		return nil, err
	}

	return channel, nil
}

func handleRelayResult(
	c *gin.Context,
	bizErr adaptor.Error,
	retry bool,
	retryTimes int,
) (done bool) {
	if bizErr == nil {
		return true
	}
	if !retry ||
		retryTimes == 0 ||
		c.Request.Context().Err() != nil {
		ErrorWithRequestID(c, bizErr)
		return true
	}
	return false
}

func initRetryState(
	retryTimes int,
	channel *initialChannel,
	meta *meta.Meta,
	result *controller.HandleResult,
	price model.Price,
) *retryState {
	state := &retryState{
		retryTimes:       retryTimes,
		ignoreChannelIDs: channel.ignoreChannelIDs,
		errorRates:       channel.errorRates,
		meta:             meta,
		result:           result,
		price:            price,
		requestUsage:     meta.RequestUsage,
		migratedChannels: channel.migratedChannels,
	}

	if channel.designatedChannel {
		state.exhausted = true
	}

	if !channelHasPermission(result.Error) {
		state.ignoreChannelIDs = append(state.ignoreChannelIDs, int64(channel.channel.ID))
	} else {
		state.lastHasPermissionChannel = channel.channel
	}

	return state
}

func retryLoop(c *gin.Context, mode mode.Mode, state *retryState, relayController RelayHandler) {
	log := middleware.GetLogger(c)

	// do not use for i := range state.retryTimes, because the retryTimes is constant
	i := 0

	for {
		lastStatusCode := state.result.Error.StatusCode()
		lastChannelID := state.meta.Channel.ID
		newChannel, err := getRetryChannel(state)
		if err == nil {
			err = prepareRetry(c)
		}
		if err != nil {
			if !errors.Is(err, ErrChannelsExhausted) {
				log.Errorf("prepare retry failed: %+v", err)
			}
			// when the last request has not recorded the result, record the result
			if state.meta != nil && state.result != nil {
				recordResult(
					c,
					state.meta,
					state.price,
					state.result,
					i,
					true,
					middleware.GetRequestUser(c),
					middleware.GetRequestMetadata(c),
				)
			}
			break
		}
		// when the last request has not recorded the result, record the result
		if state.meta != nil && state.result != nil {
			recordResult(
				c,
				state.meta,
				state.price,
				state.result,
				i,
				false,
				middleware.GetRequestUser(c),
				middleware.GetRequestMetadata(c),
			)
			state.meta = nil
			state.result = nil
		}

		log.Data["retry"] = strconv.Itoa(i + 1)

		log.Warnf("using channel %s (type: %d, id: %d) to retry (remain times %d)",
			newChannel.Name,
			newChannel.Type,
			newChannel.ID,
			state.retryTimes-i,
		)

		// Check if we should delay (using the same channel)
		if shouldDelay(lastStatusCode, lastChannelID, newChannel.ID) {
			relayDelay()
		}

		state.meta = NewMetaByContext(
			c,
			newChannel,
			mode,
			meta.WithRequestUsage(state.requestUsage),
			meta.WithRetryAt(time.Now()),
		)
		var retry bool
		state.result, retry = RelayHelper(c, state.meta, relayController)

		done := handleRetryResult(c, retry, newChannel, state)
		if done || i == state.retryTimes-1 {
			recordResult(
				c,
				state.meta,
				state.price,
				state.result,
				i+1,
				true,
				middleware.GetRequestUser(c),
				middleware.GetRequestMetadata(c),
			)
			break
		}

		i++
	}

	if state.result.Error != nil {
		ErrorWithRequestID(c, state.result.Error)
	}
}

func getRetryChannel(state *retryState) (*model.Channel, error) {
	if state.exhausted {
		if state.lastHasPermissionChannel == nil {
			return nil, ErrChannelsExhausted
		}
		return state.lastHasPermissionChannel, nil
	}

	newChannel, err := getRandomChannel(
		state.migratedChannels,
		state.errorRates,
		state.ignoreChannelIDs...)
	if err != nil {
		if !errors.Is(err, ErrChannelsExhausted) || state.lastHasPermissionChannel == nil {
			return nil, err
		}
		state.exhausted = true
		return state.lastHasPermissionChannel, nil
	}

	return newChannel, nil
}

func prepareRetry(c *gin.Context) error {
	requestBody, err := common.GetRequestBody(c.Request)
	if err != nil {
		return fmt.Errorf("get request body failed in prepare retry: %w", err)
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	return nil
}

func handleRetryResult(
	ctx *gin.Context,
	retry bool,
	newChannel *model.Channel,
	state *retryState,
) (done bool) {
	if ctx.Request.Context().Err() != nil {
		return true
	}
	if !retry || state.result.Error == nil {
		return true
	}

	hasPermission := channelHasPermission(state.result.Error)

	if state.exhausted {
		if !hasPermission {
			return true
		}
	} else {
		if !hasPermission {
			state.ignoreChannelIDs = append(state.ignoreChannelIDs, int64(newChannel.ID))
			state.retryTimes++
		} else {
			state.lastHasPermissionChannel = newChannel
		}
	}

	return false
}

var channelNoRetryStatusCodesMap = map[int]struct{}{
	http.StatusBadRequest:                 {},
	http.StatusRequestEntityTooLarge:      {},
	http.StatusUnprocessableEntity:        {},
	http.StatusUnavailableForLegalReasons: {},
}

// 仅当是channel错误时，才需要记录，用户请求参数错误时，不需要记录
func shouldRetry(_ *gin.Context, relayErr adaptor.Error) bool {
	_, ok := channelNoRetryStatusCodesMap[relayErr.StatusCode()]
	return !ok
}

var channelNoPermissionStatusCodesMap = map[int]struct{}{
	http.StatusUnauthorized:    {},
	http.StatusPaymentRequired: {},
	http.StatusForbidden:       {},
	http.StatusNotFound:        {},
}

func channelHasPermission(relayErr adaptor.Error) bool {
	_, ok := channelNoPermissionStatusCodesMap[relayErr.StatusCode()]
	return !ok
}

// shouldDelay checks if we need to add a delay before retrying
// Only adds delay when retrying with the same channel for rate limiting issues
func shouldDelay(statusCode, lastChannelID, newChannelID int) bool {
	if lastChannelID != newChannelID {
		return false
	}

	// Only delay for rate limiting or service unavailable errors
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusServiceUnavailable
}

func relayDelay() {
	time.Sleep(time.Duration(rand.Float64()*float64(time.Second)) + time.Second)
}

func RelayNotImplemented(c *gin.Context) {
	ErrorWithRequestID(c,
		relaymodel.NewOpenAIError(http.StatusNotImplemented, relaymodel.OpenAIError{
			Message: "API not implemented",
			Type:    relaymodel.ErrorTypeAIPROXY,
			Code:    "api_not_implemented",
		}),
	)
}

func ErrorWithRequestID(c *gin.Context, relayErr adaptor.Error) {
	requestID := middleware.GetRequestID(c)
	if requestID == "" {
		c.JSON(relayErr.StatusCode(), relayErr)
		return
	}
	log := middleware.GetLogger(c)
	data, err := relayErr.MarshalJSON()
	if err != nil {
		log.Errorf("marshal error failed: %+v", err)
		c.JSON(relayErr.StatusCode(), relayErr)
		return
	}
	node, err := sonic.Get(data)
	if err != nil {
		log.Errorf("get node failed: %+v", err)
		c.JSON(relayErr.StatusCode(), relayErr)
		return
	}
	_, err = node.Set("aiproxy", ast.NewString(requestID))
	if err != nil {
		log.Errorf("set request id failed: %+v", err)
		c.JSON(relayErr.StatusCode(), relayErr)
		return
	}
	c.JSON(relayErr.StatusCode(), &node)
}
