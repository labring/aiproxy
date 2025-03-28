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

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/common"
	"github.com/labring/aiproxy/common/config"
	"github.com/labring/aiproxy/common/consume"
	"github.com/labring/aiproxy/common/notify"
	"github.com/labring/aiproxy/middleware"
	"github.com/labring/aiproxy/model"
	"github.com/labring/aiproxy/monitor"
	"github.com/labring/aiproxy/relay/channeltype"
	"github.com/labring/aiproxy/relay/controller"
	"github.com/labring/aiproxy/relay/meta"
	"github.com/labring/aiproxy/relay/mode"
	relaymodel "github.com/labring/aiproxy/relay/model"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

// https://platform.openai.com/docs/api-reference/chat

type (
	RelayHandler    func(*meta.Meta, *gin.Context) *controller.HandleResult
	GetRequestUsage func(*gin.Context, *model.ModelConfig) (model.Usage, error)
	GetRequestPrice func(*gin.Context, *model.ModelConfig) (model.Price, error)
)

type RelayController struct {
	GetRequestUsage GetRequestUsage
	GetRequestPrice GetRequestPrice
	Handler         RelayHandler
}

func relayHandler(meta *meta.Meta, c *gin.Context) *controller.HandleResult {
	log := middleware.GetLogger(c)
	middleware.SetLogFieldsFromMeta(meta, log.Data)
	return controller.Handle(meta, c)
}

func relayController(m mode.Mode) RelayController {
	c := RelayController{
		Handler: relayHandler,
	}
	switch m {
	case mode.ImagesGenerations, mode.Edits:
		c.GetRequestPrice = controller.GetImageRequestPrice
		c.GetRequestUsage = controller.GetImageRequestUsage
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
	case mode.ChatCompletions:
		c.GetRequestPrice = controller.GetChatRequestPrice
		c.GetRequestUsage = controller.GetChatRequestUsage
	case mode.Embeddings:
		c.GetRequestPrice = controller.GetEmbedRequestPrice
		c.GetRequestUsage = controller.GetEmbedRequestUsage
	case mode.Completions:
		c.GetRequestPrice = controller.GetCompletionsRequestPrice
		c.GetRequestUsage = controller.GetCompletionsRequestUsage
	}
	return c
}

