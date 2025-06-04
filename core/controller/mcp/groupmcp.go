package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

type GroupMCPResponse struct {
	model.GroupMCP
	Endpoints MCPEndpoint `json:"endpoints"`
}

func (mcp *GroupMCPResponse) MarshalJSON() ([]byte, error) {
	type Alias GroupMCPResponse
	a := &struct {
		*Alias
		CreatedAt int64 `json:"created_at"`
		UpdateAt  int64 `json:"update_at"`
	}{
		Alias:     (*Alias)(mcp),
		CreatedAt: mcp.CreatedAt.UnixMilli(),
		UpdateAt:  mcp.UpdateAt.UnixMilli(),
	}
	return sonic.Marshal(a)
}

func NewGroupMCPResponse(host string, mcp model.GroupMCP) GroupMCPResponse {
	ep := MCPEndpoint{}
	switch mcp.Type {
	case model.GroupMCPTypeProxySSE,
		model.GroupMCPTypeProxyStreamable,
		model.GroupMCPTypeOpenAPI:
		groupMCPHost := config.GetGroupMCPHost()
		if groupMCPHost == "" {
			ep.Host = host
			ep.SSE = fmt.Sprintf("/mcp/group/%s/sse", mcp.ID)
			ep.StreamableHTTP = "/mcp/group/" + mcp.ID
		} else {
			ep.Host = fmt.Sprintf("%s.%s", mcp.ID, groupMCPHost)
			ep.SSE = "/sse"
			ep.StreamableHTTP = "/mcp"
		}
	}
	return GroupMCPResponse{
		GroupMCP:  mcp,
		Endpoints: ep,
	}
}

func NewGroupMCPResponses(host string, mcps []model.GroupMCP) []GroupMCPResponse {
	responses := make([]GroupMCPResponse, len(mcps))
	for i, mcp := range mcps {
		responses[i] = NewGroupMCPResponse(host, mcp)
	}
	return responses
}

// GetGroupMCPs godoc
//
//	@Summary		Get Group MCPs
//	@Description	Get a list of Group MCPs with pagination and filtering
//	@Tags			mcp
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group		path		string	true	"Group ID"
//	@Param			page		query		int		false	"Page number"
//	@Param			per_page	query		int		false	"Items per page"
//	@Param			type		query		string	false	"MCP type"
//	@Param			keyword		query		string	false	"Search keyword"
//	@Param			status		query		int		false	"MCP status"
//	@Success		200			{object}	middleware.APIResponse{data=[]GroupMCPResponse}
//	@Router			/api/mcp/group/{group} [get]
func GetGroupMCPs(c *gin.Context) {
	groupID := c.Param("group")
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "Group ID is required")
		return
	}

	page, perPage := utils.ParsePageParams(c)
	mcpType := model.PublicMCPType(c.Query("type"))
	keyword := c.Query("keyword")
	status, _ := strconv.Atoi(c.Query("status"))

	if status == 0 {
		status = int(model.GroupMCPStatusEnabled)
	}

	mcps, total, err := model.GetGroupMCPs(
		groupID,
		page,
		perPage,
		mcpType,
		keyword,
		model.GroupMCPStatus(status),
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"mcps":  NewGroupMCPResponses(c.Request.Host, mcps),
		"total": total,
	})
}

// GetAllGroupMCPs godoc
//
//	@Summary		Get all Group MCPs
//	@Description	Get all Group MCPs with filtering
//	@Tags			mcp
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			status	query		int	false	"MCP status"
//	@Success		200		{object}	middleware.APIResponse{data=[]GroupMCPResponse}
//	@Router			/api/mcp/group/all [get]
func GetAllGroupMCPs(c *gin.Context) {
	status, _ := strconv.Atoi(c.Query("status"))

	if status == 0 {
		status = int(model.GroupMCPStatusEnabled)
	}
	mcps, err := model.GetAllGroupMCPs(model.GroupMCPStatus(status))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	middleware.SuccessResponse(c, NewGroupMCPResponses(c.Request.Host, mcps))
}

