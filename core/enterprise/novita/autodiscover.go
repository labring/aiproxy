//go:build enterprise

package novita

import (
	"context"
	"log"
	"slices"

	"github.com/labring/aiproxy/core/controller"
	"github.com/labring/aiproxy/core/enterprise/synccommon"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
	"golang.org/x/sync/singleflight"
)

// discoverGroup collapses concurrent auto-discovery calls for the same model
// into a single execution, preventing redundant DB writes and cache rebuilds.
var discoverGroup singleflight.Group

func init() {
	controller.RegisterPassthroughSuccessHook(onPassthroughFirstSuccess)
}

// onPassthroughFirstSuccess is called (in a background goroutine) after a
// passthrough-unknown request succeeds for the first time. For Novita channels
// it fetches pricing from the management API and registers a ModelConfig so
// subsequent requests are billed correctly and the model appears in users'
// "My Access" model list.
func onPassthroughFirstSuccess(
	ctx context.Context,
	_ int,
	channelType model.ChannelType,
	modelName string,
) {
	switch channelType {
	case model.ChannelTypeNovita:
		_, _, _ = discoverGroup.Do(modelName, func() (any, error) {
			doDiscoverChat(ctx, modelName)
			return nil, nil
		})
	case model.ChannelTypeNovitaMultimodal:
		_, _, _ = discoverGroup.Do(modelName, func() (any, error) {
			doDiscoverMultimodal(ctx, modelName)
			return nil, nil
		})
	}
}

// doDiscoverChat handles auto-discovery for Novita OpenAI-compatible channels.
// Looks up the model in the V2 management API to get pricing and config.
func doDiscoverChat(ctx context.Context, modelName string) {
	if modelExists(modelName) {
		return
	}

	client, err := NewNovitaClient()
	if err != nil {
		log.Printf("novita autodiscover: client creation failed: %v", err)
		return
	}

	cfg := GetNovitaConfig()
	if cfg.MgmtToken == "" {
		registerFallbackModel(modelName)
		return
	}

	remoteModel := discoverV2Model(ctx, client, cfg.MgmtToken, modelName)
	if remoteModel == nil {
		registerFallbackModel(modelName)
		return
	}

	if err := registerNovitaChatModel(modelName, remoteModel, cfg.ExchangeRate); err != nil {
		log.Printf("novita autodiscover: failed to register %s: %v", modelName, err)
		return
	}

	addModelToNovitaChannels(modelName, remoteModel)

	if err := model.InitModelConfigAndChannelCache(); err != nil {
		log.Printf(
			"novita autodiscover: cache refresh failed after registering %s: %v",
			modelName,
			err,
		)
	}

	log.Printf("novita autodiscover: registered model %s (V2 API)", modelName)
}

// doDiscoverMultimodal handles auto-discovery for Novita native multimodal channels.
// Fetches pricing from the multimodal console API, falls back to V2 management API.
func doDiscoverMultimodal(ctx context.Context, modelName string) {
	if modelExists(modelName) {
		return
	}

	client, err := NewNovitaClient()
	if err != nil {
		log.Printf("novita autodiscover: client creation failed: %v", err)
		registerMultimodalFallback(modelName)

		return
	}

	cfg := GetNovitaConfig()

	var perRequestPrice float64

	if cfg.MgmtToken != "" {
		perRequestPrice = discoverMultimodalPrice(
			ctx,
			client,
			cfg.MgmtToken,
			modelName,
			cfg.ExchangeRate,
		)

		// Fallback: try V2 management API for token-based pricing
		if perRequestPrice == 0 {
			if remoteModel := discoverV2Model(
				ctx,
				client,
				cfg.MgmtToken,
				modelName,
			); remoteModel != nil {
				if err := registerNovitaChatModel(
					modelName,
					remoteModel,
					cfg.ExchangeRate,
				); err != nil {
					log.Printf("novita autodiscover: failed to register %s: %v", modelName, err)
					return
				}

				finalizeMultimodalDiscovery(modelName, "V2 API")

				return
			}
		}
	}

	// Register with per-request pricing (or zero if no pricing found).
	// Default to PPIONative for genuinely unknown multimodal models;
	// the next daily sync will correct the type if V2 data becomes available.
	mc := model.ModelConfig{
		Model: modelName,
		Owner: model.ModelOwnerNovita,
		Type:  mode.PPIONative,
		RPM:   60,
		TPM:   1000000,
	}

	if perRequestPrice > 0 {
		mc.Price.PerRequestPrice = model.ZeroNullFloat64(perRequestPrice)
	}

	if err := model.DB.Save(&mc).Error; err != nil {
		log.Printf("novita autodiscover: failed to register %s: %v", modelName, err)
		return
	}

	finalizeMultimodalDiscovery(modelName, "multimodal API")
}

