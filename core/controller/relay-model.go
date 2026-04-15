package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func buildOpenAIModel(name string, mc model.ModelConfig) *OpenAIModels {
	t := mc.Type
	rpm := mc.RPM
	tpm := mc.TPM

	m := &OpenAIModels{
		ID:         name,
		Object:     "model",
		Created:    1626777600,
		OwnedBy:    string(mc.Owner),
		Root:       name,
		Permission: permission,
		Parent:     nil,
		Type:       &t,
	}

	price := mc.Price
	if price.InputPrice != 0 || price.OutputPrice != 0 || price.PerRequestPrice != 0 ||
		len(price.ConditionalPrices) > 0 {
		m.Price = &price
	}

	if len(mc.ImagePrices) > 0 {
		m.ImagePrices = mc.ImagePrices
	}

	if len(mc.Config) > 0 {
		m.Config = mc.Config
	}

	if rpm > 0 {
		m.RPM = &rpm
	}

	if tpm > 0 {
		m.TPM = &tpm
	}

	return m
}

// ListModels godoc
//
//	@Summary		List models
//	@Description	List all models with pricing and capabilities
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	object{object=string,data=[]OpenAIModels}
//	@Router			/v1/models [get]
func ListModels(c *gin.Context) {
	enabledModelConfigsMap := middleware.GetModelCaches(c).EnabledModelConfigsMap
	token := middleware.GetToken(c)
	group := middleware.GetGroup(c)

	availableOpenAIModels := make([]*OpenAIModels, 0)

	token.Range(func(modelName string) bool {
		if mc, ok := enabledModelConfigsMap[modelName]; ok {
			adjusted := middleware.GetGroupAdjustedModelConfig(group, mc)
			availableOpenAIModels = append(availableOpenAIModels, buildOpenAIModel(modelName, adjusted))
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
//	@Description	Retrieve a model with pricing and capabilities
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

	group := middleware.GetGroup(c)
	adjusted := middleware.GetGroupAdjustedModelConfig(group, mc)

	c.JSON(http.StatusOK, buildOpenAIModel(modelName, adjusted))
}
