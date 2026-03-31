//go:build enterprise

package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	gosync "sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// syncStats tracks sync statistics for logging
type syncStats struct {
	totalDepts      int
	deptsWithName   int
	totalUsers      int
	usersWithName   int
	usersWithEmail  int
	departedUsers   int // users in DB but no longer in Feishu (deactivated during sync)
	syncedOpenIDs   map[string]struct{} // all open_ids seen from Feishu API during this sync
}

// SyncStatus holds the result of the last Feishu sync operation (API response type).
type SyncStatus struct {
	LastSyncAt     time.Time `json:"last_sync_at"`
	Status         string    `json:"status"`
	TotalDepts     int       `json:"total_depts"`
	DeptsWithName  int       `json:"depts_with_name"`
	TotalUsers     int       `json:"total_users"`
	UsersWithName  int       `json:"users_with_name"`
	UsersWithEmail int       `json:"users_with_email"`
	DepartedUsers  int       `json:"departed_users"`
	DurationMs     int64     `json:"duration_ms"`
	Error          string    `json:"error,omitempty"`
}

var (
	lastSyncStatus SyncStatus
	syncStatusMu   gosync.Mutex
)

// GetSyncStatus returns the current sync status from in-memory cache.
// Falls back to DB on first call (e.g. after service restart).
func GetSyncStatus() SyncStatus {
	syncStatusMu.Lock()
	defer syncStatusMu.Unlock()

	if lastSyncStatus.Status == "" && model.DB != nil {
		// Cold start: load from DB
		var h models.FeishuSyncHistory
		if err := model.DB.Order("id DESC").First(&h).Error; err == nil {
			lastSyncStatus = historyToStatus(&h)
		}
	}

	return lastSyncStatus
}

func setSyncStatus(s SyncStatus) {
	syncStatusMu.Lock()
	defer syncStatusMu.Unlock()

	lastSyncStatus = s
}

// historyToStatus converts a DB record to the API response type.
func historyToStatus(h *models.FeishuSyncHistory) SyncStatus {
	return SyncStatus{
		LastSyncAt:     h.SyncedAt,
		Status:         h.Status,
		TotalDepts:     h.TotalDepts,
		DeptsWithName:  h.DeptsWithName,
		TotalUsers:     h.TotalUsers,
		UsersWithName:  h.UsersWithName,
		UsersWithEmail: h.UsersWithEmail,
		DepartedUsers:  h.DepartedUsers,
		DurationMs:     h.DurationMs,
		Error:          h.Error,
	}
}

// createSyncHistory inserts a new sync history record and returns its ID.
func createSyncHistory(db *gorm.DB, status string) int64 {
	h := models.FeishuSyncHistory{
		Status: status,
	}
	if err := db.Create(&h).Error; err != nil {
		log.Errorf("feishu sync: failed to create sync history: %v", err)
		return 0
	}

	return h.ID
}

// updateSyncHistory updates an existing sync history record with final results.
func updateSyncHistory(db *gorm.DB, id int64, h *models.FeishuSyncHistory) {
	if id == 0 {
		return
	}

	if err := db.Model(&models.FeishuSyncHistory{}).Where("id = ?", id).Updates(h).Error; err != nil {
		log.Errorf("feishu sync: failed to update sync history %d: %v", id, err)
	}
}

// GetSyncStatusHandler returns the current Feishu sync status.
func GetSyncStatusHandler(c *gin.Context) {
	middleware.SuccessResponse(c, GetSyncStatus())
}

// GetSyncHistoryHandler returns paginated sync history records.
func GetSyncHistoryHandler(c *gin.Context) {
	page, perPage := utils.ParsePageParams(c)

	var records []models.FeishuSyncHistory
	var total int64

	tx := model.DB.Model(&models.FeishuSyncHistory{})

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

// feishuEvent is the top-level event payload from Feishu.
type feishuEvent struct {
	Schema    string          `json:"schema"`
	Header    feishuHeader    `json:"header"`
	Event     json.RawMessage `json:"event"`
	Challenge string          `json:"challenge"`
	Token     string          `json:"token"`
	Type      string          `json:"type"`
}

type feishuHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key"`
}

