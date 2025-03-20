package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/middleware"
	"github.com/labring/aiproxy/monitor"
)

// GetAllChannelModelErrorRates godoc
//
//	@Summary		Get all channel model error rates
//	@Description	Returns a list of all channel model error rates
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[int64]map[string]float64}
//	@Router			/api/monitor [get]
func GetAllChannelModelErrorRates(c *gin.Context) {
	rates, err := monitor.GetAllChannelModelErrorRates(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}
	middleware.SuccessResponse(c, rates)
}

// GetChannelModelErrorRates godoc
//
//	@Summary		Get channel model error rates
//	@Description	Returns a list of channel model error rates
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Channel ID"
//	@Success		200	{object}	middleware.APIResponse{data=[]map[string]float64}
//	@Router			/api/monitor/{id} [get]
func GetChannelModelErrorRates(c *gin.Context) {
	channelID := c.Param("id")
	channelIDInt, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, "Invalid channel ID")
		return
	}
	rates, err := monitor.GetChannelModelErrorRates(c.Request.Context(), channelIDInt)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}
	middleware.SuccessResponse(c, rates)
}

// ClearAllModelErrors godoc
//
//	@Summary		Clear all model errors
//	@Description	Clears all model errors
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		204	{object}	middleware.APIResponse
//	@Router			/api/monitor [delete]
func ClearAllModelErrors(c *gin.Context) {
	err := monitor.ClearAllModelErrors(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// ClearChannelAllModelErrors godoc
//
//	@Summary		Clear channel all model errors
//	@Description	Clears all model errors for a specific channel
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Channel ID"
//	@Success		204	{object}	middleware.APIResponse
//	@Router			/api/monitor/{id} [delete]
func ClearChannelAllModelErrors(c *gin.Context) {
	channelID := c.Param("id")
	channelIDInt, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, "Invalid channel ID")
		return
	}
	err = monitor.ClearChannelAllModelErrors(c.Request.Context(), int(channelIDInt))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// ClearChannelModelErrors godoc
//
//	@Summary		Clear channel model errors
//	@Description	Clears model errors for a specific channel and model
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int		true	"Channel ID"
//	@Param			model	path		string	true	"Model name"
//	@Success		204		{object}	middleware.APIResponse
//	@Router			/api/monitor/{id}/{model} [delete]
func ClearChannelModelErrors(c *gin.Context) {
	channelID := c.Param("id")
	model := c.Param("model")
	channelIDInt, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, "Invalid channel ID")
		return
	}
	err = monitor.ClearChannelModelErrors(c.Request.Context(), model, int(channelIDInt))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// GetModelsErrorRate godoc
//
//	@Summary		Get models error rate
//	@Description	Returns a list of models error rate
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[string]float64}
//	@Router			/api/monitor/models [get]
func GetModelsErrorRate(c *gin.Context) {
	rates, err := monitor.GetModelsErrorRate(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}
	middleware.SuccessResponse(c, rates)
}

// GetAllBannedModelChannels godoc
//
//	@Summary		Get all banned model channels
//	@Description	Returns a list of all banned model channels
//	@Tags			monitor
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[string][]int64}
//	@Router			/api/monitor/banned_channels [get]
func GetAllBannedModelChannels(c *gin.Context) {
	channels, err := monitor.GetAllBannedModelChannels(c.Request.Context())
	if err != nil {
		middleware.ErrorResponse(c, http.StatusOK, err.Error())
		return
	}
	middleware.SuccessResponse(c, channels)
}
