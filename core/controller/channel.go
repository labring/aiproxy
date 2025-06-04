package controller

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	log "github.com/sirupsen/logrus"
)

// ChannelTypeMetas godoc
//
//	@Summary		Get channel type metadata
//	@Description	Returns metadata for all channel types
//	@Tags			channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=map[int]adaptors.AdaptorMeta}
//	@Router			/api/channels/type_metas [get]
func ChannelTypeMetas(c *gin.Context) {
	middleware.SuccessResponse(c, adaptors.ChannelMetas)
}

// GetChannels godoc
//
//	@Summary		Get channels with pagination
//	@Description	Returns a paginated list of channels with optional filters
//	@Tags			channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			id				query		int		false	"Filter by id"
//	@Param			name			query		string	false	"Filter by name"
//	@Param			key				query		string	false	"Filter by key"
//	@Param			channel_type	query		int		false	"Filter by channel type"
//	@Param			base_url		query		string	false	"Filter by base URL"
//	@Param			order			query		string	false	"Order by field"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{channels=[]model.Channel,total=int}}
//	@Router			/api/channels/ [get]
func GetChannels(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	id, _ := strconv.Atoi(c.Query("id"))
	name := c.Query("name")
	key := c.Query("key")
	channelType, _ := strconv.Atoi(c.Query("channel_type"))
	baseURL := c.Query("base_url")
	order := c.Query("order")
	channels, total, err := model.GetChannels(
		page,
		perPage,
		id,
		name,
		key,
		channelType,
		baseURL,
		order,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, gin.H{
		"channels": channels,
		"total":    total,
	})
}

// GetAllChannels godoc
//
//	@Summary		Get all channels
//	@Description	Returns a list of all channels without pagination
//	@Tags			channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	middleware.APIResponse{data=[]model.Channel}
//	@Router			/api/channels/all [get]
func GetAllChannels(c *gin.Context) {
	channels, err := model.GetAllChannels()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, channels)
}

// AddChannels godoc
//
//	@Summary		Add multiple channels
//	@Description	Adds multiple channels in a batch operation
//	@Tags			channels
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			channels	body		[]AddChannelRequest	true	"Channel information"
//	@Success		200			{object}	middleware.APIResponse
//	@Router			/api/channels/ [post]
func AddChannels(c *gin.Context) {
	channels := make([]*AddChannelRequest, 0)
	err := c.ShouldBindJSON(&channels)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	_channels := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		channels, err := channel.ToChannels()
		if err != nil {
			middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
			return
		}
		_channels = append(_channels, channels...)
	}
	err = model.BatchInsertChannels(_channels)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// SearchChannels godoc
//
//	@Summary		Search channels
//	@Description	Search channels with keyword and optional filters
//	@Tags			channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			keyword			query		string	true	"Search keyword"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			id				query		int		false	"Filter by id"
//	@Param			name			query		string	false	"Filter by name"
//	@Param			key				query		string	false	"Filter by key"
//	@Param			channel_type	query		int		false	"Filter by channel type"
//	@Param			base_url		query		string	false	"Filter by base URL"
//	@Param			order			query		string	false	"Order by field"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{channels=[]model.Channel,total=int}}
//	@Router			/api/channels/search [get]
func SearchChannels(c *gin.Context) {
	keyword := c.Query("keyword")
	page, perPage := utils.ParsePageParams(c)
	id, _ := strconv.Atoi(c.Query("id"))
	name := c.Query("name")
	key := c.Query("key")
	channelType, _ := strconv.Atoi(c.Query("channel_type"))
	baseURL := c.Query("base_url")
	order := c.Query("order")
	channels, total, err := model.SearchChannels(
		keyword,
		page,
		perPage,
		id,
		name,
		key,
		channelType,
		baseURL,
		order,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, gin.H{
		"channels": channels,
		"total":    total,
	})
}

// GetChannel godoc
//
//	@Summary		Get a channel by ID
//	@Description	Returns detailed information about a specific channel
//	@Tags			channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Channel ID"
//	@Success		200	{object}	middleware.APIResponse{data=model.Channel}
//	@Router			/api/channel/{id} [get]
func GetChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	channel, err := model.GetChannelByID(id)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, channel)
}

// AddChannelRequest represents the request body for adding a channel
type AddChannelRequest struct {
	ModelMapping map[string]string    `json:"model_mapping"`
	Config       *model.ChannelConfig `json:"config"`
	Name         string               `json:"name"`
	Key          string               `json:"key"`
	BaseURL      string               `json:"base_url"`
	Models       []string             `json:"models"`
	Type         model.ChannelType    `json:"type"`
	Priority     int32                `json:"priority"`
	Status       int                  `json:"status"`
	Sets         []string             `json:"sets"`
}

