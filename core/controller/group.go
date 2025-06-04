package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

type GroupResponse struct {
	*model.Group
	AccessedAt time.Time `json:"accessed_at,omitempty"`
}

func (g *GroupResponse) MarshalJSON() ([]byte, error) {
	type Alias model.Group
	return sonic.Marshal(&struct {
		*Alias
		CreatedAt  int64 `json:"created_at,omitempty"`
		AccessedAt int64 `json:"accessed_at,omitempty"`
	}{
		Alias:      (*Alias)(g.Group),
		CreatedAt:  g.CreatedAt.UnixMilli(),
		AccessedAt: g.AccessedAt.UnixMilli(),
	})
}

// GetGroups godoc
//
//	@Summary		Get all groups
//	@Description	Returns a list of all groups with pagination
//	@Tags			groups
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			page		query		int	false	"Page number"
//	@Param			per_page	query		int	false	"Items per page"
//	@Success		200			{object}	middleware.APIResponse{data=map[string]any{groups=[]GroupResponse,total=int}}
//	@Router			/api/groups/ [get]
func GetGroups(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	order := c.DefaultQuery("order", "")
	groups, total, err := model.GetGroups(page, perPage, order, false)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	groupResponses := make([]*GroupResponse, len(groups))
	for i, group := range groups {
		lastRequestAt, _ := model.GetGroupLastRequestTime(group.ID)
		groupResponses[i] = &GroupResponse{
			Group:      group,
			AccessedAt: lastRequestAt,
		}
	}
	middleware.SuccessResponse(c, gin.H{
		"groups": groupResponses,
		"total":  total,
	})
}

// SearchGroups godoc
//
//	@Summary		Search groups
//	@Description	Search groups with keyword and pagination
//	@Tags			groups
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			keyword		query		string	true	"Search keyword"
//	@Param			page		query		int		false	"Page number"
//	@Param			per_page	query		int		false	"Items per page"
//	@Param			status		query		int		false	"Status"
//	@Param			order		query		string	false	"Order"
//	@Success		200			{object}	middleware.APIResponse{data=map[string]any{groups=[]GroupResponse,total=int}}
//	@Router			/api/groups/search [get]
func SearchGroups(c *gin.Context) {
	keyword := c.Query("keyword")
	page, perPage := utils.ParsePageParams(c)
	order := c.DefaultQuery("order", "")
	status, _ := strconv.Atoi(c.Query("status"))
	groups, total, err := model.SearchGroup(keyword, page, perPage, order, status)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	groupResponses := make([]*GroupResponse, len(groups))
	for i, group := range groups {
		lastRequestAt, _ := model.GetGroupLastRequestTime(group.ID)
		groupResponses[i] = &GroupResponse{
			Group:      group,
			AccessedAt: lastRequestAt,
		}
	}
	middleware.SuccessResponse(c, gin.H{
		"groups": groupResponses,
		"total":  total,
	})
}

// GetGroup godoc
//
//	@Summary		Get a group
//	@Description	Returns detailed information about a specific group
//	@Tags			group
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Success		200		{object}	middleware.APIResponse{data=GroupResponse}
//	@Router			/api/group/{group} [get]
func GetGroup(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group id is empty")
		return
	}
	_group, err := model.GetGroupByID(group, false)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	lastRequestAt, _ := model.GetGroupLastRequestTime(group)
	groupResponse := &GroupResponse{
		Group:      _group,
		AccessedAt: lastRequestAt,
	}
	middleware.SuccessResponse(c, groupResponse)
}

type UpdateGroupRPMRatioRequest struct {
	RPMRatio float64 `json:"rpm_ratio"`
}

