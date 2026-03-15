//go:build enterprise

package feishu

import (
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// RegisterRoutes registers all Feishu-related routes on the public and admin groups.
// Either public or admin may be nil; only non-nil groups get routes registered.
func RegisterRoutes(public, admin *gin.RouterGroup) {
	if public != nil {
		// Public routes (no admin auth required)
		public.GET("/auth/feishu/login", HandleLogin)
		public.GET("/auth/feishu/callback", HandleCallback)
		public.POST("/feishu/webhook", HandleWebhook)
	}

	if admin != nil {
		// Admin routes (require admin auth)
		admin.GET("/feishu/users", GetFeishuUsers)
		admin.GET("/feishu/departments", GetFeishuDepartments)
		admin.POST("/feishu/sync", TriggerSync)
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
func TriggerSync(c *gin.Context) {
	go func() {
		if err := SyncAll(model.DB); err != nil {
			log.Errorf("feishu manual sync failed: %v", err)
		}
	}()

	middleware.SuccessResponse(c, gin.H{
		"message": "sync started",
	})
}
