package controller

import (
	"net/http"
	"slices"
	"sort"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptors"
	log "github.com/sirupsen/logrus"
)

// https://platform.openai.com/docs/api-reference/models/list

type OpenAIModelPermission struct {
	Group              *string `json:"group"`
	ID                 string  `json:"id"`
	Object             string  `json:"object"`
	Organization       string  `json:"organization"`
	Created            int     `json:"created"`
	AllowCreateEngine  bool    `json:"allow_create_engine"`
	AllowSampling      bool    `json:"allow_sampling"`
	AllowLogprobs      bool    `json:"allow_logprobs"`
	AllowSearchIndices bool    `json:"allow_search_indices"`
	AllowView          bool    `json:"allow_view"`
	AllowFineTuning    bool    `json:"allow_fine_tuning"`
	IsBlocking         bool    `json:"is_blocking"`
}

type OpenAIModels struct {
	Parent     *string                 `json:"parent"`
	ID         string                  `json:"id"`
	Object     string                  `json:"object"`
	OwnedBy    string                  `json:"owned_by"`
	Root       string                  `json:"root"`
	Permission []OpenAIModelPermission `json:"permission"`
	Created    int                     `json:"created"`
}

type BuiltinModelConfig model.ModelConfig

func (c *BuiltinModelConfig) MarshalJSON() ([]byte, error) {
	type Alias BuiltinModelConfig
	return sonic.Marshal(&struct {
		*Alias
		CreatedAt int64 `json:"created_at,omitempty"`
		UpdatedAt int64 `json:"updated_at,omitempty"`
	}{
		Alias: (*Alias)(c),
	})
}

func SortBuiltinModelConfigsFunc(i, j BuiltinModelConfig) int {
	return model.SortModelConfigsFunc((model.ModelConfig)(i), (model.ModelConfig)(j))
}

var (
	builtinModels             []BuiltinModelConfig
	builtinModelsMap          map[string]*OpenAIModels
	builtinChannelType2Models map[model.ChannelType][]BuiltinModelConfig
)

var permission = []OpenAIModelPermission{
	{
		ID:                 "modelperm-LwHkVFn8AcMItP432fKKDIKJ",
		Object:             "model_permission",
		Created:            1626777600,
		AllowCreateEngine:  true,
		AllowSampling:      true,
		AllowLogprobs:      true,
		AllowSearchIndices: false,
		AllowView:          true,
		AllowFineTuning:    false,
		Organization:       "*",
		Group:              nil,
		IsBlocking:         false,
	},
}

func init() {
	builtinChannelType2Models = make(map[model.ChannelType][]BuiltinModelConfig)
	builtinModelsMap = make(map[string]*OpenAIModels)
	// https://platform.openai.com/docs/models/model-endpoint-compatibility
	for i, adaptor := range adaptors.ChannelAdaptor {
		modelNames := adaptor.Metadata().Models
		builtinChannelType2Models[i] = make([]BuiltinModelConfig, len(modelNames))
		for idx, _model := range modelNames {
			if _model.Owner == "" {
				_model.Owner = model.ModelOwner(i.String())
			}
			if v, ok := builtinModelsMap[_model.Model]; !ok {
				builtinModelsMap[_model.Model] = &OpenAIModels{
					ID:         _model.Model,
					Object:     "model",
					Created:    1626777600,
					OwnedBy:    string(_model.Owner),
					Permission: permission,
					Root:       _model.Model,
					Parent:     nil,
				}
				builtinModels = append(builtinModels, (BuiltinModelConfig)(_model))
			} else if v.OwnedBy != string(_model.Owner) {
				log.Fatalf("model %s owner mismatch, expect %s, actual %s", _model.Model, string(_model.Owner), v.OwnedBy)
			}
			builtinChannelType2Models[i][idx] = (BuiltinModelConfig)(_model)
		}
	}
	for _, models := range builtinChannelType2Models {
		sort.Slice(models, func(i, j int) bool {
			return models[i].Model < models[j].Model
		})
		slices.SortStableFunc(models, SortBuiltinModelConfigsFunc)
	}
	slices.SortStableFunc(builtinModels, SortBuiltinModelConfigsFunc)
}

// BuiltinModels godoc
//
//	@Summary		Get builtin models
//	@Description	Returns a list of builtin models
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=[]BuiltinModelConfig}
//	@Router			/api/models/builtin [get]
func BuiltinModels(c *gin.Context) {
	middleware.SuccessResponse(c, builtinModels)
}

// ChannelBuiltinModels godoc
//
//	@Summary		Get channel builtin models
//	@Description	Returns a list of channel builtin models
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[int][]BuiltinModelConfig}
//	@Router			/api/models/builtin/channel [get]
func ChannelBuiltinModels(c *gin.Context) {
	middleware.SuccessResponse(c, builtinChannelType2Models)
}

