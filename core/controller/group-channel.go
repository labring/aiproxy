package controller

import (
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/render"
	log "github.com/sirupsen/logrus"
)

type GroupChannelResponse struct {
	*model.GroupChannel
	AccessedAt time.Time `json:"accessed_at,omitempty"`
}

func (c *GroupChannelResponse) MarshalJSON() ([]byte, error) {
	type Alias model.GroupChannel

	accessedAt := int64(0)
	if !c.AccessedAt.IsZero() {
		accessedAt = c.AccessedAt.UnixMilli()
	}

	return sonic.Marshal(&struct {
		*Alias
		CreatedAt       int64 `json:"created_at"`
		LastTestErrorAt int64 `json:"last_test_error_at"`
		AccessedAt      int64 `json:"accessed_at,omitempty"`
	}{
		Alias:           (*Alias)(c.GroupChannel),
		CreatedAt:       c.CreatedAt.UnixMilli(),
		LastTestErrorAt: c.LastTestErrorAt.UnixMilli(),
		AccessedAt:      accessedAt,
	})
}

func buildGroupChannelResponse(channel *model.GroupChannel) *GroupChannelResponse {
	return &GroupChannelResponse{GroupChannel: channel}
}

func buildGroupChannelResponses(channels []*model.GroupChannel) []*GroupChannelResponse {
	lastRequestTimes := getGroupChannelLastRequestTimes(channels)

	responses := make([]*GroupChannelResponse, len(channels))
	for i, channel := range channels {
		responses[i] = buildGroupChannelResponse(channel)
		responses[i].AccessedAt = lastRequestTimes[channel.ID]
	}

	return responses
}

func getGroupChannelLastRequestTimes(channels []*model.GroupChannel) map[int]time.Time {
	if len(channels) == 0 {
		return nil
	}

	groupID := channels[0].GroupID

	channelIDs := make([]int, 0, len(channels))
	for _, channel := range channels {
		if channel.GroupID != groupID {
			return getGroupChannelLastRequestTimesByChannel(channels)
		}

		channelIDs = append(channelIDs, channel.ID)
	}

	lastRequestTimes, err := model.GetGroupChannelLastRequestTimesMinute(groupID, channelIDs)
	if err != nil {
		return nil
	}

	return lastRequestTimes
}

func getGroupChannelLastRequestTimesByChannel(channels []*model.GroupChannel) map[int]time.Time {
	lastRequestTimes := make(map[int]time.Time, len(channels))
	for _, channel := range channels {
		lastRequestAt, err := model.GetGroupChannelLastRequestTimeMinute(
			channel.GroupID,
			channel.ID,
		)
		if err == nil {
			lastRequestTimes[channel.ID] = lastRequestAt
		}
	}

	return lastRequestTimes
}

func groupParam(c *gin.Context) string {
	return c.Param("group")
}

type GroupChannelEnabledModelChannel struct {
	ID       int               `json:"id"`
	GroupID  string            `json:"group_id"`
	Type     model.ChannelType `json:"type"`
	Name     string            `json:"name"`
	Priority int32             `json:"priority"`
	Weight   float64           `json:"weight"`
}

func newGroupChannelEnabledModelChannel(ch *model.GroupChannel) GroupChannelEnabledModelChannel {
	return GroupChannelEnabledModelChannel{
		ID:       ch.ID,
		GroupID:  ch.GroupID,
		Type:     ch.Type,
		Name:     ch.Name,
		Priority: ch.GetPriority(),
	}
}

func calculateGroupChannelWeights(channels []GroupChannelEnabledModelChannel) {
	if len(channels) == 0 {
		return
	}

	totalWeight := 0.0
	for _, ch := range channels {
		if ch.Priority > 0 {
			totalWeight += float64(ch.Priority)
		}
	}

	if totalWeight <= 0 {
		return
	}

	for i := range channels {
		if channels[i].Priority > 0 {
			channels[i].Weight = float64(channels[i].Priority) / totalWeight * 100
		}
	}
}

func loadGroupChannelModelsBySet(
	group string,
) (
	map[string][]model.ModelConfig,
	map[string]map[string][]GroupChannelEnabledModelChannel,
	error,
) {
	if group == "" {
		return nil, nil, errors.New("group id is required")
	}

	groupCache, err := model.CacheGetGroup(group)
	if err != nil {
		return nil, nil, err
	}

	channelsCache, err := model.CacheGetGroupChannels(group)
	if err != nil {
		return nil, nil, err
	}

	scopeConfigs, err := model.CacheGetGroupScopeModelConfigs(group)
	if err != nil {
		return nil, nil, err
	}

	configsBySet := make(map[string][]model.ModelConfig)
	channelsByModelSet := make(map[string]map[string][]GroupChannelEnabledModelChannel)
	appendedConfigs := make(map[string]map[string]struct{})

	for _, channel := range channelsCache.Channels {
		if channel.Status != model.ChannelStatusEnabled {
			continue
		}

		for _, modelName := range model.GroupChannelAccessModels(channel) {
			mc, ok := scopeConfigs.Configs[modelName]
			if !ok {
				continue
			}

			mc = middleware.GetGroupScopeAdjustedModelConfig(*groupCache, mc)
			for _, set := range channel.GetSets() {
				if _, ok := appendedConfigs[set]; !ok {
					appendedConfigs[set] = make(map[string]struct{})
				}

				if _, ok := appendedConfigs[set][mc.Model]; !ok {
					configsBySet[set] = append(configsBySet[set], mc)
					appendedConfigs[set][mc.Model] = struct{}{}
				}

				if _, ok := channelsByModelSet[mc.Model]; !ok {
					channelsByModelSet[mc.Model] = make(
						map[string][]GroupChannelEnabledModelChannel,
					)
				}

				channelsByModelSet[mc.Model][set] = append(
					channelsByModelSet[mc.Model][set],
					newGroupChannelEnabledModelChannel(channel),
				)
			}
		}
	}

	for set := range configsBySet {
		slices.SortStableFunc(configsBySet[set], model.SortModelConfigsFunc)
	}

	for modelName, sets := range channelsByModelSet {
		for set, channels := range sets {
			calculateGroupChannelWeights(channels)
			channelsByModelSet[modelName][set] = channels
		}
	}

	return configsBySet, channelsByModelSet, nil
}

