package common

import (
	"github.com/labring/aiproxy/core/common/env"
)

var UsingSQLite = false

// LikeOp returns the appropriate LIKE operator for the current database.
// PostgreSQL requires ILIKE for case-insensitive matching;
// SQLite's LIKE is already case-insensitive for ASCII.
func LikeOp() string {
	if UsingSQLite {
		return "LIKE"
	}
	return "ILIKE"
}

var (
	SQLitePath        = env.String("SQLITE_PATH", "aiproxy.db")
	SQLiteBusyTimeout = env.Int64("SQLITE_BUSY_TIMEOUT", 3000)
)
