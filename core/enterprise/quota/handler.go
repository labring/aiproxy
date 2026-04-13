//go:build enterprise

package quota

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/enterprise/feishu"
	"github.com/labring/aiproxy/core/enterprise/models"
	enterprisenotify "github.com/labring/aiproxy/core/enterprise/notify"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const maxSyncConcurrency = 10

// PolicyPeriodTypeToTokenPeriodType converts a policy PeriodType int (1/2/3) to
// a token PeriodType string ("daily"/"weekly"/"monthly").
func PolicyPeriodTypeToTokenPeriodType(pt int) string {
	switch pt {
	case models.PeriodTypeDaily:
		return model.PeriodTypeDaily
	case models.PeriodTypeWeekly:
		return model.PeriodTypeWeekly
	case models.PeriodTypeMonthly:
		return model.PeriodTypeMonthly
	default:
		log.Warnf("unknown policy PeriodType %d, falling back to monthly", pt)
		return model.PeriodTypeMonthly
	}
}

// withGroupTokens resolves a FeishuUser's group and calls fn for every enabled token in that group.
// This ensures policy sync covers ALL keys (auto-created + user-created), not just feishu_user.token_id.
func withGroupTokens(openID string, fn func(tokenID int)) {
	var user models.FeishuUser
	if err := model.DB.Where("open_id = ?", openID).First(&user).Error; err != nil {
		return
	}

	if user.GroupID == "" {
		return
	}

	var tokenIDs []int
	if err := model.DB.Model(&model.Token{}).
		Select("id").
		Where("group_id = ? AND status = ?", user.GroupID, model.TokenStatusEnabled).
		Find(&tokenIDs).Error; err != nil {
		log.Errorf("withGroupTokens: failed to list tokens for group %s: %v", user.GroupID, err)
		return
	}

	for _, id := range tokenIDs {
		fn(id)
	}
}

// syncPolicyToToken updates ALL tokens in the user's group with PeriodQuota/PeriodType from the given policy.
// When BlockAtTier3 is false, Token PeriodQuota is set to 0 so the token-level hard check
// does not block requests — the enterprise tier system (CheckQuotaTier) handles graduated
// throttling instead. When BlockAtTier3 is true, PeriodQuota is synced normally as a
// defence-in-depth hard limit.
// When PeriodType changes, it proactively snapshots UsedAmount to PeriodLastUpdateAmount
// so the new period starts fresh (instead of waiting for lazy reset on next relay request).
func syncPolicyToToken(openID string, policy *models.QuotaPolicy) {
	newPeriodType := PolicyPeriodTypeToTokenPeriodType(policy.PeriodType)

	withGroupTokens(openID, func(tokenID int) {
		periodQuota := float64(0)
		if policy.BlockAtTier3 {
			periodQuota = policy.PeriodQuota
		}
		req := model.UpdateTokenRequest{
			PeriodQuota: &periodQuota,
			PeriodType:  &newPeriodType,
		}

		// Detect PeriodType change: snapshot usage for clean period boundary
		currentToken, err := model.GetTokenByID(tokenID)
		if err == nil && currentToken.PeriodType != model.EmptyNullString(newPeriodType) {
			now := time.Now().UnixMilli()
			req.PeriodLastUpdateTime = &now
			req.PeriodLastUpdateAmount = &currentToken.UsedAmount
		}

		if _, err := model.UpdateToken(tokenID, req); err != nil {
			log.Errorf("sync policy to token for user %s (token %d): %v", openID, tokenID, err)
		}
	})
}

// clearUserToken resets PeriodQuota to 0 for ALL tokens in the user's group.
func clearUserToken(openID string) {
	withGroupTokens(openID, func(tokenID int) {
		zero := float64(0)
		if _, err := model.UpdateToken(tokenID, model.UpdateTokenRequest{
			PeriodQuota: &zero,
		}); err != nil {
			log.Errorf("clear token quota for user %s (token %d): %v", openID, tokenID, err)
		}
	})
}

// runBounded executes fn for each item with bounded concurrency.
func runBounded(items []string, fn func(string)) {
	sem := make(chan struct{}, maxSyncConcurrency)
	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(id)
		}(item)
	}
	wg.Wait()
}

// syncPolicyToTokenBatch syncs token quota for multiple users with bounded concurrency.
func syncPolicyToTokenBatch(openIDs []string, policy *models.QuotaPolicy) {
	runBounded(openIDs, func(id string) { syncPolicyToToken(id, policy) })
}

