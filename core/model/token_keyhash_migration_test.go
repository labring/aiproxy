//nolint:testpackage
package model

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// legacyToken is the pre-key_hash model shape.
type legacyToken struct {
	CreatedAt time.Time `gorm:"type:datetime"`
	Key       string    `gorm:"type:varchar(48);uniqueIndex"`
	Name      string    `gorm:"size:32;index;uniqueIndex:idx_group_name;not null"`
	GroupID   string    `gorm:"size:64;index;uniqueIndex:idx_group_name"`
	Status    int       `gorm:"default:1;index"`
	ID        int       `gorm:"primaryKey"`
}

func (*legacyToken) TableName() string { return "tokens" }

// TestMigrateTokenKeyHash exercises the startup migration on a database with
// the legacy shape: raw keys without key_hash plus (from the unreleased
// dual-form build) an h:-prefixed row.
func TestMigrateTokenKeyHash(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "token_migration.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyToken{}))

	plainKey := GenerateTokenKey()
	insert := func(key, name string) {
		require.NoError(t, db.Exec(
			"INSERT INTO tokens (created_at, key, name, group_id, status) VALUES (?, ?, ?, ?, 1)",
			time.Now(), key, name, "migration-group",
		).Error)
	}
	insert(plainKey, "legacy-plain")
	insert(GenerateTokenKey(), "legacy-plain-2")
	insert("h:"+HashTokenKey("dual-form-key"), "dual-form")

	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	modelLocalCache.Flush()
	require.NoError(t, migrateDB())

	var migrated []*Token
	require.NoError(t, DB.Order("id").Find(&migrated).Error)
	require.Len(t, migrated, 3)

	for _, token := range migrated {
		require.NotEmpty(t, token.KeyHash)
	}

	require.Equal(t, plainKey, migrated[0].Key)
	require.Equal(t, HashTokenKey(plainKey), migrated[0].KeyHash)

	// the dual-form row moved its hash out of key
	require.Empty(t, migrated[2].Key)
	require.Equal(t, HashTokenKey("dual-form-key"), migrated[2].KeyHash)

	// the legacy unique index on key is gone, so hashed tokens (empty key)
	// can be inserted
	hashed := &Token{
		Name:    "new-hashed",
		GroupID: "migration-group",
		KeyHash: HashTokenKey(GenerateTokenKey()),
	}
	require.NoError(t, DB.Create(hashed).Error)

	// legacy plain keys authenticate through the backfilled hash
	got, err := GetAndValidateToken(plainKey)
	require.NoError(t, err)
	require.Equal(t, migrated[0].ID, got.ID)

	require.True(t, DB.Migrator().HasIndex(&Token{}, "idx_tokens_key_hash"))
}

// TestMigrateTokenKeyHashIdempotentAndBatched covers the production concerns:
// a second start skips the work, and more rows than one batch backfill fully.
func TestMigrateTokenKeyHashIdempotentAndBatched(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "token_migration_batches.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyToken{}))

	rows := tokenKeyHashMigrationBatch + 50
	for i := range rows {
		require.NoError(t, db.Exec(
			"INSERT INTO tokens (created_at, key, name, group_id, status) VALUES (?, ?, ?, ?, 1)",
			time.Now(), GenerateTokenKey(), "bulk-"+strconv.Itoa(i), "migration-group",
		).Error)
	}

	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	modelLocalCache.Flush()
	require.NoError(t, migrateDB())

	var missing int64
	require.NoError(t, DB.Model(&Token{}).
		Where("key != '' AND key_hash = ''").Count(&missing).Error)
	require.Zero(t, missing)

	var hashed int64
	require.NoError(t, DB.Model(&Token{}).
		Where("key != '' AND key_hash != ''").Count(&hashed).Error)
	require.EqualValues(t, rows, hashed)

	// second start finds nothing to do and stays correct
	require.NoError(t, migrateDB())
	require.NoError(t, DB.Model(&Token{}).
		Where("key_hash = ''").Count(&missing).Error)
	require.Zero(t, missing)
}
