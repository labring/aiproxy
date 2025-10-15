package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
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
	enabledModelConfigsMap := middleware.GetModelCaches(c).EnabledModelConfigsMap
	token := middleware.GetToken(c)

	availableOpenAIModels := make([]*OpenAIModels, 0)

	token.Range(func(model string) bool {
		if mc, ok := enabledModelConfigsMap[model]; ok {
			availableOpenAIModels = append(availableOpenAIModels, &OpenAIModels{
				ID:         model,
				Object:     "model",
				Created:    1626777600,
				OwnedBy:    string(mc.Owner),
				Root:       model,
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
	token := middleware.GetToken(c)
	modelName := c.Param("model")
	findModelName := token.FindModel(modelName)
	enabledModelConfigsMap := middleware.GetModelCaches(c).EnabledModelConfigsMap

	mc, ok := enabledModelConfigsMap[findModelName]
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
		ID:         modelName,
		Object:     "model",
		Created:    1626777600,
		OwnedBy:    string(mc.Owner),
		Root:       modelName,
		Permission: permission,
		Parent:     nil,
	})
}
