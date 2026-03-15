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
	ID           int            `json:"id"            gorm:"primaryKey"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-"             gorm:"index"`
	OpenID       string         `json:"open_id"       gorm:"size:64;uniqueIndex;not null"`
	UnionID      string         `json:"union_id"      gorm:"size:64;index"`
	UserID       string         `json:"user_id"       gorm:"size:64;index"`
	Name         string         `json:"name"          gorm:"size:128"`
	Email        string         `json:"email"         gorm:"size:256"`
	Avatar       string         `json:"avatar"        gorm:"size:512"`
	DepartmentID string         `json:"department_id" gorm:"size:64;index"`
	GroupID      string         `json:"group_id"      gorm:"size:64;index;not null"`
	TokenID      int            `json:"token_id"      gorm:"index"`
	Role         string         `json:"role"          gorm:"size:32;default:viewer;index"`
	Status       int            `json:"status"        gorm:"default:1;index"`
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
	DepartmentID     string         `json:"department_id"     gorm:"size:64;uniqueIndex;not null"`
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
