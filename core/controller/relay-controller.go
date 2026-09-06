package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/cache"
	"github.com/labring/aiproxy/core/relay/plugin/cachefollow"
	monitorplugin "github.com/labring/aiproxy/core/relay/plugin/monitor"
	"github.com/labring/aiproxy/core/relay/plugin/patch"
	"github.com/labring/aiproxy/core/relay/plugin/streamfake"
	"github.com/labring/aiproxy/core/relay/plugin/thinksplit"
	"github.com/labring/aiproxy/core/relay/plugin/timeout"
	websearch "github.com/labring/aiproxy/core/relay/plugin/web-search"
	log "github.com/sirupsen/logrus"
)

// https://platform.openai.com/docs/api-reference/chat

type (
	RelayHandler    func(*gin.Context, *meta.Meta) *controller.HandleResult
	GetRequestUsage func(*gin.Context, model.ModelConfig) (controller.RequestUsage, error)
	GetRequestPrice func(*gin.Context, model.ModelConfig) (model.Price, error)
	ValidateRequest func(*gin.Context, model.ModelConfig) error
)

type RelayController struct {
	GetRequestUsage GetRequestUsage
	GetRequestPrice GetRequestPrice
	ValidateRequest ValidateRequest
	Handler         RelayHandler
}

var AdaptorStore adaptor.Store = &storeImpl{}

type storeImpl struct{}

func (s *storeImpl) GetStore(group string, tokenID int, id string) (adaptor.StoreCache, error) {
	store, err := model.CacheGetStore(group, tokenID, id)
	if err != nil {
		return adaptor.StoreCache{}, err
	}

	return adaptor.StoreCache{
		ID:        store.ID,
		GroupID:   store.GroupID,
		TokenID:   store.TokenID,
		ChannelID: store.ChannelID,
		Model:     store.Model,
		Metadata:  store.Metadata,
		CreatedAt: store.CreatedAt,
		UpdatedAt: store.UpdatedAt,
		ExpiresAt: store.ExpiresAt,
	}, nil
}

func (s *storeImpl) SaveStore(store adaptor.StoreCache) error {
	return s.SaveStoreWithOption(store, adaptor.SaveStoreOption{})
}

func (s *storeImpl) SaveStoreWithOption(
	store adaptor.StoreCache,
	opt adaptor.SaveStoreOption,
) error {
	_, err := model.SaveStoreWithOption(&model.StoreV2{
		ID:        store.ID,
		GroupID:   store.GroupID,
		TokenID:   store.TokenID,
		ChannelID: store.ChannelID,
		Model:     store.Model,
		Metadata:  store.Metadata,
		CreatedAt: store.CreatedAt,
		UpdatedAt: store.UpdatedAt,
		ExpiresAt: store.ExpiresAt,
	}, model.SaveStoreOption{
		MinUpdateInterval: opt.MinUpdateInterval,
	})

	return err
}

func (s *storeImpl) SaveIfNotExistStore(store adaptor.StoreCache) error {
	_, err := model.SaveIfNotExistStore(&model.StoreV2{
		ID:        store.ID,
		GroupID:   store.GroupID,
		TokenID:   store.TokenID,
		ChannelID: store.ChannelID,
		Model:     store.Model,
		Metadata:  store.Metadata,
		CreatedAt: store.CreatedAt,
		UpdatedAt: store.UpdatedAt,
		ExpiresAt: store.ExpiresAt,
	})

	return err
}

func wrapPlugin(ctx context.Context, mc *model.ModelCaches, a adaptor.Adaptor) adaptor.Adaptor {
	return plugin.WrapperAdaptor(a,
		monitorplugin.NewGroupMonitorPlugin(),
		cache.NewCachePlugin(common.RDB),
		cachefollow.NewCacheFollowPlugin(),
		streamfake.NewStreamFakePlugin(),
		timeout.NewTimeoutPlugin(),
		websearch.NewWebSearchPlugin(func(modelName string) (*model.Channel, error) {
			return getWebSearchChannel(ctx, mc, modelName)
		}),
		thinksplit.NewThinkPlugin(),
		monitorplugin.NewChannelMonitorPlugin(),
		patch.NewPatchPlugin(),
	)
}