// ChannelBuiltinModelsByType godoc
//
//	@Summary		Get channel builtin models by type
//	@Description	Returns a list of channel builtin models by type
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			type	path		model.ChannelType	true	"Channel type"
//	@Success		200		{object}	middleware.APIResponse{data=[]BuiltinModelConfig}
//	@Router			/api/models/builtin/channel/{type} [get]
func ChannelBuiltinModelsByType(c *gin.Context) {
	channelType := c.Param("type")
	if channelType == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "type is required")
		return
	}
	channelTypeInt, err := strconv.Atoi(channelType)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid type")
		return
	}
	middleware.SuccessResponse(c, builtinChannelType2Models[model.ChannelType(channelTypeInt)])
}

// ChannelDefaultModelsAndMapping godoc
//
//	@Summary		Get channel default models and mapping
//	@Description	Returns a list of channel default models and mapping
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[string]any{models=[]string,mapping=map[string]string}}
//	@Router			/api/models/default [get]
func ChannelDefaultModelsAndMapping(c *gin.Context) {
	middleware.SuccessResponse(c, gin.H{
		"models":  config.GetDefaultChannelModels(),
		"mapping": config.GetDefaultChannelModelMapping(),
	})
}

// ChannelDefaultModelsAndMappingByType godoc
//
//	@Summary		Get channel default models and mapping by type
//	@Description	Returns a list of channel default models and mapping by type
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			type	path		string	true	"Channel type"
//	@Success		200		{object}	middleware.APIResponse{data=map[string]any{models=[]string,mapping=map[string]string}}
//	@Router			/api/models/default/{type} [get]
func ChannelDefaultModelsAndMappingByType(c *gin.Context) {
	channelType := c.Param("type")
	if channelType == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "type is required")
		return
	}
	channelTypeInt, err := strconv.Atoi(channelType)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid type")
		return
	}
	middleware.SuccessResponse(c, gin.H{
		"models":  config.GetDefaultChannelModels()[channelTypeInt],
		"mapping": config.GetDefaultChannelModelMapping()[channelTypeInt],
	})
}

// EnabledModels godoc
//
//	@Summary		Get enabled models
//	@Description	Returns a list of enabled models
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[string][]model.ModelConfig}
//	@Router			/api/models/enabled [get]
func EnabledModels(c *gin.Context) {
	middleware.SuccessResponse(c, model.LoadModelCaches().EnabledModelConfigsBySet)
}

// EnabledModelsSet godoc
//
//	@Summary		Get enabled models by set
//	@Description	Returns a list of enabled models by set
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			set	path		string	true	"Models set"
//	@Success		200	{object}	middleware.APIResponse{data=[]model.ModelConfig}
//	@Router			/api/models/enabled/{set} [get]
func EnabledModelsSet(c *gin.Context) {
	set := c.Param("set")
	if set == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "set is required")
		return
	}
	middleware.SuccessResponse(c, model.LoadModelCaches().EnabledModelConfigsBySet[set])
}

type EnabledModelChannel struct {
	ID   int               `json:"id"`
	Type model.ChannelType `json:"type"`
	Name string            `json:"name"`
}

func newEnabledModelChannel(ch *model.Channel) EnabledModelChannel {
	return EnabledModelChannel{
		ID:   ch.ID,
		Type: ch.Type,
		Name: ch.Name,
	}
}

// EnabledModelChannels godoc
//
//	@Summary		Get enabled models and channels
//	@Description	Returns a list of enabled models
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[string]map[string][]EnabledModelChannel}
//	@Router			/api/models/channel [get]
func EnabledModelChannels(c *gin.Context) {
	raw := model.LoadModelCaches().EnabledModel2ChannelsBySet
	result := make(map[string]map[string][]EnabledModelChannel)

	for set, modelChannels := range raw {
		result[set] = make(map[string][]EnabledModelChannel)
		for model, channels := range modelChannels {
			chs := make([]EnabledModelChannel, len(channels))
			for i, channel := range channels {
				chs[i] = newEnabledModelChannel(channel)
			}
			result[set][model] = chs
		}
	}

	middleware.SuccessResponse(c, result)
}

// EnabledModelChannelsSet godoc
//
//	@Summary		Get enabled models and channels by set
//	@Description	Returns a list of enabled models and channels by set
//	@Tags			model
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			set	path		string	true	"Models set"
//	@Success		200	{object}	middleware.APIResponse{data=map[string][]EnabledModelChannel}
//	@Router			/api/models/channel/{set} [get]
func EnabledModelChannelsSet(c *gin.Context) {
	set := c.Param("set")
	if set == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "set is required")
		return
	}
	raw := model.LoadModelCaches().EnabledModel2ChannelsBySet[set]
	result := make(map[string][]EnabledModelChannel, len(raw))
	for model, channels := range raw {
		chs := make([]EnabledModelChannel, len(channels))
		for i, channel := range channels {
			chs[i] = newEnabledModelChannel(channel)
		}
		result[model] = chs
	}
	middleware.SuccessResponse(c, result)
}
