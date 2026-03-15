//go:build enterprise

package quota

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"gorm.io/gorm"
)

// ListPolicies returns all quota policies with pagination.
func ListPolicies(c *gin.Context) {
	page, perPage := parsePageParams(c)

	var policies []models.QuotaPolicy
	var total int64

	tx := model.DB.Model(&models.QuotaPolicy{})

	if err := tx.Count(&total).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	if page > 0 && perPage > 0 {
		tx = tx.Offset((page - 1) * perPage).Limit(perPage)
	} else if perPage > 0 {
		tx = tx.Limit(perPage)
	}

	if err := tx.Order("id DESC").Find(&policies).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"policies": policies,
		"total":    total,
	})
}

// GetPolicy returns a single quota policy by ID.
func GetPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid policy id")
		return
	}

	var policy models.QuotaPolicy
	if err := model.DB.First(&policy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.ErrorResponse(c, http.StatusNotFound, "policy not found")
			return
		}

		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	middleware.SuccessResponse(c, policy)
}

// CreatePolicy creates a new quota policy.
func CreatePolicy(c *gin.Context) {
	var policy models.QuotaPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	policy.ID = 0

	if err := model.DB.Create(&policy).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, policy)
}

// UpdatePolicy updates an existing quota policy.
func UpdatePolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid policy id")
		return
	}

	var existing models.QuotaPolicy
	if err := model.DB.First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.ErrorResponse(c, http.StatusNotFound, "policy not found")
			return
		}

		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	var update models.QuotaPolicy
	if err := c.ShouldBindJSON(&update); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	update.ID = id

	if err := model.DB.Model(&existing).Select("*").Omit("id", "created_at", "deleted_at").Updates(&update).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Invalidate caches for all groups using this policy
	invalidatePolicyCaches(id)

	middleware.SuccessResponse(c, update)
}

// DeletePolicy deletes a quota policy by ID.
func DeletePolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid policy id")
		return
	}

	// Find all groups using this policy before deleting
	var bindings []models.GroupQuotaPolicy

	model.DB.Where("quota_policy_id = ?", id).Find(&bindings)

	// Delete associated bindings first
	if len(bindings) > 0 {
		model.DB.Where("quota_policy_id = ?", id).Delete(&models.GroupQuotaPolicy{})

		for _, binding := range bindings {
			_ = InvalidateGroupQuotaPolicy(context.Background(), binding.GroupID)
		}
	}

	if err := model.DB.Delete(&models.QuotaPolicy{}, id).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, nil)
}

type bindRequest struct {
	GroupID       string `json:"group_id"        binding:"required"`
	QuotaPolicyID int    `json:"quota_policy_id" binding:"required"`
}

// BindPolicyToGroup binds a quota policy to a group.
func BindPolicyToGroup(c *gin.Context) {
	var req bindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Verify policy exists
	var policy models.QuotaPolicy
	if err := model.DB.First(&policy, req.QuotaPolicyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.ErrorResponse(c, http.StatusNotFound, "policy not found")
			return
		}

		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	binding := models.GroupQuotaPolicy{
		GroupID:       req.GroupID,
		QuotaPolicyID: req.QuotaPolicyID,
	}

	// Upsert: if binding exists, update; otherwise create
	var existing models.GroupQuotaPolicy

	err := model.DB.Where("group_id = ?", req.GroupID).First(&existing).Error
	if err == nil {
		existing.QuotaPolicyID = req.QuotaPolicyID
		if err := model.DB.Save(&existing).Error; err != nil {
			middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
			return
		}

		binding = existing
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := model.DB.Create(&binding).Error; err != nil {
			middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	_ = InvalidateGroupQuotaPolicy(context.Background(), req.GroupID)

	middleware.SuccessResponse(c, binding)
}

// UnbindPolicyFromGroup removes the quota policy binding for a group.
func UnbindPolicyFromGroup(c *gin.Context) {
	groupID := c.Param("group_id")
	if groupID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group_id is required")
		return
	}

	result := model.DB.Where("group_id = ?", groupID).Delete(&models.GroupQuotaPolicy{})
	if result.Error != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, result.Error.Error())
		return
	}

	if result.RowsAffected == 0 {
		middleware.ErrorResponse(c, http.StatusNotFound, "no policy binding found for this group")
		return
	}

	_ = InvalidateGroupQuotaPolicy(context.Background(), groupID)

	middleware.SuccessResponse(c, nil)
}

// invalidatePolicyCaches invalidates cached quota policies for all groups bound to a given policy.
func invalidatePolicyCaches(policyID int) {
	var bindings []models.GroupQuotaPolicy

	model.DB.Where("quota_policy_id = ?", policyID).Find(&bindings)

	for _, binding := range bindings {
		_ = InvalidateGroupQuotaPolicy(context.Background(), binding.GroupID)
	}
}

func parsePageParams(c *gin.Context) (int, int) {
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = c.Query("p")
	}

	page, _ := strconv.Atoi(pageStr)
	perPage, _ := strconv.Atoi(c.Query("per_page"))

	return page, perPage
}
