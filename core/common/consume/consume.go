package consume

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

var consumeWaitGroup sync.WaitGroup

func Wait() {
	consumeWaitGroup.Wait()
}

func AsyncConsume(
	postGroupConsumer balance.PostGroupConsumer,
	code int,
	firstByteAt time.Time,
	meta *meta.Meta,
	usage model.Usage,
	usageContext model.UsageContext,
	modelPrice model.Price,
	content string,
	ip string,
	retryTimes int,
	requestDetail *model.RequestDetail,
	downstreamResult bool,
	metadata map[string]string,
	upstreamID string,
	asyncUsageStatus model.AsyncUsageStatus,
) {
	if !checkNeedRecordConsume(code, meta) {
		return
	}

	consumeWaitGroup.Add(1)
	go func() {
		defer func() {
			consumeWaitGroup.Done()

			if r := recover(); r != nil {
				log.Errorf("panic in consume: %v", r)
			}
		}()

		Consume(
			context.Background(),
			time.Now(),
			postGroupConsumer,
			firstByteAt,
			code,
			meta,
			usage,
			usageContext,
			modelPrice,
			content,
			ip,
			retryTimes,
			requestDetail,
			downstreamResult,
			metadata,
			upstreamID,
			asyncUsageStatus,
		)
	}()
}

func Consume(
	ctx context.Context,
	now time.Time,
	postGroupConsumer balance.PostGroupConsumer,
	firstByteAt time.Time,
	code int,
	meta *meta.Meta,
	usage model.Usage,
	usageContext model.UsageContext,
	modelPrice model.Price,
	content string,
	ip string,
	retryTimes int,
	requestDetail *model.RequestDetail,
	downstreamResult bool,
	metadata map[string]string,
	upstreamID string,
	asyncUsageStatus model.AsyncUsageStatus,
) {
	if !checkNeedRecordConsume(code, meta) {
		return
	}

	groupChannel := meta != nil && meta.Channel.Scope == model.ChannelScopeGroup
	if groupChannel {
		asyncUsageStatus = model.AsyncUsageStatusNone
	}

	recordUsage := usage

	amountDetail := model.Amount{}
	if asyncUsageStatus == model.AsyncUsageStatusPending {
		recordUsage = model.Usage{}
	} else {
		amountDetail = CalculateAmountDetailWithOptions(
			code,
			recordUsage,
			usageContext,
			modelPrice,
			priceSelectionOptions(meta),
		)
	}

	if downstreamResult {
		if !groupChannel {
			// TODO: add record actual consume amount
			_ = consumeAmount(ctx, amountDetail.UsedAmount, postGroupConsumer, meta)
		}
	} else if amountDetail.UsedAmount != 0 {
		log.Warnf(
			"not downstream result but used amount is not zero, request_id: %s, used_amount: %f",
			meta.RequestID,
			amountDetail.UsedAmount,
		)
	}

	selectedModelPrice := model.Price{}
	if asyncUsageStatus != model.AsyncUsageStatusPending {
		selectedModelPrice = modelPrice.SelectConditionalPriceWithOptions(
			usage,
			usageContext,
			priceSelectionOptions(meta),
		)
		selectedModelPrice.ConditionalPrices = nil
	}

	err := recordConsume(
		now,
		meta,
		code,
		firstByteAt,
		recordUsage,
		usageContext,
		selectedModelPrice,
		content,
		ip,
		requestDetail,
		amountDetail,
		retryTimes,
		downstreamResult,
		metadata,
		upstreamID,
		asyncUsageStatus,
	)
	if err != nil {
		log.Error("error batch record consume: " + err.Error())

		if groupChannel {
			return
		}

		notify.ErrorThrottle("recordConsume", time.Minute*5, "record consume failed", err.Error())
	}
}

func Summary(
	code int,
	firstByteAt time.Time,
	meta *meta.Meta,
	usage model.Usage,
	usageContext model.UsageContext,
	modelPrice model.Price,
	downstreamResult bool,
) {
	amountDetail := CalculateAmountDetailWithOptions(
		code,
		usage,
		usageContext,
		modelPrice,
		priceSelectionOptions(meta),
	)

	recordSummary(
		time.Now(),
		meta,
		code,
		firstByteAt,
		usage,
		amountDetail,
		downstreamResult,
		usageContext.ServiceTier,
	)
}