// GetGroupChannelEnabledModels godoc
//
//	@Summary		Get enabled group channel models
//	@Description	Returns group channel model configs grouped by set for a group
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Success		200		{object}	middleware.APIResponse{data=map[string][]model.ModelConfig}
//	@Router			/api/group/{group}/channel-models/enabled [get]
func GetGroupChannelEnabledModels(c *gin.Context) {
	configsBySet, _, err := loadGroupChannelModelsBySet(groupParam(c))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, configsBySet)
}

// GetGroupChannelEnabledModelsSet godoc
//
//	@Summary		Get enabled group channel models by set
//	@Description	Returns group channel model configs for a specific set
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Param			set		path		string	true	"Models set"
//	@Success		200		{object}	middleware.APIResponse{data=[]model.ModelConfig}
//	@Router			/api/group/{group}/channel-models/enabled/{set} [get]
func GetGroupChannelEnabledModelsSet(c *gin.Context) {
	set := c.Param("set")
	if set == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "set is required")
		return
	}

	configsBySet, _, err := loadGroupChannelModelsBySet(groupParam(c))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, configsBySet[set])
}

// GetGlobalGroupChannels godoc
//
//	@Summary		Get group channels with pagination
//	@Description	Returns a paginated list of group channels across groups with optional filters
//	@Tags			group_channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			query		string	false	"Filter by group"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			id				query		int		false	"Filter by id"
//	@Param			name			query		string	false	"Filter by name"
//	@Param			key				query		string	false	"Filter by key"
//	@Param			channel_type	query		int		false	"Filter by channel type"
//	@Param			base_url		query		string	false	"Filter by base URL"
//	@Param			order			query		string	false	"Order by field"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{channels=[]model.GroupChannel,total=int}}
//	@Router			/api/group_channels/ [get]
func GetGlobalGroupChannels(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	id, _ := strconv.Atoi(c.Query("id"))
	channelType, _ := strconv.Atoi(c.Query("channel_type"))

	channels, total, err := model.GetGlobalGroupChannels(
		c.Query("group"),
		page,
		perPage,
		id,
		c.Query("name"),
		c.Query("key"),
		channelType,
		c.Query("base_url"),
		c.Query("order"),
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"channels": buildGroupChannelResponses(channels),
		"total":    total,
	})
}

// GetGroupChannels godoc
//
//	@Summary		Get group channels with pagination
//	@Description	Returns a paginated list of group channels for a specific group
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group ID"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			id				query		int		false	"Filter by id"
//	@Param			name			query		string	false	"Filter by name"
//	@Param			key				query		string	false	"Filter by key"
//	@Param			channel_type	query		int		false	"Filter by channel type"
//	@Param			base_url		query		string	false	"Filter by base URL"
//	@Param			order			query		string	false	"Order by field"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{channels=[]model.GroupChannel,total=int}}
//	@Router			/api/group/{group}/channels/ [get]
func GetGroupChannels(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	id, _ := strconv.Atoi(c.Query("id"))
	channelType, _ := strconv.Atoi(c.Query("channel_type"))

	channels, total, err := model.GetGroupChannels(
		groupParam(c),
		page,
		perPage,
		id,
		c.Query("name"),
		c.Query("key"),
		channelType,
		c.Query("base_url"),
		c.Query("order"),
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"channels": buildGroupChannelResponses(channels),
		"total":    total,
	})
}

// SearchGlobalGroupChannels godoc
//
//	@Summary		Search group channels
//	@Description	Search group channels across groups with keyword and optional filters
//	@Tags			group_channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			query		string	false	"Filter by group"
//	@Param			keyword			query		string	false	"Search keyword"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			id				query		int		false	"Filter by id"
//	@Param			name			query		string	false	"Filter by name"
//	@Param			key				query		string	false	"Filter by key"
//	@Param			channel_type	query		int		false	"Filter by channel type"
//	@Param			base_url		query		string	false	"Filter by base URL"
//	@Param			order			query		string	false	"Order by field"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{channels=[]model.GroupChannel,total=int}}
//	@Router			/api/group_channels/search [get]
func SearchGlobalGroupChannels(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	id, _ := strconv.Atoi(c.Query("id"))
	channelType, _ := strconv.Atoi(c.Query("channel_type"))

	channels, total, err := model.SearchGlobalGroupChannels(
		c.Query("group"),
		c.Query("keyword"),
		page,
		perPage,
		id,
		c.Query("name"),
		c.Query("key"),
		channelType,
		c.Query("base_url"),
		c.Query("order"),
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"channels": buildGroupChannelResponses(channels),
		"total":    total,
	})
}

// SearchGroupChannels godoc
//
//	@Summary		Search group channels
//	@Description	Search group channels for a specific group with keyword and optional filters
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group ID"
//	@Param			keyword			query		string	false	"Search keyword"
//	@Param			page			query		int		false	"Page number"
//	@Param			per_page		query		int		false	"Items per page"
//	@Param			id				query		int		false	"Filter by id"
//	@Param			name			query		string	false	"Filter by name"
//	@Param			key				query		string	false	"Filter by key"
//	@Param			channel_type	query		int		false	"Filter by channel type"
//	@Param			base_url		query		string	false	"Filter by base URL"
//	@Param			order			query		string	false	"Order by field"
//	@Success		200				{object}	middleware.APIResponse{data=map[string]any{channels=[]model.GroupChannel,total=int}}
//	@Router			/api/group/{group}/channels/search [get]
func SearchGroupChannels(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	id, _ := strconv.Atoi(c.Query("id"))
	channelType, _ := strconv.Atoi(c.Query("channel_type"))

	channels, total, err := model.SearchGroupChannels(
		groupParam(c),
		c.Query("keyword"),
		page,
		perPage,
		id,
		c.Query("name"),
		c.Query("key"),
		channelType,
		c.Query("base_url"),
		c.Query("order"),
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"channels": buildGroupChannelResponses(channels),
		"total":    total,
	})
}