// feishuUserEvent holds the user object inside a user event.
type feishuUserEvent struct {
	Object *feishuUserObject `json:"object"`
}

type feishuUserObject struct {
	OpenID        string           `json:"open_id"`
	UnionID       string           `json:"union_id"`
	UserID        string           `json:"user_id"`
	Name          string           `json:"name"`
	Email         string           `json:"email"`
	Avatar        *feishuAvatarObj `json:"avatar"`
	DepartmentIDs []string         `json:"department_ids"`
}

type feishuAvatarObj struct {
	AvatarOrigin string `json:"avatar_origin"`
}

// feishuDeptEvent holds the department object inside a department event.
type feishuDeptEvent struct {
	Object *feishuDeptObject `json:"object"`
}

type feishuDeptObject struct {
	DepartmentID       string `json:"department_id"`
	OpenDepartmentID   string `json:"open_department_id"`
	ParentDepartmentID string `json:"parent_department_id"`
	Name               string `json:"name"`
	MemberCount        int    `json:"member_count"`
	Order              int    `json:"order"`
}

// HandleWebhook processes Feishu event subscription callbacks.
// It handles URL verification challenge and user/department events.
func HandleWebhook(c *gin.Context) {
	var evt feishuEvent

	if err := c.ShouldBindJSON(&evt); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	// Handle URL verification challenge
	if evt.Type == "url_verification" || evt.Challenge != "" {
		c.JSON(http.StatusOK, gin.H{
			"challenge": evt.Challenge,
		})

		return
	}

	// Process event by type
	eventType := evt.Header.EventType

	tenantKey := evt.Header.TenantKey

	switch eventType {
	case "contact.user.created_v3", "contact.user.updated_v3":
		handleUserEvent(evt.Event, tenantKey)
	case "contact.user.deleted_v3":
		handleUserDeletedEvent(evt.Event)
	case "contact.department.created_v3", "contact.department.updated_v3":
		handleDeptEvent(evt.Event)
	case "contact.department.deleted_v3":
		handleDeptDeletedEvent(evt.Event)
	default:
		log.Infof("feishu webhook: unhandled event type: %s", eventType)
	}

	// Always respond 200 to acknowledge receipt
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func handleUserEvent(data json.RawMessage, tenantKey string) {
	var evt feishuUserEvent
	if err := sonic.Unmarshal(data, &evt); err != nil {
		log.Errorf("feishu webhook: failed to unmarshal user event: %v", err)
		return
	}

	if evt.Object == nil || evt.Object.OpenID == "" {
		log.Warn("feishu webhook: user event missing open_id")
		return
	}

	obj := evt.Object
	groupID := fmt.Sprintf("feishu_%s", obj.OpenID)

	var deptID string
	var deptIDsJSON string

	if len(obj.DepartmentIDs) > 0 {
		deptID = obj.DepartmentIDs[0]

		if encoded, err := sonic.Marshal(obj.DepartmentIDs); err == nil {
			deptIDsJSON = string(encoded)
		}
	}

	var avatar string
	if obj.Avatar != nil {
		avatar = obj.Avatar.AvatarOrigin
	}

	// Compute department hierarchy
	deptPath := GetDepartmentPath(deptID)

	assignFields := models.FeishuUser{
		UnionID:        obj.UnionID,
		UserID:         obj.UserID,
		TenantID:       tenantKey,
		Name:           obj.Name,
		Email:          obj.Email,
		Avatar:         avatar,
		DepartmentID:   deptID,
		DepartmentIDs:  deptIDsJSON,
		Level1DeptID:   deptPath.Level1ID,
		Level1DeptName: deptPath.Level1Name,
		Level2DeptID:   deptPath.Level2ID,
		Level2DeptName: deptPath.Level2Name,
		DeptFullPath:   deptPath.FullPath,
		GroupID:        groupID,
		Status:         1,
	}

	feishuUser := models.FeishuUser{
		OpenID:         obj.OpenID,
		UnionID:        obj.UnionID,
		UserID:         obj.UserID,
		TenantID:       tenantKey,
		Name:           obj.Name,
		Email:          obj.Email,
		Avatar:         avatar,
		DepartmentID:   deptID,
		DepartmentIDs:  deptIDsJSON,
		Level1DeptID:   deptPath.Level1ID,
		Level1DeptName: deptPath.Level1Name,
		Level2DeptID:   deptPath.Level2ID,
		Level2DeptName: deptPath.Level2Name,
		DeptFullPath:   deptPath.FullPath,
		GroupID:        groupID,
		Status:         1,
	}

	result := model.DB.
		Where("open_id = ?", obj.OpenID).
		Assign(assignFields).
		FirstOrCreate(&feishuUser)
	if result.Error != nil {
		log.Errorf("feishu webhook: failed to upsert user %s: %v", obj.OpenID, result.Error)
		return
	}

	// Ensure the group exists
	group := &model.Group{ID: groupID}
	if err := model.OnConflictDoNothing().Create(group).Error; err != nil {
		log.Errorf("feishu webhook: failed to create group for user %s: %v", obj.OpenID, err)
	}
}

func handleUserDeletedEvent(data json.RawMessage) {
	var evt feishuUserEvent
	if err := sonic.Unmarshal(data, &evt); err != nil {
		log.Errorf("feishu webhook: failed to unmarshal user deleted event: %v", err)
		return
	}

	if evt.Object == nil || evt.Object.OpenID == "" {
		log.Warn("feishu webhook: user deleted event missing open_id")
		return
	}

	deactivateFeishuUser(model.DB, evt.Object.OpenID, "webhook")
}

// deactivateFeishuUser disables all tokens in the user's group and soft-deletes the feishu_user record.
// This is the single offboarding path used by both webhook events and full-sync departed detection.
// The `source` parameter is for logging only ("webhook" or "sync").
func deactivateFeishuUser(db *gorm.DB, openID, source string) {
	var feishuUser models.FeishuUser

	err := db.Where("open_id = ?", openID).First(&feishuUser).Error
	if err != nil {
		log.Errorf("feishu %s: user %s not found for deactivation: %v", source, openID, err)
		return
	}

	// Disable ALL tokens in the user's group (not just the auto-created one).
	// This revokes all API access, including keys the user created manually.
	if feishuUser.GroupID != "" {
		disabled, err := model.DisableAllGroupTokens(feishuUser.GroupID)
		if err != nil {
			log.Errorf("feishu %s: failed to disable tokens for group %s: %v", source, feishuUser.GroupID, err)
		} else if disabled > 0 {
			log.Infof("feishu %s: disabled %d token(s) for departed user %s (%s)",
				source, disabled, feishuUser.Name, openID)
		}
	}

	// Soft-delete the feishu user (sets deleted_at, JWT auth will fail on user lookup)
	if err := db.Delete(&feishuUser).Error; err != nil {
		log.Errorf("feishu %s: failed to soft-delete user %s (%s): %v", source, feishuUser.Name, openID, err)
		return
	}

	log.Infof("feishu %s: deactivated user %s (%s), group=%s",
		source, feishuUser.Name, openID, feishuUser.GroupID)
}

func handleDeptEvent(data json.RawMessage) {
	var evt feishuDeptEvent
	if err := sonic.Unmarshal(data, &evt); err != nil {
		log.Errorf("feishu webhook: failed to unmarshal department event: %v", err)
		return
	}

	if evt.Object == nil || evt.Object.DepartmentID == "" {
		log.Warn("feishu webhook: department event missing department_id")
		return
	}

	obj := evt.Object
	dept := models.FeishuDepartment{
		DepartmentID:     obj.DepartmentID,
		OpenDepartmentID: obj.OpenDepartmentID,
		ParentID:         obj.ParentDepartmentID,
		Name:             obj.Name,
		MemberCount:      obj.MemberCount,
		Order:            obj.Order,
		Status:           1,
	}

	result := model.DB.
		Where("department_id = ?", obj.DepartmentID).
		Assign(models.FeishuDepartment{
			OpenDepartmentID: obj.OpenDepartmentID,
			ParentID:         obj.ParentDepartmentID,
			Name:             obj.Name,
			MemberCount:      obj.MemberCount,
			Order:            obj.Order,
			Status:           1,
		}).
		FirstOrCreate(&dept)
	if result.Error != nil {
		log.Errorf("feishu webhook: failed to upsert department %s: %v", obj.DepartmentID, result.Error)
	}
}

func handleDeptDeletedEvent(data json.RawMessage) {
	var evt feishuDeptEvent
	if err := sonic.Unmarshal(data, &evt); err != nil {
		log.Errorf("feishu webhook: failed to unmarshal department deleted event: %v", err)
		return
	}

	if evt.Object == nil || evt.Object.DepartmentID == "" {
		log.Warn("feishu webhook: department deleted event missing department_id")
		return
	}

	model.DB.Where("department_id = ?", evt.Object.DepartmentID).Delete(&models.FeishuDepartment{})
}

// SyncAll performs a full synchronization of all departments and users from Feishu.
func SyncAll(db *gorm.DB) error {
	ctx := context.Background()
	startTime := time.Now()

	stats := &syncStats{
		syncedOpenIDs: make(map[string]struct{}),
	}

	// Persist "syncing" status to DB
	historyID := createSyncHistory(db, "syncing")
	setSyncStatus(SyncStatus{
		LastSyncAt: startTime,
		Status:     "syncing",
	})

	log.Info("feishu sync: starting full organization sync")

	// Get the app's tenant info (tenant_id + name) to populate on all users
	tenantInfo, err := GetTenantInfo(ctx)
	if err != nil {
		log.Warnf("feishu sync: GetTenantInfo API failed: %v", err)
		tenantInfo = &TenantInfo{}
	}

	if tenantInfo.TenantKey != "" {
		log.Infof("feishu sync: resolved tenant via API — id=%s, name=%s", tenantInfo.TenantKey, tenantInfo.Name)
	} else {
		// Fallback: look up tenant_id from existing users who logged in via OAuth
		var existing models.FeishuUser
		if db.Where("tenant_id != '' AND tenant_id IS NOT NULL").First(&existing).Error == nil {
			tenantInfo.TenantKey = existing.TenantID
			log.Infof("feishu sync: resolved tenant via DB fallback — id=%s", tenantInfo.TenantKey)
		} else {
			log.Warn("feishu sync: could not resolve tenant_id from API or DB, tenant_id will be empty")
		}
	}

	// Sync departments recursively starting from root "0"
	// Returns only the department IDs actually fetched from Feishu API
	syncedDeptIDs, err := syncDepartmentsRecursive(ctx, db, "0", stats)
	if err != nil {
		errMsg := fmt.Sprintf("failed to sync departments: %v", err)
		durationMs := time.Since(startTime).Milliseconds()

		failedHistory := &models.FeishuSyncHistory{
			Status:     "failed",
			DurationMs: durationMs,
			Error:      errMsg,
		}
		failedHistory.SyncedAt = startTime
		setSyncStatus(historyToStatus(failedHistory))
		updateSyncHistory(db, historyID, failedHistory)

		return fmt.Errorf("%s", errMsg)
	}

	log.Infof("feishu sync: departments done — total=%d, with_name=%d, missing_name=%d",
		stats.totalDepts, stats.deptsWithName, stats.totalDepts-stats.deptsWithName)

	// Only iterate departments that came from the Feishu API (not mock data in DB)
	for _, deptID := range syncedDeptIDs {
		if err := syncDepartmentUsers(ctx, db, deptID, tenantInfo, stats); err != nil {
			log.Errorf("feishu sync: failed to sync users for department %s: %v", deptID, err)

			continue
		}
	}

	// Also sync root department users
	if err := syncDepartmentUsers(ctx, db, "0", tenantInfo, stats); err != nil {
		log.Errorf("feishu sync: failed to sync root department users: %v", err)
	}

	log.Infof("feishu sync: users done — total=%d, with_name=%d, with_email=%d, missing_name=%d",
		stats.totalUsers, stats.usersWithName, stats.usersWithEmail, stats.totalUsers-stats.usersWithName)

	if stats.totalUsers > 0 && stats.usersWithName == 0 {
		log.Warn("feishu sync: ALL users are missing names — check Feishu app permissions: " +
			"contact:user.base:readonly, contact:user.email:readonly, contact:user.department:readonly")
	}

	if stats.totalDepts > 0 && stats.deptsWithName == 0 {
		log.Warn("feishu sync: ALL departments are missing names — check Feishu app permissions: " +
			"contact:department.base:readonly")
	}

	// Detect departed users: active in DB but no longer returned by Feishu API.
	// Safety guard: only run if we synced a reasonable number of users (>0) to avoid
	// mass-deactivation due to API errors returning empty lists.
	if len(stats.syncedOpenIDs) > 0 {
		deactivateDepartedUsers(db, stats)
	}

	durationMs := time.Since(startTime).Milliseconds()
	log.Infof("feishu sync: full organization sync completed in %dms", durationMs)

	successHistory := &models.FeishuSyncHistory{
		Status:         "success",
		TotalDepts:     stats.totalDepts,
		DeptsWithName:  stats.deptsWithName,
		TotalUsers:     stats.totalUsers,
		UsersWithName:  stats.usersWithName,
		UsersWithEmail: stats.usersWithEmail,
		DepartedUsers:  stats.departedUsers,
		DurationMs:     durationMs,
	}
	successHistory.SyncedAt = startTime
	setSyncStatus(historyToStatus(successHistory))
	updateSyncHistory(db, historyID, successHistory)

	return nil
}

// deactivateDepartedUsers compares all active feishu_users in DB against the open_ids
// returned by the Feishu API during this sync. Users present in DB but absent from
// Feishu are considered departed (resigned/removed) and are deactivated.
//
// Safety: this only runs when syncedOpenIDs is non-empty. If the Feishu API returned
// zero users (e.g., due to permission revocation), this function is skipped entirely
// to avoid mass-deactivation.
func deactivateDepartedUsers(db *gorm.DB, stats *syncStats) {
	// Load all active (non-soft-deleted) feishu_users from DB
	var dbUsers []models.FeishuUser
	if err := db.Select("open_id", "name").Find(&dbUsers).Error; err != nil {
		log.Errorf("feishu sync: failed to load DB users for departed check: %v", err)
		return
	}

	// Find users in DB but not in Feishu API response
	var departedOpenIDs []string
	for _, u := range dbUsers {
		if _, exists := stats.syncedOpenIDs[u.OpenID]; !exists {
			departedOpenIDs = append(departedOpenIDs, u.OpenID)
		}
	}

	if len(departedOpenIDs) == 0 {
		return
	}

	// Safety: if more than 50% of users would be deactivated, something is likely wrong
	// with the Feishu API response. Log a warning and skip to prevent mass-deactivation.
	ratio := float64(len(departedOpenIDs)) / float64(len(dbUsers))
	if ratio > 0.5 && len(departedOpenIDs) > 5 {
		log.Warnf("feishu sync: skipping departed user deactivation — %d/%d (%.0f%%) users would be deactivated, "+
			"likely an API issue. Feishu returned %d users, DB has %d.",
			len(departedOpenIDs), len(dbUsers), ratio*100,
			len(stats.syncedOpenIDs), len(dbUsers))
		return
	}

	for _, openID := range departedOpenIDs {
		deactivateFeishuUser(db, openID, "sync")
		stats.departedUsers++
	}

	log.Infof("feishu sync: deactivated %d departed user(s)", stats.departedUsers)
}

// syncDepartmentsRecursive fetches departments from Feishu API recursively and
// returns the list of department IDs that were actually synced from the API.
func syncDepartmentsRecursive(ctx context.Context, db *gorm.DB, parentID string, stats *syncStats) ([]string, error) {
	departments, err := ListDepartments(ctx, parentID)
	if err != nil {
		return nil, err
	}

	var syncedIDs []string

	for _, dept := range departments {
		stats.totalDepts++

		if dept.Name != "" {
			stats.deptsWithName++
		} else {
			log.Warnf("feishu sync: department %s has empty name (parent=%s)", dept.DepartmentID, dept.ParentID)
		}

		// Find ALL matching records (there may be duplicates from previous syncs
		// that used a different ID format). Merge them into one canonical record.
		var matches []models.FeishuDepartment
		db.Where("department_id = ? OR open_department_id = ? OR (department_id = ? AND department_id != '')",
			dept.DepartmentID, dept.OpenDepartmentID, dept.OpenDepartmentID).
			Find(&matches)

		if len(matches) > 1 {
			// Keep the first match, delete the rest to resolve duplicates
			log.Infof("feishu sync: merging %d duplicate records for department %s (%s)",
				len(matches), dept.DepartmentID, dept.Name)

			for _, dup := range matches[1:] {
				db.Unscoped().Delete(&dup)
			}

			// Re-fetch the kept record
			matches = matches[:1]
		}

		if len(matches) == 1 {
			// Update the existing record
			db.Model(&matches[0]).Updates(map[string]interface{}{
				"department_id":      dept.DepartmentID,
				"open_department_id": dept.OpenDepartmentID,
				"parent_id":          dept.ParentID,
				"name":               dept.Name,
				"member_count":       dept.MemberCount,
				"order":              dept.Order,
				"status":             1,
			})
		} else {
			// No existing record — create new
			record := models.FeishuDepartment{
				DepartmentID:     dept.DepartmentID,
				OpenDepartmentID: dept.OpenDepartmentID,
				ParentID:         dept.ParentID,
				Name:             dept.Name,
				MemberCount:      dept.MemberCount,
				Order:            dept.Order,
				Status:           1,
			}
			if err := db.Create(&record).Error; err != nil {
				log.Errorf("feishu sync: failed to create department %s: %v", dept.DepartmentID, err)
			}
		}

		syncedIDs = append(syncedIDs, dept.DepartmentID)

		// Always recurse into child departments regardless of upsert result
		childIDs, err := syncDepartmentsRecursive(ctx, db, dept.DepartmentID, stats)
		if err != nil {
			log.Errorf("feishu sync: failed to sync children of department %s: %v", dept.DepartmentID, err)
		} else {
			syncedIDs = append(syncedIDs, childIDs...)
		}
	}

	return syncedIDs, nil
}

func syncDepartmentUsers(ctx context.Context, db *gorm.DB, departmentID string, tenantInfo *TenantInfo, stats *syncStats) error {
	users, err := ListDepartmentUsers(ctx, departmentID)
	if err != nil {
		return err
	}

	for _, u := range users {
		if u.OpenID == "" {
			continue
		}

		// Track all open_ids seen from Feishu API for departed user detection
		stats.syncedOpenIDs[u.OpenID] = struct{}{}

		stats.totalUsers++

		if u.Name != "" {
			stats.usersWithName++
		}

		if u.Email != "" {
			stats.usersWithEmail++
		}

		groupID := fmt.Sprintf("feishu_%s", u.OpenID)

		// Use the department from API response; fallback to the department being iterated
		// when Feishu doesn't return department info (insufficient permissions)
		userDeptID := u.DepartmentID
		if userDeptID == "" {
			userDeptID = departmentID
		}

		userDeptIDs := u.DepartmentIDs
		if len(userDeptIDs) == 0 && departmentID != "0" {
			userDeptIDs = []string{departmentID}
		}

		// Serialize all department IDs
		var deptIDsJSON string
		if len(userDeptIDs) > 0 {
			if encoded, err := sonic.Marshal(userDeptIDs); err == nil {
				deptIDsJSON = string(encoded)
			}
		}

		// Compute department hierarchy
		deptPath := GetDepartmentPath(userDeptID)

		assignFields := models.FeishuUser{
			UnionID:        u.UnionID,
			UserID:         u.UserID,
			TenantID:       tenantInfo.TenantKey,
			Name:           u.Name,
			Email:          u.Email,
			Avatar:         u.Avatar,
			DepartmentID:   userDeptID,
			DepartmentIDs:  deptIDsJSON,
			Level1DeptID:   deptPath.Level1ID,
			Level1DeptName: deptPath.Level1Name,
			Level2DeptID:   deptPath.Level2ID,
			Level2DeptName: deptPath.Level2Name,
			DeptFullPath:   deptPath.FullPath,
			GroupID:        groupID,
			Status:         1,
		}

		feishuUser := models.FeishuUser{
			OpenID:         u.OpenID,
			UnionID:        u.UnionID,
			UserID:         u.UserID,
			TenantID:       tenantInfo.TenantKey,
			Name:           u.Name,
			Email:          u.Email,
			Avatar:         u.Avatar,
			DepartmentID:   userDeptID,
			DepartmentIDs:  deptIDsJSON,
			Level1DeptID:   deptPath.Level1ID,
			Level1DeptName: deptPath.Level1Name,
			Level2DeptID:   deptPath.Level2ID,
			Level2DeptName: deptPath.Level2Name,
			DeptFullPath:   deptPath.FullPath,
			GroupID:        groupID,
			Status:         1,
		}

		result := db.
			Where("open_id = ?", u.OpenID).
			Assign(assignFields).
			FirstOrCreate(&feishuUser)
		if result.Error != nil {
			log.Errorf("feishu sync: failed to upsert user %s: %v", u.OpenID, result.Error)

			continue
		}

		// Force-update tenant_id using map (struct Assign skips zero-value fields for existing records)
		if tenantInfo.TenantKey != "" && feishuUser.TenantID != tenantInfo.TenantKey {
			db.Model(&feishuUser).Update("tenant_id", tenantInfo.TenantKey)
		}

		// Ensure group exists
		group := &model.Group{ID: groupID}
		if err := model.OnConflictDoNothing().Create(group).Error; err != nil {
			log.Errorf("feishu sync: failed to create group for user %s: %v", u.OpenID, err)
		}
	}

	return nil
}

// StartSyncScheduler starts a background goroutine that performs a full sync every 6 hours.
// It waits for DB initialization before performing the initial sync.
func StartSyncScheduler(ctx context.Context) {
	go func() {
		// Wait for model.DB to be initialized (max 30 seconds)
		log.Info("feishu sync: waiting for database initialization")
		for i := 0; i < 60; i++ {
			if model.DB != nil {
				break
			}

			time.Sleep(500 * time.Millisecond)
		}

		if model.DB == nil {
			log.Error("feishu sync: database not initialized after 30s, skipping initial sync")
		} else {
			// Perform initial sync on startup
			log.Info("feishu sync: performing initial sync on startup")
			if err := SyncAll(model.DB); err != nil {
				log.Errorf("feishu initial sync failed: %v", err)
			} else {
				log.Info("feishu initial sync completed successfully")
			}
		}

		// Start periodic sync
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("feishu sync scheduler stopped")
				return
			case <-ticker.C:
				if err := SyncAll(model.DB); err != nil {
					log.Errorf("feishu scheduled sync failed: %v", err)
				}
			}
		}
	}()

	log.Info("feishu sync scheduler started (interval: 6h)")
}