func relayHandler(c *gin.Context, meta *meta.Meta, mc *model.ModelCaches) *controller.HandleResult {
	log := common.GetLogger(c)
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

	adaptor = wrapPlugin(c.Request.Context(), mc, adaptor)

	return controller.Handle(adaptor, c, meta, AdaptorStore, buildBodyDetailOption(meta))
}

func defaultPriceFunc(_ *gin.Context, mc model.ModelConfig) (model.Price, error) {
	return mc.Price, nil
}

func relayController(m mode.Mode) RelayController {
	c := RelayController{
		Handler: func(c *gin.Context, meta *meta.Meta) *controller.HandleResult {
			return relayHandler(c, meta, middleware.GetModelCaches(c))
		},
		GetRequestPrice: defaultPriceFunc,
	}

	switch m {
	case mode.ImagesGenerations:
		c.ValidateRequest = controller.ValidateImagesRequest
		c.GetRequestPrice = controller.GetImagesRequestPrice
		c.GetRequestUsage = controller.GetImagesRequestUsage
	case mode.ImagesEdits:
		c.ValidateRequest = controller.ValidateImagesEditsRequest
		c.GetRequestPrice = controller.GetImagesEditsRequestPrice
		c.GetRequestUsage = controller.GetImagesEditsRequestUsage
	case mode.AudioSpeech:
		c.GetRequestUsage = controller.GetTTSRequestUsage
	case mode.AudioTranslation, mode.AudioTranscription:
		c.GetRequestUsage = controller.GetSTTRequestUsage
	case mode.ParsePdf:
		c.GetRequestUsage = controller.GetPdfRequestUsage
	case mode.Rerank:
		c.GetRequestUsage = controller.GetRerankRequestUsage
	case mode.Anthropic:
		c.GetRequestUsage = controller.GetAnthropicRequestUsage
	case mode.ChatCompletions:
		c.GetRequestUsage = controller.GetChatRequestUsage
	case mode.Gemini:
		c.GetRequestUsage = controller.GetGeminiRequestUsage
	case mode.Embeddings:
		c.GetRequestUsage = controller.GetEmbedRequestUsage
	case mode.Completions:
		c.GetRequestUsage = controller.GetCompletionsRequestUsage
	case mode.VideoGenerationsJobs:
		c.ValidateRequest = controller.ValidateVideoGenerationJobRequest
		c.GetRequestPrice = controller.GetVideoGenerationJobRequestPrice
		c.GetRequestUsage = controller.GetVideoGenerationJobRequestUsage
	case mode.Videos, mode.VideosRemix, mode.VideosEdits, mode.VideosExtensions:
		c.ValidateRequest = controller.ValidateVideosRequest
		c.GetRequestPrice = controller.GetVideosRequestPrice
		c.GetRequestUsage = controller.GetVideosRequestUsage
	case mode.GeminiVideo:
		c.ValidateRequest = controller.ValidateGeminiVideoRequest
		c.GetRequestPrice = controller.GetGeminiVideoRequestPrice
		c.GetRequestUsage = controller.GetGeminiVideoRequestUsage
	case mode.AliVideo:
		c.ValidateRequest = controller.ValidateAliVideoRequest
		c.GetRequestPrice = controller.GetAliVideoRequestPrice
		c.GetRequestUsage = controller.GetAliVideoRequestUsage
	case mode.DoubaoVideo:
		c.ValidateRequest = controller.ValidateDoubaoVideoRequest
		c.GetRequestPrice = controller.GetDoubaoVideoRequestPrice
		c.GetRequestUsage = controller.GetDoubaoVideoRequestUsage
	case mode.Responses, mode.ResponsesCompact:
		c.GetRequestUsage = controller.GetResponsesRequestUsage
	}

	return c
}

