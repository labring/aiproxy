package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
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
	monitorplugin "github.com/labring/aiproxy/core/relay/plugin/monitor"
	"github.com/labring/aiproxy/core/relay/plugin/patch"
	"github.com/labring/aiproxy/core/relay/plugin/streamfake"
	"github.com/labring/aiproxy/core/relay/plugin/thinksplit"
	"github.com/labring/aiproxy/core/relay/plugin/timeout"
	websearch "github.com/labring/aiproxy/core/relay/plugin/web-search"
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

var adaptorStore adaptor.Store = &storeImpl{}

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
		ExpiresAt: store.ExpiresAt,
	}, nil
}

func (s *storeImpl) SaveStore(store adaptor.StoreCache) error {
	_, err := model.SaveStore(&model.StoreV2{
		ID:        store.ID,
		GroupID:   store.GroupID,
		TokenID:   store.TokenID,
		ChannelID: store.ChannelID,
		Model:     store.Model,
		ExpiresAt: store.ExpiresAt,
	})

	return err
}

func wrapPlugin(ctx context.Context, mc *model.ModelCaches, a adaptor.Adaptor) adaptor.Adaptor {
	return plugin.WrapperAdaptor(a,
		monitorplugin.NewGroupMonitorPlugin(),
		cache.NewCachePlugin(common.RDB),
		streamfake.NewStreamFakePlugin(),
		patch.NewPatchPlugin(),
		timeout.NewTimeoutPlugin(),
		websearch.NewWebSearchPlugin(func(modelName string) (*model.Channel, error) {
			return getWebSearchChannel(ctx, mc, modelName)
		}),
		thinksplit.NewThinkPlugin(),
		monitorplugin.NewChannelMonitorPlugin(),
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

	return controller.Handle(adaptor, c, meta, adaptorStore)
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
		c.GetRequestPrice = controller.GetImagesRequestPrice
		c.GetRequestUsage = controller.GetImagesRequestUsage
	case mode.ImagesEdits:
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
		c.GetRequestUsage = controller.GetVideoGenerationJobRequestUsage
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

		meta.RequestUsage = requestUsage
	}

	gbc := middleware.GetGroupBalanceConsumerFromContext(c)
	if !gbc.CheckBalance(consume.CalculateAmount(http.StatusOK, meta.RequestUsage, price)) {
		middleware.AbortLogWithMessageWithMode(mode, c,
			http.StatusForbidden,
			fmt.Sprintf("group (%s) balance not enough", gbc.Group),
			relaymodel.WithType(middleware.GroupBalanceNotEnough),
		)

		return
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
	if config.GetSaveAllLogDetail() || meta.ModelConfig.ForceSaveDetail || code != http.StatusOK {
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
		log := common.GetLogger(c)
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
	)
}

type retryState struct {
	retryTimes               int
	lastHasPermissionChannel *model.Channel
	ignoreChannelIDs         map[int64]struct{}
	errorRates               map[int64]float64
	exhausted                bool
	failedChannelIDs         map[int64]struct{} // Track all failed channels in this request

	meta             *meta.Meta
	price            model.Price
	requestUsage     model.Usage
	result           *controller.HandleResult
	migratedChannels []*model.Channel
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
		failedChannelIDs: make(map[int64]struct{}),
	}

	// Record initial failed channel
	state.failedChannelIDs[int64(meta.Channel.ID)] = struct{}{}

	if channel.designatedChannel {
		state.exhausted = true
	}

	if !monitorplugin.ChannelHasPermission(result.Error) {
		if state.ignoreChannelIDs == nil {
			state.ignoreChannelIDs = make(map[int64]struct{})
		}

		state.ignoreChannelIDs[int64(channel.channel.ID)] = struct{}{}
	} else {
		state.lastHasPermissionChannel = channel.channel
	}

	return state
}

func retryLoop(c *gin.Context, mode mode.Mode, state *retryState, relayController RelayHandler) {
	log := common.GetLogger(c)

	// do not use for i := range state.retryTimes, because the retryTimes is constant
	i := 0

	for {
		lastStatusCode := state.result.Error.StatusCode()
		lastChannelID := state.meta.Channel.ID

		newChannel, err := getRetryChannel(state, i, state.retryTimes)
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

		// Record failed channel if retry is needed
		if !done && state.result.Error != nil {
			state.failedChannelIDs[int64(newChannel.ID)] = struct{}{}
		}

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

	if state.exhausted {
		if !hasPermission {
			return true
		}
	} else {
		if !hasPermission {
			if state.ignoreChannelIDs == nil {
				state.ignoreChannelIDs = make(map[int64]struct{})
			}

			state.ignoreChannelIDs[int64(newChannel.ID)] = struct{}{}
			state.retryTimes++
		} else {
			state.lastHasPermissionChannel = newChannel
		}
	}

	return false
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

	log := common.GetLogger(c)

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