// clearUserTokenBatch clears token quota for multiple users with bounded concurrency.
func clearUserTokenBatch(openIDs []string) {
	runBounded(openIDs, clearUserToken)
}

// getDepartmentUserIDsWithoutOverride returns OpenIDs of all users in a department (and descendants)
// that do not have personal UserQuotaPolicy bindings.
func getDepartmentUserIDsWithoutOverride(departmentID string) []string {
	descendantIDs := feishu.GetDescendantDepartmentIDs(departmentID)
	if len(descendantIDs) == 0 {
		return nil
	}

	var users []models.FeishuUser
	model.DB.Where("department_id IN ? OR level1_dept_id IN ? OR level2_dept_id IN ?",
		descendantIDs, descendantIDs, descendantIDs).Find(&users)

	allOpenIDs := make([]string, 0, len(users))
	for _, u := range users {
		allOpenIDs = append(allOpenIDs, u.OpenID)
	}

	userOverrides := make(map[string]bool)
	if len(allOpenIDs) > 0 {
		var overrides []models.UserQuotaPolicy
		model.DB.Where("open_id IN ?", allOpenIDs).Find(&overrides)

		for _, o := range overrides {
			userOverrides[o.OpenID] = true
		}
	}

	result := make([]string, 0, len(users))
	for _, u := range users {
		if !userOverrides[u.OpenID] {
			result = append(result, u.OpenID)
		}
	}

	return result
}

// syncPolicyToDepartmentUsers syncs Token PeriodQuota for all users in a department (and descendants).
// Users with personal (UserQuotaPolicy) bindings are skipped.
func syncPolicyToDepartmentUsers(departmentID string, policy *models.QuotaPolicy) {
	openIDs := getDepartmentUserIDsWithoutOverride(departmentID)
	if len(openIDs) > 0 {
		syncPolicyToTokenBatch(openIDs, policy)
	}
}