func checkNeedRecordConsume(code int, meta *meta.Meta) bool {
	if meta == nil {
		return true
	}

	switch meta.Mode {
	case mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.VideosGet,
		mode.VideosContent,
		mode.VideosDelete,
		mode.GeminiFiles,
		mode.GeminiVideoOperations,
		mode.AliVideoTasks,
		mode.DoubaoVideoTasks,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems:
		return code != http.StatusOK
	case mode.DoubaoVideoTasksDelete:
		return code != http.StatusOK && code != http.StatusNoContent
	default:
		return true
	}
}

func NeedRecordConsumeForTest(code int, meta *meta.Meta) bool {
	return checkNeedRecordConsume(code, meta)
}

func consumeAmount(
	ctx context.Context,
	amount float64,
	postGroupConsumer balance.PostGroupConsumer,
	meta *meta.Meta,
) float64 {
	if amount > 0 && postGroupConsumer != nil {
		return processGroupConsume(ctx, amount, postGroupConsumer, meta)
	}
	return amount
}

func CalculateAmountDetail(
	code int,
	usage model.Usage,
	usageContext model.UsageContext,
	modelPrice model.Price,
) model.Amount {
	return CalculateAmountDetailWithOptions(
		code,
		usage,
		usageContext,
		modelPrice,
		model.PriceSelectionOptions{},
	)
}

func CalculateAmountDetailWithOptions(
	code int,
	usage model.Usage,
	usageContext model.UsageContext,
	modelPrice model.Price,
	options model.PriceSelectionOptions,
) model.Amount {
	if modelPrice.PerRequestPrice != 0 {
		if code != http.StatusOK {
			return model.Amount{}
		}

		return model.Amount{
			UsedAmount: float64(modelPrice.PerRequestPrice),
		}
	}

	modelPrice = modelPrice.SelectConditionalPriceWithOptions(usage, usageContext, options)

	inputTokens := usage.InputTokens
	if modelPrice.ImageInputPrice > 0 {
		inputTokens -= usage.ImageInputTokens
	}

	if modelPrice.AudioInputPrice > 0 {
		inputTokens -= usage.AudioInputTokens
	}

	if modelPrice.VideoInputPrice > 0 {
		inputTokens -= usage.VideoInputTokens
	}

	if modelPrice.CachedPrice > 0 {
		inputTokens -= usage.CachedTokens
	}

	if modelPrice.CacheCreationPrice > 0 {
		inputTokens -= usage.CacheCreationTokens
	}

	outputTokens := usage.OutputTokens
	if modelPrice.ImageOutputPrice > 0 {
		outputTokens -= usage.ImageOutputTokens
	}

	if modelPrice.AudioOutputPrice > 0 {
		outputTokens -= usage.AudioOutputTokens
	}

	outputPrice := float64(modelPrice.OutputPrice)

	outputPriceUnit := modelPrice.GetOutputPriceUnit()
	if usage.ReasoningTokens != 0 && modelPrice.ThinkingModeOutputPrice != 0 {
		outputPrice = float64(modelPrice.ThinkingModeOutputPrice)
		if modelPrice.ThinkingModeOutputPriceUnit != 0 {
			outputPriceUnit = int64(modelPrice.ThinkingModeOutputPriceUnit)
		}
	}

	inputAmount := decimal.NewFromInt(int64(inputTokens)).
		Mul(decimal.NewFromFloat(float64(modelPrice.InputPrice))).
		Div(decimal.NewFromInt(modelPrice.GetInputPriceUnit()))

	imageInputAmount := decimal.NewFromInt(int64(usage.ImageInputTokens)).
		Mul(decimal.NewFromFloat(float64(modelPrice.ImageInputPrice))).
		Div(decimal.NewFromInt(modelPrice.GetImageInputPriceUnit()))

	audioInputAmount := decimal.NewFromInt(int64(usage.AudioInputTokens)).
		Mul(decimal.NewFromFloat(float64(modelPrice.AudioInputPrice))).
		Div(decimal.NewFromInt(modelPrice.GetAudioInputPriceUnit()))

	videoInputAmount := decimal.NewFromInt(int64(usage.VideoInputTokens)).
		Mul(decimal.NewFromFloat(float64(modelPrice.VideoInputPrice))).
		Div(decimal.NewFromInt(modelPrice.GetVideoInputPriceUnit()))

	cachedAmount := decimal.NewFromInt(int64(usage.CachedTokens)).
		Mul(decimal.NewFromFloat(float64(modelPrice.CachedPrice))).
		Div(decimal.NewFromInt(modelPrice.GetCachedPriceUnit()))

	cacheCreationAmount := decimal.NewFromInt(int64(usage.CacheCreationTokens)).
		Mul(decimal.NewFromFloat(float64(modelPrice.CacheCreationPrice))).
		Div(decimal.NewFromInt(modelPrice.GetCacheCreationPriceUnit()))

	webSearchAmount := decimal.NewFromInt(int64(usage.WebSearchCount)).
		Mul(decimal.NewFromFloat(float64(modelPrice.WebSearchPrice))).
		Div(decimal.NewFromInt(modelPrice.GetWebSearchPriceUnit()))

	outputAmount := decimal.NewFromInt(int64(outputTokens)).
		Mul(decimal.NewFromFloat(outputPrice)).
		Div(decimal.NewFromInt(outputPriceUnit))

	imageOutputAmount := decimal.NewFromInt(int64(usage.ImageOutputTokens)).
		Mul(decimal.NewFromFloat(float64(modelPrice.ImageOutputPrice))).
		Div(decimal.NewFromInt(modelPrice.GetImageOutputPriceUnit()))

	audioOutputAmount := decimal.NewFromInt(int64(usage.AudioOutputTokens)).
		Mul(decimal.NewFromFloat(float64(modelPrice.AudioOutputPrice))).
		Div(decimal.NewFromInt(modelPrice.GetAudioOutputPriceUnit()))

	usedAmount := inputAmount.
		Add(imageInputAmount).
		Add(audioInputAmount).
		Add(videoInputAmount).
		Add(cachedAmount).
		Add(cacheCreationAmount).
		Add(webSearchAmount).
		Add(outputAmount).
		Add(imageOutputAmount).
		Add(audioOutputAmount).
		InexactFloat64()

	return model.Amount{
		InputAmount:         inputAmount.InexactFloat64(),
		ImageInputAmount:    imageInputAmount.InexactFloat64(),
		AudioInputAmount:    audioInputAmount.InexactFloat64(),
		VideoInputAmount:    videoInputAmount.InexactFloat64(),
		OutputAmount:        outputAmount.InexactFloat64(),
		ImageOutputAmount:   imageOutputAmount.InexactFloat64(),
		AudioOutputAmount:   audioOutputAmount.InexactFloat64(),
		CachedAmount:        cachedAmount.InexactFloat64(),
		CacheCreationAmount: cacheCreationAmount.InexactFloat64(),
		WebSearchAmount:     webSearchAmount.InexactFloat64(),
		UsedAmount:          usedAmount,
	}
}

