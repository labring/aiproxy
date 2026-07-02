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

var (
	errRelayModeMismatch   = errors.New("relay mode mismatch")
	errModelConfigNotFound = errors.New("model config not found")
)

type relayModeMismatchError struct {
	modelName string
}

func (e relayModeMismatchError) Error() string {
	return fmt.Sprintf("The model `%s` does not exist on this endpoint.", e.modelName)
}

func (e relayModeMismatchError) Is(target error) bool {
	return target == errRelayModeMismatch
}

type modelConfigNotFoundError struct {
	modelName string
}

func (e modelConfigNotFoundError) Error() string {
	return fmt.Sprintf(
		"The model `%s` does not exist or you do not have access to it.",
		e.modelName,
	)
}

func (e modelConfigNotFoundError) Is(target error) bool {
	return target == errModelConfigNotFound
}

type RelayController struct {
	GetRequestUsage GetRequestUsage
	GetRequestPrice GetRequestPrice
	ValidateRequest ValidateRequest
	Handler         RelayHandler
}

var AdaptorStore adaptor.Store = &storeImpl{}

type storeImpl struct{}

func (s *storeImpl) GetStoreByScope(
	group string,
	tokenID int,
	id string,
	scope model.ChannelScope,
) (adaptor.StoreCache, error) {
	store, err := model.CacheGetStoreByScope(group, tokenID, id, scope)
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

func (s *storeImpl) SaveStore(store adaptor.StoreCache, scope model.ChannelScope) error {
	return s.SaveStoreWithOption(store, scope, adaptor.SaveStoreOption{})
}

func (s *storeImpl) SaveStoreWithOption(
	store adaptor.StoreCache,
	scope model.ChannelScope,
	opt adaptor.SaveStoreOption,
) error {
	_, err := model.SaveStoreWithOptionByScope(&model.StoreV2{
		ID:        store.ID,
		GroupID:   store.GroupID,
		TokenID:   store.TokenID,
		ChannelID: store.ChannelID,
		Model:     store.Model,
		Metadata:  store.Metadata,
		CreatedAt: store.CreatedAt,
		UpdatedAt: store.UpdatedAt,
		ExpiresAt: store.ExpiresAt,
	}, scope, model.SaveStoreOption{
		MinUpdateInterval: opt.MinUpdateInterval,
	})

	return err
}

func (s *storeImpl) SaveIfNotExistStore(
	store adaptor.StoreCache,
	scope model.ChannelScope,
) error {
	_, err := model.SaveIfNotExistStoreByScope(&model.StoreV2{
		ID:        store.ID,
		GroupID:   store.GroupID,
		TokenID:   store.TokenID,
		ChannelID: store.ChannelID,
		Model:     store.Model,
		Metadata:  store.Metadata,
		CreatedAt: store.CreatedAt,
		UpdatedAt: store.UpdatedAt,
		ExpiresAt: store.ExpiresAt,
	}, scope)

	return err
}

type scopedAdaptorStore struct {
	adaptor.Store
	meta *meta.Meta
}

func newScopedAdaptorStore(base adaptor.Store, meta *meta.Meta) adaptor.Store {
	return &scopedAdaptorStore{Store: base, meta: meta}
}

func (s *scopedAdaptorStore) storeWithMetaDefaults(store adaptor.StoreCache) adaptor.StoreCache {
	if s.meta != nil {
		if store.GroupID == "" {
			store.GroupID = s.meta.Group.ID
		}

		if store.TokenID == 0 {
			store.TokenID = s.meta.Token.ID
		}

		if store.ChannelID == 0 {
			store.ChannelID = s.meta.Channel.ID
		}

		if store.Model == "" {
			store.Model = s.meta.OriginModel
		}
	}

	return store
}

func (s *scopedAdaptorStore) effectiveScope(scope model.ChannelScope) model.ChannelScope {
	scope = model.NormalizeChannelScope(scope)

	if s.meta != nil && s.meta.Channel.Scope == model.ChannelScopeGroup {
		return model.ChannelScopeGroup
	}

	return scope
}

func (s *scopedAdaptorStore) GetStoreByScope(
	group string,
	tokenID int,
	id string,
	scope model.ChannelScope,
) (adaptor.StoreCache, error) {
	scope = s.effectiveScope(scope)
	return s.Store.GetStoreByScope(group, tokenID, id, scope)
}

func (s *scopedAdaptorStore) SaveStore(
	store adaptor.StoreCache,
	scope model.ChannelScope,
) error {
	scope = s.effectiveScope(scope)
	return s.Store.SaveStore(s.storeWithMetaDefaults(store), scope)
}

func (s *scopedAdaptorStore) SaveStoreWithOption(
	store adaptor.StoreCache,
	scope model.ChannelScope,
	opt adaptor.SaveStoreOption,
) error {
	scope = s.effectiveScope(scope)
	return s.Store.SaveStoreWithOption(s.storeWithMetaDefaults(store), scope, opt)
}

func (s *scopedAdaptorStore) SaveIfNotExistStore(
	store adaptor.StoreCache,
	scope model.ChannelScope,
) error {
	scope = s.effectiveScope(scope)
	return s.Store.SaveIfNotExistStore(s.storeWithMetaDefaults(store), scope)
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

	return controller.Handle(
		adaptor,
		c,
		meta,
		newScopedAdaptorStore(AdaptorStore, meta),
		buildBodyDetailOption(meta),
	)
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
	case mode.Responses:
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

func NewMetaByScopedChannel(
	c *gin.Context,
	channel *scopedChannel,
	mode mode.Mode,
	opts ...meta.Option,
) *meta.Meta {
	if channel == nil {
		return middleware.NewMetaByContext(c, nil, mode, opts...)
	}

	opts = append(opts, meta.WithChannelScope(channel.scope, channel.groupID))

	return middleware.NewMetaByContext(c, channel.channel, mode, opts...)
}

func resolveScopedModelConfig(
	group model.GroupCache,
	modelCaches *model.ModelCaches,
	channel *scopedChannel,
	modelName string,
) (model.ModelConfig, bool) {
	if channel != nil && channel.isGroupChannel() {
		modelConfig, ok := model.ResolveGroupScopeModelConfig(group.ID, modelName)
		if !ok {
			return model.ModelConfig{}, false
		}

		return middleware.GetGroupScopeAdjustedModelConfig(group, modelConfig), true
	}

	if modelCaches == nil || modelCaches.ModelConfig == nil {
		return model.ModelConfig{}, false
	}

	modelConfig, ok := modelCaches.ModelConfig.GetModelConfig(modelName)
	if !ok {
		return model.ModelConfig{}, false
	}

	return middleware.GetGroupAdjustedModelConfig(group, modelConfig), true
}

type relayAttempt struct {
	channel             *scopedChannel
	modelConfig         model.ModelConfig
	price               model.Price
	requestUsage        model.Usage
	requestUsageContext model.UsageContext
	meta                *meta.Meta
}

func prepareRelayAttempt(
	c *gin.Context,
	m mode.Mode,
	relayController RelayController,
	channel *scopedChannel,
	modelCaches *model.ModelCaches,
	modelName string,
	checkGroupModelLimit bool,
) (*relayAttempt, error) {
	group := middleware.GetGroup(c)

	modelConfig, ok := resolveScopedModelConfig(group, modelCaches, channel, modelName)
	if !ok {
		return nil, modelConfigNotFoundError{modelName: modelName}
	}

	if err := checkRelayModeForAttempt(m, modelName, modelConfig); err != nil {
		return nil, err
	}

	if err := validateRelayRequest(c, relayController, modelConfig); err != nil {
		return nil, err
	}

	attemptMeta := NewMetaByScopedChannel(
		c,
		channel,
		m,
		meta.WithModelConfig(modelConfig),
	)

	token := middleware.GetToken(c)
	if checkGroupModelLimit {
		if err := middleware.CheckGroupModelRPMAndTPM(
			c,
			group,
			modelConfig,
			token.Name,
			channel.scope,
			channel.channel.ID,
		); err != nil {
			consume.Summary(
				http.StatusTooManyRequests,
				time.Time{},
				attemptMeta,
				model.Usage{},
				model.UsageContext{ServiceTier: attemptMeta.RequestServiceTier},
				model.Price{},
				true,
			)

			return nil, err
		}
	}

	price := model.Price{}

	var err error
	if relayController.GetRequestPrice != nil {
		price, err = relayController.GetRequestPrice(c, modelConfig)
		if err != nil {
			return nil, fmt.Errorf("get request price failed: %w", err)
		}
	}

	if relayController.GetRequestUsage != nil {
		requestUsage, err := relayController.GetRequestUsage(c, modelConfig)
		if err != nil {
			return nil, fmt.Errorf("get request usage failed: %w", err)
		}

		attemptMeta.RequestUsage = requestUsage.Usage
		attemptMeta.RequestUsageContext = requestUsage.Context
	}

	attemptMeta.RequestUsageContext.ServiceTier = attemptMeta.RequestServiceTier

	return &relayAttempt{
		channel:             channel,
		modelConfig:         modelConfig,
		price:               price,
		requestUsage:        attemptMeta.RequestUsage,
		requestUsageContext: attemptMeta.RequestUsageContext,
		meta:                attemptMeta,
	}, nil
}

func validateRelayRequest(
	c *gin.Context,
	relayController RelayController,
	modelConfig model.ModelConfig,
) error {
	if relayController.ValidateRequest == nil {
		return nil
	}

	return relayController.ValidateRequest(c, modelConfig)
}

func checkRelayModeForAttempt(m mode.Mode, modelName string, modelConfig model.ModelConfig) error {
	if middleware.CheckRelayMode(m, modelConfig.Type) {
		return nil
	}

	return relayModeMismatchError{modelName: modelName}
}

func relay(c *gin.Context, mode mode.Mode, relayController RelayController) {
	requestModel := middleware.GetRequestModel(c)
	modelCaches := middleware.GetModelCaches(c)

	// Get initial channel
	initialChannel, err := getInitialChannel(c, requestModel, mode)
	if err != nil || initialChannel == nil || initialChannel.channel == nil {
		middleware.AbortLogWithMessageWithMode(mode, c,
			http.StatusServiceUnavailable,
			"the upstream load is saturated, please try again later",
		)

		return
	}

	attempt, err := prepareRelayAttempt(
		c,
		mode,
		relayController,
		initialChannel.channel,
		modelCaches,
		requestModel,
		true,
	)
	if err != nil {
		abortRelayPreparationError(c, mode, err)
		return
	}

	gbc := middleware.GetGroupBalanceConsumerFromContext(c)

	requiredBalance := math.Max(
		consume.CalculateAmountWithOptions(
			http.StatusOK,
			attempt.meta.RequestUsage,
			attempt.meta.RequestUsageContext,
			attempt.price,
			model.PriceSelectionOptions{
				DisableResolutionFuzzyMatch: attempt.modelConfig.DisableResolutionFuzzyMatch,
			},
		),
		middleware.GroupMinimumBalance,
	)
	if attempt.channel.isGroupChannel() {
		requiredBalance = middleware.GroupMinimumBalance
	}

	if !gbc.CheckBalance(requiredBalance) {
		middleware.AbortLogWithMessageWithMode(mode, c,
			http.StatusForbidden,
			fmt.Sprintf("group (%s) balance not enough", gbc.Group),
			relaymodel.WithType(middleware.GroupBalanceNotEnough),
		)

		return
	}

	// First attempt
	result, retry := RelayHelper(c, attempt.meta, relayController.Handler)

	retryTimes := int(config.GetRetryTimes())
	if attempt.modelConfig.RetryTimes > 0 {
		retryTimes = int(attempt.modelConfig.RetryTimes)
	}

	if handleRelayResult(c, result.Error, retry, retryTimes) {
		recordResult(
			c,
			attempt.meta,
			attempt.price,
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
		attempt.meta,
		result,
		attempt.price,
		time.Now(),
		modelCaches,
		requestModel,
		relayController,
		true,
	)

	// Retry loop
	retryLoop(c, mode, retryState, relayController.Handler)
}

func abortRelayPreparationError(c *gin.Context, m mode.Mode, err error) {
	statusCode := http.StatusInternalServerError

	var requestParamErr *controller.RequestParamError
	switch {
	case errors.As(err, &requestParamErr):
		statusCode = requestParamErr.StatusCode
	case errors.Is(err, middleware.ErrRequestRateLimitExceeded),
		errors.Is(err, middleware.ErrRequestTpmLimitExceeded):
		statusCode = http.StatusTooManyRequests
	case errors.Is(err, errRelayModeMismatch):
		statusCode = http.StatusNotFound
	case errors.Is(err, errModelConfigNotFound):
		statusCode = http.StatusNotFound
	}

	middleware.AbortLogWithMessageWithMode(m, c, statusCode, err.Error())
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
		},
	)
	if amount > 0 {
		log := common.GetLogger(c)
		log.Data["amount"] = strconv.FormatFloat(amount, 'f', -1, 64)
	}

	asyncUsageStatus := model.AsyncUsageStatusNone
	if shouldRecordAsyncUsage(meta, result, downstreamResult) {
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

func shouldRecordAsyncUsage(
	meta *meta.Meta,
	result *controller.HandleResult,
	downstreamResult bool,
) bool {
	return meta.Channel.Scope != model.ChannelScopeGroup &&
		downstreamResult &&
		result.Error == nil &&
		result.AsyncUsage
}

func saveAsyncUsageInfo(
	meta *meta.Meta,
	price model.Price,
	result *controller.HandleResult,
) {
	if meta.Channel.Scope == model.ChannelScopeGroup {
		return
	}

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
	retryTimes                           int
	lastMinErrorRateHasPermissionChannel *scopedChannel
	preferChannelKeys                    []string
	ignoreChannelIDs                     map[string]struct{}
	exhausted                            bool
	groupRetryOnly                       bool
	failedChannelIDs                     map[string]struct{} // Track all failed channel monitor keys in this request

	meta                   *meta.Meta
	price                  model.Price
	modelCaches            *model.ModelCaches
	modelName              string
	relayController        RelayController
	groupModelLimitChecked bool
	groupModelLimitKeys    map[string]struct{}
	requestUsage           model.Usage
	requestUsageContext    model.UsageContext
	result                 *controller.HandleResult
	migratedChannels       []*scopedChannel
	channelRetryInfo       map[string]channelRetryInfo
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
	initialEndAt time.Time,
	modelCaches *model.ModelCaches,
	modelName string,
	relayController RelayController,
	groupModelLimitChecked bool,
) *retryState {
	state := &retryState{
		retryTimes:             retryTimes,
		preferChannelKeys:      channel.preferChannelKeys,
		ignoreChannelIDs:       channel.ignoreChannelIDs,
		groupRetryOnly:         channel.groupRetryOnly,
		meta:                   meta,
		result:                 result,
		price:                  price,
		modelCaches:            modelCaches,
		modelName:              modelName,
		relayController:        relayController,
		groupModelLimitChecked: groupModelLimitChecked,
		requestUsage:           meta.RequestUsage,
		requestUsageContext:    meta.RequestUsageContext,
		migratedChannels:       channel.migratedChannels,
		failedChannelIDs:       make(map[string]struct{}),
		channelRetryInfo:       make(map[string]channelRetryInfo),
		groupModelLimitKeys:    make(map[string]struct{}),
	}

	if groupModelLimitChecked {
		state.markGroupModelLimitChecked(channel.channel)
	}

	// Record initial failed channel
	state.failedChannelIDs[meta.ChannelMonitorKey()] = struct{}{}
	if shouldBackoffStatus(result.Error.StatusCode()) {
		state.recordChannelFailure(meta.ChannelMonitorKey(), initialEndAt)
	}

	if channel.designatedChannel {
		state.exhausted = true
	}

	if !monitorplugin.ChannelHasPermission(result.Error) {
		if state.ignoreChannelIDs == nil {
			state.ignoreChannelIDs = make(map[string]struct{})
		}

		state.ignoreChannelIDs[channel.channel.monitorKey()] = struct{}{}
	} else {
		state.lastMinErrorRateHasPermissionChannel = channel.channel
	}

	return state
}

func (s *retryState) markGroupModelLimitChecked(channel *scopedChannel) {
	if channel == nil {
		return
	}

	if channel.isGroupChannel() {
		if s.groupModelLimitKeys == nil {
			s.groupModelLimitKeys = make(map[string]struct{})
		}

		s.groupModelLimitKeys[channel.monitorKey()] = struct{}{}

		return
	}

	s.groupModelLimitChecked = true
}

func (s *retryState) shouldCheckGroupModelLimit(channel *scopedChannel) bool {
	if channel == nil {
		return false
	}

	if channel.isGroupChannel() {
		_, ok := s.groupModelLimitKeys[channel.monitorKey()]
		return !ok
	}

	return !s.groupModelLimitChecked
}

func (s *retryState) recordChannelFailure(channelID string, endAt time.Time) {
	if s.channelRetryInfo == nil {
		s.channelRetryInfo = make(map[string]channelRetryInfo)
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
	channelID string,
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

	// do not use for i := range state.retryTimes, because the retryTimes is constant
	i := 0

	for {
		newChannel, err := getRetryChannel(c.Request.Context(), state)
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
				middleware.GetRequestMetadata(c),
			)
			state.meta = nil
			state.result = nil
		}

		log.Data["retry"] = strconv.Itoa(i + 1)

		log.Warnf("using channel %s (type: %d, id: %d) to retry (remain times %d)",
			newChannel.channel.Name,
			newChannel.channel.Type,
			newChannel.channel.ID,
			state.retryTimes-i,
		)

		relayDelay(state, newChannel.monitorKey())

		attempt, err := prepareRelayAttempt(
			c,
			mode,
			state.relayController,
			newChannel,
			state.modelCaches,
			state.modelName,
			state.shouldCheckGroupModelLimit(newChannel),
		)
		if err != nil {
			abortRelayPreparationError(c, mode, err)

			state.result = nil
			return
		}

		state.markGroupModelLimitChecked(newChannel)

		state.meta = attempt.meta
		state.meta.RetryAt = time.Now()
		state.price = attempt.price
		state.requestUsage = attempt.requestUsage
		state.requestUsageContext = attempt.requestUsageContext

		var retry bool

		state.result, retry = RelayHelper(c, state.meta, relayController)
		if state.result.Error != nil && shouldBackoffStatus(state.result.Error.StatusCode()) {
			state.recordChannelFailure(newChannel.monitorKey(), time.Now())
		}

		done := handleRetryResult(c, retry, newChannel, state)

		// Record failed channel if retry is needed
		if !done && state.result.Error != nil {
			state.failedChannelIDs[newChannel.monitorKey()] = struct{}{}
		}

		if done || i == state.retryTimes-1 {
			recordResult(
				c,
				state.meta,
				state.price,
				state.result,
				i+1,
				true,
				middleware.GetRequestMetadata(c),
			)

			break
		}

		i++
	}

	if state.result != nil && state.result.Error != nil {
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
	newChannel *scopedChannel,
	state *retryState,
) (done bool) {
	if ctx.Request.Context().Err() != nil {
		return true
	}

	if !retry || state.result.Error == nil {
		return true
	}

	hasPermission := monitorplugin.ChannelHasPermission(state.result.Error)

	if state.exhausted {
		if !hasPermission {
			return true
		}
	} else {
		if !hasPermission {
			if state.ignoreChannelIDs == nil {
				state.ignoreChannelIDs = make(map[string]struct{})
			}

			state.ignoreChannelIDs[newChannel.monitorKey()] = struct{}{}
			state.retryTimes++
		} else {
			if state.lastMinErrorRateHasPermissionChannel == nil {
				state.lastMinErrorRateHasPermissionChannel = newChannel
				return false
			}

			currentErrorRate, err := getScopedChannelModelErrorRateByKey(
				ctx.Request.Context(),
				state.lastMinErrorRateHasPermissionChannel.scope,
				state.meta.OriginModel,
				state.lastMinErrorRateHasPermissionChannel.monitorKey(),
			)
			if err != nil {
				return false
			}

			newErrorRate, err := getScopedChannelModelErrorRateByKey(
				ctx.Request.Context(),
				newChannel.scope,
				state.meta.OriginModel,
				newChannel.monitorKey(),
			)
			if err != nil {
				return false
			}

			state.lastMinErrorRateHasPermissionChannel = pickMinErrorRateHasPermissionChannel(
				state.lastMinErrorRateHasPermissionChannel,
				currentErrorRate,
				newChannel,
				newErrorRate,
			)
		}
	}

	return false
}

func shouldBackoffStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusServiceUnavailable
}

func relayDelay(state *retryState, channelID string) {
	jitter := time.Duration(rand.Int64N(int64(relayRetryMaxJitter)))

	delay := state.remainingRelayDelay(channelID, time.Now(), jitter)
	if delay <= 0 {
		return
	}

	time.Sleep(delay)
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