// ─── helpers ────────────────────────────────────────────────────────────────

// modelExists checks if a ModelConfig entry already exists for the given model.
func modelExists(modelName string) bool {
	var count int64
	if err := model.DB.Model(&model.ModelConfig{}).
		Where("model = ?", modelName).
		Count(&count).Error; err != nil {
		log.Printf("novita autodiscover: count check failed for %s: %v", modelName, err)
		return false
	}

	return count > 0
}

// discoverV2Model searches the V2 management API for a model by name.
func discoverV2Model(
	ctx context.Context,
	client *NovitaClient,
	mgmtToken, modelName string,
) *NovitaModelV2 {
	all, err := client.FetchAllModels(ctx, mgmtToken)
	if err != nil {
		log.Printf("novita autodiscover: FetchAllModels failed (non-fatal): %v", err)
		return nil
	}

	for i := range all {
		if all[i].ID == modelName {
			return &all[i]
		}
	}

	return nil
}

// discoverMultimodalPrice fetches the multimodal model catalog and returns the
// minimum SKU price for the given model. Returns 0 if not found.
func discoverMultimodalPrice(
	ctx context.Context,
	client *NovitaClient,
	mgmtToken, modelName string,
	exchangeRate float64,
) float64 {
	mmModels, err := client.FetchMultimodalModels(ctx, mgmtToken)
	if err != nil {
		log.Printf("novita autodiscover: FetchMultimodalModels failed (non-fatal): %v", err)
		return 0
	}

	for i := range mmModels {
		if mmModels[i].FusionConfig.Name != modelName {
			continue
		}

		skuCodes := mmModels[i].collectSKUCodes()
		if len(skuCodes) == 0 {
			return 0
		}

		prices, priceErr := client.FetchMultimodalPrices(ctx, mgmtToken, skuCodes)
		if priceErr != nil {
			log.Printf(
				"novita autodiscover: FetchMultimodalPrices failed (non-fatal): %v",
				priceErr,
			)
			return 0
		}

		return mmModels[i].minSKUPrice(prices, exchangeRate)
	}

	return 0
}

// registerNovitaChatModel creates a ModelConfig entry for a Novita chat model
// using V2 management API data.
func registerNovitaChatModel(
	modelName string,
	remoteModel *NovitaModelV2,
	exchangeRate float64,
) error {
	mc := model.ModelConfig{
		Model:  modelName,
		Owner:  model.ModelOwnerNovita,
		Type:   modeFromEndpoints(remoteModel.ModelType, remoteModel.Endpoints),
		RPM:    60,
		TPM:    1000000,
		Config: synccommon.ToModelConfigKeys(buildConfigFromV2Model(remoteModel)),
	}

	if remoteModel.RPM > 0 {
		mc.RPM = int64(remoteModel.RPM)
	}

	if remoteModel.TPM > 0 {
		mc.TPM = int64(remoteModel.TPM)
	}

	setPriceFromV2Model(&mc.Price, remoteModel, exchangeRate)

	return model.DB.Save(&mc).Error
}