// UpdateGroupRPMRatio godoc
//
//	@Summary		Update group RPM ratio
//	@Description	Updates the RPM (Requests Per Minute) ratio for a group
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string						true	"Group name"
//	@Param			data	body		UpdateGroupRPMRatioRequest	true	"RPM ratio information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/rpm_ratio [post]
func UpdateGroupRPMRatio(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	req := UpdateGroupRPMRatioRequest{}
	err := sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	err = model.UpdateGroupRPMRatio(group, req.RPMRatio)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

type UpdateGroupTPMRatioRequest struct {
	TPMRatio float64 `json:"tpm_ratio"`
}

// UpdateGroupTPMRatio godoc
//
//	@Summary		Update group TPM ratio
//	@Description	Updates the TPM (Tokens Per Minute) ratio for a group
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string						true	"Group name"
//	@Param			data	body		UpdateGroupTPMRatioRequest	true	"TPM ratio information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/tpm_ratio [post]
func UpdateGroupTPMRatio(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	req := UpdateGroupTPMRatioRequest{}
	err := sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	err = model.UpdateGroupTPMRatio(group, req.TPMRatio)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

type UpdateGroupStatusRequest struct {
	Status int `json:"status"`
}

// UpdateGroupStatus godoc
//
//	@Summary		Update group status
//	@Description	Updates the status of a group
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string						true	"Group name"
//	@Param			status	body		UpdateGroupStatusRequest	true	"Status information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/status [post]
func UpdateGroupStatus(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	req := UpdateGroupStatusRequest{}
	err := sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	err = model.UpdateGroupStatus(group, req.Status)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// DeleteGroup godoc
//
//	@Summary		Delete a group
//	@Description	Deletes a group by its name
//	@Tags			group
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group} [delete]
func DeleteGroup(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	err := model.DeleteGroupByID(group)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// DeleteGroups godoc
//
//	@Summary		Delete multiple groups
//	@Description	Deletes multiple groups by their IDs
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			ids	body		[]string	true	"Group IDs"
//	@Success		200	{object}	middleware.APIResponse
//	@Router			/api/groups/batch_delete [post]
func DeleteGroups(c *gin.Context) {
	ids := []string{}
	err := c.ShouldBindJSON(&ids)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	err = model.DeleteGroupsByIDs(ids)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

type UpdateGroupsStatusRequest struct {
	Status int      `json:"status"`
	Groups []string `json:"groups"`
}

// UpdateGroupsStatus godoc
//
//	@Summary		Update multiple groups status
//	@Description	Updates the status of multiple groups
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			data	body		UpdateGroupsStatusRequest	true	"Group IDs and status"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/groups/batch_status [post]
func UpdateGroupsStatus(c *gin.Context) {
	req := UpdateGroupsStatusRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	_, err = model.UpdateGroupsStatus(req.Groups, req.Status)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

type CreateGroupRequest struct {
	RPMRatio      float64  `json:"rpm_ratio"`
	TPMRatio      float64  `json:"tpm_ratio"`
	AvailableSets []string `json:"available_sets"`

	BalanceAlertEnabled   bool    `json:"balance_alert_enabled"`
	BalanceAlertThreshold float64 `json:"balance_alert_threshold"`
}

func (r *CreateGroupRequest) ToGroup() *model.Group {
	return &model.Group{
		RPMRatio:      r.RPMRatio,
		TPMRatio:      r.TPMRatio,
		AvailableSets: r.AvailableSets,

		BalanceAlertEnabled:   r.BalanceAlertEnabled,
		BalanceAlertThreshold: r.BalanceAlertThreshold,
	}
}

// CreateGroup godoc
//
//	@Summary		Create a new group
//	@Description	Creates a new group with the given information
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string				true	"Group name"
//	@Param			data	body		CreateGroupRequest	true	"Group information"
//	@Success		200		{object}	middleware.APIResponse{data=model.Group}
//	@Router			/api/group/{group} [post]
func CreateGroup(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	req := CreateGroupRequest{}
	err := sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	g := req.ToGroup()
	g.ID = group
	if err := model.CreateGroup(g); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, g)
}

// UpdateGroup godoc
//
//	@Summary		Update a group
//	@Description	Updates an existing group with the given information
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string				true	"Group name"
//	@Param			data	body		CreateGroupRequest	true	"Updated group information"
//	@Success		200		{object}	middleware.APIResponse{data=model.Group}
//	@Router			/api/group/{group} [put]
func UpdateGroup(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	req := CreateGroupRequest{}
	err := sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	g := req.ToGroup()
	err = model.UpdateGroup(group, g)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, g)
}

type SaveGroupModelConfigRequest struct {
	Model string `json:"model"`

	OverrideLimit bool  `json:"override_limit"`
	RPM           int64 `json:"rpm"`
	TPM           int64 `json:"tpm"`

	OverridePrice bool               `json:"override_price"`
	ImagePrices   map[string]float64 `json:"image_prices"`
	Price         model.Price        `json:"price"`
}

func (r *SaveGroupModelConfigRequest) ToGroupModelConfig(groupID string) model.GroupModelConfig {
	return model.GroupModelConfig{
		GroupID: groupID,
		Model:   r.Model,

		OverrideLimit: r.OverrideLimit,
		RPM:           r.RPM,
		TPM:           r.TPM,

		OverridePrice: r.OverridePrice,
		ImagePrices:   r.ImagePrices,
		Price:         r.Price,
	}
}

// SaveGroupModelConfigs godoc
//
//	@Summary		Save group model configs
//	@Description	Save group model configs
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string							true	"Group name"
//	@Param			data	body		[]SaveGroupModelConfigRequest	true	"Group model config information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/model_configs/ [post]
func SaveGroupModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	req := []SaveGroupModelConfigRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	configs := make([]model.GroupModelConfig, len(req))
	for i, config := range req {
		configs[i] = config.ToGroupModelConfig(group)
	}
	err = model.SaveGroupModelConfigs(group, configs)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// SaveGroupModelConfig godoc
//
//	@Summary		Save group model config
//	@Description	Save group model config
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string						true	"Group name"
//	@Param			data	body		SaveGroupModelConfigRequest	true	"Group model config information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/model_config/{model} [post]
func SaveGroupModelConfig(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	modelName := c.Param("model")
	if modelName == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	req := SaveGroupModelConfigRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	modelConfig := req.ToGroupModelConfig(group)
	modelConfig.Model = modelName
	err = model.SaveGroupModelConfig(modelConfig)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// DeleteGroupModelConfig godoc
//
//	@Summary		Delete group model config
//	@Description	Delete group model config
//	@Tags			group
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/model_config/{model} [delete]
func DeleteGroupModelConfig(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	modelName := c.Param("model")
	if modelName == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	err := model.DeleteGroupModelConfig(group, modelName)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// DeleteGroupModelConfigs godoc
//
//	@Summary		Delete group model configs
//	@Description	Delete group model configs
//	@Tags			group
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string		true	"Group name"
//	@Param			models	body		[]string	true	"Model names"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/model_configs/ [delete]
func DeleteGroupModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	models := []string{}
	err := c.ShouldBindJSON(&models)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	err = model.DeleteGroupModelConfigs(group, models)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// GetGroupModelConfigs godoc
//
//	@Summary		Get group model configs
//	@Description	Get group model configs
//	@Tags			group
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Success		200		{object}	middleware.APIResponse{data=[]model.GroupModelConfig}
//	@Router			/api/group/{group}/model_configs/ [get]
func GetGroupModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	modelConfigs, err := model.GetGroupModelConfigs(group)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, modelConfigs)
}

// GetGroupModelConfig godoc
//
//	@Summary		Get group model config
//	@Description	Get group model config
//	@Tags			group
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group name"
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	middleware.APIResponse{data=model.GroupModelConfig}
//	@Router			/api/group/{group}/model_config/{model} [get]
func GetGroupModelConfig(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	modelName := c.Param("model")
	if modelName == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	modelConfig, err := model.GetGroupModelConfig(group, modelName)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, modelConfig)
}

// UpdateGroupModelConfig godoc
//
//	@Summary		Update group model config
//	@Description	Update group model config
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string						true	"Group name"
//	@Param			model	path		string						true	"Model name"
//	@Param			data	body		SaveGroupModelConfigRequest	true	"Group model config information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/model_config/{model} [put]
func UpdateGroupModelConfig(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	modelName := c.Param("model")
	if modelName == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	req := SaveGroupModelConfigRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	modelConfig := req.ToGroupModelConfig(group)
	modelConfig.Model = modelName
	err = model.UpdateGroupModelConfig(modelConfig)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// UpdateGroupModelConfigs godoc
//
//	@Summary		Update group model configs
//	@Description	Update group model configs
//	@Tags			group
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string							true	"Group name"
//	@Param			data	body		[]SaveGroupModelConfigRequest	true	"Group model config information"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/group/{group}/model_configs/ [put]
func UpdateGroupModelConfigs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}

	req := []SaveGroupModelConfigRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid parameter")
		return
	}
	configs := make([]model.GroupModelConfig, len(req))
	for i, config := range req {
		configs[i] = config.ToGroupModelConfig(group)
	}
	err = model.UpdateGroupModelConfigs(group, configs)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, nil)
}

// GetIPGroupList godoc
//
//	@Summary		Get IP group list
//	@Description	Get IP group list
//	@Tags			groups
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			threshold		query		int	false	"Threshold"
//	@Param			start_timestamp	query		int	false	"Start timestamp"
//	@Param			end_timestamp	query		int	false	"End timestamp"
//	@Success		200				{object}	middleware.APIResponse{data=map[string][]string}
//	@Router			/api/groups/ip_groups [get]
func GetIPGroupList(c *gin.Context) {
	threshold, _ := strconv.Atoi(c.Query("threshold"))
	startTime, endTime := parseTimeRange(c)
	ipGroupList, err := model.GetIPGroups(threshold, startTime, endTime)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, ipGroupList)
}
