//go:build enterprise

package enterprise

import (
	"context"

	"github.com/labring/aiproxy/core/enterprise/feishu"
	enterprisenotify "github.com/labring/aiproxy/core/enterprise/notify"
	"github.com/labring/aiproxy/core/enterprise/quota"
	log "github.com/sirupsen/logrus"
)

// Initialize performs early enterprise module initialization (before DB).
// Called from core/startup_enterprise.go via init() hook.
func Initialize() {
	enterprisenotify.Init()
	quota.Init()

	log.Info("enterprise module initialized (pre-DB)")
}

// PostDBInit performs enterprise initialization that requires the database.
// Must be called after model.InitDB().
func PostDBInit() {
	// Load role permissions into memory cache
	LoadRolePermissions()

	// Start Feishu organization sync scheduler (every 6 hours)
	ctx := context.Background()
	feishu.StartSyncScheduler(ctx)

	log.Info("enterprise module post-DB initialized")
}
