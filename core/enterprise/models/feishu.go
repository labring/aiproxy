//go:build enterprise

package models

import (
	"time"

	"gorm.io/gorm"
)

// Enterprise user roles.
const (
	RoleViewer  = "viewer"  // can only see own department data
	RoleAnalyst = "analyst" // can see all departments + ranking + export
	RoleAdmin   = "admin"   // full access, equivalent to AdminKey
)

// FeishuUser maps a Feishu (Lark) user to an AI Proxy group and token.
type FeishuUser struct {
	ID             int            `json:"id"              gorm:"primaryKey"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-"               gorm:"index"`
	OpenID         string         `json:"open_id"         gorm:"size:64;index;not null"`
	UnionID        string         `json:"union_id"        gorm:"size:64;index"`
	UserID         string         `json:"user_id"         gorm:"size:64;index"`
	TenantID       string         `json:"tenant_id"       gorm:"size:64;index"`
	Name           string         `json:"name"            gorm:"size:128"`
	Email          string         `json:"email"           gorm:"size:256"`
	Avatar         string         `json:"avatar"          gorm:"size:512"`
	DepartmentID   string         `json:"department_id"   gorm:"size:64;index"`
	DepartmentIDs  string         `json:"department_ids"  gorm:"size:1024"`
	Level1DeptID   string         `json:"level1_dept_id"  gorm:"size:64;index"`
	Level1DeptName string         `json:"level1_dept_name" gorm:"size:256"`
	Level2DeptID   string         `json:"level2_dept_id"  gorm:"size:64;index"`
	Level2DeptName string         `json:"level2_dept_name" gorm:"size:256"`
	DeptFullPath   string         `json:"dept_full_path"  gorm:"size:1024"`
	GroupID        string         `json:"group_id"        gorm:"size:64;index;not null"`
	TokenID        int            `json:"token_id"        gorm:"index"`
	Role           string         `json:"role"            gorm:"size:32;default:viewer;index"`
	Status         int            `json:"status"          gorm:"default:1;index"`
}

func (FeishuUser) TableName() string {
	return "feishu_users"
}

// FeishuDepartment stores the Feishu department tree for analytics aggregation.
type FeishuDepartment struct {
	ID               int            `json:"id"                gorm:"primaryKey"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-"                 gorm:"index"`
	DepartmentID     string         `json:"department_id"     gorm:"size:64;index;not null"`
	ParentID         string         `json:"parent_id"         gorm:"size:64;index"`
	Name             string         `json:"name"              gorm:"size:256;not null"`
	OpenDepartmentID string         `json:"open_department_id" gorm:"size:64;index"`
	MemberCount      int            `json:"member_count"      gorm:"default:0"`
	Order            int            `json:"order"             gorm:"default:0"`
	Status           int            `json:"status"            gorm:"default:1"`
}

func (FeishuDepartment) TableName() string {
	return "feishu_departments"
}

// FeishuSyncHistory records the result of each Feishu organization sync operation.
// Persists sync status to DB so it survives service restarts.
type FeishuSyncHistory struct {
	ID                  int64     `json:"id"                    gorm:"primaryKey"`
	SyncedAt            time.Time `json:"synced_at"             gorm:"autoCreateTime;index"`
	Status              string    `json:"status"                gorm:"size:32;not null;index"` // syncing, success, failed
	TotalDepts          int       `json:"total_depts"           gorm:"default:0"`
	DeptsWithName       int       `json:"depts_with_name"       gorm:"default:0"`
	TotalUsers          int       `json:"total_users"           gorm:"default:0"`
	UsersWithName       int       `json:"users_with_name"       gorm:"default:0"`
	UsersWithEmail      int       `json:"users_with_email"      gorm:"default:0"`
	DepartedUsers       int       `json:"departed_users"        gorm:"default:0"`
	FailedDepts         int       `json:"failed_depts"          gorm:"default:0"`
	SkippedDeactivation bool      `json:"skipped_deactivation"  gorm:"default:false"`
	DurationMs          int64     `json:"duration_ms"           gorm:"default:0"`
	Error               string    `json:"error,omitempty"       gorm:"size:1024"`
	CreatedAt           time.Time `json:"created_at"            gorm:"autoCreateTime"`
}

func (FeishuSyncHistory) TableName() string {
	return "feishu_sync_histories"
}
