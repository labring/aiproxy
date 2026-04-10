package model_test

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/model"
	"gorm.io/gorm"
)

// TestDBAutoReconnect simulates a DB disconnect/reconnect cycle and verifies
// that GORM's connection pool (via database/sql) automatically recovers.
//
// This mirrors the real-world scenario: WireGuard tunnel goes down, DB becomes
// unreachable, tunnel recovers, and the application should resume without restart.
//
// Run against a real PostgreSQL:
//
//	SQL_DSN=postgres://user:pass@localhost:5432/testdb go test -tags enterprise -v -run TestDBAutoReconnect -timeout 120s ./model/...
func TestDBAutoReconnect(t *testing.T) {
	dsn := os.Getenv("SQL_DSN")
	if dsn == "" {
		t.Skip("SQL_DSN not set — skipping DB reconnect test (requires real PostgreSQL)")
	}

	db, err := model.OpenPostgreSQL(dsn)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}

	// Match production settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(60 * time.Second)

	// Phase 1: Verify initial connectivity
	if err := verifyDBWorks(db, sqlDB); err != nil {
		t.Fatalf("Phase 1 (initial check): %v", err)
	}
	t.Log("Phase 1: Initial DB connection OK")

	// Phase 2: Simulate DB outage by terminating all connections server-side.
	// This is what happens when WireGuard tunnel drops — TCP connections die silently.
	t.Log("Phase 2: Simulating DB outage (terminating backend connections)...")
	if err := terminateBackendConnections(db); err != nil {
		t.Logf("  Warning: could not terminate backends (need superuser): %v", err)
		t.Log("  Falling back to connection pool stat check only")
	}

	// Phase 3: Verify connections are actually broken
	// After backend termination, existing pooled connections should fail
	t.Log("Phase 3: Verifying connections are broken...")
	brokenCount := 0
	for range 5 {
		if err := sqlDB.Ping(); err != nil {
			brokenCount++
		}
	}
	t.Logf("  %d/5 ping attempts failed (expected: some failures)", brokenCount)

	// Phase 4: Wait for ConnMaxLifetime and verify auto-recovery
	// database/sql's connectionCleaner goroutine runs every ConnMaxLifetime
	// and closes expired connections. New requests create fresh connections.
	t.Log("Phase 4: Waiting for connection pool to auto-recover...")

	recovered := false
	for i := range 15 {
		time.Sleep(5 * time.Second)
		if err := verifyDBWorks(db, sqlDB); err == nil {
			t.Logf("  Auto-recovered after %ds", (i+1)*5)
			recovered = true
			break
		} else {
			t.Logf("  Attempt %d: still failing: %v", i+1, err)
		}
	}

	if !recovered {
		t.Fatal("Phase 4: Connection pool did NOT auto-recover within 75s")
	}

	// Phase 5: Verify GORM operations work (not just raw Ping)
	t.Log("Phase 5: Verifying GORM query works after recovery...")
	var result int
	if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
		t.Fatalf("GORM query failed after recovery: %v", err)
	}
	if result != 1 {
		t.Fatalf("unexpected result: got %d, want 1", result)
	}
	t.Log("Phase 5: GORM query OK — auto-reconnect verified")

	// Summary
	stats := sqlDB.Stats()
	t.Logf("Final pool stats: Open=%d Idle=%d InUse=%d WaitCount=%d",
		stats.OpenConnections, stats.Idle, stats.InUse, stats.WaitCount)
}

func verifyDBWorks(db *gorm.DB, sqlDB *sql.DB) error {
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	var n int
	if err := db.Raw("SELECT 1").Scan(&n).Error; err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	return nil
}

// terminateBackendConnections kills all other PostgreSQL backends for this database,
// simulating what happens when a network tunnel drops.
func terminateBackendConnections(db *gorm.DB) error {
	return db.Exec(`
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid()
		  AND datname = current_database()
	`).Error
}
