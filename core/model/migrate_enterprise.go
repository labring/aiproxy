//go:build enterprise

package model

import "github.com/labring/aiproxy/core/enterprise/models"

func init() {
	enterpriseMigrator = models.EnterpriseAutoMigrate
}