func CalculateAmount(
	code int,
	usage model.Usage,
	usageContext model.UsageContext,
	modelPrice model.Price,
) float64 {
	return CalculateAmountDetail(code, usage, usageContext, modelPrice).UsedAmount
}

func CalculateAmountWithOptions(
	code int,
	usage model.Usage,
	usageContext model.UsageContext,
	modelPrice model.Price,
	options model.PriceSelectionOptions,
) float64 {
	return CalculateAmountDetailWithOptions(
		code,
		usage,
		usageContext,
		modelPrice,
		options,
	).UsedAmount
}

func priceSelectionOptions(meta *meta.Meta) model.PriceSelectionOptions {
	if meta == nil {
		return model.PriceSelectionOptions{}
	}

	return model.PriceSelectionOptions{
		DisableResolutionFuzzyMatch: meta.ModelConfig.DisableResolutionFuzzyMatch,
	}
}

func processGroupConsume(
	ctx context.Context,
	amount float64,
	postGroupConsumer balance.PostGroupConsumer,
	meta *meta.Meta,
) float64 {
	consumedAmount, err := postGroupConsumer.PostGroupConsume(ctx, meta.Token.Name, amount)
	if err != nil {
		log.Error("error consuming token remain amount: " + err.Error())

		if err := model.CreateConsumeError(
			meta.RequestID,
			meta.RequestAt,
			meta.Group.ID,
			meta.Token.Name,
			meta.OriginModel,
			err.Error(),
			amount,
			meta.Token.ID,
		); err != nil {
			log.Error("failed to create consume error: " + err.Error())
		}

		return amount
	}

	return consumedAmount
}
