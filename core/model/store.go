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

// Store represents channel-associated data storage for various purposes:
// - Video generation jobs and their results
// - File storage with associated metadata
// - Any other channel-specific data that needs persistence
type Store struct {
	ID        string    `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	ExpiresAt time.Time
	GroupID   string
	TokenID   int
	ChannelID int
	Model     string
}

func (s *Store) BeforeSave(_ *gorm.DB) error {
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

func SaveStore(s *Store) (*Store, error) {
	if err := LogDB.Save(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

func GetStore(id string) (*Store, error) {
	var s Store
	err := LogDB.Where("id = ?", id).First(&s).Error
	return &s, HandleNotFound(err, ErrStoreNotFound)
}
