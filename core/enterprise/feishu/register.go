//go:build enterprise

package feishu

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// FeishuMiddleware holds permission middleware functions passed from the enterprise package
// to avoid circular imports.
type FeishuMiddleware struct {
	UserManageView   gin.HandlerFunc
	UserManageManage gin.HandlerFunc
	AdminOnly        gin.HandlerFunc
}

// RegisterRoutes registers all Feishu-related routes on the public, admin, and enterpriseAuth groups.
// Any parameter may be nil; only non-nil groups get routes registered.
// mw provides permission middleware (pass nil when enterpriseAuth is nil).
func RegisterRoutes(public, admin, enterpriseAuth *gin.RouterGroup, mw *FeishuMiddleware) {
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

	if enterpriseAuth != nil && mw != nil {
		// Read-only department data — all roles
		enterpriseAuth.GET("/feishu/departments", GetFeishuDepartments)
		enterpriseAuth.GET("/feishu/department-levels", GetDepartmentLevels)
		enterpriseAuth.GET("/feishu/sync-status", GetSyncStatusHandler)

		// User management — view requires user_manage_view
		umView := enterpriseAuth.Group("", mw.UserManageView)
		umView.GET("/feishu/users", GetFeishuUsers)

		// Sync and role update — requires user_manage_manage + admin role
		umManage := enterpriseAuth.Group("", mw.UserManageManage)
		umManage.POST("/feishu/sync", mw.AdminOnly, TriggerSync)
		umManage.PUT("/feishu/users/:open_id/role", mw.AdminOnly, UpdateUserRole)
	}
}