func RelayHelper(
	c *gin.Context,
	meta *meta.Meta,
	handel RelayHandler,
) (*controller.HandleResult, bool) {
	result := handel(c, meta)
	if result.Error == nil {
		return result, false
	}

	return result, monitorplugin.ShouldRetry(result.Error)
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

	if relayController.ValidateRequest != nil {
		if err := relayController.ValidateRequest(c, mc); err != nil {
			statusCode := http.StatusInternalServerError

			if requestParamErr, ok := errors.AsType[*controller.RequestParamError](err); ok {
				statusCode = requestParamErr.StatusCode
			}

			middleware.AbortLogWithMessageWithMode(mode, c,
				statusCode,
				err.Error(),
			)

			return
		}
	}

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

		meta.RequestUsage = requestUsage.Usage
		meta.RequestUsageContext = requestUsage.Context
	}

	meta.RequestUsageContext.ServiceTier = meta.RequestServiceTier

	gbc := middleware.GetGroupBalanceConsumerFromContext(c)

	requiredBalance := math.Max(
		consume.CalculateAmountWithOptions(
			http.StatusOK,
			meta.RequestUsage,
			meta.RequestUsageContext,
			price,
			model.PriceSelectionOptions{
				DisableResolutionFuzzyMatch: mc.DisableResolutionFuzzyMatch,
				RequestAt:                   meta.RequestAt,
			},
		),
		middleware.GroupMinimumBalance,
	)
	if !gbc.CheckBalance(requiredBalance) {
		middleware.AbortLogWithMessageWithMode(mode, c,
			http.StatusForbidden,
			fmt.Sprintf("group (%s) balance not enough", gbc.Group),
			relaymodel.WithType(middleware.GroupBalanceNotEnough),
		)

		return
	}

	// First attempt
	firstAttemptAt := time.Now()
	retryTimes, retryDeadline := getRetryLimits(
		mc,
		config.GetRetryTimes(),
		config.GetRetryBudget(),
		firstAttemptAt,
	)
	result, retry := RelayHelper(c, meta, relayController.Handler)

	if handleRelayResult(c, result.Error, retry, retryTimes, retryDeadline) {
		recordResult(
			c,
			meta,
			price,
			result,
			0,
			true,
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
		time.Now(),
	)
	retryState.retryDeadline = retryDeadline

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

	var firstByteAt time.Time
	if result.BodyDetail != nil {
		firstByteAt = result.BodyDetail.FirstByteAt
	}

	forceSaveDetail := config.GetSaveAllLogDetail() || meta.ModelConfig.ForceSaveDetail
	if forceSaveDetail || code != http.StatusOK {
		detail = buildRequestDetailForLog(
			result.BodyDetail,
			meta.ModelConfig,
			code,
			forceSaveDetail,
		)
	}

	gbc := middleware.GetGroupBalanceConsumerFromContext(c)
	usageContext := result.UsageContext.WithFallback(meta.RequestUsageContext)

	amount := consume.CalculateAmountWithOptions(
		code,
		result.Usage,
		usageContext,
		price,
		model.PriceSelectionOptions{
			DisableResolutionFuzzyMatch: meta.ModelConfig.DisableResolutionFuzzyMatch,
			RequestAt:                   meta.RequestAt,
		},
	)
	if amount > 0 {
		log := common.GetLogger(c)
		log.Data["amount"] = strconv.FormatFloat(amount, 'f', -1, 64)
	}

	asyncUsageStatus := model.AsyncUsageStatusNone
	if downstreamResult && result.Error == nil && result.AsyncUsage {
		asyncUsageStatus = model.AsyncUsageStatusPending
	}

	consume.AsyncConsume(
		gbc.Consumer,
		code,
		firstByteAt,
		meta,
		result.Usage,
		usageContext,
		price,
		content,
		c.ClientIP(),
		retryTimes,
		detail,
		downstreamResult,
		metadata,
		result.UpstreamID,
		asyncUsageStatus,
	)

	if asyncUsageStatus == model.AsyncUsageStatusPending {
		saveAsyncUsageInfo(meta, price, result)
	}
}

