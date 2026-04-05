//go:build enterprise

package ppio

import (
	"context"
	"log"

	"golang.org/x/sync/singleflight"

	"github.com/labring/aiproxy/core/controller"
	"github.com/labring/aiproxy/core/enterprise/synccommon"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

// discoverGroup collapses concurrent auto-discovery calls for the same model
// into a single execution, preventing redundant DB writes and cache rebuilds.
var discoverGroup singleflight.Group

func init() {
	controller.PassthroughSuccessHook = onPassthroughFirstSuccess
}

// onPassthroughFirstSuccess is called (in a background goroutine) after a
// passthrough-unknown request succeeds for the first time. For PPIO multimodal
// channels it fetches pricing from the management API and registers a
// ModelConfig so subsequent requests are billed correctly and the model
// appears in users' "My Access" model list.
func onPassthroughFirstSuccess(ctx context.Context, _ int, channelType model.ChannelType, modelName string) {
	if channelType != model.ChannelTypePPIOMultimodal {
		return
	}

	// singleflight collapses concurrent calls for the same model into one
	// execution, preventing redundant DB writes and cache rebuilds.
	discoverGroup.Do(modelName, func() (any, error) { //nolint:errcheck
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

	// Try to fetch pricing from the management API.
	var remoteModel *PPIOModelV2

	client, clientErr := NewPPIOClient()
	if clientErr == nil {
		cfg := GetPPIOConfig()
		if cfg.MgmtToken != "" {
			all, fetchErr := client.FetchAllModels(ctx, cfg.MgmtToken)
			if fetchErr == nil {
				for i := range all {
					if all[i].ID == modelName {
						remoteModel = &all[i]
						break
					}
				}
			} else {
				log.Printf("ppio autodiscover: FetchAllModels failed (non-fatal): %v", fetchErr)
			}
		}
	} else {
		log.Printf("ppio autodiscover: client creation failed (non-fatal): %v", clientErr)
	}

	if err := registerPPIONativeModel(modelName, remoteModel); err != nil {
		log.Printf("ppio autodiscover: failed to register %s: %v", modelName, err)
		return
	}

	if err := model.InitModelConfigAndChannelCache(); err != nil {
		log.Printf("ppio autodiscover: cache refresh failed after registering %s: %v", modelName, err)
	}

	log.Printf("ppio autodiscover: registered model %s", modelName)
}

// registerPPIONativeModel creates a ModelConfig entry for a PPIO native
// multimodal model. When remoteModel is non-nil, pricing and config are
// sourced from the management API; otherwise sensible zero-cost defaults apply.
func registerPPIONativeModel(modelName string, remoteModel *PPIOModelV2) error {
	mc := model.ModelConfig{
		Model: modelName,
		Owner: model.ModelOwnerPPIO,
		Type:  mode.PPIONative,
		RPM:   60,
		TPM:   1000000,
	}

	if remoteModel != nil {
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
