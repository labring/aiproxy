//go:build enterprise

package models

import "strings"

// Module keys for grouping permissions in the UI.
const (
	PermModuleDashboard        = "dashboard"
	PermModuleRanking          = "ranking"
	PermModuleDepartmentDetail = "department_detail"
	PermModuleExport           = "export"
	PermModuleCustomReport     = "custom_report"
	PermModuleQuotaManage      = "quota_manage"
	PermModuleUserManage       = "user_manage"
	PermModuleAccessControl    = "access_control"
)

// AllModules is the ordered list of permission modules.
var AllModules = []string{
	PermModuleDashboard, PermModuleRanking, PermModuleDepartmentDetail, PermModuleExport,
	PermModuleCustomReport, PermModuleQuotaManage, PermModuleUserManage, PermModuleAccessControl,
}

// Permission keys for enterprise features (view + manage per module).
const (
	PermDashboardView        = "dashboard_view"
	PermDashboardManage      = "dashboard_manage"
	PermRankingView          = "ranking_view"
	PermRankingManage        = "ranking_manage"
	PermDepartmentDetailView = "department_detail_view"
	PermDepartmentDetailManage = "department_detail_manage"
	PermExportView           = "export_view"
	PermExportManage         = "export_manage"
	PermCustomReportView     = "custom_report_view"
	PermCustomReportManage   = "custom_report_manage"
	PermQuotaManageView      = "quota_manage_view"
	PermQuotaManageManage    = "quota_manage_manage"
	PermUserManageView       = "user_manage_view"
	PermUserManageManage     = "user_manage_manage"
	PermAccessControlView    = "access_control_view"
	PermAccessControlManage  = "access_control_manage"
)

// ViewPermission returns the view permission key for a module.
func ViewPermission(module string) string {
	return module + "_view"
}

// ManagePermission returns the manage permission key for a module.
func ManagePermission(module string) string {
	return module + "_manage"
}

// IsViewPermission returns true if the permission key is a view permission.
func IsViewPermission(perm string) bool {
	return strings.HasSuffix(perm, "_view")
}

// AllPermissions is the full list of permission keys.
var AllPermissions = []string{
	PermDashboardView, PermDashboardManage,
	PermRankingView, PermRankingManage,
	PermDepartmentDetailView, PermDepartmentDetailManage,
	PermExportView, PermExportManage,
	PermCustomReportView, PermCustomReportManage,
	PermQuotaManageView, PermQuotaManageManage,
	PermUserManageView, PermUserManageManage,
	PermAccessControlView, PermAccessControlManage,
}

// ModuleDisplayNames maps module keys to human-readable names.
var ModuleDisplayNames = map[string]string{
	PermModuleDashboard:        "Dashboard",
	PermModuleRanking:          "Ranking",
	PermModuleDepartmentDetail: "Department Detail",
	PermModuleExport:           "Data Export",
	PermModuleCustomReport:     "Custom Report",
	PermModuleQuotaManage:      "Quota Management",
	PermModuleUserManage:       "User Management",
	PermModuleAccessControl:    "Access Control",
}

// DefaultRolePermissions defines the out-of-box permission set per role.
var DefaultRolePermissions = map[string][]string{
	RoleViewer: {
		PermDashboardView, PermDepartmentDetailView, PermQuotaManageView,
	},
	RoleAnalyst: {
		PermDashboardView, PermDashboardManage,
		PermRankingView, PermRankingManage,
		PermDepartmentDetailView, PermDepartmentDetailManage,
		PermExportView, PermExportManage,
		PermCustomReportView, PermCustomReportManage,
		PermUserManageView, PermQuotaManageView,
	},
	RoleAdmin: AllPermissions,
}

// RolePermission stores a single role→permission grant.
type RolePermission struct {
	ID         int    `json:"id"         gorm:"primaryKey"`
	Role       string `json:"role"       gorm:"size:32;uniqueIndex:idx_role_perm;not null"`
	Permission string `json:"permission" gorm:"size:64;uniqueIndex:idx_role_perm;not null"`
}

func (RolePermission) TableName() string {
	return "enterprise_role_permissions"
}