func getGroupChannelBatchInfoRequest(c *gin.Context) ([]int, bool) {
	ids := []int{}
	if err := c.ShouldBindJSON(&ids); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return nil, false
	}

	return ids, true
}

// GetGlobalGroupChannelBatchInfo godoc
//
//	@Summary		Get basic info for multiple group channels
//	@Description	Returns id, group, name, and type for a batch of group channel IDs across groups
//	@Tags			group_channels
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			ids	body		[]int	true	"Group channel IDs"
//	@Success		200	{object}	middleware.APIResponse{data=[]model.GroupChannelBasicInfo}
//	@Router			/api/group_channels/batch_info [post]
func GetGlobalGroupChannelBatchInfo(c *gin.Context) {
	ids, ok := getGroupChannelBatchInfoRequest(c)
	if !ok {
		return
	}

	channels, err := model.GetGlobalGroupChannelsBasicInfoByIDs(ids)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, channels)
}

// GetGroupChannelBatchInfo godoc
//
//	@Summary		Get basic info for multiple group channels in a group
//	@Description	Returns id, group, name, and type for a batch of group channel IDs filtered by group
//	@Tags			group-channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Param			ids		body		[]int	true	"Group channel IDs"
//	@Success		200		{object}	middleware.APIResponse{data=[]model.GroupChannelBasicInfo}
//	@Router			/api/group/{group}/channels/batch_info [post]
func GetGroupChannelBatchInfo(c *gin.Context) {
	ids, ok := getGroupChannelBatchInfoRequest(c)
	if !ok {
		return
	}

	channels, err := model.GetGroupChannelsBasicInfoByIDs(groupParam(c), ids)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, channels)
}

type AddGroupChannelRequest struct {
	ModelMapping           map[string]string    `json:"model_mapping"`
	Configs                model.ChannelConfigs `json:"configs"`
	GroupID                string               `json:"group_id"`
	Name                   string               `json:"name"`
	Key                    string               `json:"key"`
	BaseURL                string               `json:"base_url"`
	ProxyURL               string               `json:"proxy_url"`
	Models                 []string             `json:"models"`
	Sets                   []string             `json:"sets"`
	Type                   model.ChannelType    `json:"type"`
	Priority               int32                `json:"priority"`
	Status                 int                  `json:"status"`
	SkipTLSVerify          bool                 `json:"skip_tls_verify"`
	EnabledNoPermissionBan bool                 `json:"enabled_no_permission_ban"`
	MaxErrorRate           float64              `json:"max_error_rate"`
}

func (r *AddGroupChannelRequest) toGroupChannel(group string) (*model.GroupChannel, error) {
	a, ok := adaptors.GetAdaptor(r.Type)
	if !ok {
		return nil, fmt.Errorf("invalid channel type: %d", r.Type)
	}

	if validator := adaptors.GetKeyValidator(a); validator != nil {
		metadata := a.Metadata()
		if err := validator.ValidateKey(r.Key); err != nil {
			if metadata.KeyHelp == "" {
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
				metadata.KeyHelp,
			)
		}
	}

	return &model.GroupChannel{
		GroupID:                group,
		Type:                   r.Type,
		Name:                   r.Name,
		Key:                    r.Key,
		BaseURL:                r.BaseURL,
		ProxyURL:               r.ProxyURL,
		Models:                 slices.Clone(r.Models),
		ModelMapping:           maps.Clone(r.ModelMapping),
		Priority:               r.Priority,
		Status:                 r.Status,
		Configs:                r.Configs,
		Sets:                   slices.Clone(r.Sets),
		SkipTLSVerify:          r.SkipTLSVerify,
		EnabledNoPermissionBan: r.EnabledNoPermissionBan,
		MaxErrorRate:           r.MaxErrorRate,
	}, nil
}

