package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// ListModels godoc
//
//	@Summary		List models
//	@Description	List all models
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	object{object=string,data=[]OpenAIModels}
//	@Router			/v1/models [get]
func ListModels(c *gin.Context) {
	modelCaches := middleware.GetModelCaches(c)
	group := middleware.GetGroup(c)
	groupChannelMode := middleware.GetGroupChannelMode(c)
	allowedModels := middleware.GetActiveTokenModels(c)

	availableOpenAIModels := make([]*OpenAIModels, 0)

	model.RangeModelsWithAllowList(
		allowedModels,
		middleware.GetActiveAvailableSets(c),
		middleware.GetActiveAvailableModels(c),
		func(modelName string) bool {
			if mc, ok := middleware.ResolveModelConfig(
				group,
				groupChannelMode,
				modelCaches,
				modelName,
			); ok {
				availableOpenAIModels = append(availableOpenAIModels, &OpenAIModels{
					ID:         modelName,
					Object:     "model",
					Created:    1626777600,
					OwnedBy:    string(mc.Owner),
					Root:       modelName,
					Permission: permission,
					Parent:     nil,
				})
			}

			return true
		})

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   availableOpenAIModels,
	})
}

// RetrieveModel godoc
//
//	@Summary		Retrieve model
//	@Description	Retrieve a model
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	OpenAIModels
//	@Router			/v1/models/{model} [get]
func RetrieveModel(c *gin.Context) {
	modelName := c.Param("model")
	findModelName := model.FindModelWithAllowList(
		middleware.GetActiveTokenModels(c),
		modelName,
		middleware.GetActiveAvailableSets(c),
		middleware.GetActiveAvailableModels(c),
	)

	if findModelName == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": &relaymodel.OpenAIError{
				Message: fmt.Sprintf("the model '%s' does not exist", modelName),
				Type:    "invalid_request_error",
				Param:   "model",
				Code:    "model_not_found",
			},
		})

		return
	}

	mc, ok := middleware.ResolveModelConfig(
		middleware.GetGroup(c),
		middleware.GetGroupChannelMode(c),
		middleware.GetModelCaches(c),
		findModelName,
	)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": &relaymodel.OpenAIError{
				Message: fmt.Sprintf("the model '%s' does not exist", modelName),
				Type:    "invalid_request_error",
				Param:   "model",
				Code:    "model_not_found",
			},
		})

		return
	}

	c.JSON(http.StatusOK, &OpenAIModels{
		ID:         findModelName,
		Object:     "model",
		Created:    1626777600,
		OwnedBy:    string(mc.Owner),
		Root:       findModelName,
		Permission: permission,
		Parent:     nil,
	})
}
