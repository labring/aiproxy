package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
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
	storeV2Table             = "store_v2"
	groupChannelStoreV2Table = "group_channel_store_v2"
)

const (
	StorePrefixResponse        = "response"
	StorePrefixVideoJob        = "video_job"
	StorePrefixVideoGeneration = "video_generation"
	StorePrefixGeminiFile      = "gemini_file"
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
	Model     string `gorm:"size:128"`
	Metadata  string `gorm:"type:text"`
}

type GroupChannelStoreV2 struct {
	ID        string    `gorm:"size:128;primaryKey:3"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
	ExpiresAt time.Time `gorm:"index"`
	GroupID   string    `gorm:"size:64;primaryKey:1"`
	TokenID   int       `gorm:"primaryKey:2"`
	ChannelID int
	Model     string `gorm:"size:128"`
	Metadata  string `gorm:"type:text"`
}

func (s *StoreV2) BeforeSave(_ *gorm.DB) error {
	return prepareStoreForSave(s, time.Now())
}

func (s *GroupChannelStoreV2) BeforeSave(_ *gorm.DB) error {
	now := time.Now()

	return prepareStoreBeforeSave(
		&s.ID,
		s.GroupID,
		s.TokenID,
		s.ChannelID,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.ExpiresAt,
		now,
	)
}

func prepareStoreBeforeSave(
	id *string,
	groupID string,
	tokenID int,
	channelID int,
	createdAt *time.Time,
	updatedAt *time.Time,
	expiresAt *time.Time,
	now time.Time,
) error {
	if groupID != "" {
		if tokenID == 0 {
			return errors.New("token id is required")
		}
	}

	if channelID == 0 {
		return errors.New("channel id is required")
	}

	if *id == "" {
		*id = common.ShortUUID()
	}

	if createdAt.IsZero() {
		*createdAt = now
	}

	if expiresAt.IsZero() {
		*expiresAt = createdAt.Add(time.Hour * 24 * 30)
	}

	if updatedAt.IsZero() {
		*updatedAt = *createdAt
	}

	return nil
}

func SaveStore(s *StoreV2) (*StoreV2, error) {
	return SaveStoreWithOptionByScope(s, ChannelScopeGlobal, SaveStoreOption{})
}

func SaveStoreWithOption(s *StoreV2, opt SaveStoreOption) (*StoreV2, error) {
	return SaveStoreWithOptionByScope(s, ChannelScopeGlobal, opt)
}

func SaveStoreByScope(s *StoreV2, scope ChannelScope) (*StoreV2, error) {
	return SaveStoreWithOptionByScope(s, scope, SaveStoreOption{})
}

func SaveStoreWithOptionByScope(
	s *StoreV2,
	scope ChannelScope,
	opt SaveStoreOption,
) (*StoreV2, error) {
	scope = normalizeChannelScope(scope)

	if opt.MinUpdateInterval > 0 {
		if existing, ok := getStoreFastPath(s, scope, opt); ok {
			return existing, nil
		}
	}

	return upsertStore(s, scope, opt)
}

func SaveIfNotExistStore(s *StoreV2) (*StoreV2, error) {
	return SaveIfNotExistStoreByScope(s, ChannelScopeGlobal)
}

func SaveIfNotExistStoreByScope(s *StoreV2, scope ChannelScope) (*StoreV2, error) {
	scope = normalizeChannelScope(scope)

	if existing, ok := getStoreFastPath(s, scope, SaveStoreOption{}); ok {
		return existing, nil
	}

	if err := prepareStoreForSave(s, time.Now()); err != nil {
		return nil, err
	}

	tx := storeScopedDB(scope).Clauses(clause.OnConflict{DoNothing: true}).Create(s)
	if tx.Error != nil {
		return nil, tx.Error
	}

	if tx.RowsAffected > 0 {
		if err := CacheSetStoreByScope(s.ToStoreCache(), scope); err != nil {
			return nil, err
		}

		return s, nil
	}

	existing, err := getStore(s.GroupID, s.TokenID, s.ID, scope, true)
	if err != nil {
		return nil, err
	}

	if existing.ExpiresAt.After(time.Now()) {
		if err := CacheSetStoreByScope(existing.ToStoreCache(), scope); err != nil {
			return nil, err
		}

		return existing, nil
	}

	tx = LogDB.Session(&gorm.Session{SkipHooks: true}).
		Table(storeTable(scope)).
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
			"metadata":   s.Metadata,
		})
	if tx.Error != nil {
		return nil, tx.Error
	}

	if tx.RowsAffected > 0 {
		if err := CacheSetStoreByScope(s.ToStoreCache(), scope); err != nil {
			return nil, err
		}

		return s, nil
	}

	existing, err = GetStoreByScope(s.GroupID, s.TokenID, s.ID, scope)
	if err != nil {
		return nil, err
	}

	if err := CacheSetStoreByScope(existing.ToStoreCache(), scope); err != nil {
		return nil, err
	}

	return existing, nil
}

func getStoreFastPath(s *StoreV2, scope ChannelScope, opt SaveStoreOption) (*StoreV2, bool) {
	sc, ok := cachePeekStore(s.GroupID, s.TokenID, s.ID, scope)
	if !ok {
		return nil, false
	}

	if opt.MinUpdateInterval > 0 {
		if sc.UpdatedAt.IsZero() || time.Since(sc.UpdatedAt) >= opt.MinUpdateInterval {
			return nil, false
		}
	}

	store := sc.ToStoreV2()
	if store == nil || store.ID == "" {
		return nil, false
	}

	return store, true
}

func normalizeChannelScope(scope ChannelScope) ChannelScope {
	return NormalizeChannelScope(scope)
}

func storeTable(scope ChannelScope) string {
	if normalizeChannelScope(scope) == ChannelScopeGroup {
		return groupChannelStoreV2Table
	}

	return storeV2Table
}

func storeScopedDB(scope ChannelScope) *gorm.DB {
	return LogDB.Table(storeTable(scope))
}

func saveStoreWithMinUpdateInterval(
	s *StoreV2,
	scope ChannelScope,
	opt SaveStoreOption,
) (*StoreV2, error) {
	now := time.Now()
	if err := prepareStoreForSave(s, now); err != nil {
		return nil, err
	}

	cutoff := now.Add(-opt.MinUpdateInterval)

	tx := storeScopedDB(scope).Clauses(clause.OnConflict{
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
			"metadata":   s.Metadata,
		}),
		Where: storeUpsertUpdateWhere(cutoff, now),
	}).Create(s)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return loadAndCacheStore(s.GroupID, s.TokenID, s.ID, scope)
}

func upsertStore(s *StoreV2, scope ChannelScope, opt SaveStoreOption) (*StoreV2, error) {
	if opt.MinUpdateInterval > 0 {
		return saveStoreWithMinUpdateInterval(s, scope, opt)
	}

	now := time.Now()
	if err := prepareStoreForSave(s, now); err != nil {
		return nil, err
	}

	tx := storeScopedDB(scope).Clauses(clause.OnConflict{
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
			"metadata":   s.Metadata,
		}),
	}).Create(s)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return loadAndCacheStore(s.GroupID, s.TokenID, s.ID, scope)
}

func prepareStoreForSave(s *StoreV2, now time.Time) error {
	if s == nil {
		return errors.New("store is required")
	}

	if s.GroupID != "" && s.TokenID == 0 {
		return errors.New("token id is required")
	}

	if s.ChannelID == 0 {
		return errors.New("channel id is required")
	}

	if s.ID == "" {
		s.ID = common.ShortUUID()
	}

	return prepareStoreBeforeSave(
		&s.ID,
		s.GroupID,
		s.TokenID,
		s.ChannelID,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.ExpiresAt,
		now,
	)
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

func loadAndCacheStore(group string, tokenID int, id string, scope ChannelScope) (*StoreV2, error) {
	current, err := getStore(group, tokenID, id, scope, true)
	if err != nil {
		return nil, err
	}

	if err := CacheSetStoreByScope(current.ToStoreCache(), scope); err != nil {
		return nil, err
	}

	return current, nil
}

func GetStore(group string, tokenID int, id string) (*StoreV2, error) {
	return GetStoreByScope(group, tokenID, id, ChannelScopeGlobal)
}

func GetStoreByScope(group string, tokenID int, id string, scope ChannelScope) (*StoreV2, error) {
	return getStore(group, tokenID, id, scope, false)
}

func getStore(
	group string,
	tokenID int,
	id string,
	scope ChannelScope,
	includeExpired bool,
) (*StoreV2, error) {
	scope = normalizeChannelScope(scope)

	var s StoreV2

	tx := storeScopedDB(scope).Where("group_id = ? and token_id = ? and id = ?", group, tokenID, id)
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

func GeminiFileStoreID(fileID string) string {
	return StoreID(StorePrefixGeminiFile, fileID)
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

func StoreChannelKey(store *StoreCache, scope ChannelScope) string {
	if store == nil {
		return ""
	}

	switch normalizeChannelScope(scope) {
	case ChannelScopeGroup:
		return GroupChannelMonitorKey(store.GroupID, store.ChannelID)
	case ChannelScopeGlobal:
	default:
		return ""
	}

	if store.ChannelID == 0 {
		return ""
	}

	return strconv.Itoa(store.ChannelID)
}
