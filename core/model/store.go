package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ErrStoreNotFound = "store id"
)

const (
	StorePrefixResponse        = "response"
	StorePrefixVideoJob        = "video_job"
	StorePrefixVideoGeneration = "video_generation"
	StorePrefixPromptCacheKey  = "prompt_cache_key"
	StorePrefixCacheFollow     = "cachefollow"
	StorePrefixCacheFollowUser = "cachefollow_user"
)

type CacheKeyType string

const (
	CacheKeyTypeStable CacheKeyType = "stable"
	CacheKeyTypeRecent CacheKeyType = "recent"
)

type SaveStoreOption struct {
	MinUpdateInterval time.Duration
}

// StoreV2 represents channel-associated data storage for various purposes:
// - Video generation jobs and their results
// - File storage with associated metadata
// - Any other channel-specific data that needs persistence
type StoreV2 struct {
	ID        string    `gorm:"size:128;primaryKey:3"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	ExpiresAt time.Time `gorm:"index"`
	GroupID   string    `gorm:"size:64;primaryKey:1"`
	TokenID   int       `gorm:"primaryKey:2"`
	ChannelID int
	Model     string `gorm:"size:64"`
}

func (s *StoreV2) BeforeSave(_ *gorm.DB) error {
	if s.GroupID != "" {
		if s.TokenID == 0 {
			return errors.New("token id is required")
		}
	}

	if s.ChannelID == 0 {
		return errors.New("channel id is required")
	}

	if s.ID == "" {
		s.ID = common.ShortUUID()
	}

	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}

	if s.ExpiresAt.IsZero() {
		s.ExpiresAt = s.CreatedAt.Add(time.Hour * 24 * 30)
	}

	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = s.CreatedAt
	}

	return nil
}

func SaveStore(s *StoreV2) (*StoreV2, error) {
	return SaveStoreWithOption(s, SaveStoreOption{})
}

func SaveStoreWithOption(s *StoreV2, opt SaveStoreOption) (*StoreV2, error) {
	if opt.MinUpdateInterval > 0 {
		if existing, ok := getStoreFastPath(s, opt); ok {
			return existing, nil
		}
	}

	return upsertStore(s, opt)
}

func SaveIfNotExistStore(s *StoreV2) (*StoreV2, error) {
	if existing, ok := getStoreFastPath(s, SaveStoreOption{}); ok {
		return existing, nil
	}

	tx := LogDB.Clauses(clause.OnConflict{DoNothing: true}).Create(s)
	if tx.Error != nil {
		return nil, tx.Error
	}

	if tx.RowsAffected > 0 {
		if err := CacheSetStore(s.ToStoreCache()); err != nil {
			return nil, err
		}

		return s, nil
	}

	existing, err := getStore(s.GroupID, s.TokenID, s.ID, true)
	if err != nil {
		return nil, err
	}

	if existing.ExpiresAt.After(time.Now()) {
		if err := CacheSetStore(existing.ToStoreCache()); err != nil {
			return nil, err
		}

		return existing, nil
	}

	tx = LogDB.Session(&gorm.Session{SkipHooks: true}).
		Model(&StoreV2{}).
		Where(
			"group_id = ? and token_id = ? and id = ? and expires_at <= ?",
			s.GroupID,
			s.TokenID,
			s.ID,
			time.Now(),
		).
		UpdateColumns(map[string]any{
			"updated_at": s.UpdatedAt,
			"expires_at": s.ExpiresAt,
			"channel_id": s.ChannelID,
			"model":      s.Model,
		})
	if tx.Error != nil {
		return nil, tx.Error
	}

	if tx.RowsAffected > 0 {
		if err := CacheSetStore(s.ToStoreCache()); err != nil {
			return nil, err
		}

		return s, nil
	}

	existing, err = GetStore(s.GroupID, s.TokenID, s.ID)
	if err != nil {
		return nil, err
	}

	if err := CacheSetStore(existing.ToStoreCache()); err != nil {
		return nil, err
	}

	return existing, nil
}

func getStoreFastPath(s *StoreV2, opt SaveStoreOption) (*StoreV2, bool) {
	sc, ok := cachePeekStore(s.GroupID, s.TokenID, s.ID)
	if !ok {
		return nil, false
	}

	if opt.MinUpdateInterval > 0 {
		if sc.UpdatedAt.IsZero() || time.Since(sc.UpdatedAt) >= opt.MinUpdateInterval {
			return nil, false
		}
	}

	return sc.ToStoreV2(), true
}

func saveStoreWithMinUpdateInterval(s *StoreV2, opt SaveStoreOption) (*StoreV2, error) {
	now := time.Now()
	prepareStoreForSave(s, now)

	cutoff := now.Add(-opt.MinUpdateInterval)

	tx := LogDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "group_id"},
			{Name: "token_id"},
			{Name: "id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"updated_at": s.UpdatedAt,
			"expires_at": s.ExpiresAt,
			"channel_id": s.ChannelID,
			"model":      s.Model,
		}),
		Where: storeUpsertUpdateWhere(cutoff, now),
	}).Create(s)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return loadAndCacheStore(s.GroupID, s.TokenID, s.ID)
}

func upsertStore(s *StoreV2, opt SaveStoreOption) (*StoreV2, error) {
	if opt.MinUpdateInterval > 0 {
		return saveStoreWithMinUpdateInterval(s, opt)
	}

	now := time.Now()
	prepareStoreForSave(s, now)

	tx := LogDB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "group_id"},
			{Name: "token_id"},
			{Name: "id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"updated_at": s.UpdatedAt,
			"expires_at": s.ExpiresAt,
			"channel_id": s.ChannelID,
			"model":      s.Model,
		}),
	}).Create(s)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return loadAndCacheStore(s.GroupID, s.TokenID, s.ID)
}

func prepareStoreForSave(s *StoreV2, now time.Time) {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}

	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}

	if s.ExpiresAt.IsZero() {
		s.ExpiresAt = s.CreatedAt.Add(time.Hour * 24 * 30)
	}
}

func storeUpsertUpdateWhere(cutoff, now time.Time) clause.Where {
	return clause.Where{
		Exprs: []clause.Expression{
			clause.Or(
				clause.Lte{
					Column: clause.Column{Table: clause.CurrentTable, Name: "updated_at"},
					Value:  cutoff,
				},
				clause.Lte{
					Column: clause.Column{Table: clause.CurrentTable, Name: "expires_at"},
					Value:  now,
				},
			),
		},
	}
}

func loadAndCacheStore(group string, tokenID int, id string) (*StoreV2, error) {
	current, err := getStore(group, tokenID, id, true)
	if err != nil {
		return nil, err
	}

	if err := CacheSetStore(current.ToStoreCache()); err != nil {
		return nil, err
	}

	return current, nil
}

func GetStore(group string, tokenID int, id string) (*StoreV2, error) {
	return getStore(group, tokenID, id, false)
}

func getStore(group string, tokenID int, id string, includeExpired bool) (*StoreV2, error) {
	var s StoreV2

	tx := LogDB.Where("group_id = ? and token_id = ? and id = ?", group, tokenID, id)
	if !includeExpired {
		tx = tx.Where("expires_at > ?", time.Now())
	}

	err := tx.First(&s).Error

	return &s, HandleNotFound(err, ErrStoreNotFound)
}

func StoreID(prefix, id string) string {
	if id == "" {
		return ""
	}

	nsPrefix := prefix + ":"
	if strings.HasPrefix(id, nsPrefix) {
		return id
	}

	return nsPrefix + id
}

func HashedStoreID(prefix string, parts ...string) string {
	if len(parts) == 0 {
		return ""
	}

	sum := sha256.Sum256(fmt.Appendf(nil, "%s", strings.Join(parts, ":")))

	return StoreID(prefix, hex.EncodeToString(sum[:]))
}

func ResponseStoreID(responseID string) string {
	return StoreID(StorePrefixResponse, responseID)
}

func VideoJobStoreID(jobID string) string {
	return StoreID(StorePrefixVideoJob, jobID)
}

func VideoGenerationStoreID(generationID string) string {
	return StoreID(StorePrefixVideoGeneration, generationID)
}

func PromptCacheStoreID(modelName, promptCacheKey string, keyType CacheKeyType) string {
	return HashedStoreID(StorePrefixPromptCacheKey, string(keyType), modelName, promptCacheKey)
}

func CacheFollowStoreID(modelName string, keyType CacheKeyType) string {
	return HashedStoreID(StorePrefixCacheFollow, string(keyType), modelName)
}

func CacheFollowUserStoreID(modelName, user string, keyType CacheKeyType) string {
	return HashedStoreID(StorePrefixCacheFollowUser, string(keyType), modelName, user)
}