func saveAsyncUsageInfo(
	meta *meta.Meta,
	price model.Price,
	result *controller.HandleResult,
) {
	if result.UpstreamID == "" {
		log.Warnf("skip async usage without upstream id, request_id: %s", meta.RequestID)
		return
	}

	if err := model.CreateAsyncUsageInfo(&model.AsyncUsageInfo{
		RequestID:                   meta.RequestID,
		RequestAt:                   meta.RequestAt,
		Mode:                        int(meta.Mode),
		Model:                       meta.OriginModel,
		ChannelID:                   meta.Channel.ID,
		BaseURL:                     meta.Channel.BaseURL,
		GroupID:                     meta.Group.ID,
		TokenID:                     meta.Token.ID,
		TokenName:                   meta.Token.Name,
		Price:                       price,
		UpstreamID:                  result.UpstreamID,
		UsageContext:                result.UsageContext.WithFallback(meta.RequestUsageContext),
		DisableResolutionFuzzyMatch: meta.ModelConfig.DisableResolutionFuzzyMatch,
	}); err != nil {
		log.Errorf("failed to save async usage info: %v", err)
	}
}

func effectiveDetailBodyMaxSize(modelLimit, globalLimit int64) int64 {
	if modelLimit != 0 {
		return modelLimit
	}

	return globalLimit
}

func buildRequestDetailForLog(
	bodyDetail *controller.BodyDetail,
	modelConfig model.ModelConfig,
	code int,
	forceSaveDetail bool,
) *model.RequestDetail {
	if bodyDetail == nil {
		return nil
	}

	requestBodyMaxSize := effectiveDetailBodyMaxSize(
		modelConfig.RequestBodyStorageMaxSize,
		config.GetLogDetailRequestBodyMaxSize(),
	)
	responseBodyMaxSize := effectiveDetailBodyMaxSize(
		modelConfig.ResponseBodyStorageMaxSize,
		config.GetLogDetailResponseBodyMaxSize(),
	)

	detail := &model.RequestDetail{
		RequestBody:  bodyDetail.RequestBody,
		ResponseBody: bodyDetail.ResponseBody,
	}
	detail.DropInvalidUTF8Bodies()

	if controller.ShouldSkipRequestBodyDetailForStatus(code) && !forceSaveDetail {
		detail.RequestBody = ""
	}

	detail.ApplyBodySizeLimits(requestBodyMaxSize, responseBodyMaxSize)

	return detail
}

func buildBodyDetailOption(meta *meta.Meta) controller.BodyDetailOption {
	requestBodyMaxSize := effectiveDetailBodyMaxSize(
		meta.ModelConfig.RequestBodyStorageMaxSize,
		config.GetLogDetailRequestBodyMaxSize(),
	)
	responseBodyMaxSize := effectiveDetailBodyMaxSize(
		meta.ModelConfig.ResponseBodyStorageMaxSize,
		config.GetLogDetailResponseBodyMaxSize(),
	)

	return controller.BodyDetailOption{
		IncludeRequestBody:  requestBodyMaxSize >= 0,
		IncludeResponseBody: responseBodyMaxSize >= 0,
		MaxRequestBodySize:  requestBodyMaxSize,
		MaxResponseBodySize: responseBodyMaxSize,
	}
}

