package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// GetModelConfigs godoc
//
//	@Summary		Get model configs
//	@Description	Returns a list of model configs with pagination
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			model	query		string	false	"Model name"
//	@Success		200		{object}	middleware.APIResponse{data=map[string]any{configs=[]model.ModelConfig,total=int}}
//	@Router			/api/model_configs/ [get]
func GetModelConfigs(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	_model := c.Query("model")
	configs, total, err := model.GetModelConfigs(page, perPage, _model)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, gin.H{
		"configs": configs,
		"total":   total,
	})
}

// GetAllModelConfigs godoc
//
//	@Summary		Get all model configs
//	@Description	Returns a list of all model configs
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=[]model.ModelConfig}
//	@Router			/api/model_configs/all [get]
func GetAllModelConfigs(c *gin.Context) {
	configs, err := model.GetAllModelConfigs()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, configs)
}

type GetModelConfigsByModelsContainsRequest struct {
	Models []string `json:"models"`
}

// GetModelConfigsByModelsContains godoc
//
//	@Summary		Get model configs by models contains
//	@Description	Returns a list of model configs by models contains
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			models	body		GetModelConfigsByModelsContainsRequest	true	"Models"
//	@Success		200		{object}	middleware.APIResponse{data=[]model.ModelConfig}
//	@Router			/api/model_configs/contains [post]
func GetModelConfigsByModelsContains(c *gin.Context) {
	request := GetModelConfigsByModelsContainsRequest{}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	configs, err := model.GetModelConfigsByModels(request.Models)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, configs)
}

// SearchModelConfigs godoc
//
//	@Summary		Search model configs
//	@Description	Returns a list of model configs by keyword
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			keyword		query		string	false	"Keyword"
//	@Param			model		query		string	false	"Model name"
//	@Param			owner		query		string	false	"Owner"
//	@Param			page		query		int		false	"Page"
//	@Param			per_page	query		int		false	"Per page"
//	@Success		200			{object}	middleware.APIResponse{data=map[string]any{configs=[]model.ModelConfig,total=int}}
//	@Router			/api/model_configs/search [get]
func SearchModelConfigs(c *gin.Context) {
	keyword := c.Query("keyword")
	page, perPage := utils.ParsePageParams(c)
	_model := c.Query("model")
	owner := c.Query("owner")
	configs, total, err := model.SearchModelConfigs(
		keyword,
		page,
		perPage,
		_model,
		model.ModelOwner(owner),
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, gin.H{
		"configs": configs,
		"total":   total,
	})
}

type SaveModelConfigsRequest struct {
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
	model.ModelConfig
}

// SaveModelConfigs godoc
//
//	@Summary		Save model configs
//	@Description	Saves a list of model configs
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			configs	body		[]SaveModelConfigsRequest	true	"Model configs"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/model_configs/ [post]
func SaveModelConfigs(c *gin.Context) {
	var configs []SaveModelConfigsRequest
	if err := c.ShouldBindJSON(&configs); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	modelConfigs := make([]model.ModelConfig, len(configs))
	for i, config := range configs {
		modelConfigs[i] = config.ModelConfig
	}
	err := model.SaveModelConfigs(modelConfigs)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// SaveModelConfig godoc
//
//	@Summary		Save model config
//	@Description	Saves a model config
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			config	body		SaveModelConfigsRequest	true	"Model config"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/model_config/ [post]
func SaveModelConfig(c *gin.Context) {
	var config SaveModelConfigsRequest
	if err := c.ShouldBindJSON(&config); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	err := model.SaveModelConfig(config.ModelConfig)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// DeleteModelConfig godoc
//
//	@Summary		Delete model config
//	@Description	Deletes a model config
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/model_config/{model} [delete]
func DeleteModelConfig(c *gin.Context) {
	_model := c.Param("model")
	err := model.DeleteModelConfig(_model)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// DeleteModelConfigs godoc
//
//	@Summary		Delete model configs
//	@Description	Deletes a list of model configs
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			models	body		[]string	true	"Model names"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/model_configs/batch_delete [post]
func DeleteModelConfigs(c *gin.Context) {
	models := []string{}
	err := c.ShouldBindJSON(&models)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	err = model.DeleteModelConfigsByModels(models)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// GetModelConfig godoc
//
//	@Summary		Get model config
//	@Description	Returns a model config
//	@Tags			modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	middleware.APIResponse{data=model.ModelConfig}
//	@Router			/api/model_config/{model} [get]
func GetModelConfig(c *gin.Context) {
	_model := c.Param("model")
	config, err := model.GetModelConfig(_model)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, config)
}
