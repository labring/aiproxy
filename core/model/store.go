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
)

// StoreV2 represents channel-associated data storage for various purposes:
// - Video generation jobs and their results
// - File storage with associated metadata
// - Any other channel-specific data that needs persistence
type StoreV2 struct {
	ID        string    `gorm:"size:128;primaryKey:3"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
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

	return nil
}

func SaveStore(s *StoreV2) (*StoreV2, error) {
	if err := LogDB.Save(s).Error; err != nil {
		return nil, err
	}

	if err := CacheSetStore(s.ToStoreCache()); err != nil {
		return nil, err
	}

	return s, nil
}

func SaveIfNotExistStore(s *StoreV2) (*StoreV2, error) {
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
			"created_at": s.CreatedAt,
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

func ResponseStoreID(responseID string) string {
	return StoreID(StorePrefixResponse, responseID)
}

func VideoJobStoreID(jobID string) string {
	return StoreID(StorePrefixVideoJob, jobID)
}

func VideoGenerationStoreID(generationID string) string {
	return StoreID(StorePrefixVideoGeneration, generationID)
}

func PromptCacheStoreID(modelName, promptCacheKey string) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s:%s", modelName, promptCacheKey))
	return StoreID(StorePrefixPromptCacheKey, hex.EncodeToString(sum[:]))
}