// GetGroupMCPByID godoc
//
//	@Summary		Get Group MCP by ID
//	@Description	Get a specific Group MCP by its ID and Group ID
//	@Tags			mcp
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		string	true	"MCP ID"
//	@Param			group	path		string	true	"Group ID"
//	@Success		200		{object}	middleware.APIResponse{data=GroupMCPResponse}
//	@Router			/api/mcp/group/{group}/{id} [get]
func GetGroupMCPByID(c *gin.Context) {
	id := c.Param("id")
	groupID := c.Param("group")

	if id == "" || groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "MCP ID and Group ID are required")
		return
	}

	mcp, err := model.GetGroupMCPByID(id, groupID)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	middleware.SuccessResponse(c, NewGroupMCPResponse(c.Request.Host, mcp))
}

// CreateGroupMCP godoc
//
//	@Summary		Create Group MCP
//	@Description	Create a new Group MCP
//	@Tags			mcp
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string			true	"Group ID"
//	@Param			mcp		body		model.GroupMCP	true	"Group MCP object"
//	@Success		200		{object}	middleware.APIResponse{data=GroupMCPResponse}
//	@Router			/api/mcp/group/{group} [post]
func CreateGroupMCP(c *gin.Context) {
	groupID := c.Param("group")
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "Group ID is required")
		return
	}

	var mcp model.GroupMCP
	if err := c.ShouldBindJSON(&mcp); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	mcp.GroupID = groupID

	if err := model.CreateGroupMCP(&mcp); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, NewGroupMCPResponse(c.Request.Host, mcp))
}

// UpdateGroupMCP godoc
//
//	@Summary		Update Group MCP
//	@Description	Update an existing Group MCP
//	@Tags			mcp
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		string			true	"MCP ID"
//	@Param			group	path		string			true	"Group ID"
//	@Param			mcp		body		model.GroupMCP	true	"Group MCP object"
//	@Success		200		{object}	middleware.APIResponse{data=GroupMCPResponse}
//	@Router			/api/mcp/group/{group}/{id} [put]
func UpdateGroupMCP(c *gin.Context) {
	id := c.Param("id")
	groupID := c.Param("group")

	if id == "" || groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "MCP ID and Group ID are required")
		return
	}

	var mcp model.GroupMCP
	if err := c.ShouldBindJSON(&mcp); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	mcp.ID = id
	mcp.GroupID = groupID

	if err := model.UpdateGroupMCP(&mcp); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, NewGroupMCPResponse(c.Request.Host, mcp))
}

type UpdateGroupMCPStatusRequest struct {
	Status model.GroupMCPStatus `json:"status"`
}

// UpdateGroupMCPStatus godoc
//
//	@Summary		Update Group MCP status
//	@Description	Update the status of a Group MCP
//	@Tags			mcp
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		string						true	"MCP ID"
//	@Param			group	path		string						true	"Group ID"
//	@Param			status	body		UpdateGroupMCPStatusRequest	true	"MCP status"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/mcp/group/{group}/{id}/status [post]
func UpdateGroupMCPStatus(c *gin.Context) {
	id := c.Param("id")
	groupID := c.Param("group")

	if id == "" || groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "MCP ID and Group ID are required")
		return
	}

	var status UpdateGroupMCPStatusRequest
	if err := c.ShouldBindJSON(&status); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := model.UpdateGroupMCPStatus(id, groupID, status.Status); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

// DeleteGroupMCP godoc
//
//	@Summary		Delete Group MCP
//	@Description	Delete a Group MCP by ID and Group ID
//	@Tags			mcp
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			id		path		string	true	"MCP ID"
//	@Param			group	path		string	true	"Group ID"
//	@Success		200		{object}	middleware.APIResponse
//	@Router			/api/mcp/group/{group}/{id} [delete]
func DeleteGroupMCP(c *gin.Context) {
	id := c.Param("id")
	groupID := c.Param("group")

	if id == "" || groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "MCP ID and Group ID are required")
		return
	}

	if err := model.DeleteGroupMCP(id, groupID); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}