func RelayHelper(meta *meta.Meta, c *gin.Context, handel RelayHandler) (*controller.HandleResult, bool) {
	result := handel(meta, c)
	if result.Error == nil {
		if _, _, err := monitor.AddRequest(
			context.Background(),
			meta.OriginModel,
			int64(meta.Channel.ID),
			false,
			false,
		); err != nil {
			log.Errorf("add request failed: %+v", err)
		}
		return result, false
	}
	shouldRetry := shouldRetry(c, *result.Error)
	if shouldRetry {
		hasPermission := channelHasPermission(*result.Error)
		beyondThreshold, banExecution, err := monitor.AddRequest(
			context.Background(),
			meta.OriginModel,
			int64(meta.Channel.ID),
			true,
			!hasPermission,
		)
		if err != nil {
			log.Errorf("add request failed: %+v", err)
		}
		switch {
		case banExecution:
			notify.ErrorThrottle(
				fmt.Sprintf("autoBanned:%d:%s", meta.Channel.ID, meta.OriginModel),
				time.Minute,
				fmt.Sprintf("%s `%s` Auto Banned", meta.Channel.Name, meta.OriginModel),
				fmt.Sprintf(
					"channel %s (type: %d, type name: %s, id: %d)\nmodel %s\nmode: %s\nstatus code: %d\nauto banned: %s",
					meta.Channel.Name,
					meta.Channel.Type,
					meta.Channel.TypeName,
					meta.Channel.ID,
					meta.OriginModel,
					meta.Mode,
					result.Error.StatusCode,
					result.Error.JSONOrEmpty(),
				),
			)
		case beyondThreshold:
			notify.WarnThrottle(
				fmt.Sprintf("beyondThreshold:%d:%s", meta.Channel.ID, meta.OriginModel),
				time.Minute,
				fmt.Sprintf("%s `%s` Error Rate Beyond Threshold", meta.Channel.Name, meta.OriginModel),
				fmt.Sprintf(
					"channel %s (type: %d, type name: %s, id: %d)\nmodel %s\nmode: %s\nstatus code: %d\nerror rate is beyond threshold: %s",
					meta.Channel.Name,
					meta.Channel.Type,
					meta.Channel.TypeName,
					meta.Channel.ID,
					meta.OriginModel,
					meta.Mode,
					result.Error.StatusCode,
					result.Error.JSONOrEmpty(),
				),
			)
		case !hasPermission:
			notify.ErrorThrottle(
				fmt.Sprintf("channelHasPermission:%d:%s", meta.Channel.ID, meta.OriginModel),
				time.Minute,
				fmt.Sprintf("%s `%s` No Permission", meta.Channel.Name, meta.OriginModel),
				fmt.Sprintf(
					"channel %s (type: %d, type name: %s, id: %d)\nmodel %s\nmode: %s\nstatus code: %d\nhas no permission: %s",
					meta.Channel.Name,
					meta.Channel.Type,
					meta.Channel.TypeName,
					meta.Channel.ID,
					meta.OriginModel,
					meta.Mode,
					result.Error.StatusCode,
					result.Error.JSONOrEmpty(),
				),
			)
		}
	}
	return result, shouldRetry
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

func GetRandomChannel(mc *model.ModelCaches, availableSet []string, modelName string, errorRates map[int64]float64, ignoreChannel ...int64) (*model.Channel, []*model.Channel, error) {
	channelMap := make(map[int]*model.Channel)
	for _, set := range availableSet {
		for _, channel := range mc.EnabledModel2ChannelsBySet[set][modelName] {
			channelMap[channel.ID] = channel
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

//nolint:gosec
func getRandomChannel(channels []*model.Channel, errorRates map[int64]float64, ignoreChannel ...int64) (*model.Channel, error) {
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

func getChannelWithFallback(cache *model.ModelCaches, availableSet []string, modelName string, errorRates map[int64]float64, ignoreChannelIDs ...int64) (*model.Channel, []*model.Channel, error) {
	channel, migratedChannels, err := GetRandomChannel(cache, availableSet, modelName, errorRates, ignoreChannelIDs...)
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

func NewMetaByContext(c *gin.Context, channel *model.Channel, mode mode.Mode, opts ...meta.Option) *meta.Meta {
	channelTypeName := channeltype.GetChannelName(channel.Type)
	if channelTypeName != "" {
		opts = append(opts, meta.WithChannelTypeName(channelTypeName))
	}
	return middleware.NewMetaByContext(c, channel, mode, opts...)
}

func relay(c *gin.Context, mode mode.Mode, relayController RelayController) {
	log := middleware.GetLogger(c)
	requestModel := middleware.GetRequestModel(c)
	mc := middleware.GetModelConfig(c)

	// Get initial channel
	initialChannel, err := getInitialChannel(c, requestModel, log)
	if err != nil || initialChannel == nil || initialChannel.channel == nil {
		middleware.AbortLogWithMessage(c,
			http.StatusServiceUnavailable,
			"the upstream load is saturated, please try again later",
		)
		return
	}

	billingEnabled := config.GetBillingEnabled()

	price := model.Price{}
	if billingEnabled && relayController.GetRequestPrice != nil {
		price, err = relayController.GetRequestPrice(c, mc)
		if err != nil {
			middleware.AbortLogWithMessage(c,
				http.StatusInternalServerError,
				"get request price failed: "+err.Error(),
			)
			return
		}
	}

	meta := NewMetaByContext(c, initialChannel.channel, mode)

	if billingEnabled && relayController.GetRequestUsage != nil {
		requestUsage, err := relayController.GetRequestUsage(c, mc)
		if err != nil {
			middleware.AbortLogWithMessage(c,
				http.StatusInternalServerError,
				"get request usage failed: "+err.Error(),
			)
			return
		}
		gbc := middleware.GetGroupBalanceConsumerFromContext(c)
		if !gbc.CheckBalance(getPreConsumedAmount(requestUsage, price)) {
			middleware.AbortLogWithMessage(c,
				http.StatusForbidden,
				fmt.Sprintf("group (%s) balance not enough", gbc.Group),
				&middleware.ErrorField{
					Code: middleware.GroupBalanceNotEnough,
				},
			)
			return
		}

		meta.InputTokens = requestUsage.InputTokens
	}

	// First attempt
	result, retry := RelayHelper(meta, c, relayController.Handler)

	retryTimes := int(config.GetRetryTimes())
	if handleRelayResult(c, result.Error, retry, retryTimes) {
		recordResult(c, meta, price, result, 0, true)
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
	retryLoop(c, mode, retryState, relayController.Handler, log)
}

func getPreConsumedAmount(usage model.Usage, price model.Price) float64 {
	if usage.InputTokens == 0 || price.InputPrice == 0 {
		return 0
	}
	return decimal.
		NewFromInt(int64(usage.InputTokens)).
		Mul(decimal.NewFromFloat(price.InputPrice)).
		Div(decimal.NewFromInt(model.PriceUnit)).
		InexactFloat64()
}

// recordResult records the consumption for the final result
func recordResult(c *gin.Context, meta *meta.Meta, price model.Price, result *controller.HandleResult, retryTimes int, downstreamResult bool) {
	code := http.StatusOK
	content := ""
	if result.Error != nil {
		code = result.Error.StatusCode
		content = result.Error.JSONOrEmpty()
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
	inputTokens      int64
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

func getInitialChannel(c *gin.Context, modelName string, log *log.Entry) (*initialChannel, error) {
	if channel := middleware.GetChannel(c); channel != nil {
		log.Data["designated_channel"] = "true"
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

	group := middleware.GetGroup(c)
	availableSet := group.GetAvailableSets()

	channel, migratedChannels, err := getChannelWithFallback(mc, availableSet, modelName, errorRates, ids...)
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

func handleRelayResult(c *gin.Context, bizErr *relaymodel.ErrorWithStatusCode, retry bool, retryTimes int) (done bool) {
	if bizErr == nil {
		return true
	}
	if !retry ||
		retryTimes == 0 ||
		c.Request.Context().Err() != nil {
		bizErr.Error.Message = middleware.MessageWithRequestID(c, bizErr.Error.Message)
		c.JSON(bizErr.StatusCode, bizErr)
		return true
	}
	return false
}

func initRetryState(retryTimes int, channel *initialChannel, meta *meta.Meta, result *controller.HandleResult, price model.Price) *retryState {
	state := &retryState{
		retryTimes:       retryTimes,
		ignoreChannelIDs: channel.ignoreChannelIDs,
		errorRates:       channel.errorRates,
		meta:             meta,
		result:           result,
		price:            price,
		inputTokens:      meta.InputTokens,
		migratedChannels: channel.migratedChannels,
	}

	if channel.designatedChannel {
		state.exhausted = true
	}

	if !channelHasPermission(*result.Error) {
		state.ignoreChannelIDs = append(state.ignoreChannelIDs, int64(channel.channel.ID))
	} else {
		state.lastHasPermissionChannel = channel.channel
	}

	return state
}

func retryLoop(c *gin.Context, mode mode.Mode, state *retryState, relayController RelayHandler, log *log.Entry) {
	// do not use for i := range state.retryTimes, because the retryTimes is constant
	i := 0

	for {
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
				recordResult(c, state.meta, state.price, state.result, i, true)
			}
			break
		}
		// when the last request has not recorded the result, record the result
		if state.meta != nil && state.result != nil {
			recordResult(c, state.meta, state.price, state.result, i, false)
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

		state.meta = NewMetaByContext(
			c,
			newChannel,
			mode,
			meta.WithInputTokens(state.inputTokens),
			meta.WithRetryAt(time.Now()),
		)
		var retry bool
		state.result, retry = RelayHelper(state.meta, c, relayController)

		done := handleRetryResult(c, retry, newChannel, state)
		if done || i == state.retryTimes-1 {
			recordResult(c, state.meta, state.price, state.result, i+1, true)
			break
		}

		i++
	}

	if state.result.Error != nil {
		state.result.Error.Error.Message = middleware.MessageWithRequestID(c, state.result.Error.Error.Message)
		c.JSON(state.result.Error.StatusCode, state.result.Error)
	}
}

func getRetryChannel(state *retryState) (*model.Channel, error) {
	if state.exhausted {
		if state.lastHasPermissionChannel == nil {
			return nil, ErrChannelsExhausted
		}
		if shouldDelay(state.result.Error.StatusCode) {
			//nolint:gosec
			time.Sleep(time.Duration(rand.Float64()*float64(time.Second)) + time.Second)
		}
		return state.lastHasPermissionChannel, nil
	}

	newChannel, err := getRandomChannel(state.migratedChannels, state.errorRates, state.ignoreChannelIDs...)
	if err != nil {
		if !errors.Is(err, ErrChannelsExhausted) || state.lastHasPermissionChannel == nil {
			return nil, err
		}
		state.exhausted = true
		if shouldDelay(state.result.Error.StatusCode) {
			//nolint:gosec
			time.Sleep(time.Duration(rand.Float64()*float64(time.Second)) + time.Second)
		}
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

func handleRetryResult(ctx *gin.Context, retry bool, newChannel *model.Channel, state *retryState) (done bool) {
	if ctx.Request.Context().Err() != nil {
		return true
	}
	if !retry || state.result.Error == nil {
		return true
	}

	hasPermission := channelHasPermission(*state.result.Error)

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
func shouldRetry(_ *gin.Context, relayErr relaymodel.ErrorWithStatusCode) bool {
	if relayErr.Error.Code == controller.ErrInvalidChannelTypeCode {
		return false
	}
	_, ok := channelNoRetryStatusCodesMap[relayErr.StatusCode]
	return !ok
}

var channelNoPermissionStatusCodesMap = map[int]struct{}{
	http.StatusUnauthorized:    {},
	http.StatusPaymentRequired: {},
	http.StatusForbidden:       {},
	http.StatusNotFound:        {},
}

func channelHasPermission(relayErr relaymodel.ErrorWithStatusCode) bool {
	if relayErr.Error.Code == controller.ErrInvalidChannelTypeCode {
		return false
	}
	_, ok := channelNoPermissionStatusCodesMap[relayErr.StatusCode]
	return !ok
}

func shouldDelay(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusServiceUnavailable
}

func RelayNotImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": &relaymodel.Error{
			Message: "API not implemented",
			Type:    middleware.ErrorTypeAIPROXY,
			Code:    "api_not_implemented",
		},
	})
}