// FeishuUserWithDepartment extends FeishuUser with department path information
type FeishuUserWithDepartment struct {
	models.FeishuUser
	DepartmentPath  *DepartmentPath `json:"department_path"`
	EffectivePolicy *string         `json:"effective_policy,omitempty"`
	PolicySource    *string         `json:"policy_source,omitempty"` // "user" or "department"
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
		op := common.LikeOp()
		tx = tx.Where("name "+op+" ? OR email "+op+" ? OR open_id "+op+" ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// Role filter
	roleFilter := c.Query("role")
	if roleFilter != "" {
		tx = tx.Where("role = ?", roleFilter)
	}

	// Department filters — match by descendant department IDs
	// This works whether or not level1_dept_id is populated on the user record,
	// because it matches the user's department_id against all descendants of the selected dept.
	level1Dept := c.Query("level1_department")
	level2Dept := c.Query("level2_department")

	if level2Dept != "" {
		matchingDepts := GetDescendantDepartmentIDs(level2Dept)
		if len(matchingDepts) > 0 {
			tx = tx.Where("department_id IN ?", matchingDepts)
		}
	} else if level1Dept != "" {
		matchingDepts := GetDescendantDepartmentIDs(level1Dept)
		if len(matchingDepts) > 0 {
			tx = tx.Where("department_id IN ?", matchingDepts)
		}
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

	// Build response with department path
	// When stored level fields are populated, resolve names from department table.
	// When they are empty, fall back to GetDepartmentPath which traverses the parent chain.
	deptNameMap := batchResolveDepartmentNames(users)

	// Batch resolve effective quota policies
	userPolicyMap, deptPolicyMap := batchResolveEffectivePolicies(users)

	usersWithDept := make([]FeishuUserWithDepartment, len(users))
	for i, user := range users {
		var deptPath *DepartmentPath

		if user.Level1DeptID != "" {
			// Use stored level fields with batch-resolved names
			l1Name := resolveDeptName(deptNameMap, user.Level1DeptID, user.Level1DeptName)
			l2Name := resolveDeptName(deptNameMap, user.Level2DeptID, user.Level2DeptName)

			fullPath := user.DeptFullPath
			if l1Name != user.Level1DeptName || l2Name != user.Level2DeptName {
				var parts []string
				if l1Name != "" {
					parts = append(parts, l1Name)
				}

				if l2Name != "" {
					parts = append(parts, l2Name)
				}

				if len(parts) > 0 {
					fullPath = strings.Join(parts, " > ")
				}
			}

			deptPath = &DepartmentPath{
				Level1ID:   user.Level1DeptID,
				Level1Name: l1Name,
				Level2ID:   user.Level2DeptID,
				Level2Name: l2Name,
				FullPath:   fullPath,
			}
		} else if user.DepartmentID != "" {
			// Fallback: resolve department path dynamically from department table
			deptPath = GetDepartmentPath(user.DepartmentID)
		} else {
			deptPath = &DepartmentPath{}
		}

		entry := FeishuUserWithDepartment{
			FeishuUser:     user,
			DepartmentPath: deptPath,
		}

		// Resolve effective policy
		if up, ok := userPolicyMap[user.OpenID]; ok {
			entry.EffectivePolicy = &up
			src := "user"
			entry.PolicySource = &src
		} else {
			// Check department hierarchy: leaf → level2 → level1
			for _, deptID := range []string{user.DepartmentID, user.Level2DeptID, user.Level1DeptID} {
				if deptID == "" {
					continue
				}

				if dp, ok := deptPolicyMap[deptID]; ok {
					entry.EffectivePolicy = &dp
					src := "department"
					entry.PolicySource = &src

					break
				}
			}
		}

		usersWithDept[i] = entry
	}

	middleware.SuccessResponse(c, gin.H{
		"users": usersWithDept,
		"total": total,
	})
}

// batchResolveEffectivePolicies returns two maps:
// 1. openID → policy name (for users with UserQuotaPolicy)
// 2. departmentID → policy name (for departments with DepartmentQuotaPolicy, all ID forms)
func batchResolveEffectivePolicies(users []models.FeishuUser) (map[string]string, map[string]string) {
	userPolicyMap := make(map[string]string)
	deptPolicyMap := make(map[string]string)

	if len(users) == 0 {
		return userPolicyMap, deptPolicyMap
	}

	// Collect all open_ids and department_ids
	openIDs := make([]string, 0, len(users))
	deptIDSet := make(map[string]struct{})

	for _, u := range users {
		openIDs = append(openIDs, u.OpenID)
		for _, dID := range []string{u.DepartmentID, u.Level2DeptID, u.Level1DeptID} {
			if dID != "" {
				deptIDSet[dID] = struct{}{}
			}
		}
	}

	// Batch load user-level policies
	if len(openIDs) > 0 {
		var userPolicies []models.UserQuotaPolicy
		model.DB.Preload("QuotaPolicy").Where("open_id IN ?", openIDs).Find(&userPolicies)

		for _, up := range userPolicies {
			if up.QuotaPolicy != nil {
				userPolicyMap[up.OpenID] = up.QuotaPolicy.Name
			}
		}
	}

	// Batch load department-level policies
	if len(deptIDSet) > 0 {
		deptIDs := make([]string, 0, len(deptIDSet))
		for id := range deptIDSet {
			deptIDs = append(deptIDs, id)
		}

		var deptPolicies []models.DepartmentQuotaPolicy
		model.DB.Preload("QuotaPolicy").Where("department_id IN ?", deptIDs).Find(&deptPolicies)

		for _, dp := range deptPolicies {
			if dp.QuotaPolicy != nil {
				deptPolicyMap[dp.DepartmentID] = dp.QuotaPolicy.Name
			}
		}
	}

	return userPolicyMap, deptPolicyMap
}

// GetDescendantDepartmentIDs returns all department IDs (both department_id and open_department_id)
// that are the given department or its descendants. This ensures matching works regardless of which
// ID format is stored in the user's department_id field.
func GetDescendantDepartmentIDs(departmentID string) []string {
	idSet := make(map[string]struct{})
	visited := make(map[string]bool)

	var collect func(id string)
	collect = func(id string) {
		if visited[id] {
			return
		}

		visited[id] = true

		// Find the department record(s) for this ID
		var depts []models.FeishuDepartment
		model.DB.Where("(department_id = ? OR open_department_id = ?) AND status = 1", id, id).Find(&depts)

		// Collect all ID forms for this department
		parentIDs := []string{id}
		idSet[id] = struct{}{}

		for _, dept := range depts {
			if dept.DepartmentID != "" {
				idSet[dept.DepartmentID] = struct{}{}
				parentIDs = append(parentIDs, dept.DepartmentID)
			}

			if dept.OpenDepartmentID != "" {
				idSet[dept.OpenDepartmentID] = struct{}{}
				parentIDs = append(parentIDs, dept.OpenDepartmentID)
			}
		}

		// Find children whose parent_id matches any known ID form of this department
		var children []models.FeishuDepartment
		model.DB.Where("parent_id IN ? AND status = 1", parentIDs).Find(&children)

		for _, child := range children {
			collect(child.DepartmentID)
		}
	}

	collect(departmentID)

	result := make([]string, 0, len(idSet))
	for id := range idSet {
		result = append(result, id)
	}

	return result
}

// getDepartmentAllIDs returns all possible ID forms for a department
// (both department_id and open_department_id from all matching records).
// This handles the case where the same logical department has multiple DB records.
func getDepartmentAllIDs(deptID string) []string {
	var departments []models.FeishuDepartment
	model.DB.Where("department_id = ? OR open_department_id = ?", deptID, deptID).Find(&departments)

	idSet := make(map[string]struct{})
	for _, d := range departments {
		if d.DepartmentID != "" {
			idSet[d.DepartmentID] = struct{}{}
		}

		if d.OpenDepartmentID != "" {
			idSet[d.OpenDepartmentID] = struct{}{}
		}
	}

	result := make([]string, 0, len(idSet))
	for id := range idSet {
		result = append(result, id)
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
		op := common.LikeOp()
		tx = tx.Where("name "+op+" ? OR department_id "+op+" ?",
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
// Access is controlled by RequirePermission(PermUserManage) + RequireRole(RoleAdmin) middleware.
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

// UpdateUserRole updates the role of a Feishu user.
// Access is controlled by RequirePermission(PermUserManage) + RequireRole(RoleAdmin) middleware.
func UpdateUserRole(c *gin.Context) {
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

// batchResolveDepartmentNames loads department names for all department IDs
// referenced by the given users. Returns a map from any department ID (both
// department_id and open_department_id) to the department's display name.
func batchResolveDepartmentNames(users []models.FeishuUser) map[string]string {
	// Collect all unique department IDs
	idSet := make(map[string]struct{})
	for _, u := range users {
		if u.Level1DeptID != "" {
			idSet[u.Level1DeptID] = struct{}{}
		}

		if u.Level2DeptID != "" {
			idSet[u.Level2DeptID] = struct{}{}
		}

		if u.DepartmentID != "" {
			idSet[u.DepartmentID] = struct{}{}
		}
	}

	if len(idSet) == 0 {
		return nil
	}

	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	// Load all matching departments in one query
	var departments []models.FeishuDepartment
	model.DB.Where("department_id IN ? OR open_department_id IN ?", ids, ids).Find(&departments)

	// Build lookup: any ID form -> best name (prefer non-empty names)
	nameMap := make(map[string]string)
	for _, dept := range departments {
		if dept.Name == "" {
			continue
		}

		if dept.DepartmentID != "" {
			nameMap[dept.DepartmentID] = dept.Name
		}

		if dept.OpenDepartmentID != "" {
			nameMap[dept.OpenDepartmentID] = dept.Name
		}
	}

	return nameMap
}

// resolveDeptName returns the resolved department name:
// first tries the nameMap lookup, then falls back to the stored name.
func resolveDeptName(nameMap map[string]string, deptID, storedName string) string {
	if name, ok := nameMap[deptID]; ok {
		return name
	}

	return storedName
}