// ListPolicies returns all quota policies with pagination.
func ListPolicies(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)

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

	// Collect group bindings before deletion (needed for cache invalidation)
	var groupBindings []models.GroupQuotaPolicy
	model.DB.Where("quota_policy_id = ?", id).Find(&groupBindings)

	// Cascade-delete all bindings + the policy itself in one transaction
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("quota_policy_id = ?", id).Delete(&models.GroupQuotaPolicy{}).Error; err != nil {
			return err
		}
		if err := tx.Where("quota_policy_id = ?", id).Delete(&models.UserQuotaPolicy{}).Error; err != nil {
			return err
		}
		if err := tx.Where("quota_policy_id = ?", id).Delete(&models.DepartmentQuotaPolicy{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.QuotaPolicy{}, id).Error
	}); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	for _, binding := range groupBindings {
		_ = InvalidateGroupQuotaPolicy(context.Background(), binding.GroupID)
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

// bindDepartmentPolicyCore is the shared logic for binding a policy to a department.
// Uses Unscoped to find soft-deleted records and restore them, avoiding unique index conflicts.
func bindDepartmentPolicyCore(departmentID string, quotaPolicyID int) (*models.DepartmentQuotaPolicy, *models.QuotaPolicy, error) {
	var policy models.QuotaPolicy
	if err := model.DB.First(&policy, quotaPolicyID).Error; err != nil {
		return nil, nil, err
	}

	var existing models.DepartmentQuotaPolicy
	err := model.DB.Unscoped().Where("department_id = ?", departmentID).First(&existing).Error
	if err == nil {
		existing.QuotaPolicyID = quotaPolicyID
		existing.DeletedAt = gorm.DeletedAt{}
		if err := model.DB.Unscoped().Save(&existing).Error; err != nil {
			return nil, nil, err
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	} else {
		existing = models.DepartmentQuotaPolicy{
			DepartmentID:  departmentID,
			QuotaPolicyID: quotaPolicyID,
		}
		if err := model.DB.Create(&existing).Error; err != nil {
			return nil, nil, err
		}
	}

	if policy.PeriodQuota > 0 {
		go syncPolicyToDepartmentUsers(departmentID, &policy)
	}

	// Notify affected department users about policy change
	go notifyPolicyChangeDepartment(departmentID, &policy)

	return &existing, &policy, nil
}

// BindPolicyToDepartment binds a quota policy to a department.
func BindPolicyToDepartment(c *gin.Context) {
	var req struct {
		DepartmentID  string `json:"department_id"  binding:"required"`
		QuotaPolicyID int    `json:"quota_policy_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	binding, _, err := bindDepartmentPolicyCore(req.DepartmentID, req.QuotaPolicyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.ErrorResponse(c, http.StatusNotFound, "policy not found")
			return
		}

		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	middleware.SuccessResponse(c, binding)
}

// bindUserPolicyCore is the shared logic for binding a policy to a user (upsert).
// It does NOT spawn goroutines for token sync — callers handle that.
// Uses Unscoped to find soft-deleted records and restore them, avoiding unique index conflicts.
func bindUserPolicyCore(openID string, policy *models.QuotaPolicy) (*models.UserQuotaPolicy, error) {
	var existing models.UserQuotaPolicy
	err := model.DB.Unscoped().Where("open_id = ?", openID).First(&existing).Error
	if err == nil {
		existing.QuotaPolicyID = policy.ID
		existing.DeletedAt = gorm.DeletedAt{}
		if err := model.DB.Unscoped().Save(&existing).Error; err != nil {
			return nil, err
		}

		return &existing, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	binding := models.UserQuotaPolicy{
		OpenID:        openID,
		QuotaPolicyID: policy.ID,
	}
	if err := model.DB.Create(&binding).Error; err != nil {
		return nil, err
	}

	return &binding, nil
}

// BindPolicyToUser binds a quota policy to a specific user (overrides department policy).
func BindPolicyToUser(c *gin.Context) {
	var req struct {
		OpenID        string `json:"open_id"        binding:"required"`
		QuotaPolicyID int    `json:"quota_policy_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	var policy models.QuotaPolicy
	if err := model.DB.First(&policy, req.QuotaPolicyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.ErrorResponse(c, http.StatusNotFound, "policy not found")
			return
		}

		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	binding, err := bindUserPolicyCore(req.OpenID, &policy)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	if policy.PeriodQuota > 0 {
		syncPolicyToToken(req.OpenID, &policy)
	}

	go notifyPolicyChangeForUser(req.OpenID, &policy)

	middleware.SuccessResponse(c, binding)
}

// UnbindPolicyFromDepartment removes the quota policy binding for a department.
// Users without personal overrides have their Token PeriodQuota cleared.
func UnbindPolicyFromDepartment(c *gin.Context) {
	deptID := c.Param("department_id")
	if deptID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "department_id is required")
		return
	}

	result := model.DB.Where("department_id = ?", deptID).Delete(&models.DepartmentQuotaPolicy{})
	if result.Error != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, result.Error.Error())
		return
	}

	if result.RowsAffected == 0 {
		middleware.ErrorResponse(c, http.StatusNotFound, "no policy binding found")
		return
	}

	// Clear Token PeriodQuota for users without personal override
	go func() {
		openIDs := getDepartmentUserIDsWithoutOverride(deptID)
		if len(openIDs) > 0 {
			clearUserTokenBatch(openIDs)
		}
	}()

	middleware.SuccessResponse(c, nil)
}

// UnbindPolicyFromUser removes the quota policy binding for a user.
// Falls back to department policy if available, otherwise clears Token PeriodQuota.
func UnbindPolicyFromUser(c *gin.Context) {
	openID := c.Param("open_id")
	if openID == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "open_id is required")
		return
	}

	result := model.DB.Where("open_id = ?", openID).Delete(&models.UserQuotaPolicy{})
	if result.Error != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, result.Error.Error())
		return
	}

	if result.RowsAffected == 0 {
		middleware.ErrorResponse(c, http.StatusNotFound, "no policy binding found")
		return
	}

	// Try falling back to department policy, otherwise clear
	policy, err := GetPolicyForUser(c.Request.Context(), openID)
	if err == nil && policy != nil && policy.PeriodQuota > 0 {
		syncPolicyToToken(openID, policy)
	} else {
		clearUserToken(openID)
	}

	middleware.SuccessResponse(c, nil)
}

