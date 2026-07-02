package controller

import (
	"errors"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/mark3labs/mcp-go/mcp"
	"gorm.io/gorm"
)

func IsHostedMCP(t model.PublicMCPType) bool {
	return t == model.PublicMCPTypeEmbed ||
		t == model.PublicMCPTypeOpenAPI ||
		t == model.PublicMCPTypeProxySSE ||
		t == model.PublicMCPTypeProxyStreamable
}

type GroupPublicMCPResponse struct {
	model.PublicMCP
	Hosted bool `json:"hosted"`
}

func (r *GroupPublicMCPResponse) MarshalJSON() ([]byte, error) {
	type Alias GroupPublicMCPResponse

	a := &struct {
		*Alias
		CreatedAt int64 `json:"created_at,omitempty"`
		UpdateAt  int64 `json:"update_at,omitempty"`
	}{
		Alias: (*Alias)(r),
	}
	if !r.CreatedAt.IsZero() {
		a.CreatedAt = r.CreatedAt.UnixMilli()
	}

	if !r.UpdateAt.IsZero() {
		a.UpdateAt = r.UpdateAt.UnixMilli()
	}

	return sonic.Marshal(a)
}

func NewGroupPublicMCPResponse(mcp model.PublicMCP) GroupPublicMCPResponse {
	r := GroupPublicMCPResponse{
		PublicMCP: mcp,
		Hosted:    IsHostedMCP(mcp.Type),
	}
	r.Type = ""
	r.Readme = ""
	r.ReadmeCN = ""
	r.ReadmeURL = ""
	r.ReadmeCNURL = ""
	r.ProxyConfig = nil
	r.EmbedConfig = nil
	r.OpenAPIConfig = nil
	r.TestConfig = nil

	return r
}

func NewGroupPublicMCPResponses(mcps []model.PublicMCP) []GroupPublicMCPResponse {
	responses := make([]GroupPublicMCPResponse, len(mcps))
	for i, mcp := range mcps {
		responses[i] = NewGroupPublicMCPResponse(mcp)
	}

	return responses
}

type GroupPublicMCPDetailResponse struct {
	model.PublicMCP
	Hosted    bool                          `json:"hosted"`
	Endpoints MCPEndpoint                   `json:"endpoints"`
	Reusing   map[string]model.ReusingParam `json:"reusing"`
	Params    map[string]string             `json:"params"`
	Tools     []mcp.Tool                    `json:"tools"`
}

func (r *GroupPublicMCPDetailResponse) MarshalJSON() ([]byte, error) {
	type Alias GroupPublicMCPDetailResponse

	a := &struct {
		*Alias
		CreatedAt int64 `json:"created_at,omitempty"`
		UpdateAt  int64 `json:"update_at,omitempty"`
	}{
		Alias: (*Alias)(r),
	}
	if !r.CreatedAt.IsZero() {
		a.CreatedAt = r.CreatedAt.UnixMilli()
	}

	if !r.UpdateAt.IsZero() {
		a.UpdateAt = r.UpdateAt.UnixMilli()
	}

	return sonic.Marshal(a)
}

func NewGroupPublicMCPDetailResponse(
	ctx *gin.Context,
	host string,
	mcp model.PublicMCP,
	groupID string,
) (GroupPublicMCPDetailResponse, error) {
	r := GroupPublicMCPDetailResponse{
		PublicMCP: mcp,
		Hosted:    IsHostedMCP(mcp.Type),
	}

	var testConfig model.TestConfig
	if mcp.TestConfig != nil {
		testConfig = *mcp.TestConfig
	}

	r.Type = ""
	r.ProxyConfig = nil
	r.EmbedConfig = nil
	r.OpenAPIConfig = nil
	r.TestConfig = nil

	switch mcp.Type {
	case model.PublicMCPTypeProxySSE, model.PublicMCPTypeProxyStreamable:
		r.Reusing = make(map[string]model.ReusingParam, len(mcp.ProxyConfig.Reusing))
		for _, v := range mcp.ProxyConfig.Reusing {
			r.Reusing[v.Name] = v.ReusingParam
		}
	case model.PublicMCPTypeEmbed:
		r.Reusing = mcp.EmbedConfig.Reusing
	default:
		return r, nil
	}

	reusingParams, err := model.CacheGetPublicMCPReusingParam(mcp.ID, groupID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return r, err
		}
	}

	r.Params = reusingParams.Params

	tools, err := getPublicMCPTools(ctx.Request.Context(), mcp, testConfig, r.Params, r.Reusing)
	if err != nil {
		log := common.GetLogger(ctx)
		log.Errorf("get public mcp tools error: %s", err.Error())
	} else {
		r.Tools = tools
	}

	if checkParamsIsFull(r.Params, r.Reusing) {
		r.Endpoints = NewPublicMCPEndpoint(host, mcp)
	}

	return r, nil
}

// GetGroupPublicMCPs godoc
//
//	@Summary		Get MCPs by group
//	@Description	Get MCPs by group
//	@Tags			mcp
//	@Tags			group
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group		path		string	true	"Group ID"
//	@Param			page		query		int		false	"Page"
//	@Param			per_page	query		int		false	"Per Page"
//	@Param			id			query		string	false	"MCP ID"
//	@Param			type		query		string	false	"hosted or local"
//	@Param			keyword		query		string	false	"Keyword"
//	@Success		200			{object}	middleware.APIResponse{data=[]GroupPublicMCPResponse}
//	@Router			/api/group/{group}/mcp/ [get]
func GetGroupPublicMCPs(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)
	id := c.Query("id")
	mcpType := c.Query("type")

	var mcpTypes []model.PublicMCPType
	switch mcpType {
	case "hosted":
		mcpTypes = getHostedMCPTypes()
	case "local":
		mcpTypes = getLocalMCPTypes()
	}

	keyword := c.Query("keyword")

	mcps, total, err := model.GetPublicMCPs(
		page,
		perPage,
		id,
		mcpTypes,
		keyword,
		model.PublicMCPStatusEnabled,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	responses := NewGroupPublicMCPResponses(mcps)

	middleware.SuccessResponse(c, gin.H{
		"mcps":  responses,
		"total": total,
	})
}

// GetGroupPublicMCPByID godoc
//
//	@Summary		Get MCP by ID
//	@Description	Get a specific MCP by its ID
//	@Tags			mcp
//	@Tags			group
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string	true	"Group ID"
//	@Param			id		path		string	true	"MCP ID"
//	@Success		200		{object}	middleware.APIResponse{data=GroupPublicMCPDetailResponse}
//	@Router			/api/group/{group}/mcp/{id} [get]
func GetGroupPublicMCPByID(c *gin.Context) {
	groupID := c.Param("group")

	id := c.Param("id")
	if id == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "MCP ID is required")
		return
	}

	mcp, err := model.GetPublicMCPByID(id)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	response, err := NewGroupPublicMCPDetailResponse(
		c,
		c.Request.Host,
		mcp,
		groupID,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, response)
}
