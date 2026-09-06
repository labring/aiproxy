package controller

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/monitor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
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

type ChannelResponse struct {
	*model.Channel
	AccessedAt time.Time `json:"accessed_at,omitempty"`
}

func (c *ChannelResponse) MarshalJSON() ([]byte, error) {
	type Alias model.Channel

	accessedAt := int64(0)
	if !c.AccessedAt.IsZero() {
		accessedAt = c.AccessedAt.UnixMilli()
	}

	return sonic.Marshal(&struct {
		*Alias
		CreatedAt        int64 `json:"created_at"`
		BalanceUpdatedAt int64 `json:"balance_updated_at"`
		LastTestErrorAt  int64 `json:"last_test_error_at"`
		AccessedAt       int64 `json:"accessed_at,omitempty"`
	}{
		Alias:            (*Alias)(c.Channel),
		CreatedAt:        c.CreatedAt.UnixMilli(),
		BalanceUpdatedAt: c.BalanceUpdatedAt.UnixMilli(),
		LastTestErrorAt:  c.LastTestErrorAt.UnixMilli(),
		AccessedAt:       accessedAt,
	})
}

func buildChannelResponse(channel *model.Channel) *ChannelResponse {
	lastRequestAt, _ := model.GetChannelLastRequestTimeMinute(channel.ID)

	return &ChannelResponse{
		Channel:    channel,
		AccessedAt: lastRequestAt,
	}
}

func buildChannelResponses(channels []*model.Channel) []*ChannelResponse {
	responses := make([]*ChannelResponse, len(channels))
	for i, channel := range channels {
		responses[i] = buildChannelResponse(channel)
	}

	return responses
}

