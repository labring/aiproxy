//go:build enterprise

package router

import "github.com/labring/aiproxy/core/enterprise"

func init() {
	enterpriseRouter = enterprise.RegisterRoutes
	// Allow Feishu admin users to access the admin panel
	adminAuth = enterprise.EnterpriseAdminAuth
}