type retryState struct {
	channelSelectionState

	retryTimes        int
	retryDeadline     time.Time
	designatedChannel *model.Channel
	preferChannelIDs  []int
	ignoreChannelIDs  map[int64]struct{}
	failedChannelIDs  map[int64]struct{} // Track failed channels in the current retry round

	meta                *meta.Meta
	price               model.Price
	requestUsage        model.Usage
	requestUsageContext model.UsageContext
	result              *controller.HandleResult
	migratedChannels    []*model.Channel
	channelRetryInfo    map[int]channelRetryInfo
}

type channelRetryInfo struct {
	failures  int
	lastEndAt time.Time
}

const (
	relayRetryBaseDelay = time.Second
	relayRetryMaxDelay  = 5 * time.Second
	relayRetryMaxJitter = time.Second
)

func getRetryLimits(
	mc model.ModelConfig,
	defaultRetryTimes, defaultRetryBudget int64,
	startedAt time.Time,
) (int, time.Time) {
	retryTimes, budget := mc.RetryLimits(defaultRetryTimes, defaultRetryBudget)

	var deadline time.Time
	if budget > 0 {
		deadline = startedAt.Add(budget)
	}

	return int(retryTimes), deadline
}

func (s *retryState) canRetry(attempts int, now time.Time) bool {
	return (s.retryTimes < 0 || attempts < s.retryTimes) &&
		(s.retryDeadline.IsZero() || now.Before(s.retryDeadline))
}

func handleRelayResult(
	c *gin.Context,
	bizErr adaptor.Error,
	retry bool,
	retryTimes int,
	retryDeadline time.Time,
) (done bool) {
	if bizErr == nil {
		return true
	}

	if !retry ||
		retryTimes == 0 ||
		(!retryDeadline.IsZero() && !time.Now().Before(retryDeadline)) ||
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
	initialEndAt time.Time,
) *retryState {
	state := &retryState{
		channelSelectionState: channel.channelSelectionState,
		retryTimes:            retryTimes,
		preferChannelIDs:      channel.preferChannelIDs,
		ignoreChannelIDs:      channel.ignoreChannelIDs,
		meta:                  meta,
		result:                result,
		price:                 price,
		requestUsage:          meta.RequestUsage,
		requestUsageContext:   meta.RequestUsageContext,
		migratedChannels:      channel.migratedChannels,
		failedChannelIDs:      make(map[int64]struct{}),
		channelRetryInfo:      make(map[int]channelRetryInfo),
	}

	// Record initial failed channel
	state.failedChannelIDs[int64(meta.Channel.ID)] = struct{}{}
	if shouldBackoffStatus(result.Error.StatusCode()) {
		state.recordChannelFailure(meta.Channel.ID, initialEndAt)
	}

	if channel.designatedChannel {
		state.designatedChannel = channel.channel
	}

	if !monitorplugin.ChannelHasPermission(result.Error) {
		if state.ignoreChannelIDs == nil {
			state.ignoreChannelIDs = make(map[int64]struct{})
		}

		state.ignoreChannelIDs[int64(channel.channel.ID)] = struct{}{}
	}

	return state
}

func (s *retryState) recordChannelFailure(channelID int, endAt time.Time) {
	if s.channelRetryInfo == nil {
		s.channelRetryInfo = make(map[int]channelRetryInfo)
	}

	info := s.channelRetryInfo[channelID]
	info.failures++
	info.lastEndAt = endAt
	s.channelRetryInfo[channelID] = info
}

func calculateRelayBackoffDelay(failures int, jitter time.Duration) time.Duration {
	if failures <= 0 {
		return 0
	}

	if jitter < 0 {
		jitter = 0
	}

	if jitter > relayRetryMaxJitter {
		jitter = relayRetryMaxJitter
	}

	delay := time.Duration(failures)*relayRetryBaseDelay + jitter
	if delay > relayRetryMaxDelay {
		return relayRetryMaxDelay
	}

	return delay
}

