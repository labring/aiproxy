//go:build enterprise

package analytics

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// getCallerGroupID extracts the enterprise user's GroupID from gin context.
func getCallerGroupID(c *gin.Context) string {
	user, exists := c.Get("enterprise_user")
	if !exists {
		return ""
	}

	if fu, ok := user.(*models.FeishuUser); ok {
		return fu.GroupID
	}

	return ""
}

func isAdmin(c *gin.Context) bool {
	role, _ := c.Get("enterprise_role")
	r, _ := role.(string)
	return r == models.RoleAdmin
}

// HandleListTemplates returns templates visible to the caller.
func HandleListTemplates(c *gin.Context) {
	groupID := getCallerGroupID(c)
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	var templates []models.ReportTemplate
	q := model.DB.Order("created_at DESC")
	if !isAdmin(c) {
		q = q.Where("created_by = ?", groupID)
	}

	if err := q.Find(&templates).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, templates)
}

type createTemplateReq struct {
	Name       string   `json:"name"       binding:"required"`
	Dimensions []string `json:"dimensions" binding:"required"`
	Measures   []string `json:"measures"   binding:"required"`
	ChartType  string   `json:"chart_type"`
	ViewMode   string   `json:"view_mode"`
	SortBy     string   `json:"sort_by"`
	SortOrder  string   `json:"sort_order"`
}

// HandleCreateTemplate saves a new report template.
func HandleCreateTemplate(c *gin.Context) {
	groupID := getCallerGroupID(c)
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	var req createTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	dimJSON, _ := marshalJSON(req.Dimensions)
	measJSON, _ := marshalJSON(req.Measures)

	tpl := models.ReportTemplate{
		Name:       req.Name,
		CreatedBy:  groupID,
		Dimensions: dimJSON,
		Measures:   measJSON,
		ChartType:  req.ChartType,
		ViewMode:   req.ViewMode,
		SortBy:     req.SortBy,
		SortOrder:  req.SortOrder,
	}

	if err := model.DB.Create(&tpl).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, tpl)
}

type updateTemplateReq struct {
	Name       string   `json:"name"`
	Dimensions []string `json:"dimensions"`
	Measures   []string `json:"measures"`
	ChartType  string   `json:"chart_type"`
	ViewMode   string   `json:"view_mode"`
	SortBy     string   `json:"sort_by"`
	SortOrder  string   `json:"sort_order"`
}

// HandleUpdateTemplate updates an existing template (owner or admin only).
func HandleUpdateTemplate(c *gin.Context) {
	groupID := getCallerGroupID(c)
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid id")
		return
	}

	var tpl models.ReportTemplate
	if err := model.DB.First(&tpl, id).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusNotFound, "template not found")
		return
	}

	if tpl.CreatedBy != groupID && !isAdmin(c) {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	var req updateTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}

	if len(req.Dimensions) > 0 {
		j, _ := marshalJSON(req.Dimensions)
		updates["dimensions"] = j
	}

	if len(req.Measures) > 0 {
		j, _ := marshalJSON(req.Measures)
		updates["measures"] = j
	}

	if req.ChartType != "" {
		updates["chart_type"] = req.ChartType
	}

	if req.ViewMode != "" {
		updates["view_mode"] = req.ViewMode
	}

	if req.SortBy != "" {
		updates["sort_by"] = req.SortBy
	}

	if req.SortOrder != "" {
		updates["sort_order"] = req.SortOrder
	}

	if err := model.DB.Model(&tpl).Updates(updates).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Re-read to return the updated state.
	if err := model.DB.First(&tpl, id).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, tpl)
}

// HandleDeleteTemplate deletes a template (owner or admin only).
func HandleDeleteTemplate(c *gin.Context) {
	groupID := getCallerGroupID(c)
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid id")
		return
	}

	q := model.DB.Where("id = ?", id)
	if !isAdmin(c) {
		q = q.Where("created_by = ?", groupID)
	}

	result := q.Delete(&models.ReportTemplate{})
	if result.Error != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, result.Error.Error())
		return
	}

	if result.RowsAffected == 0 {
		middleware.ErrorResponse(c, http.StatusNotFound, "template not found or forbidden")
		return
	}

	middleware.SuccessResponse(c, nil)
}

func marshalJSON(v []string) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}