// registerFallbackModel creates a zero-cost ModelConfig entry when no pricing
// data is available. The model will be properly priced on the next daily sync.
func registerFallbackModel(modelName string) {
	mc := model.ModelConfig{
		Model: modelName,
		Owner: model.ModelOwnerNovita,
		Type:  mode.ChatCompletions,
		RPM:   60,
		TPM:   1000000,
	}

	if err := model.DB.Save(&mc).Error; err != nil {
		log.Printf("novita autodiscover: failed to register fallback %s: %v", modelName, err)
		return
	}

	addModelToFirstChannelByType(modelName, model.ChannelTypeNovita)

	if err := model.InitModelConfigAndChannelCache(); err != nil {
		log.Printf(
			"novita autodiscover: cache refresh failed after registering %s: %v",
			modelName,
			err,
		)
	}

	log.Printf("novita autodiscover: registered model %s (fallback, zero-cost)", modelName)
}

// registerMultimodalFallback creates a zero-cost multimodal ModelConfig entry.
func registerMultimodalFallback(modelName string) {
	mc := model.ModelConfig{
		Model: modelName,
		Owner: model.ModelOwnerNovita,
		Type:  mode.PPIONative,
		RPM:   60,
		TPM:   1000000,
	}

	if err := model.DB.Save(&mc).Error; err != nil {
		log.Printf(
			"novita autodiscover: failed to register multimodal fallback %s: %v",
			modelName,
			err,
		)
		return
	}

	finalizeMultimodalDiscovery(modelName, "fallback")
}

// addModelToNovitaChannels adds a model to the appropriate Novita channels
// (OpenAI and optionally Anthropic) based on its endpoint configuration.
func addModelToNovitaChannels(modelName string, remoteModel *NovitaModelV2) {
	addModelToFirstChannelByType(modelName, model.ChannelTypeNovita)

	if remoteModel != nil && slices.Contains(remoteModel.Endpoints, "anthropic") {
		addModelToAnthropicChannel(modelName)
	}
}

// addModelToFirstChannelByType adds a model to the first channel of the given type.
// Silently returns if no such channel exists or the model is already present.
func addModelToFirstChannelByType(modelName string, channelType model.ChannelType) {
	var ch model.Channel
	if err := model.DB.Where("type = ?", channelType).First(&ch).Error; err != nil {
		return
	}

	if slices.Contains(ch.Models, modelName) {
		return
	}

	ch.Models = append(ch.Models, modelName)

	if err := model.DB.Save(&ch).Error; err != nil {
		log.Printf(
			"novita autodiscover: failed to add %s to channel type %d: %v",
			modelName,
			channelType,
			err,
		)
	}
}

// addModelToAnthropicChannel adds a model to the first Novita Anthropic channel.
// Uses novitaChannelWhere because Anthropic channels share ChannelTypeAnthropic
// with non-Novita channels, so we filter by base_url as well.
func addModelToAnthropicChannel(modelName string) {
	var channels []model.Channel
	if err := model.DB.Where(novitaChannelWhere(), novitaChannelArgs()...).
		Where("type = ?", model.ChannelTypeAnthropic).
		Find(&channels).Error; err != nil || len(channels) == 0 {
		return
	}

	ch := &channels[0]
	if slices.Contains(ch.Models, modelName) {
		return
	}

	ch.Models = append(ch.Models, modelName)

	if err := model.DB.Save(ch).Error; err != nil {
		log.Printf("novita autodiscover: failed to add %s to Anthropic channel: %v", modelName, err)
	}
}

// finalizeMultimodalDiscovery adds the model to the multimodal channel and
// refreshes the cache. Called after successful model registration.
func finalizeMultimodalDiscovery(modelName, source string) {
	addModelToFirstChannelByType(modelName, model.ChannelTypeNovitaMultimodal)

	if err := model.InitModelConfigAndChannelCache(); err != nil {
		log.Printf(
			"novita autodiscover: cache refresh failed after registering %s: %v",
			modelName,
			err,
		)
	}

	log.Printf("novita autodiscover: registered model %s (%s)", modelName, source)
}