func (s *retryState) remainingRelayDelay(
	channelID int,
	now time.Time,
	jitter time.Duration,
) time.Duration {
	info, ok := s.channelRetryInfo[channelID]
	if !ok || info.failures <= 0 || info.lastEndAt.IsZero() {
		return 0
	}

	requiredDelay := calculateRelayBackoffDelay(info.failures, jitter)

	elapsed := now.Sub(info.lastEndAt)
	if elapsed >= requiredDelay {
		return 0
	}

	return requiredDelay - elapsed
}

func retryLoop(c *gin.Context, mode mode.Mode, state *retryState, relayController RelayHandler) {
	log := common.GetLogger(c)

	// The budget limits scheduling and backoff, while in-flight requests keep their own timeout.
	ctx := c.Request.Context()
	if !state.retryDeadline.IsZero() {
		var cancel context.CancelFunc

		ctx, cancel = context.WithDeadline(ctx, state.retryDeadline)
		defer cancel()
	}

	i := 0

	for state.canRetry(i, time.Now()) && ctx.Err() == nil {
		newChannel, err := getRetryChannel(ctx, state)
		if err == nil {
			err = prepareRetry(c)
		}

		if err == nil {
			err = relayDelay(ctx, state, newChannel.ID)
		}

		if err != nil {
			if !errors.Is(err, ErrChannelsExhausted) && ctx.Err() == nil {
				log.Errorf("prepare retry failed: %+v", err)
			}

			break
		}

		if !state.canRetry(i, time.Now()) || ctx.Err() != nil {
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
				middleware.GetRequestMetadata(c),
			)
		}

		log.Data["retry"] = strconv.Itoa(i + 1)

		log.Warnf("using channel %s (type: %d, id: %d) to retry (attempt %d)",
			newChannel.Name,
			newChannel.Type,
			newChannel.ID,
			i+1,
		)

		state.meta = NewMetaByContext(
			c,
			newChannel,
			mode,
			meta.WithRequestUsage(state.requestUsage),
			meta.WithRequestUsageContext(state.requestUsageContext),
			meta.WithRetryAt(time.Now()),
		)

		var retry bool

		state.result, retry = RelayHelper(c, state.meta, relayController)
		i++

		if state.result.Error != nil && shouldBackoffStatus(state.result.Error.StatusCode()) {
			state.recordChannelFailure(newChannel.ID, time.Now())
		}

		done := handleRetryResult(c, retry, newChannel, state)

		// Record failed channel if retry is needed
		if !done && state.result.Error != nil {
			state.failedChannelIDs[int64(newChannel.ID)] = struct{}{}
		}

		if done {
			break
		}
	}

	recordResult(
		c,
		state.meta,
		state.price,
		state.result,
		i,
		true,
		middleware.GetRequestMetadata(c),
	)

	if state.result.Error != nil {
		ErrorWithRequestID(c, state.result.Error)
	}
}

func prepareRetry(c *gin.Context) error {
	requestBody, err := common.GetRequestBodyReusable(c.Request)
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

	hasPermission := monitorplugin.ChannelHasPermission(state.result.Error)

	if state.designatedChannel != nil {
		return !hasPermission
	}

	if !hasPermission {
		if state.ignoreChannelIDs == nil {
			state.ignoreChannelIDs = make(map[int64]struct{})
		}

		state.ignoreChannelIDs[int64(newChannel.ID)] = struct{}{}
	}

	return false
}

func shouldBackoffStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusServiceUnavailable
}

func relayDelay(ctx context.Context, state *retryState, channelID int) error {
	jitter := time.Duration(rand.Int64N(int64(relayRetryMaxJitter)))

	delay := state.remainingRelayDelay(channelID, time.Now(), jitter)
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
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

	log := common.GetLogger(c)

	data, err := relayErr.MarshalJSON()
	if err != nil {
		log.Errorf("marshal error failed: %+v", err)
		c.JSON(relayErr.StatusCode(), relayErr)
		return
	}

	node, err := common.GetJSONNodeNoCopy(data)
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
