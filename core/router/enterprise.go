//go:build enterprise

package router

import "github.com/labring/aiproxy/core/enterprise"

func init() {
	enterpriseRouter = enterprise.RegisterRoutes
}
