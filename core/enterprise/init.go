//go:build enterprise

package enterprise

import (
	"context"
	"sync"

	"github.com/labring/aiproxy/core/enterprise/feishu"
	enterprisenotify "github.com/labring/aiproxy/core/enterprise/notify"
	"github.com/labring/aiproxy/core/enterprise/novita"
	"github.com/labring/aiproxy/core/enterprise/ppio"
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

	// Refresh PPIO and Novita channel model lists in parallel.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := ppio.EnsurePPIOChannels(false, nil, nil, ppio.PPIOConfigResult{}); err != nil {
			log.Warnf("PPIO channel refresh on startup: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := novita.EnsureNovitaChannels(false, nil, nil, novita.NovitaConfigResult{}); err != nil {
			log.Warnf("Novita channel refresh on startup: %v", err)
		}
	}()

	wg.Wait()

	ctx := context.Background()

	// Start Feishu organization sync scheduler (every 6 hours)
	feishu.StartSyncScheduler(ctx)

	// Start PPIO and Novita daily model sync schedulers (02:00 and 02:15 respectively)
	ppio.StartSyncScheduler(ctx)
	novita.StartSyncScheduler(ctx)

	log.Info("enterprise module post-DB initialized")
}
