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
		// //go:build enterprise
		enterpriseAuth.GET("/feishu/users", GetFeishuUsers)
		enterpriseAuth.GET("/feishu/departments", GetFeishuDepartments)
		enterpriseAuth.GET("/feishu/department-levels", GetDepartmentLevels)
		enterpriseAuth.GET("/feishu/sync-status", GetSyncStatusHandler)
		enterpriseAuth.POST("/feishu/sync", TriggerSync)
		enterpriseAuth.PUT("/feishu/users/:open_id/role", UpdateUserRole)
	}
}

// FeishuUserWithDepartment extends FeishuUser with department path information
type FeishuUserWithDepartment struct {
	models.FeishuUser
	DepartmentPath *DepartmentPath `json:"department_path"`
}

// GetFeishuUsers returns a paginated list of Feishu users with department information.
func GetFeishuUsers(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)

	var users []models.FeishuUser

	var total int64

	tx := model.DB.Model(&models.FeishuUser{})

	// Keyword search
	keyword := c.Query("keyword")
	if keyword != "" {
		tx = tx.Where("name LIKE ? OR email LIKE ? OR open_id LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// Department filters — use stored level fields for faster filtering
	level1Dept := c.Query("level1_department")
	level2Dept := c.Query("level2_department")

	if level2Dept != "" {
		// Filter by level 2 department (and all its children)
		matchingDepts := getDescendantDepartmentIDs(level2Dept)
		if len(matchingDepts) > 0 {
			tx = tx.Where("department_id IN ?", matchingDepts)
		}
	} else if level1Dept != "" {
		// Use stored level1_dept_id for indexed lookup
		tx = tx.Where("level1_dept_id = ?", level1Dept)
	}

	if err := tx.Count(&total).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	if total <= 0 {
		middleware.SuccessResponse(c, gin.H{
			"users": []FeishuUserWithDepartment{},
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

	// Support sorting
	sortBy := c.Query("sort_by")
	order := c.Query("order")
	if sortBy == "" {
		sortBy = "id"
	}

	if order == "" {
		order = "desc"
	}

	// Validate sort_by field to prevent SQL injection
	validSortFields := map[string]bool{
		"id":              true,
		"name":            true,
		"role":            true,
		"department_id":   true,
		"level1_dept_name": true,
		"level2_dept_name": true,
		"group_id":        true,
		"created_at":      true,
		"email":           true,
	}
	if !validSortFields[sortBy] {
		sortBy = "id"
	}

	// Validate order
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	orderClause := sortBy + " " + order
	if err := tx.Order(orderClause).Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Build response with department path from stored fields
	usersWithDept := make([]FeishuUserWithDepartment, len(users))
	for i, user := range users {
		usersWithDept[i] = FeishuUserWithDepartment{
			FeishuUser: user,
			DepartmentPath: &DepartmentPath{
				Level1ID:   user.Level1DeptID,
				Level1Name: user.Level1DeptName,
				Level2ID:   user.Level2DeptID,
				Level2Name: user.Level2DeptName,
				FullPath:   user.DeptFullPath,
			},
		}
	}

	middleware.SuccessResponse(c, gin.H{
		"users": usersWithDept,
		"total": total,
	})
}

// getDescendantDepartmentIDs returns all department IDs that are descendants of the given department
func getDescendantDepartmentIDs(departmentID string) []string {
	var result []string
	result = append(result, departmentID)

	var children []models.FeishuDepartment
	model.DB.Where("parent_id = ? AND status = 1", departmentID).Find(&children)

	for _, child := range children {
		result = append(result, getDescendantDepartmentIDs(child.DepartmentID)...)
	}

	return result
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

// GetDepartmentLevels returns departments grouped by level for filtering
func GetDepartmentLevels(c *gin.Context) {
	level1Depts, err := GetLevel1Departments()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	level1ID := c.Query("level1_id")

	var level2Depts []*models.FeishuDepartment
	if level1ID != "" {
		level2Depts, err = GetLevel2Departments(level1ID)
		if err != nil {
			middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	middleware.SuccessResponse(c, gin.H{
		"level1_departments": level1Depts,
		"level2_departments": level2Depts,
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
