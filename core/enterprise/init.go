//go:build enterprise

package enterprise

import (
	enterprisenotify "github.com/labring/aiproxy/core/enterprise/notify"
	"github.com/labring/aiproxy/core/enterprise/quota"
	log "github.com/sirupsen/logrus"
)

// Initialize performs all enterprise module initialization.
// Called from core/startup_enterprise.go via init() hook.
func Initialize() {
	enterprisenotify.Init()
	quota.Init()
	log.Info("enterprise module initialized")
}