// AddGroupChannel godoc
//
//	@Summary		Add a group channel
//	@Description	Adds a group channel to a specific group
//	@Tags			group-channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string					true	"Group ID"
//	@Param			channel	body		AddGroupChannelRequest	true	"Group channel information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/channel/ [post]
func AddGroupChannel(c *gin.Context) {
	req := AddGroupChannelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	channel, err := req.toGroupChannel(groupParam(c))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.BatchInsertGroupChannels([]*model.GroupChannel{channel}); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// AddGlobalGroupChannel godoc
//
//	@Summary		Add a group channel
//	@Description	Adds a group channel from the global management view. The request body must include group_id.
//	@Tags			group_channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			channel	body		AddGroupChannelRequest	true	"Group channel information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group_channel/ [post]
func AddGlobalGroupChannel(c *gin.Context) {
	req := AddGroupChannelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	channel, err := req.toGroupChannel(req.GroupID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if channel.GroupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group id is required")
		return
	}

	if err := model.BatchInsertGroupChannels([]*model.GroupChannel{channel}); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// AddGroupChannels godoc
//
//	@Summary		Add multiple group channels
//	@Description	Adds group channels to a specific group
//	@Tags			group-channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group		path		string						true	"Group ID"
//	@Param			channels	body		[]AddGroupChannelRequest	true	"Group channel information"
//	@Success		200			{object}	middleware.APIResponse
//	@Router			/api/group/{group}/channels/ [post]
func AddGroupChannels(c *gin.Context) {
	reqs := make([]*AddGroupChannelRequest, 0)
	if err := c.ShouldBindJSON(&reqs); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	channels := make([]*model.GroupChannel, 0, len(reqs))
	for _, req := range reqs {
		channel, err := req.toGroupChannel(groupParam(c))
		if err != nil {
			middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
			return
		}

		channels = append(channels, channel)
	}

	if err := model.BatchInsertGroupChannels(channels); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// AddGlobalGroupChannels godoc
//
//	@Summary		Add multiple group channels
//	@Description	Adds group channels from the global management view. Each request item must include group_id.
//	@Tags			group_channels
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			channels	body		[]AddGroupChannelRequest	true	"Group channel information"
//	@Success		200			{object}	middleware.APIResponse
//	@Router			/api/group_channels/ [post]
func AddGlobalGroupChannels(c *gin.Context) {
	reqs := make([]*AddGroupChannelRequest, 0)
	if err := c.ShouldBindJSON(&reqs); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	channels := make([]*model.GroupChannel, 0, len(reqs))
	for _, req := range reqs {
		channel, err := req.toGroupChannel(req.GroupID)
		if err != nil {
			middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
			return
		}

		if channel.GroupID == "" {
			middleware.ErrorResponse(c, http.StatusBadRequest, "group id is required")
			return
		}

		channels = append(channels, channel)
	}

	if err := model.BatchInsertGroupChannels(channels); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// GetGroupChannel godoc
//
//	@Summary		Get a group channel by ID
//	@Description	Returns detailed information about a group channel in a specific group
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Param			id		path		int		true	"Group channel ID"
//	@Success		200		{object}	middleware.APIResponse{data=model.GroupChannel}
//	@Router			/api/group/{group}/channel/{id} [get]
func GetGroupChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	channel, err := model.GetGroupChannelByID(groupParam(c), id)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, buildGroupChannelResponse(channel))
}

// GetGlobalGroupChannel godoc
//
//	@Summary		Get a group channel by ID
//	@Description	Returns detailed information about a group channel across groups
//	@Tags			group_channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Group channel ID"
//	@Success		200	{object}	middleware.APIResponse{data=model.GroupChannel}
//	@Router			/api/group_channel/{id} [get]
func GetGlobalGroupChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	channel, err := model.GetGlobalGroupChannelByID(id)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, buildGroupChannelResponse(channel))
}

// UpdateGroupChannel godoc
//
//	@Summary		Update a group channel
//	@Description	Updates a group channel by ID in a specific group
//	@Tags			group-channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string					true	"Group ID"
//	@Param			id		path		int						true	"Group channel ID"
//	@Param			channel	body		AddGroupChannelRequest	true	"Updated group channel information"
//	@Success		200		{object}	middleware.APIResponse{data=model.GroupChannel}
//	@Router			/api/group/{group}/channel/{id} [put]
func UpdateGroupChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	req := AddGroupChannelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	ch, err := req.toGroupChannel(groupParam(c))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	ch.ID = id
	if err := model.UpdateGroupChannel(ch); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, ch)
}

// UpdateGlobalGroupChannel godoc
//
//	@Summary		Update a group channel
//	@Description	Updates a group channel by ID from the global management view
//	@Tags			group_channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int						true	"Group channel ID"
//	@Param			channel	body		AddGroupChannelRequest	true	"Updated group channel information"
//	@Success		200		{object}	middleware.APIResponse{data=model.GroupChannel}
//	@Router			/api/group_channel/{id} [put]
func UpdateGlobalGroupChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	req := AddGroupChannelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	ch, err := req.toGroupChannel(req.GroupID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	ch.ID = id
	if err := model.UpdateGlobalGroupChannel(ch); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, ch)
}

