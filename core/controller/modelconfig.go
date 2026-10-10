package controller

import (
	"net/http"
	"strings"

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

type SaveModelConfigsRequest = model.ModelConfig

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

	err := model.SaveModelConfigs(configs)
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
//	@Router			/api/model_config/{model} [post]
func SaveModelConfig(c *gin.Context) {
	var config SaveModelConfigsRequest
	if err := c.ShouldBindJSON(&config); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	modelName := strings.TrimPrefix(c.Param("model"), "/")
	if modelName == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	config.Model = modelName

	err := model.SaveModelConfig(config)
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
	_model := strings.TrimPrefix(c.Param("model"), "/")

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
	_model := strings.TrimPrefix(c.Param("model"), "/")

	config, err := model.GetModelConfig(_model)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, config)
}

// GetGroupScopeModelConfigs godoc
//
//	@Summary		Get group scope model configs
//	@Description	Returns group-channel scope model configs with pagination for a specific group
//	@Tags			group-scope-modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group		path		string	true	"Group name"
//	@Param			model		query		string	false	"Model name"
//	@Param			page		query		int		false	"Page"
//	@Param			per_page	query		int		false	"Per page"
//	@Success		200			{object}	middleware.APIResponse{data=map[string]any{configs=[]model.GroupScopeModelConfig,total=int}}
//	@Router			/api/group/{group}/scope_model_configs/ [get]
func GetGroupScopeModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	page, perPage := utils.ParsePageParams(c)

	configs, total, err := model.GetGroupScopeModelConfigs(
		group,
		page,
		perPage,
		c.Query("model"),
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

// GetAllGroupScopeModelConfigs godoc
//
//	@Summary		Get all group scope model configs
//	@Description	Returns all group-channel scope model configs for a specific group
//	@Tags			group-scope-modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Success		200		{object}	middleware.APIResponse{data=[]model.GroupScopeModelConfig}
//	@Router			/api/group/{group}/scope_model_configs/all [get]
func GetAllGroupScopeModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	configs, err := model.GetAllGroupScopeModelConfigs(group)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, configs)
}

// GetGroupScopeModelConfigsByModelsContains godoc
//
//	@Summary		Get group scope model configs by models
//	@Description	Returns group-channel scope model configs for the requested models in a specific group
//	@Tags			group-scope-modelconfig
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string									true	"Group name"
//	@Param			models	body		GetModelConfigsByModelsContainsRequest	true	"Models"
//	@Success		200		{object}	middleware.APIResponse{data=[]model.GroupScopeModelConfig}
//	@Router			/api/group/{group}/scope_model_configs/contains [post]
func GetGroupScopeModelConfigsByModelsContains(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	request := GetModelConfigsByModelsContainsRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	configs, err := model.GetGroupScopeModelConfigsByModels(group, request.Models)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, configs)
}

// SearchGroupScopeModelConfigs godoc
//
//	@Summary		Search group scope model configs
//	@Description	Returns group-channel scope model configs by keyword for a specific group
//	@Tags			group-scope-modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group		path		string	true	"Group name"
//	@Param			keyword		query		string	false	"Keyword"
//	@Param			model		query		string	false	"Model name"
//	@Param			owner		query		string	false	"Owner"
//	@Param			page		query		int		false	"Page"
//	@Param			per_page	query		int		false	"Per page"
//	@Success		200			{object}	middleware.APIResponse{data=map[string]any{configs=[]model.GroupScopeModelConfig,total=int}}
//	@Router			/api/group/{group}/scope_model_configs/search [get]
func SearchGroupScopeModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	page, perPage := utils.ParsePageParams(c)

	configs, total, err := model.SearchGroupScopeModelConfigs(
		group,
		c.Query("keyword"),
		c.Query("model"),
		page,
		perPage,
		model.ModelOwner(c.Query("owner")),
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

type SaveGroupScopeModelConfigsRequest = model.GroupScopeModelConfig

// SaveGroupScopeModelConfigs godoc
//
//	@Summary		Save group scope model configs
//	@Description	Saves group-channel scope model configs for a specific group
//	@Tags			group-scope-modelconfig
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string								true	"Group name"
//	@Param			configs	body		[]SaveGroupScopeModelConfigsRequest	true	"Model configs"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/scope_model_configs/ [post]
func SaveGroupScopeModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	var configs []SaveGroupScopeModelConfigsRequest
	if err := c.ShouldBindJSON(&configs); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.SaveGroupScopeModelConfigs(group, configs); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// SaveGroupScopeModelConfig godoc
//
//	@Summary		Save group scope model config
//	@Description	Saves a group-channel scope model config for a specific group
//	@Tags			group-scope-modelconfig
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string								true	"Group name"
//	@Param			model	path		string								true	"Model name"
//	@Param			config	body		SaveGroupScopeModelConfigsRequest	true	"Model config"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/scope_model_config/{model} [post]
func SaveGroupScopeModelConfig(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	var config SaveGroupScopeModelConfigsRequest
	if err := c.ShouldBindJSON(&config); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	modelName := strings.TrimPrefix(c.Param("model"), "/")
	if modelName == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	config.GroupID = group

	config.Model = modelName
	if err := model.SaveGroupScopeModelConfig(config); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteGroupScopeModelConfig godoc
//
//	@Summary		Delete group scope model config
//	@Description	Deletes a group-channel scope model config for a specific group
//	@Tags			group-scope-modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/scope_model_config/{model} [delete]
func DeleteGroupScopeModelConfig(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	modelName := strings.TrimPrefix(c.Param("model"), "/")
	if err := model.DeleteGroupScopeModelConfig(group, modelName); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteGroupScopeModelConfigs godoc
//
//	@Summary		Delete group scope model configs
//	@Description	Deletes group-channel scope model configs by model names for a specific group
//	@Tags			group-scope-modelconfig
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string		true	"Group name"
//	@Param			models	body		[]string	true	"Model names"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/scope_model_configs/batch_delete [post]
func DeleteGroupScopeModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	models := []string{}
	if err := c.ShouldBindJSON(&models); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.DeleteGroupScopeModelConfigsByModels(group, models); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// GetGroupScopeModelConfig godoc
//
//	@Summary		Get group scope model config
//	@Description	Returns a group-channel scope model config for a specific group
//	@Tags			group-scope-modelconfig
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	middleware.APIResponse{data=model.GroupScopeModelConfig}
//	@Router			/api/group/{group}/scope_model_config/{model} [get]
func GetGroupScopeModelConfig(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group is required")
		return
	}

	modelName := strings.TrimPrefix(c.Param("model"), "/")

	config, err := model.GetGroupScopeModelConfig(group, modelName)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, config)
}
