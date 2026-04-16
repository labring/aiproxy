//go:build enterprise

package ppio

import (
	"context"
	"fmt"
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
// passthrough-unknown request succeeds for the first time. For PPIO multimodal
// channels it fetches pricing from the management API and registers a
// ModelConfig so subsequent requests are billed correctly and the model
// appears in users' "My Access" model list.
func onPassthroughFirstSuccess(
	ctx context.Context,
	_ int,
	channelType model.ChannelType,
	modelName string,
) {
	if channelType != model.ChannelTypePPIOMultimodal {
		return
	}

	_, _, _ = discoverGroup.Do(modelName, func() (any, error) {
		doDiscover(ctx, modelName)
		return nil, nil
	})
}

func doDiscover(ctx context.Context, modelName string) {
	// Guard: skip if the model was already registered between the relay
	// response and this goroutine being scheduled.
	var count int64
	if err := model.DB.Model(&model.ModelConfig{}).
		Where("model = ?", modelName).
		Count(&count).Error; err != nil {
		log.Printf("ppio autodiscover: count check failed for %s: %v", modelName, err)
		return
	}

	if count > 0 {
		return
	}

	// Try to fetch pricing from the multimodal console API first (covers
	// image/video/audio models that the V2 management API doesn't return).
	var perRequestPrice float64

	client, clientErr := NewPPIOClient()
	if clientErr == nil {
		cfg := GetPPIOConfig()
		if cfg.MgmtToken != "" {
			perRequestPrice = discoverMultimodalPrice(ctx, client, cfg.MgmtToken, modelName)

			// Fallback: try V2 management API for token-based pricing
			if perRequestPrice == 0 {
				if remoteModel := discoverV2Model(
					ctx,
					client,
					cfg.MgmtToken,
					modelName,
				); remoteModel != nil {
					if err := registerPPIONativeModel(modelName, remoteModel); err != nil {
						log.Printf("ppio autodiscover: failed to register %s: %v", modelName, err)
						return
					}

					finalizeDiscovery(modelName, "V2 API")

					return
				}
			}
		}
	} else {
		log.Printf("ppio autodiscover: client creation failed (non-fatal): %v", clientErr)
	}

	// Register with per-request pricing (or zero if no pricing found)
	mc := model.ModelConfig{
		Model: modelName,
		Owner: model.ModelOwnerPPIO,
		Type:  mode.PPIONative,
		RPM:   60,
		TPM:   1000000,
	}

	if perRequestPrice > 0 {
		mc.Price.PerRequestPrice = model.ZeroNullFloat64(perRequestPrice)
	}

	if err := model.DB.Save(&mc).Error; err != nil {
		log.Printf("ppio autodiscover: failed to register %s: %v", modelName, err)
		return
	}

	finalizeDiscovery(modelName, fmt.Sprintf("per_request_price=%.4f", perRequestPrice))
}

// finalizeDiscovery adds the model to the multimodal channel and refreshes the
// cache. Called after successful model registration in doDiscover.
func finalizeDiscovery(modelName, source string) {
	addModelToMultimodalChannel(modelName)

	if err := model.InitModelConfigAndChannelCache(); err != nil {
		log.Printf(
			"ppio autodiscover: cache refresh failed after registering %s: %v",
			modelName,
			err,
		)
	}

	log.Printf("ppio autodiscover: registered model %s (%s)", modelName, source)
}

// discoverMultimodalPrice fetches the multimodal model catalog and returns the
// minimum SKU price for the given model. Returns 0 if the model is not found or
// pricing is unavailable.
func discoverMultimodalPrice(
	ctx context.Context,
	client *PPIOClient,
	mgmtToken, modelName string,
) float64 {
	mmModels, err := client.FetchMultimodalModels(ctx, mgmtToken)
	if err != nil {
		log.Printf("ppio autodiscover: FetchMultimodalModels failed (non-fatal): %v", err)
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
			log.Printf("ppio autodiscover: FetchMultimodalPrices failed (non-fatal): %v", priceErr)
			return 0
		}

		return mmModels[i].minSKUPrice(prices)
	}

	return 0
}

// discoverV2Model searches the V2 management API for a model by name.
// Returns nil if not found.
func discoverV2Model(
	ctx context.Context,
	client *PPIOClient,
	mgmtToken, modelName string,
) *PPIOModelV2 {
	all, err := client.FetchAllModels(ctx, mgmtToken)
	if err != nil {
		log.Printf("ppio autodiscover: FetchAllModels failed (non-fatal): %v", err)
		return nil
	}

	for i := range all {
		if all[i].ID == modelName {
			return &all[i]
		}
	}

	return nil
}

// registerPPIONativeModel creates a ModelConfig entry for a PPIO native
// multimodal model. When remoteModel is non-nil, pricing, config, and the
// mode type are sourced from the management API; otherwise sensible
// zero-cost defaults apply with PPIONative as the type.
func registerPPIONativeModel(modelName string, remoteModel *PPIOModelV2) error {
	mc := model.ModelConfig{
		Model: modelName,
		Owner: model.ModelOwnerPPIO,
		Type:  mode.PPIONative,
		RPM:   60,
		TPM:   1000000,
	}

	if remoteModel != nil {
		// Use the V2 model's model_type and endpoints to infer the correct
		// mode — this prevents non-multimodal models (e.g. embedding, chat)
		// from being misclassified as PPIONative.
		mc.Type = inferModeFromPPIO(remoteModel.ModelType, remoteModel.Endpoints)

		mc.Config = synccommon.ToModelConfigKeys(buildConfigFromPPIOModelV2(remoteModel))
		if remoteModel.RPM > 0 {
			mc.RPM = int64(remoteModel.RPM)
		}

		if remoteModel.TPM > 0 {
			mc.TPM = int64(remoteModel.TPM)
		}

		setPriceFromV2Model(&mc.Price, remoteModel)
	}

	return model.DB.Save(&mc).Error
}

// addModelToMultimodalChannel adds a model to the first PPIO multimodal
// channel's model list. This fixes a bug where auto-discovered models were
// registered in ModelConfig but never added to the channel, making them
// invisible to the routing system until the next daily sync.
func addModelToMultimodalChannel(modelName string) {
	var ch model.Channel
	if err := model.DB.Where("type = ?", model.ChannelTypePPIOMultimodal).
		First(&ch).
		Error; err != nil {
		return
	}

	if slices.Contains(ch.Models, modelName) {
		return
	}

	ch.Models = append(ch.Models, modelName)

	if err := model.DB.Save(&ch).Error; err != nil {
		log.Printf("ppio autodiscover: failed to add %s to multimodal channel: %v", modelName, err)
	}
}