// DeleteGroupChannel godoc
//
//	@Summary		Delete a group channel
//	@Description	Deletes a group channel by ID in a specific group
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Param			id		path		int		true	"Group channel ID"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/channel/{id} [delete]
func DeleteGroupChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.DeleteGroupChannelByID(groupParam(c), id); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteGlobalGroupChannel godoc
//
//	@Summary		Delete a group channel
//	@Description	Deletes a group channel by ID from the global management view
//	@Tags			group_channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id	path		int	true	"Group channel ID"
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/group_channel/{id} [delete]
func DeleteGlobalGroupChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.DeleteGlobalGroupChannelByID(id); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteGroupChannels godoc
//
//	@Summary		Delete multiple group channels
//	@Description	Deletes group channels by IDs in a specific group
//	@Tags			group-channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Param			ids		body		[]int	true	"Group channel IDs"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/channels/batch_delete [post]
func DeleteGroupChannels(c *gin.Context) {
	ids := []int{}
	if err := c.ShouldBindJSON(&ids); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.DeleteGroupChannelsByIDs(groupParam(c), ids); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteGlobalGroupChannels godoc
//
//	@Summary		Delete multiple group channels
//	@Description	Deletes group channels by IDs from the global management view
//	@Tags			group_channels
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			ids	body		[]int	true	"Group channel IDs"
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/group_channels/batch_delete [post]
func DeleteGlobalGroupChannels(c *gin.Context) {
	ids := []int{}
	if err := c.ShouldBindJSON(&ids); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.DeleteGlobalGroupChannelsByIDs(ids); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// UpdateGroupChannelStatus godoc
//
//	@Summary		Update group channel status
//	@Description	Updates the status of a group channel by ID in a specific group
//	@Tags			group-channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string						true	"Group ID"
//	@Param			id		path		int							true	"Group channel ID"
//	@Param			status	body		UpdateChannelStatusRequest	true	"Status information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/channel/{id}/status [post]
func UpdateGroupChannelStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	status := UpdateChannelStatusRequest{}
	if err := c.ShouldBindJSON(&status); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.UpdateGroupChannelStatusByID(groupParam(c), id, status.Status); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// UpdateGlobalGroupChannelStatus godoc
//
//	@Summary		Update group channel status
//	@Description	Updates the status of a group channel by ID from the global management view
//	@Tags			group_channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		int							true	"Group channel ID"
//	@Param			status	body		UpdateChannelStatusRequest	true	"Status information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group_channel/{id}/status [post]
func UpdateGlobalGroupChannelStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	status := UpdateChannelStatusRequest{}
	if err := c.ShouldBindJSON(&status); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.UpdateGlobalGroupChannelStatusByID(id, status.Status); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

func groupChannelTestSaveFunc(
	groupChannel *model.GroupChannel,
) func(*meta.Meta, bool, string, int) (*model.ChannelTest, error) {
	return func(testMeta *meta.Meta, success bool, response string, code int) (*model.ChannelTest, error) {
		groupTest, err := groupChannel.UpdateModelTest(
			testMeta.RequestAt,
			testMeta.OriginModel,
			testMeta.ActualModel,
			testMeta.Mode,
			time.Since(testMeta.RequestAt).Seconds(),
			success,
			response,
			code,
		)
		if err != nil {
			return nil, err
		}

		return groupChannelTestToChannelTest(groupTest), nil
	}
}

func groupChannelTestToChannelTest(groupTest *model.GroupChannelTest) *model.ChannelTest {
	if groupTest == nil {
		return nil
	}

	return &model.ChannelTest{
		TestAt:      groupTest.TestAt,
		Model:       groupTest.Model,
		ActualModel: groupTest.ActualModel,
		Response:    groupTest.Response,
		ChannelName: groupTest.ChannelName,
		ChannelType: groupTest.ChannelType,
		ChannelID:   groupTest.GroupChannelID,
		Took:        groupTest.Took,
		Success:     groupTest.Success,
		Mode:        groupTest.Mode,
		Code:        groupTest.Code,
	}
}

func channelTestToGroupChannelTest(
	group string,
	channel *model.GroupChannel,
	test *model.ChannelTest,
) *model.GroupChannelTest {
	if test == nil {
		return nil
	}

	return &model.GroupChannelTest{
		TestAt:         test.TestAt,
		Model:          test.Model,
		ActualModel:    test.ActualModel,
		Response:       test.Response,
		GroupID:        group,
		ChannelName:    test.ChannelName,
		ChannelType:    test.ChannelType,
		GroupChannelID: channel.ID,
		Took:           test.Took,
		Success:        test.Success,
		Mode:           test.Mode,
		Code:           test.Code,
	}
}

type TestGroupChannelRequest AddGroupChannelRequest

func (r *TestGroupChannelRequest) toGroupChannel(group string) *model.GroupChannel {
	req := (*AddGroupChannelRequest)(r)

	return &model.GroupChannel{
		GroupID:                group,
		Type:                   req.Type,
		Name:                   req.Name,
		Key:                    req.Key,
		BaseURL:                req.BaseURL,
		ProxyURL:               req.ProxyURL,
		Models:                 slices.Clone(req.Models),
		ModelMapping:           maps.Clone(req.ModelMapping),
		SkipTLSVerify:          req.SkipTLSVerify,
		Configs:                req.Configs,
		Sets:                   slices.Clone(req.Sets),
		EnabledNoPermissionBan: req.EnabledNoPermissionBan,
		MaxErrorRate:           req.MaxErrorRate,
	}
}

type TestSingleGroupChannelRequest struct {
	Type          int                  `json:"type"            binding:"required"`
	Key           string               `json:"key"             binding:"required"`
	BaseURL       string               `json:"base_url"`
	ProxyURL      string               `json:"proxy_url"`
	GroupID       string               `json:"group_id"`
	Name          string               `json:"name"`
	Model         string               `json:"model"           binding:"required"`
	ModelMapping  map[string]string    `json:"model_mapping"`
	SkipTLSVerify bool                 `json:"skip_tls_verify"`
	Configs       model.ChannelConfigs `json:"configs"`
	Sets          []string             `json:"sets"`
}

func (r *TestSingleGroupChannelRequest) toGroupChannel(group string) *model.GroupChannel {
	return &model.GroupChannel{
		GroupID:       group,
		Type:          model.ChannelType(r.Type),
		Name:          r.Name,
		Key:           r.Key,
		BaseURL:       r.BaseURL,
		ProxyURL:      r.ProxyURL,
		Models:        []string{r.Model},
		ModelMapping:  maps.Clone(r.ModelMapping),
		SkipTLSVerify: r.SkipTLSVerify,
		Configs:       r.Configs,
		Sets:          slices.Clone(r.Sets),
	}
}

func testSingleGroupChannelModel(
	mc *model.ModelCaches,
	groupChannel *model.GroupChannel,
	modelName string,
	saveToDB bool,
) (*model.GroupChannelTest, error) {
	group, err := model.CacheGetGroup(groupChannel.GroupID)
	if err != nil {
		return nil, err
	}

	modelConfig, err := groupChannelTestModelConfig(*group, modelName)
	if err != nil {
		return nil, err
	}

	channel := groupChannel.ToChannel()

	test, err := testSingleModelWithOptions(
		mc,
		channel,
		modelName,
		testSingleModelOptions{
			AllowMissingModelConfig: true,
			ModelConfig:             &modelConfig,
			SaveResult:              groupChannelTestSaveFunc(groupChannel),
		},
		saveToDB,
	)
	if err != nil {
		return nil, err
	}

	return channelTestToGroupChannelTest(groupChannel.GroupID, groupChannel, test), nil
}

func groupChannelTestModelConfig(
	group model.GroupCache,
	modelName string,
) (model.ModelConfig, error) {
	modelConfig, ok := model.ResolveGroupScopeModelConfig(group.ID, modelName)
	if !ok {
		return model.ModelConfig{}, fmt.Errorf("%s model config not found", modelName)
	}

	return middleware.GetGroupScopeAdjustedModelConfig(
		group,
		modelConfig,
	), nil
}

func groupChannelTestableModels(groupID string, channel *model.GroupChannel) ([]string, error) {
	if config.DisableModelConfig {
		models := model.GroupChannelAccessModels(channel)
		slices.Sort(models)
		return slices.Compact(models), nil
	}

	scopeConfigs, err := model.CacheGetGroupScopeModelConfigs(groupID)
	if err != nil {
		return nil, err
	}

	models := make([]string, 0, len(scopeConfigs.Models))
	for _, modelName := range scopeConfigs.Models {
		if _, ok := scopeConfigs.Configs[modelName]; !ok {
			continue
		}

		if !model.GroupChannelSupportsModel(channel, modelName) {
			continue
		}

		models = append(models, modelName)
	}

	slices.Sort(models)

	return models, nil
}

type GroupChannelTestResult struct {
	Data    *model.GroupChannelTest `json:"data,omitempty"`
	Message string                  `json:"message,omitempty"`
	Success bool                    `json:"success"`
}

func processGroupChannelTestResult(
	mc *model.ModelCaches,
	channel *model.GroupChannel,
	modelName string,
	saveToDB bool,
	returnSuccess, successResponseBody bool,
) *GroupChannelTestResult {
	ct, err := testSingleGroupChannelModel(mc, channel, modelName, saveToDB)

	result := &GroupChannelTestResult{
		Success: err == nil,
	}
	if err != nil {
		result.Message = fmt.Sprintf(
			"failed to test group channel %s(%d) model %s: %s",
			channel.Name,
			channel.ID,
			modelName,
			err.Error(),
		)

		return result
	}

	if !ct.Success {
		result.Data = ct
		return result
	}

	if !returnSuccess {
		return nil
	}

	if !successResponseBody {
		ct.Response = ""
	}

	result.Data = ct

	return result
}

// GetGroupChannelTests godoc
//
//	@Summary		Get group channel test results
//	@Description	Returns persisted test results for a group channel
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group ID"
//	@Param			id				path		int		true	"Group Channel ID"
//	@Param			success_body	query		bool	false	"Success body"
//	@Success		200				{object}	middleware.APIResponse{data=[]model.GroupChannelTest}
//	@Router			/api/group/{group}/channel/{id}/tests [get]
func GetGroupChannelTests(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	if _, err := model.GetGroupChannelByID(groupParam(c), id); err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "group channel not found",
		})

		return
	}

	tests, err := model.GetGroupChannelTests(groupParam(c), id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	if c.Query("success_body") != "true" {
		for _, test := range tests {
			if test.Success {
				test.Response = ""
			}
		}
	}

	c.JSON(http.StatusOK, middleware.APIResponse{
		Success: true,
		Data:    tests,
	})
}

