//go:build enterprise

package enterprise

import (
	"context"

	"github.com/labring/aiproxy/core/enterprise/feishu"
	enterprisenotify "github.com/labring/aiproxy/core/enterprise/notify"
	"github.com/labring/aiproxy/core/enterprise/quota"
	log "github.com/sirupsen/logrus"
)

// Initialize performs all enterprise module initialization.
// Called from core/startup_enterprise.go via init() hook.
func Initialize() {
	enterprisenotify.Init()
	quota.Init()

	// Start Feishu organization sync scheduler (every 6 hours)
	// Initial sync is performed in StartSyncScheduler's goroutine after DB is ready
	ctx := context.Background()
	feishu.StartSyncScheduler(ctx)

	log.Info("enterprise module initialized")
}