// BatchBindPolicyToDepartments binds a quota policy to multiple departments at once.
func BatchBindPolicyToDepartments(c *gin.Context) {
	var req struct {
		DepartmentIDs []string `json:"department_ids" binding:"required"`
		QuotaPolicyID int      `json:"quota_policy_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.DepartmentIDs) == 0 {
		middleware.ErrorResponse(c, http.StatusBadRequest, "department_ids is required")
		return
	}

	var results []models.DepartmentQuotaPolicy

	var errs []string

	for _, deptID := range req.DepartmentIDs {
		binding, _, err := bindDepartmentPolicyCore(deptID, req.QuotaPolicyID)
		if err != nil {
			errs = append(errs, deptID+": "+err.Error())

			continue
		}

		results = append(results, *binding)
	}

	if len(errs) > 0 && len(results) == 0 {
		middleware.ErrorResponse(c, http.StatusInternalServerError, strings.Join(errs, "; "))
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"bindings": results,
		"errors":   errs,
	})
}

// BatchBindPolicyToUsers binds a quota policy to multiple users at once.
func BatchBindPolicyToUsers(c *gin.Context) {
	var req struct {
		OpenIDs       []string `json:"open_ids"        binding:"required"`
		QuotaPolicyID int      `json:"quota_policy_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.OpenIDs) == 0 {
		middleware.ErrorResponse(c, http.StatusBadRequest, "open_ids is required")
		return
	}

	var policy models.QuotaPolicy
	if err := model.DB.First(&policy, req.QuotaPolicyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.ErrorResponse(c, http.StatusNotFound, "policy not found")
			return
		}

		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())

		return
	}

	var results []models.UserQuotaPolicy

	var errs []string

	var syncOpenIDs []string

	for _, openID := range req.OpenIDs {
		binding, err := bindUserPolicyCore(openID, &policy)
		if err != nil {
			errs = append(errs, openID+": "+err.Error())

			continue
		}

		results = append(results, *binding)
		syncOpenIDs = append(syncOpenIDs, openID)
	}

	// Batch sync with bounded concurrency
	if policy.PeriodQuota > 0 && len(syncOpenIDs) > 0 {
		go syncPolicyToTokenBatch(syncOpenIDs, &policy)
	}

	// Clear dedup keys and send policy change notifications for all bound users
	if len(syncOpenIDs) > 0 {
		go notifyPolicyChangeBatch(syncOpenIDs, &policy)
	}

	if len(errs) > 0 && len(results) == 0 {
		middleware.ErrorResponse(c, http.StatusInternalServerError, strings.Join(errs, "; "))
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"bindings": results,
		"errors":   errs,
	})
}

// DepartmentBindingDetail extends DepartmentQuotaPolicy with display info.
type DepartmentBindingDetail struct {
	models.DepartmentQuotaPolicy
	Level1Name    string `json:"level1_name"`
	Level2Name    string `json:"level2_name"`
	MemberCount   int    `json:"member_count"`
	OverrideCount int    `json:"override_count"`
}

// resolveDepartmentLevels walks up the parent chain from deptID to the root
// using the preloaded deptMap, then returns (level1_name, level2_name).
// If the dept is level1 itself, level2_name is empty.
// If the dept is level3+, level2_name shows the dept's own name and level1_name shows the root ancestor.
func resolveDepartmentLevels(deptID string, deptMap map[string]*models.FeishuDepartment) (level1Name, level2Name string) {
	dept := deptMap[deptID]
	if dept == nil {
		return "", ""
	}

	// Walk up to collect ancestor chain: [self, parent, grandparent, ..., root]
	chain := []*models.FeishuDepartment{dept}
	cur := dept
	for i := 0; i < 10; i++ { // safety limit
		if cur.ParentID == "" || cur.ParentID == "0" {
			break
		}
		parent := deptMap[cur.ParentID]
		if parent == nil {
			break
		}
		chain = append(chain, parent)
		cur = parent
	}

	// chain[len-1] is the root (level1), chain[0] is self
	switch len(chain) {
	case 1:
		// self is level1
		return chain[0].Name, ""
	default:
		// root is level1, self (or nearest child) is level2
		return chain[len(chain)-1].Name, chain[0].Name
	}
}