// GetGlobalGroupChannelTests godoc
//
//	@Summary		Get group channel test results
//	@Description	Returns persisted test results for a group channel from the global management view
//	@Tags			group_channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id				path		int		true	"Group Channel ID"
//	@Param			success_body	query		bool	false	"Success body"
//	@Success		200				{object}	middleware.APIResponse{data=[]model.GroupChannelTest}
//	@Router			/api/group_channel/{id}/tests [get]
func GetGlobalGroupChannelTests(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	channel, err := model.GetGlobalGroupChannelByID(id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "group channel not found",
		})

		return
	}

	tests, err := model.GetGroupChannelTests(channel.GroupID, id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	if c.Query("success_body") != "true" {
		for _, test := range tests {
			if test.Success {
				test.Response = ""
			}
		}
	}

	c.JSON(http.StatusOK, middleware.APIResponse{
		Success: true,
		Data:    tests,
	})
}

// TestGroupChannel godoc
//
//	@Summary		Test group channel model
//	@Description	Tests a single model in the group channel
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group ID"
//	@Param			id				path		int		true	"Group Channel ID"
//	@Param			model			path		string	true	"Model name"
//	@Param			success_body	query		bool	false	"Success body"
//	@Success		200				{object}	middleware.APIResponse{data=model.GroupChannelTest}
//	@Router			/api/group/{group}/channel/{id}/test/{model} [get]
func TestGroupChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	modelName := strings.TrimPrefix(c.Param("model"), "/")
	if modelName == "" {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "model is required",
		})

		return
	}

	channel, err := model.LoadGroupChannelByID(groupParam(c), id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "group channel not found",
		})

		return
	}

	if !model.GroupChannelSupportsModel(channel, modelName) {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "model not supported by group channel",
		})

		return
	}

	if _, ok := model.ResolveGroupScopeModelConfig(channel.GroupID, modelName); !ok {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "model config not found",
		})

		return
	}

	ct, err := testSingleGroupChannelModel(model.LoadModelCaches(), channel, modelName, true)
	if err != nil {
		log.Errorf(
			"failed to test group channel %s(%d) model %s: %s",
			channel.Name,
			channel.ID,
			modelName,
			err.Error(),
		)
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: fmt.Sprintf(
				"failed to test group channel %s(%d) model %s: %s",
				channel.Name,
				channel.ID,
				modelName,
				err.Error(),
			),
		})

		return
	}

	if c.Query("success_body") != "true" && ct.Success {
		ct.Response = ""
	}

	c.JSON(http.StatusOK, middleware.APIResponse{
		Success: true,
		Data:    ct,
	})
}