func parseChannelFilter(c *gin.Context) (model.ChannelFilter, error) {
	id, _ := strconv.Atoi(c.Query("id"))
	channelType, _ := strconv.Atoi(c.Query("channel_type"))
	filter := model.ChannelFilter{
		ID: id, Name: c.Query("name"), Key: c.Query("key"),
		Type: channelType, BaseURL: c.Query("base_url"),
	}

	query := c.Request.URL.Query()
	if values, present := query["remark"]; present {
		if len(values) != 1 {
			return filter, errors.New("remark must be specified once")
		}

		filter.Remark = &values[0]
	}

	if values, present := query["backup_only"]; present {
		if len(values) != 1 {
			return filter, errors.New("backup_only must be specified once")
		}

		value, err := strconv.ParseBool(values[0])
		if err != nil {
			return filter, errors.New("backup_only must be a boolean")
		}

		filter.BackupOnly = &value
	}

	return filter, nil
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
//	@Param			remark			query		string	false	"Exact remark filter; empty matches channels without remarks"
//	@Param			backup_only		query		bool	false	"Filter backup-only channels; omit for all channels"
//	@Param			key				query		string	false	"Filter by key"
//	@Param			channel_type	query		int		false	"Filter by channel type"
//	@Param			base_url		query		string	false	"Filter by base URL"
//	@Param			order			query		string	false	"Order by field"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{channels=[]model.Channel,total=int}}
//	@Router			/api/channels/ [get]
func GetChannels(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)

	filter, err := parseChannelFilter(c)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	order := c.Query("order")

	channels, total, err := model.GetChannels(
		page,
		perPage,
		filter,
		order,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"channels": buildChannelResponses(channels),
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

	middleware.SuccessResponse(c, buildChannelResponses(channels))
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
		channel, err := channel.ToChannel()
		if err != nil {
			middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
			return
		}

		_channels = append(_channels, channel)
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
//	@Description	Search channel names, remarks, keys, URLs, models and sets with a keyword, combined with optional exact filters
//	@Tags			channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			keyword			query		string	false	"Search keyword, including remark content"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			id				query		int		false	"Filter by id"
//	@Param			name			query		string	false	"Filter by name"
//	@Param			remark			query		string	false	"Exact remark filter; empty matches channels without remarks"
//	@Param			backup_only		query		bool	false	"Filter backup-only channels; omit for all channels"
//	@Param			key				query		string	false	"Filter by key"
//	@Param			channel_type	query		int		false	"Filter by channel type"
//	@Param			base_url		query		string	false	"Filter by base URL"
//	@Param			order			query		string	false	"Order by field"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{channels=[]model.Channel,total=int}}
//	@Router			/api/channels/search [get]
func SearchChannels(c *gin.Context) {
	keyword := c.Query("keyword")
	page, perPage := utils.ParsePageParams(c)

	filter, err := parseChannelFilter(c)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	order := c.Query("order")

	channels, total, err := model.SearchChannels(
		keyword,
		page,
		perPage,
		filter,
		order,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"channels": buildChannelResponses(channels),
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

	middleware.SuccessResponse(c, buildChannelResponse(channel))
}

// AddChannelRequest represents the request body for adding a channel
type AddChannelRequest struct {
	ModelMapping            map[string]string    `json:"model_mapping"`
	Configs                 model.ChannelConfigs `json:"configs"`
	Name                    string               `json:"name"`
	Remark                  string               `json:"remark"`
	Key                     string               `json:"key"`
	BaseURL                 string               `json:"base_url"`
	ProxyURL                string               `json:"proxy_url"`
	Models                  []string             `json:"models"`
	Type                    model.ChannelType    `json:"type"`
	Priority                int32                `json:"priority"`
	BackupOnly              bool                 `json:"backup_only"`
	Status                  int                  `json:"status"`
	Sets                    []string             `json:"sets"`
	EnabledAutoBalanceCheck bool                 `json:"enabled_auto_balance_check"`
	SkipTLSVerify           bool                 `json:"skip_tls_verify"`
	EnabledNoPermissionBan  bool                 `json:"enabled_no_permission_ban"`
	WarnErrorRate           float64              `json:"warn_error_rate"`
	MaxErrorRate            float64              `json:"max_error_rate"`
}

// UpdateChannelRequest treats omitted fields and null as unchanged.
// Empty strings, false, zero and empty collections are explicit updates.
type UpdateChannelRequest model.ChannelPatch

func (r *UpdateChannelRequest) Apply(current *model.Channel) (*model.Channel, error) {
	next := *current
	if r.Type != nil {
		next.Type = *r.Type
	}

	if r.Name != nil {
		next.Name = *r.Name
	}

	if r.Remark != nil {
		next.Remark = *r.Remark
	}

	if r.Key != nil {
		next.Key = *r.Key
	}

	if r.BaseURL != nil {
		next.BaseURL = *r.BaseURL
	}

	if r.ProxyURL != nil {
		next.ProxyURL = *r.ProxyURL
	}

	if r.Models != nil {
		next.Models = slices.Clone(*r.Models)
	}

	if r.ModelMapping != nil {
		next.ModelMapping = maps.Clone(*r.ModelMapping)
	}

	if r.Configs != nil {
		next.Configs = maps.Clone(*r.Configs)
	}

	if r.Priority != nil {
		next.Priority = *r.Priority
	}

	if r.BackupOnly != nil {
		next.BackupOnly = *r.BackupOnly
	}

	if r.Sets != nil {
		next.Sets = slices.Clone(*r.Sets)
	}

	if r.EnabledAutoBalanceCheck != nil {
		next.EnabledAutoBalanceCheck = *r.EnabledAutoBalanceCheck
	}

	if r.SkipTLSVerify != nil {
		next.SkipTLSVerify = *r.SkipTLSVerify
	}

	if r.EnabledNoPermissionBan != nil {
		next.EnabledNoPermissionBan = *r.EnabledNoPermissionBan
	}

	if r.WarnErrorRate != nil {
		next.WarnErrorRate = *r.WarnErrorRate
	}

	if r.MaxErrorRate != nil {
		next.MaxErrorRate = *r.MaxErrorRate
	}

	if r.BalanceThreshold != nil {
		next.BalanceThreshold = *r.BalanceThreshold
	}

	if r.ProxyURL != nil {
		if err := relayutils.ValidateProxyURL(*r.ProxyURL); err != nil {
			return nil, err
		}
	}

	if _, err := (&AddChannelRequest{Type: next.Type, Name: next.Name, Key: next.Key}).ToChannel(); err != nil {
		return nil, err
	}

	return &next, nil
}

func (r *AddChannelRequest) ToChannel() (*model.Channel, error) {
	if err := relayutils.ValidateProxyURL(r.ProxyURL); err != nil {
		return nil, err
	}

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

	return &model.Channel{
		Type:                    r.Type,
		Name:                    r.Name,
		Remark:                  r.Remark,
		Key:                     r.Key,
		BaseURL:                 r.BaseURL,
		ProxyURL:                r.ProxyURL,
		Models:                  slices.Clone(r.Models),
		ModelMapping:            maps.Clone(r.ModelMapping),
		Priority:                r.Priority,
		BackupOnly:              r.BackupOnly,
		Status:                  r.Status,
		Configs:                 r.Configs,
		Sets:                    slices.Clone(r.Sets),
		EnabledAutoBalanceCheck: r.EnabledAutoBalanceCheck,
		SkipTLSVerify:           r.SkipTLSVerify,
		EnabledNoPermissionBan:  r.EnabledNoPermissionBan,
		WarnErrorRate:           r.WarnErrorRate,
		MaxErrorRate:            r.MaxErrorRate,
	}, nil
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

	ch, err := channel.ToChannel()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	err = model.BatchInsertChannels([]*model.Channel{ch})
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

// GetChannelBatchInfo godoc
//
//	@Summary		Get basic info for multiple channels
//	@Description	Returns id, name, remark, type, current status, and backup-only status for a batch of channel IDs, including soft-deleted channels
//	@Tags			channels
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			ids	body		[]int	true	"Channel IDs"
//	@Success		200	{object}	middleware.APIResponse{data=[]model.ChannelBasicInfo}
//	@Router			/api/channels/batch_info [post]
func GetChannelBatchInfo(c *gin.Context) {
	ids := []int{}

	err := c.ShouldBindJSON(&ids)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	channels, err := model.GetChannelsBasicInfoByIDs(ids)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, channels)
}

// UpdateChannel godoc
//
//	@Summary		Update a channel
//	@Description	Updates only supplied fields. Omitted fields and null retain current values; empty strings, false, zero, empty arrays and empty objects explicitly replace values, subject to channel validation.
//	@Tags			channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int					true	"Channel ID"
//	@Param			channel	body		UpdateChannelRequest	true	"Optional channel fields to update"
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

	channel := UpdateChannelRequest{}

	err = c.ShouldBindJSON(&channel)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	current, err := model.GetChannelByID(id)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	ch, err := channel.Apply(current)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	err = model.UpdateChannel(ch, (*model.ChannelPatch)(&channel))
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
