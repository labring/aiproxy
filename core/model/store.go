package model

import (
	"errors"
	"time"

	"github.com/labring/aiproxy/core/common"
	"gorm.io/gorm"
)

const (
	ErrStoreNotFound = "store id"
)

// StoreV2 represents channel-associated data storage for various purposes:
// - Video generation jobs and their results
// - File storage with associated metadata
// - Any other channel-specific data that needs persistence
type StoreV2 struct {
	ID        string    `gorm:"size:128;primaryKey:3"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	ExpiresAt time.Time
	GroupID   string `gorm:"size:64;primaryKey:1"`
	TokenID   int    `gorm:"primaryKey:2"`
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
	return s, nil
}

func CreateStoreV2Indexes(db *gorm.DB) error {
	return db.Exec(
		"CREATE INDEX IF NOT EXISTS idx_store_v2_expires_at ON store_v2 (expires_at)",
	).Error
}

func GetStore(group string, tokenID int, id string) (*StoreV2, error) {
	var s StoreV2

	err := LogDB.Where("group_id = ? and token_id = ? and id = ?", group, tokenID, id).
		First(&s).
		Error

	return &s, HandleNotFound(err, ErrStoreNotFound)
}