// TestGlobalGroupChannel godoc
//
//	@Summary		Test group channel model
//	@Description	Tests a single model in a group channel from the global management view
//	@Tags			group_channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id				path		int		true	"Group Channel ID"
//	@Param			model			path		string	true	"Model name"
//	@Param			success_body	query		bool	false	"Success body"
//	@Success		200				{object}	middleware.APIResponse{data=model.GroupChannelTest}
//	@Router			/api/group_channel/{id}/test/{model} [get]
func TestGlobalGroupChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	modelName := strings.TrimPrefix(c.Param("model"), "/")
	if modelName == "" {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "model is required",
		})

		return
	}

	channel, err := model.LoadGlobalGroupChannelByID(id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "group channel not found",
		})

		return
	}

	if !model.GroupChannelSupportsModel(channel, modelName) {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "model not supported by group channel",
		})

		return
	}

	if _, ok := model.ResolveGroupScopeModelConfig(channel.GroupID, modelName); !ok {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "model config not found",
		})

		return
	}

	ct, err := testSingleGroupChannelModel(model.LoadModelCaches(), channel, modelName, true)
	if err != nil {
		log.Errorf(
			"failed to test group channel %s(%d) model %s: %s",
			channel.Name,
			channel.ID,
			modelName,
			err.Error(),
		)
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: fmt.Sprintf(
				"failed to test group channel %s(%d) model %s: %s",
				channel.Name,
				channel.ID,
				modelName,
				err.Error(),
			),
		})

		return
	}

	if c.Query("success_body") != "true" && ct.Success {
		ct.Response = ""
	}

	c.JSON(http.StatusOK, middleware.APIResponse{
		Success: true,
		Data:    ct,
	})
}

// TestGroupChannelModels godoc
//
//	@Summary		Test group channel models
//	@Description	Tests all models in the group channel
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group ID"
//	@Param			id				path		int		true	"Group Channel ID"
//	@Param			return_success	query		bool	false	"Return success"
//	@Param			success_body	query		bool	false	"Success body"
//	@Param			stream			query		bool	false	"Stream"
//	@Success		200				{object}	middleware.APIResponse{data=[]GroupChannelTestResult}
//	@Router			/api/group/{group}/channel/{id}/test [get]
func TestGroupChannelModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	channel, err := model.LoadGroupChannelByID(groupParam(c), id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "group channel not found",
		})

		return
	}

	returnSuccess := c.Query("return_success") == "true"
	successResponseBody := c.Query("success_body") == "true"
	isStream := c.Query("stream") == "true"

	results := make([]*GroupChannelTestResult, 0)
	resultsMutex := sync.Mutex{}
	hasError := atomic.Bool{}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)

	models, err := groupChannelTestableModels(channel.GroupID, channel)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	rand.Shuffle(len(models), func(i, j int) {
		models[i], models[j] = models[j], models[i]
	})

	mc := model.LoadModelCaches()

	for _, modelName := range models {
		wg.Add(1)

		semaphore <- struct{}{}

		go func(modelName string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			result := processGroupChannelTestResult(
				mc,
				channel,
				modelName,
				true,
				returnSuccess,
				successResponseBody,
			)
			if result == nil {
				return
			}

			if !result.Success || (result.Data != nil && !result.Data.Success) {
				hasError.Store(true)
			}

			resultsMutex.Lock()
			defer resultsMutex.Unlock()

			if isStream {
				if err := render.OpenaiObjectData(c, result); err != nil {
					log.Errorf("failed to render result: %s", err.Error())
				}
				return
			}

			results = append(results, result)
		}(modelName)
	}

	wg.Wait()

	if !hasError.Load() {
		if err := model.ClearGroupChannelLastTestErrorAt(channel.GroupID, channel.ID); err != nil {
			log.Errorf(
				"failed to clear last test error at for group channel %s(%d): %s",
				channel.Name,
				channel.ID,
				err.Error(),
			)
		}
	}

	if !isStream {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: true,
			Data:    results,
		})
	}
}

// TestGlobalGroupChannelModels godoc
//
//	@Summary		Test group channel models
//	@Description	Tests all models in a group channel from the global management view
//	@Tags			group_channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id				path		int		true	"Group Channel ID"
//	@Param			return_success	query		bool	false	"Return success"
//	@Param			success_body	query		bool	false	"Success body"
//	@Param			stream			query		bool	false	"Stream"
//	@Success		200				{object}	middleware.APIResponse{data=[]GroupChannelTestResult}
//	@Router			/api/group_channel/{id}/test [get]
func TestGlobalGroupChannelModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	channel, err := model.LoadGlobalGroupChannelByID(id)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "group channel not found",
		})

		return
	}

	returnSuccess := c.Query("return_success") == "true"
	successResponseBody := c.Query("success_body") == "true"
	isStream := c.Query("stream") == "true"

	results := make([]*GroupChannelTestResult, 0)
	resultsMutex := sync.Mutex{}
	hasError := atomic.Bool{}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)

	models, err := groupChannelTestableModels(channel.GroupID, channel)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	rand.Shuffle(len(models), func(i, j int) {
		models[i], models[j] = models[j], models[i]
	})

	mc := model.LoadModelCaches()

	for _, modelName := range models {
		wg.Add(1)

		semaphore <- struct{}{}

		go func(modelName string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			result := processGroupChannelTestResult(
				mc,
				channel,
				modelName,
				true,
				returnSuccess,
				successResponseBody,
			)
			if result == nil {
				return
			}

			if !result.Success || (result.Data != nil && !result.Data.Success) {
				hasError.Store(true)
			}

			resultsMutex.Lock()
			defer resultsMutex.Unlock()

			if isStream {
				if err := render.OpenaiObjectData(c, result); err != nil {
					log.Errorf("failed to render result: %s", err.Error())
				}
				return
			}

			results = append(results, result)
		}(modelName)
	}

	wg.Wait()

	if !hasError.Load() {
		if err := model.ClearGroupChannelLastTestErrorAt(channel.GroupID, channel.ID); err != nil {
			log.Errorf(
				"failed to clear last test error at for group channel %s(%d): %s",
				channel.Name,
				channel.ID,
				err.Error(),
			)
		}
	}

	if !isStream {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: true,
			Data:    results,
		})
	}
}

