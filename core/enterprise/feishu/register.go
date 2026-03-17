//go:build enterprise

package feishu

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// RegisterRoutes registers all Feishu-related routes on the public, admin, and enterpriseAuth groups.
// Any parameter may be nil; only non-nil groups get routes registered.
func RegisterRoutes(public, admin, enterpriseAuth *gin.RouterGroup) {
	if public != nil {
		// Public routes (no admin auth required)
		public.GET("/auth/feishu/login", HandleLogin)
		public.GET("/auth/feishu/callback", HandleCallback)
		public.POST("/feishu/webhook", HandleWebhook)
	}

	if admin != nil {
		// Admin routes (require strict admin key auth)
		// Currently empty - moved to enterpriseAuth
	}

	if enterpriseAuth != nil {
		// Enterprise auth routes (AdminKey or Feishu admin user)
		enterpriseAuth.GET("/feishu/users", GetFeishuUsers)
		enterpriseAuth.GET("/feishu/departments", GetFeishuDepartments)
		enterpriseAuth.POST("/feishu/sync", TriggerSync)
		enterpriseAuth.PUT("/feishu/users/:open_id/role", UpdateUserRole)
	}
}

// GetFeishuUsers returns a paginated list of Feishu users.
func GetFeishuUsers(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)

	var users []models.FeishuUser

	var total int64

	tx := model.DB.Model(&models.FeishuUser{})

	keyword := c.Query("keyword")
	if keyword != "" {
		tx = tx.Where("name LIKE ? OR email LIKE ? OR open_id LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := tx.Count(&total).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	if total <= 0 {
		middleware.SuccessResponse(c, gin.H{
			"users": []models.FeishuUser{},
			"total": 0,
		})

		return
	}

	limit := perPage
	if limit <= 0 {
		limit = 20
	}

	offset := (page - 1) * perPage
	if offset < 0 {
		offset = 0
	}

	if err := tx.Order("id desc").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"users": users,
		"total": total,
	})
}

// GetFeishuDepartments returns a paginated list of Feishu departments.
func GetFeishuDepartments(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)

	var departments []models.FeishuDepartment

	var total int64

	tx := model.DB.Model(&models.FeishuDepartment{})

	keyword := c.Query("keyword")
	if keyword != "" {
		tx = tx.Where("name LIKE ? OR department_id LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%")
	}

	if err := tx.Count(&total).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	if total <= 0 {
		middleware.SuccessResponse(c, gin.H{
			"departments": []models.FeishuDepartment{},
			"total":       0,
		})

		return
	}

	limit := perPage
	if limit <= 0 {
		limit = 20
	}

	offset := (page - 1) * perPage
	if offset < 0 {
		offset = 0
	}

	if err := tx.Order("id desc").Limit(limit).Offset(offset).Find(&departments).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"departments": departments,
		"total":       total,
	})
}

// TriggerSync triggers a full Feishu organization sync.
// Only admin role users can trigger sync.
func TriggerSync(c *gin.Context) {
	// Check if user has admin role
	roleVal, exists := c.Get("enterprise_role")
	log.Infof("TriggerSync: role exists=%v, roleVal=%v, roleVal type=%T", exists, roleVal, roleVal)

	if !exists {
		log.Errorf("TriggerSync: role not found in context")
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: only admin users can trigger sync")
		return
	}

	role, ok := roleVal.(string)
	log.Infof("TriggerSync: role cast ok=%v, role=%s, expected=%s", ok, role, models.RoleAdmin)

	if !ok || role != models.RoleAdmin {
		log.Errorf("TriggerSync: role check failed - ok=%v, role=%s, expected=%s", ok, role, models.RoleAdmin)
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: only admin users can trigger sync")
		return
	}

	go func() {
		if err := SyncAll(model.DB); err != nil {
			log.Errorf("feishu manual sync failed: %v", err)
		}
	}()

	middleware.SuccessResponse(c, gin.H{
		"message": "sync started",
	})
}

// UpdateUserRole updates the role of a Feishu user.
// Only admin role users can update user roles.
func UpdateUserRole(c *gin.Context) {
	// Check if user has admin role
	roleVal, exists := c.Get("enterprise_role")
	if !exists {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: only admin users can update user roles")
		return
	}

	role, ok := roleVal.(string)
	if !ok || role != models.RoleAdmin {
		middleware.ErrorResponse(c, http.StatusForbidden, "forbidden: only admin users can update user roles")
		return
	}

	openID := c.Param("open_id")
	if openID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "open_id is required")
		return
	}

	var req struct {
		Role string `json:"role" binding:"required,oneof=viewer analyst admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.FeishuUser
	if err := model.DB.Where("open_id = ?", openID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.ErrorResponse(c, http.StatusNotFound, "user not found")
			return
		}
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	user.Role = req.Role
	if err := model.DB.Save(&user).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, user)
}
