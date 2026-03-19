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

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

// syncStats tracks sync statistics for logging
type syncStats struct {
	totalDepts     int
	deptsWithName  int
	totalUsers     int
	usersWithName  int
	usersWithEmail int
}

// SyncStatus holds the result of the last Feishu sync operation.
type SyncStatus struct {
	LastSyncAt     time.Time `json:"last_sync_at"`
	Status         string    `json:"status"`
	TotalDepts     int       `json:"total_depts"`
	DeptsWithName  int       `json:"depts_with_name"`
	TotalUsers     int       `json:"total_users"`
	UsersWithName  int       `json:"users_with_name"`
	UsersWithEmail int       `json:"users_with_email"`
	Error          string    `json:"error,omitempty"`
}

var (
	lastSyncStatus SyncStatus
	syncStatusMu   gosync.Mutex
)

// GetSyncStatus returns the current sync status.
func GetSyncStatus() SyncStatus {
	syncStatusMu.Lock()
	defer syncStatusMu.Unlock()

	return lastSyncStatus
}

func setSyncStatus(s SyncStatus) {
	syncStatusMu.Lock()
	defer syncStatusMu.Unlock()

	lastSyncStatus = s
}

// GetSyncStatusHandler returns the current Feishu sync status.
func GetSyncStatusHandler(c *gin.Context) {
	middleware.SuccessResponse(c, GetSyncStatus())
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

	switch eventType {
	case "contact.user.created_v3", "contact.user.updated_v3":
		handleUserEvent(evt.Event)
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

func handleUserEvent(data json.RawMessage) {
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

	// Soft-delete the feishu user and disable the token
	var feishuUser models.FeishuUser

	err := model.DB.Where("open_id = ?", evt.Object.OpenID).First(&feishuUser).Error
	if err != nil {
		log.Errorf("feishu webhook: user %s not found for deletion: %v", evt.Object.OpenID, err)
		return
	}

	// Disable the associated token
	if feishuUser.TokenID > 0 {
		if err := model.UpdateTokenStatus(feishuUser.TokenID, model.TokenStatusDisabled); err != nil {
			log.Errorf("feishu webhook: failed to disable token %d: %v", feishuUser.TokenID, err)
		}
	}

	// Soft-delete the feishu user
	model.DB.Delete(&feishuUser)
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
	stats := &syncStats{}

	setSyncStatus(SyncStatus{
		LastSyncAt: time.Now(),
		Status:     "syncing",
	})

	log.Info("feishu sync: starting full organization sync")

	// Sync departments recursively starting from root "0"
	// Returns only the department IDs actually fetched from Feishu API
	syncedDeptIDs, err := syncDepartmentsRecursive(ctx, db, "0", stats)
	if err != nil {
		errMsg := fmt.Sprintf("failed to sync departments: %v", err)
		setSyncStatus(SyncStatus{
			LastSyncAt: time.Now(),
			Status:     "failed",
			Error:      errMsg,
		})

		return fmt.Errorf("%s", errMsg)
	}

	log.Infof("feishu sync: departments done — total=%d, with_name=%d, missing_name=%d",
		stats.totalDepts, stats.deptsWithName, stats.totalDepts-stats.deptsWithName)

	// Only iterate departments that came from the Feishu API (not mock data in DB)
	for _, deptID := range syncedDeptIDs {
		if err := syncDepartmentUsers(ctx, db, deptID, stats); err != nil {
			log.Errorf("feishu sync: failed to sync users for department %s: %v", deptID, err)

			continue
		}
	}

	// Also sync root department users
	if err := syncDepartmentUsers(ctx, db, "0", stats); err != nil {
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

	log.Info("feishu sync: full organization sync completed")

	setSyncStatus(SyncStatus{
		LastSyncAt:     time.Now(),
		Status:         "success",
		TotalDepts:     stats.totalDepts,
		DeptsWithName:  stats.deptsWithName,
		TotalUsers:     stats.totalUsers,
		UsersWithName:  stats.usersWithName,
		UsersWithEmail: stats.usersWithEmail,
	})

	return nil
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

		record := models.FeishuDepartment{
			DepartmentID:     dept.DepartmentID,
			OpenDepartmentID: dept.OpenDepartmentID,
			ParentID:         dept.ParentID,
			Name:             dept.Name,
			MemberCount:      dept.MemberCount,
			Order:            dept.Order,
			Status:           1,
		}

		// Match by department_id OR open_department_id to avoid duplicates
		// when the same department was previously synced with a different ID format
		var existing models.FeishuDepartment
		found := db.Where("department_id = ? OR (open_department_id = ? AND open_department_id != '')",
			dept.DepartmentID, dept.OpenDepartmentID).First(&existing).Error == nil

		var result *gorm.DB
		if found {
			// Update existing record, including department_id to normalize it
			result = db.Model(&existing).Updates(models.FeishuDepartment{
				DepartmentID:     dept.DepartmentID,
				OpenDepartmentID: dept.OpenDepartmentID,
				ParentID:         dept.ParentID,
				Name:             dept.Name,
				MemberCount:      dept.MemberCount,
				Order:            dept.Order,
				Status:           1,
			})
		} else {
			result = db.Create(&record)
		}

		if result.Error != nil {
			log.Errorf("feishu sync: failed to upsert department %s: %v", dept.DepartmentID, result.Error)

			continue
		}

		syncedIDs = append(syncedIDs, dept.DepartmentID)

		// Recurse into child departments
		childIDs, err := syncDepartmentsRecursive(ctx, db, dept.DepartmentID, stats)
		if err != nil {
			log.Errorf("feishu sync: failed to sync children of department %s: %v", dept.DepartmentID, err)
		} else {
			syncedIDs = append(syncedIDs, childIDs...)
		}
	}

	return syncedIDs, nil
}

func syncDepartmentUsers(ctx context.Context, db *gorm.DB, departmentID string, stats *syncStats) error {
	users, err := ListDepartmentUsers(ctx, departmentID)
	if err != nil {
		return err
	}

	for _, u := range users {
		if u.OpenID == "" {
			continue
		}

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