// TestGroupChannelPreview godoc
//
//	@Summary		Test group channel preview
//	@Description	Test a single model in a group channel without saving to database
//	@Tags			group-channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string							true	"Group ID"
//	@Param			success_body	query		bool							false	"Success body"
//	@Param			request			body		TestSingleGroupChannelRequest	true	"Group channel test request"
//	@Success		200				{object}	middleware.APIResponse{data=model.GroupChannelTest}
//	@Router			/api/group/{group}/channel/test-preview [post]
func TestGroupChannelPreview(c *gin.Context) {
	var req TestSingleGroupChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	channel := req.toGroupChannel(groupParam(c))

	ct, err := testSingleGroupChannelModel(model.LoadModelCaches(), channel, req.Model, false)
	if err != nil {
		log.Errorf("failed to test group channel preview: %s", err.Error())
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	if c.Query("success_body") != "true" && ct.Success {
		ct.Response = ""
	}

	c.JSON(http.StatusOK, middleware.APIResponse{
		Success: true,
		Data:    ct,
	})
}

// TestGlobalGroupChannelPreview godoc
//
//	@Summary		Test group channel preview
//	@Description	Test a single model in a group channel without saving to database from the global management view. The request body must include group_id.
//	@Tags			group_channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			success_body	query		bool							false	"Success body"
//	@Param			request			body		TestSingleGroupChannelRequest	true	"Group channel test request"
//	@Success		200				{object}	middleware.APIResponse{data=model.GroupChannelTest}
//	@Router			/api/group_channel/test-preview [post]
func TestGlobalGroupChannelPreview(c *gin.Context) {
	var req TestSingleGroupChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	channel := req.toGroupChannel(req.GroupID)

	ct, err := testSingleGroupChannelModel(model.LoadModelCaches(), channel, req.Model, false)
	if err != nil {
		log.Errorf("failed to test group channel preview: %s", err.Error())
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	if c.Query("success_body") != "true" && ct.Success {
		ct.Response = ""
	}

	c.JSON(http.StatusOK, middleware.APIResponse{
		Success: true,
		Data:    ct,
	})
}

// TestGroupChannelPreviewAll godoc
//
//	@Summary		Test group channel preview models
//	@Description	Test all models in a group channel without saving to database
//	@Tags			group-channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group			path		string					true	"Group ID"
//	@Param			return_success	query		bool					false	"Return success"
//	@Param			success_body	query		bool					false	"Success body"
//	@Param			stream			query		bool					false	"Stream mode (SSE)"
//	@Param			request			body		TestGroupChannelRequest	true	"Group channel test request"
//	@Success		200				{object}	middleware.APIResponse{data=[]GroupChannelTestResult}
//	@Router			/api/group/{group}/channel/test-preview-all [post]
func TestGroupChannelPreviewAll(c *gin.Context) {
	var req TestGroupChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	channel := req.toGroupChannel(groupParam(c))

	models, err := groupChannelTestableModels(channel.GroupID, channel)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	if len(models) == 0 {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "no models to test",
		})

		return
	}

	returnSuccess := c.Query("return_success") == "true"
	successResponseBody := c.Query("success_body") == "true"
	isStream := c.Query("stream") == "true"

	results := make([]*GroupChannelTestResult, 0)
	resultsMutex := sync.Mutex{}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)

	rand.Shuffle(len(models), func(i, j int) {
		models[i], models[j] = models[j], models[i]
	})

	mc := model.LoadModelCaches()

	for _, modelName := range models {
		wg.Add(1)

		semaphore <- struct{}{}

		go func(modelName string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			result := processGroupChannelTestResult(
				mc,
				channel,
				modelName,
				false,
				returnSuccess,
				successResponseBody,
			)
			if result == nil {
				return
			}

			resultsMutex.Lock()
			defer resultsMutex.Unlock()

			if isStream {
				if err := render.OpenaiObjectData(c, result); err != nil {
					log.Errorf("failed to render result: %s", err.Error())
				}
				return
			}

			results = append(results, result)
		}(modelName)
	}

	wg.Wait()

	if !isStream {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: true,
			Data:    results,
		})
	}
}

// TestGlobalGroupChannelPreviewAll godoc
//
//	@Summary		Test group channel preview models
//	@Description	Test all models in a group channel without saving to database from the global management view. The request body must include group_id.
//	@Tags			group_channel
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			return_success	query		bool					false	"Return success"
//	@Param			success_body	query		bool					false	"Success body"
//	@Param			stream			query		bool					false	"Stream mode (SSE)"
//	@Param			request			body		TestGroupChannelRequest	true	"Group channel test request"
//	@Success		200				{object}	middleware.APIResponse{data=[]GroupChannelTestResult}
//	@Router			/api/group_channel/test-preview-all [post]
func TestGlobalGroupChannelPreviewAll(c *gin.Context) {
	var req TestGroupChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	channel := req.toGroupChannel(req.GroupID)

	models, err := groupChannelTestableModels(channel.GroupID, channel)
	if err != nil {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: err.Error(),
		})

		return
	}

	if len(models) == 0 {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: false,
			Message: "no models to test",
		})

		return
	}

	returnSuccess := c.Query("return_success") == "true"
	successResponseBody := c.Query("success_body") == "true"
	isStream := c.Query("stream") == "true"

	results := make([]*GroupChannelTestResult, 0)
	resultsMutex := sync.Mutex{}

	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 5)

	rand.Shuffle(len(models), func(i, j int) {
		models[i], models[j] = models[j], models[i]
	})

	mc := model.LoadModelCaches()

	for _, modelName := range models {
		wg.Add(1)

		semaphore <- struct{}{}

		go func(modelName string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			result := processGroupChannelTestResult(
				mc,
				channel,
				modelName,
				false,
				returnSuccess,
				successResponseBody,
			)
			if result == nil {
				return
			}

			resultsMutex.Lock()
			defer resultsMutex.Unlock()

			if isStream {
				if err := render.OpenaiObjectData(c, result); err != nil {
					log.Errorf("failed to render result: %s", err.Error())
				}
				return
			}

			results = append(results, result)
		}(modelName)
	}

	wg.Wait()

	if !isStream {
		c.JSON(http.StatusOK, middleware.APIResponse{
			Success: true,
			Data:    results,
		})
	}
}
