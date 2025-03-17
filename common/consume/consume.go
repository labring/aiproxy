package consume

import (
	"context"
	"sync"
	"time"

	"github.com/labring/aiproxy/common/balance"
	"github.com/labring/aiproxy/common/notify"
	"github.com/labring/aiproxy/model"
	"github.com/labring/aiproxy/relay/meta"
	relaymodel "github.com/labring/aiproxy/relay/model"
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
	usage *relaymodel.Usage,
	meta *meta.Meta,
	inputPrice,
	outputPrice float64,
	cachedPrice float64,
	cacheCreationPrice float64,
	content string,
	ip string,
	retryTimes int,
	requestDetail *model.RequestDetail,
	downstreamResult bool,
) {
	consumeWaitGroup.Add(1)
	defer func() {
		consumeWaitGroup.Done()
		if r := recover(); r != nil {
			log.Errorf("panic in consume: %v", r)
		}
	}()

	go Consume(
		context.Background(),
		postGroupConsumer,
		code,
		usage,
		meta,
		inputPrice,
		outputPrice,
		cachedPrice,
		cacheCreationPrice,
		content,
		ip,
		retryTimes,
		requestDetail,
		downstreamResult,
	)
}

func Consume(
	ctx context.Context,
	postGroupConsumer balance.PostGroupConsumer,
	code int,
	usage *relaymodel.Usage,
	meta *meta.Meta,
	inputPrice,
	outputPrice float64,
	cachedPrice float64,
	cacheCreationPrice float64,
	content string,
	ip string,
	retryTimes int,
	requestDetail *model.RequestDetail,
	downstreamResult bool,
) {
	amount := CalculateAmount(usage, inputPrice, outputPrice, cachedPrice, cacheCreationPrice)

	amount = consumeAmount(ctx, amount, postGroupConsumer, meta)

	err := recordConsume(meta,
		code,
		usage,
		inputPrice,
		outputPrice,
		cachedPrice,
		cacheCreationPrice,
		content,
		ip,
		requestDetail,
		amount,
		retryTimes,
		downstreamResult,
	)
	if err != nil {
		log.Error("error batch record consume: " + err.Error())
		notify.ErrorThrottle("recordConsume", time.Minute, "record consume failed", err.Error())
	}
}

func consumeAmount(
	ctx context.Context,
	amount float64,
	postGroupConsumer balance.PostGroupConsumer,
	meta *meta.Meta,
) float64 {
	if amount > 0 {
		return processGroupConsume(ctx, amount, postGroupConsumer, meta)
	}
	return 0
}

func CalculateAmount(
	usage *relaymodel.Usage,
	inputPrice, outputPrice, cachedPrice, cacheCreationPrice float64,
) float64 {
	if usage == nil {
		return 0
	}

	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	var cachedTokens int
	var cacheCreationTokens int
	if usage.PromptTokensDetails != nil {
		cachedTokens = usage.PromptTokensDetails.CachedTokens
		cacheCreationTokens = usage.PromptTokensDetails.CacheCreationTokens
	}

	if cachedPrice > 0 {
		promptTokens -= cachedTokens
	}
	if cacheCreationPrice > 0 {
		promptTokens -= cacheCreationTokens
	}

	promptAmount := decimal.NewFromInt(int64(promptTokens)).
		Mul(decimal.NewFromFloat(inputPrice)).
		Div(decimal.NewFromInt(model.PriceUnit))
	completionAmount := decimal.NewFromInt(int64(completionTokens)).
		Mul(decimal.NewFromFloat(outputPrice)).
		Div(decimal.NewFromInt(model.PriceUnit))
	cachedAmount := decimal.NewFromInt(int64(cachedTokens)).
		Mul(decimal.NewFromFloat(cachedPrice)).
		Div(decimal.NewFromInt(model.PriceUnit))
	cacheCreationAmount := decimal.NewFromInt(int64(cacheCreationTokens)).
		Mul(decimal.NewFromFloat(cacheCreationPrice)).
		Div(decimal.NewFromInt(model.PriceUnit))

	return promptAmount.
		Add(completionAmount).
		Add(cachedAmount).
		Add(cacheCreationAmount).
		InexactFloat64()
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

func recordConsume(
	meta *meta.Meta,
	code int,
	usage *relaymodel.Usage,
	inputPrice,
	outputPrice float64,
	cachedPrice float64,
	cacheCreationPrice float64,
	content string,
	ip string,
	requestDetail *model.RequestDetail,
	amount float64,
	retryTimes int,
	downstreamResult bool,
) error {
	promptTokens := 0
	completionTokens := 0
	cachedTokens := 0
	cacheCreationTokens := 0
	if usage != nil {
		promptTokens = usage.PromptTokens
		completionTokens = usage.CompletionTokens
		if usage.PromptTokensDetails != nil {
			cachedTokens = usage.PromptTokensDetails.CachedTokens
			cacheCreationTokens = usage.PromptTokensDetails.CacheCreationTokens
		}
	}

	var channelID int
	if meta.Channel != nil {
		channelID = meta.Channel.ID
	}

	return model.BatchRecordConsume(
		meta.RequestID,
		meta.RequestAt,
		meta.Group.ID,
		code,
		channelID,
		promptTokens,
		completionTokens,
		cachedTokens,
		cacheCreationTokens,
		meta.OriginModel,
		meta.Token.ID,
		meta.Token.Name,
		amount,
		inputPrice,
		outputPrice,
		cachedPrice,
		cacheCreationPrice,
		meta.Endpoint,
		content,
		int(meta.Mode),
		ip,
		retryTimes,
		requestDetail,
		downstreamResult,
	)
}