func (r *AddChannelRequest) ToChannel() (*model.Channel, error) {
	a, ok := adaptors.GetAdaptor(r.Type)
	if !ok {
		return nil, fmt.Errorf("invalid channel type: %d", r.Type)
	}
	metadata := a.Metadata()
	if validator := adaptors.GetKeyValidator(a); validator != nil {
		err := validator.ValidateKey(r.Key)
		if err != nil {
			keyHelp := metadata.KeyHelp
			if keyHelp == "" {
				return nil, fmt.Errorf(
					"%s [%s(%d)] invalid key: %w",
					r.Name,
					r.Type.String(),
					r.Type,
					err,
				)
			}
			return nil, fmt.Errorf(
				"%s [%s(%d)] invalid key: %w, %s",
				r.Name,
				r.Type.String(),
				r.Type,
				err,
				keyHelp,
			)
		}
	}
	if r.Config != nil {
		for key, template := range metadata.Config {
			v, err := r.Config.Get(key)
			if err != nil {
				if errors.Is(err, ast.ErrNotExist) {
					if template.Required {
						return nil, fmt.Errorf("config %s is required: %w", key, err)
					}
					continue
				}
				return nil, fmt.Errorf("config %s is invalid: %w", key, err)
			}
			if !v.Exists() {
				if template.Required {
					return nil, fmt.Errorf("config %s is required: %w", key, err)
				}
				continue
			}
			if template.Validator != nil {
				i, err := v.Interface()
				if err != nil {
					return nil, fmt.Errorf("config %s is invalid: %w", key, err)
				}
				err = adaptor.ValidateConfigTemplateValue(template, i)
				if err != nil {
					return nil, fmt.Errorf("config %s is invalid: %w", key, err)
				}
			}
		}
	}

	return &model.Channel{
		Type:         r.Type,
		Name:         r.Name,
		Key:          r.Key,
		BaseURL:      r.BaseURL,
		Models:       slices.Clone(r.Models),
		ModelMapping: maps.Clone(r.ModelMapping),
		Priority:     r.Priority,
		Status:       r.Status,
		Config:       r.Config,
		Sets:         slices.Clone(r.Sets),
	}, nil
}

func (r *AddChannelRequest) ToChannels() ([]*model.Channel, error) {
	keys := strings.Split(r.Key, "\n")
	channels := make([]*model.Channel, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		c, err := r.ToChannel()
		if err != nil {
			return nil, err
		}
		c.Key = key
		channels = append(channels, c)
	}
	if len(channels) == 0 {
		ch, err := r.ToChannel()
		if err != nil {
			return nil, err
		}
		return []*model.Channel{ch}, nil
	}
	return channels, nil
}

// AddChannel godoc
//
//	@Summary		Add a single channel
//	@Description	Adds a new channel to the system
//	@Tags			channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			channel	body		AddChannelRequest	true	"Channel information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/channel/ [post]
func AddChannel(c *gin.Context) {
	channel := AddChannelRequest{}
	err := c.ShouldBindJSON(&channel)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	channels, err := channel.ToChannels()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	err = model.BatchInsertChannels(channels)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// DeleteChannel godoc
//
//	@Summary		Delete a channel
//	@Description	Deletes a channel by its ID
//	@Tags			channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Channel ID"
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/channel/{id} [delete]
func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	err := model.DeleteChannelByID(id)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// DeleteChannels godoc
//
//	@Summary		Delete multiple channels
//	@Description	Deletes multiple channels by their IDs
//	@Tags			channels
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			ids	body		[]int	true	"Channel IDs"
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/channels/batch_delete [post]
func DeleteChannels(c *gin.Context) {
	ids := []int{}
	err := c.ShouldBindJSON(&ids)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	err = model.DeleteChannelsByIDs(ids)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// UpdateChannel godoc
//
//	@Summary		Update a channel
//	@Description	Updates an existing channel by its ID
//	@Tags			channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int					true	"Channel ID"
//	@Param			channel	body		AddChannelRequest	true	"Updated channel information"
//	@Success		200		{object}	middleware.APIResponse{data=model.Channel}
//	@Router			/api/channel/{id} [put]
func UpdateChannel(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "id is required")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	channel := AddChannelRequest{}
	err = c.ShouldBindJSON(&channel)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ch, err := channel.ToChannel()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ch.ID = id
	err = model.UpdateChannel(ch)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	err = monitor.ClearChannelAllModelErrors(c.Request.Context(), id)
	if err != nil {
		log.Errorf("failed to clear channel all model errors: %+v", err)
	}
	middleware.SuccessResponse(c, ch)
}

// UpdateChannelStatusRequest represents the request body for updating a channel's status
type UpdateChannelStatusRequest struct {
	Status int `json:"status"`
}

// UpdateChannelStatus godoc
//
//	@Summary		Update channel status
//	@Description	Updates the status of a channel by its ID
//	@Tags			channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int							true	"Channel ID"
//	@Param			status	body		UpdateChannelStatusRequest	true	"Status information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/channel/{id}/status [post]
func UpdateChannelStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	status := UpdateChannelStatusRequest{}
	err := c.ShouldBindJSON(&status)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	err = model.UpdateChannelStatusByID(id, status.Status)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	err = monitor.ClearChannelAllModelErrors(c.Request.Context(), id)
	if err != nil {
		log.Errorf("failed to clear channel all model errors: %+v", err)
	}
	middleware.SuccessResponse(c, nil)
}