// getDescendantIDsFromMap computes all descendant department IDs (including self and all ID forms)
// from an in-memory department map, avoiding recursive DB queries.
func getDescendantIDsFromMap(deptID string, deptMap map[string]*models.FeishuDepartment) map[string]bool {
	// Deduplicate departments by DB ID
	seen := make(map[int]bool)
	var depts []*models.FeishuDepartment
	for _, d := range deptMap {
		if !seen[d.ID] {
			seen[d.ID] = true
			depts = append(depts, d)
		}
	}

	// Build parent_id → children index
	childrenOf := make(map[string][]*models.FeishuDepartment)
	for _, d := range depts {
		if d.ParentID != "" && d.ParentID != "0" {
			childrenOf[d.ParentID] = append(childrenOf[d.ParentID], d)
		}
	}

	result := make(map[string]bool)

	// BFS: queue holds department IDs to process
	queue := []string{deptID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if result[id] {
			continue
		}

		result[id] = true

		// Collect all ID forms for this dept
		d := deptMap[id]
		if d == nil {
			continue
		}

		allForms := []string{d.DepartmentID, d.OpenDepartmentID}
		for _, form := range allForms {
			if form != "" {
				result[form] = true
			}
		}

		// Enqueue children keyed by any known ID form
		for _, form := range allForms {
			if form == "" {
				continue
			}

			for _, child := range childrenOf[form] {
				if !result[child.DepartmentID] {
					queue = append(queue, child.DepartmentID)
				}
			}
		}
	}

	return result
}

// buildDepartmentLookup loads all active departments into a lookup map keyed by
// both department_id and open_department_id for O(1) access.
func buildDepartmentLookup() map[string]*models.FeishuDepartment {
	var allDepts []models.FeishuDepartment
	model.DB.Where("status = 1").Find(&allDepts)

	m := make(map[string]*models.FeishuDepartment, len(allDepts)*2)
	for i := range allDepts {
		d := &allDepts[i]
		if d.DepartmentID != "" {
			m[d.DepartmentID] = d
		}
		if d.OpenDepartmentID != "" {
			m[d.OpenDepartmentID] = d
		}
	}

	return m
}

// ListDepartmentPolicyBindings returns all department-policy bindings, optionally filtered by policy_id.
func ListDepartmentPolicyBindings(c *gin.Context) {
	tx := model.DB.Preload("QuotaPolicy").Model(&models.DepartmentQuotaPolicy{})

	policyIDStr := c.Query("policy_id")
	if policyIDStr != "" {
		policyID, err := strconv.Atoi(policyIDStr)
		if err == nil {
			tx = tx.Where("quota_policy_id = ?", policyID)
		}
	}

	var bindings []models.DepartmentQuotaPolicy
	if err := tx.Find(&bindings).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	if len(bindings) == 0 {
		middleware.SuccessResponse(c, gin.H{"bindings": []DepartmentBindingDetail{}, "total": 0})
		return
	}

	// Batch load: all departments (for name resolution) + all users + all user overrides
	deptMap := buildDepartmentLookup()

	var allUsers []models.FeishuUser
	model.DB.Find(&allUsers)

	var allOverrides []models.UserQuotaPolicy
	model.DB.Find(&allOverrides)
	overrideSet := make(map[string]bool, len(allOverrides))
	for _, o := range allOverrides {
		overrideSet[o.OpenID] = true
	}

	details := make([]DepartmentBindingDetail, 0, len(bindings))

	for _, b := range bindings {
		detail := DepartmentBindingDetail{DepartmentQuotaPolicy: b}

		detail.Level1Name, detail.Level2Name = resolveDepartmentLevels(b.DepartmentID, deptMap)

		// Compute descendants from preloaded deptMap (no additional DB queries)
		descendantSet := getDescendantIDsFromMap(b.DepartmentID, deptMap)
		for _, u := range allUsers {
			if descendantSet[u.DepartmentID] || descendantSet[u.Level1DeptID] || descendantSet[u.Level2DeptID] {
				detail.MemberCount++
				if overrideSet[u.OpenID] {
					detail.OverrideCount++
				}
			}
		}

		details = append(details, detail)
	}

	middleware.SuccessResponse(c, gin.H{
		"bindings": details,
		"total":    len(details),
	})
}

// GetNotifConfigHandler returns the current quota notification configuration
// along with whether the Feishu P2P client is available.
// GET /enterprise/quota/notif-config
func GetNotifConfigHandler(c *gin.Context) {
	n := enterprisenotify.GetEnterpriseNotifier()
	resp := NotifConfigResponse{
		NotifConfig:  GetNotifConfig(),
		P2PAvailable: n != nil && n.IsP2PAvailable(),
	}
	middleware.SuccessResponse(c, resp)
}

// UpdateNotifConfigHandler saves the quota notification configuration.
// PUT /enterprise/quota/notif-config
func UpdateNotifConfigHandler(c *gin.Context) {
	var cfg NotifConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := SetNotifConfig(cfg); err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, cfg)
}

// UserBindingDetail extends UserQuotaPolicy with display info.
type UserBindingDetail struct {
	models.UserQuotaPolicy
	UserName string `json:"user_name"`
}

// ListUserPolicyBindings returns all user-policy bindings, optionally filtered by policy_id.
func ListUserPolicyBindings(c *gin.Context) {
	tx := model.DB.Preload("QuotaPolicy").Model(&models.UserQuotaPolicy{})

	policyIDStr := c.Query("policy_id")
	if policyIDStr != "" {
		policyID, err := strconv.Atoi(policyIDStr)
		if err == nil {
			tx = tx.Where("quota_policy_id = ?", policyID)
		}
	}

	var bindings []models.UserQuotaPolicy
	if err := tx.Find(&bindings).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Batch load user names
	openIDs := make([]string, 0, len(bindings))
	for _, b := range bindings {
		openIDs = append(openIDs, b.OpenID)
	}

	userNameMap := make(map[string]string)
	if len(openIDs) > 0 {
		var users []models.FeishuUser
		model.DB.Where("open_id IN ?", openIDs).Find(&users)

		for _, u := range users {
			userNameMap[u.OpenID] = u.Name
		}
	}

	details := make([]UserBindingDetail, 0, len(bindings))
	for _, b := range bindings {
		details = append(details, UserBindingDetail{
			UserQuotaPolicy: b,
			UserName:        userNameMap[b.OpenID],
		})
	}

	middleware.SuccessResponse(c, gin.H{
		"bindings": details,
		"total":    len(details),
	})
}

// ListAlertHistory returns paginated quota alert notification records.
// GET /enterprise/quota/alert-history
func ListAlertHistory(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)

	var records []models.QuotaAlertHistory
	var total int64

	tx := model.DB.Model(&models.QuotaAlertHistory{})

	// Optional filters
	if openID := c.Query("open_id"); openID != "" {
		tx = tx.Where("open_id = ?", openID)
	}

	if keyword := c.Query("keyword"); keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("user_name "+common.LikeOp()+" ? OR open_id "+common.LikeOp()+" ?", like, like)
	}

	if status := c.Query("status"); status != "" {
		tx = tx.Where("status = ?", status)
	}

	if tierStr := c.Query("tier"); tierStr != "" {
		if tier, err := strconv.Atoi(tierStr); err == nil {
			tx = tx.Where("tier = ?", tier)
		}
	}

	if periodType := c.Query("period_type"); periodType != "" {
		tx = tx.Where("period_type = ?", periodType)
	}

	if startTime := c.Query("start_time"); startTime != "" {
		if ms, err := strconv.ParseInt(startTime, 10, 64); err == nil {
			tx = tx.Where("created_at >= ?", time.UnixMilli(ms))
		}
	}

	if endTime := c.Query("end_time"); endTime != "" {
		if ms, err := strconv.ParseInt(endTime, 10, 64); err == nil {
			tx = tx.Where("created_at <= ?", time.UnixMilli(ms))
		}
	}

	if err := tx.Count(&total).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
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

	if err := tx.Order("id DESC").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.SuccessResponse(c, gin.H{
		"records": records,
		"total":   total,
	})
}

// notifyPolicyChangeForUser looks up the Feishu user name and sends a policy change notification.
func notifyPolicyChangeForUser(openID string, policy *models.QuotaPolicy) {
	periodType := PolicyPeriodTypeToTokenPeriodType(policy.PeriodType)
	ClearUserNotifDedup(openID, periodType)

	var user models.FeishuUser
	if err := model.DB.Where("open_id = ?", openID).First(&user).Error; err != nil {
		NotifyPolicyChange(openID, "", policy)

		return
	}

	NotifyPolicyChange(openID, user.Name, policy)
}

// notifyPolicyChangeBatch sends policy change notifications to multiple users with bounded concurrency.
func notifyPolicyChangeBatch(openIDs []string, policy *models.QuotaPolicy) {
	runBounded(openIDs, func(openID string) {
		notifyPolicyChangeForUser(openID, policy)
	})
}

// notifyPolicyChangeDepartment sends policy change notifications to all users in a department
// that do not have personal overrides.
func notifyPolicyChangeDepartment(departmentID string, policy *models.QuotaPolicy) {
	openIDs := getDepartmentUserIDsWithoutOverride(departmentID)
	if len(openIDs) > 0 {
		notifyPolicyChangeBatch(openIDs, policy)
	}
}
