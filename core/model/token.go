package model

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/conv"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ErrTokenNotFound = "token"
)

const (
	PeriodTypeDaily   = "daily"
	PeriodTypeWeekly  = "weekly"
	PeriodTypeMonthly = "monthly"
)

const (
	TokenStatusEnabled  = 1
	TokenStatusDisabled = 2
)

type Token struct {
	CreatedAt time.Time       `json:"created_at"`
	Group     *Group          `json:"-"          gorm:"foreignKey:GroupID"`
	Key       string          `json:"key"        gorm:"type:char(48);uniqueIndex"`
	Name      EmptyNullString `json:"name"       gorm:"size:32;index;uniqueIndex:idx_group_name;not null"`
	GroupID   string          `json:"group"      gorm:"size:64;index;uniqueIndex:idx_group_name"`
	Subnets   []string        `json:"subnets"    gorm:"serializer:fastjson;type:text"`
	Models    []string        `json:"models"     gorm:"serializer:fastjson;type:text"`
	Status    int             `json:"status"     gorm:"default:1;index"`
	ID        int             `json:"id"         gorm:"primaryKey"`

	UsedAmount   float64 `json:"used_amount"   gorm:"index"`
	RequestCount int     `json:"request_count" gorm:"index"`

	Quota                  float64         `json:"quota"`
	PeriodQuota            float64         `json:"period_quota"`
	PeriodType             EmptyNullString `json:"period_type"               gorm:"size:20"` // daily, weekly, monthly, default is monthly
	PeriodLastUpdateTime   time.Time       `json:"period_last_update_time"`                  // Last time period was reset
	PeriodLastUpdateAmount float64         `json:"period_last_update_amount"`                // Total usage at last period reset
}

func (t *Token) BeforeCreate(_ *gorm.DB) error {
	if t.Key == "" || len(t.Key) != 48 {
		t.Key = generateKey()
	}
	return nil
}

func (t *Token) BeforeSave(_ *gorm.DB) error {
	if len(t.Name) > 32 {
		return errors.New("token name is too long")
	}
	return nil
}

// GetEffectiveQuotaStatus returns the effective quota status for token
func (t *Token) GetEffectiveQuotaStatus() (totalExceeded, periodExceeded bool, err error) {
	// Check total quota (if set)
	if t.Quota > 0 && t.UsedAmount >= t.Quota {
		totalExceeded = true
	}

	if t.PeriodQuota > 0 {
		// Check if we need to reset period usage
		if needsReset, err := t.NeedsPeriodReset(); err != nil {
			return false, false, err
		} else if needsReset {
			// Period usage should be considered as reset (0) but we don't modify the struct here
			// The actual database reset should be handled separately
			periodExceeded = false // Consider as reset, so no period limit exceeded

			// Trigger async period reset - don't wait for it to complete
			go func() {
				if err := ResetTokenPeriodUsage(t.ID); err != nil {
					log.Error("failed to reset token period usage: " + err.Error())
				}
			}()
		} else {
			// Period is still valid, check against current usage
			// Calculate period usage: current total - last recorded total at period reset
			periodUsage := t.UsedAmount - t.PeriodLastUpdateAmount
			if periodUsage >= t.PeriodQuota {
				periodExceeded = true
			}
		}
	}

	return totalExceeded, periodExceeded, nil
}

// NeedsPeriodReset checks if the period usage should be reset
// Uses PeriodLastUpdateTime to determine when the last period reset occurred
func (t *Token) NeedsPeriodReset() (bool, error) {
	// If never been reset, use PeriodStartTime as baseline
	baseTime := t.PeriodLastUpdateTime
	if baseTime.IsZero() {
		return true, nil // Never initialized
	}

	now := time.Now()

	switch t.PeriodType {
	case "", PeriodTypeMonthly:
		// Check if we've crossed a month boundary since last reset
		baseMonth := baseTime.Month()
		baseYear := baseTime.Year()
		currentMonth := now.Month()
		currentYear := now.Year()

		if currentYear > baseYear {
			return true, nil
		}

		if currentYear == baseYear && currentMonth > baseMonth {
			return true, nil
		}

		return false, nil
	case PeriodTypeWeekly:
		// Check if we've passed 7 days since last reset
		return now.Sub(baseTime) >= 7*24*time.Hour, nil
	case PeriodTypeDaily:
		// Check if we've crossed to a new day since last reset
		baseDate := baseTime.Truncate(24 * time.Hour)
		currentDate := now.Truncate(24 * time.Hour)
		return currentDate.After(baseDate), nil
	default:
		return false, fmt.Errorf("unknown period type: %s", t.PeriodType)
	}
}

const (
	keyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func generateKey() string {
	key := make([]byte, 48)
	for i := range key {
		key[i] = keyChars[rand.IntN(len(keyChars))]
	}

	return conv.BytesToString(key)
}

func getTokenOrder(order string) string {
	prefix, suffix, _ := strings.Cut(order, "-")
	switch prefix {
	case "name", "expired_at", "group", "used_amount", "request_count", "id", "created_at":
		switch suffix {
		case "asc":
			return prefix + " asc"
		default:
			return prefix + " desc"
		}
	default:
		return "id desc"
	}
}

func InsertToken(token *Token, autoCreateGroup, ignoreExist bool) error {
	if autoCreateGroup {
		group := &Group{
			ID: token.GroupID,
		}
		if err := OnConflictDoNothing().Create(group).Error; err != nil {
			return err
		}
	}

	maxTokenNum := config.GetGroupMaxTokenNum()

	err := DB.Transaction(func(tx *gorm.DB) error {
		if maxTokenNum > 0 {
			var count int64

			err := tx.Model(&Token{}).Where("group_id = ?", token.GroupID).Count(&count).Error
			if err != nil {
				return err
			}

			if count >= maxTokenNum {
				return errors.New("group max token num reached")
			}
		}

		if ignoreExist {
			return tx.
				Where("group_id = ? and name = ?", token.GroupID, token.Name).
				FirstOrCreate(token).Error
		}

		return tx.Create(token).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			if ignoreExist {
				return nil
			}
			return errors.New("token name already exists in this group")
		}

		return err
	}

	return nil
}

func GetTokens(
	group string,
	page, perPage int,
	order string,
	status int,
) (tokens []*Token, total int64, err error) {
	tx := DB.Model(&Token{})
	if group != "" {
		tx = tx.Where("group_id = ?", group)
	}

	if status != 0 {
		tx = tx.Where("status = ?", status)
	}

	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order(getTokenOrder(order)).Limit(limit).Offset(offset).Find(&tokens).Error

	return tokens, total, err
}

func SearchTokens(
	group, keyword string,
	page, perPage int,
	order string,
	status int,
	name, key string,
) (tokens []*Token, total int64, err error) {
	tx := DB.Model(&Token{})
	if group != "" {
		tx = tx.Where("group_id = ?", group)
	}

	if status != 0 {
		tx = tx.Where("status = ?", status)
	}

	if name != "" {
		tx = tx.Where("name = ?", name)
	}

	if key != "" {
		tx = tx.Where("key = ?", key)
	}

	if keyword != "" {
		var (
			conditions []string
			values     []any
		)

		if group == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "group_id ILIKE ?")
			} else {
				conditions = append(conditions, "group_id LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if name == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "name ILIKE ?")
			} else {
				conditions = append(conditions, "name LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if key == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "key ILIKE ?")
			} else {
				conditions = append(conditions, "key LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "models ILIKE ?")
		} else {
			conditions = append(conditions, "models LIKE ?")
		}

		values = append(values, "%"+keyword+"%")

		if len(conditions) > 0 {
			tx = tx.Where(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), values...)
		}
	}

	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order(getTokenOrder(order)).Limit(limit).Offset(offset).Find(&tokens).Error

	return tokens, total, err
}

func SearchGroupTokens(
	group, keyword string,
	page, perPage int,
	order string,
	status int,
	name, key string,
) (tokens []*Token, total int64, err error) {
	if group == "" {
		return nil, 0, errors.New("group is empty")
	}

	tx := DB.Model(&Token{}).
		Where("group_id = ?", group)
	if name != "" {
		tx = tx.Where("name = ?", name)
	}

	if key != "" {
		tx = tx.Where("key = ?", key)
	}

	if status != 0 {
		tx = tx.Where("status = ?", status)
	}

	if keyword != "" {
		var (
			conditions []string
			values     []any
		)

		if name == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "name ILIKE ?")
			} else {
				conditions = append(conditions, "name LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if key == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "key ILIKE ?")
			} else {
				conditions = append(conditions, "key LIKE ?")
			}

			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "models ILIKE ?")
		} else {
			conditions = append(conditions, "models LIKE ?")
		}

		values = append(values, "%"+keyword+"%")

		if len(conditions) > 0 {
			tx = tx.Where(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), values...)
		}
	}

	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}

	limit, offset := toLimitOffset(page, perPage)
	err = tx.Order(getTokenOrder(order)).Limit(limit).Offset(offset).Find(&tokens).Error

	return tokens, total, err
}

func GetTokenByKey(key string) (*Token, error) {
	if key == "" {
		return nil, errors.New("key is empty")
	}

	var token Token

	err := DB.Where("key = ?", key).First(&token).Error

	return &token, HandleNotFound(err, ErrTokenNotFound)
}

// GetAndValidateToken validates a token and checks quota limits
// This function is safe for concurrent use and handles period resets atomically
func GetAndValidateToken(key string) (token *TokenCache, err error) {
	if key == "" {
		return nil, errors.New("no token provided")
	}

	token, err = CacheGetTokenByKey(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid token")
		}

		log.Error("get token from cache failed: " + err.Error())

		return nil, errors.New("token validation failed")
	}

	if token.Status == TokenStatusDisabled {
		return nil, fmt.Errorf("token (%s[%d]) is disabled", token.Name, token.ID)
	}

	// Convert TokenCache to Token for quota checking
	tokenModel := Token{
		ID:                     token.ID,
		Quota:                  token.Quota,
		UsedAmount:             token.UsedAmount,
		PeriodQuota:            token.PeriodQuota,
		PeriodType:             EmptyNullString(token.PeriodType),
		PeriodLastUpdateTime:   time.Time(token.PeriodLastUpdateTime),
		PeriodLastUpdateAmount: token.PeriodLastUpdateAmount,
	}

	totalExceeded, periodExceeded, err := tokenModel.GetEffectiveQuotaStatus()
	if err != nil {
		return nil, fmt.Errorf("token (%s[%d]) quota check failed: %w", token.Name, token.ID, err)
	}

	if totalExceeded {
		return nil, fmt.Errorf("token (%s[%d]) total quota is exhausted", token.Name, token.ID)
	}

	if periodExceeded {
		return nil, fmt.Errorf("token (%s[%d]) period quota is exhausted", token.Name, token.ID)
	}

	return token, nil
}

func GetGroupTokenByID(group string, id int) (*Token, error) {
	if id == 0 || group == "" {
		return nil, errors.New("id or group is empty")
	}

	token := Token{}
	err := DB.
		Where("id = ? and group_id = ?", id, group).
		First(&token).Error

	return &token, HandleNotFound(err, ErrTokenNotFound)
}

func GetTokenByID(id int) (*Token, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}

	token := Token{ID: id}
	err := DB.First(&token, "id = ?", id).Error

	return &token, HandleNotFound(err, ErrTokenNotFound)
}

func UpdateTokenStatus(id, status int) (err error) {
	token := Token{ID: id}
	defer func() {
		if err == nil {
			if err := CacheUpdateTokenStatus(token.Key, status); err != nil {
				log.Error("update token status in cache failed: " + err.Error())
			}
		}
	}()

	result := DB.
		Model(&token).
		Clauses(clause.Returning{
			Columns: []clause.Column{
				{Name: "key"},
			},
		}).
		Where("id = ?", id).
		Updates(
			map[string]any{
				"status": status,
			},
		)

	return HandleUpdateResult(result, ErrTokenNotFound)
}

func UpdateGroupTokenStatus(group string, id, status int) (err error) {
	if id == 0 || group == "" {
		return errors.New("id or group is empty")
	}

	token := Token{}
	defer func() {
		if err == nil {
			if err := CacheUpdateTokenStatus(token.Key, status); err != nil {
				log.Error("update token status in cache failed: " + err.Error())
			}
		}
	}()

	result := DB.
		Model(&token).
		Clauses(clause.Returning{
			Columns: []clause.Column{
				{Name: "key"},
			},
		}).
		Where("id = ? and group_id = ?", id, group).
		Updates(
			map[string]any{
				"status": status,
			},
		)

	return HandleUpdateResult(result, ErrTokenNotFound)
}

func DeleteGroupTokenByID(groupID string, id int) (err error) {
	if id == 0 || groupID == "" {
		return errors.New("id or group is empty")
	}

	token := Token{ID: id, GroupID: groupID}
	defer func() {
		if err == nil {
			if err := CacheDeleteToken(token.Key); err != nil {
				log.Error("delete token from cache failed: " + err.Error())
			}
		}
	}()

	result := DB.
		Clauses(clause.Returning{
			Columns: []clause.Column{
				{Name: "key"},
			},
		}).
		Where(token).
		Delete(&token)

	return HandleUpdateResult(result, ErrTokenNotFound)
}

func DeleteGroupTokensByIDs(group string, ids []int) (err error) {
	if group == "" {
		return errors.New("group is empty")
	}

	if len(ids) == 0 {
		return nil
	}

	tokens := make([]Token, len(ids))
	defer func() {
		if err == nil {
			for _, token := range tokens {
				if err := CacheDeleteToken(token.Key); err != nil {
					log.Error("delete token from cache failed: " + err.Error())
				}
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.
			Clauses(clause.Returning{
				Columns: []clause.Column{
					{Name: "key"},
				},
			}).
			Where("group_id = ?", group).
			Where("id IN (?)", ids).
			Delete(&tokens).
			Error
	})
}

func DeleteTokenByID(id int) (err error) {
	if id == 0 {
		return errors.New("id is empty")
	}

	token := Token{ID: id}
	defer func() {
		if err == nil {
			if err := CacheDeleteToken(token.Key); err != nil {
				log.Error("delete token from cache failed: " + err.Error())
			}
		}
	}()

	result := DB.
		Clauses(clause.Returning{
			Columns: []clause.Column{
				{Name: "key"},
			},
		}).
		Where(token).
		Delete(&token)

	return HandleUpdateResult(result, ErrTokenNotFound)
}

func DeleteTokensByIDs(ids []int) (err error) {
	if len(ids) == 0 {
		return nil
	}

	tokens := make([]Token, len(ids))
	defer func() {
		if err == nil {
			for _, token := range tokens {
				if err := CacheDeleteToken(token.Key); err != nil {
					log.Error("delete token from cache failed: " + err.Error())
				}
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.
			Clauses(clause.Returning{
				Columns: []clause.Column{
					{Name: "key"},
				},
			}).
			Where("id IN (?)", ids).
			Delete(&tokens).
			Error
	})
}

type UpdateTokenRequest struct {
	Name    *string   `json:"name"`
	Subnets *[]string `json:"subnets"`
	Models  *[]string `json:"models"`
	Status  int       `json:"status"`
	// Quota system
	Quota                *float64 `json:"quota"`
	PeriodQuota          *float64 `json:"period_quota"`
	PeriodType           *string  `json:"period_type"`
	PeriodLastUpdateTime *int64   `json:"period_last_update_time"`
}

func UpdateToken(id int, update UpdateTokenRequest) (token *Token, err error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}

	token = &Token{
		ID:     id,
		Status: update.Status,
	}

	defer func() {
		if err == nil {
			if err := CacheDeleteToken(token.Key); err != nil {
				log.Error("delete token from cache failed: " + err.Error())
			}
		}
	}()

	selects := []string{}
	if update.Name != nil && *update.Name != "" {
		token.Name = EmptyNullString(*update.Name)

		selects = append(selects, "name")
	}

	if update.Quota != nil {
		token.Quota = *update.Quota

		selects = append(selects, "quota")
	}

	if update.PeriodQuota != nil {
		token.PeriodQuota = *update.PeriodQuota

		selects = append(selects, "period_quota")
	}

	if update.PeriodType != nil {
		token.PeriodType = EmptyNullString(*update.PeriodType)

		selects = append(selects, "period_type")
	}

	if update.PeriodLastUpdateTime != nil {
		token.PeriodLastUpdateTime = time.UnixMilli(*update.PeriodLastUpdateTime)

		selects = append(selects, "period_last_update_time")
	}

	if update.Subnets != nil {
		token.Subnets = *update.Subnets

		selects = append(selects, "subnets")
	}

	if update.Models != nil {
		token.Models = *update.Models

		selects = append(selects, "models")
	}

	if update.Status != 0 {
		selects = append(selects, "status")
	}

	if len(selects) == 0 {
		return nil, errors.New("empty update request")
	}

	result := DB.
		Select(selects).
		Where("id = ?", id).
		Clauses(clause.Returning{}).
		Updates(token)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return nil, errors.New("token name already exists in this group")
		}
	}

	return token, HandleUpdateResult(result, ErrTokenNotFound)
}

func UpdateGroupToken(
	id int,
	group string,
	update UpdateTokenRequest,
) (token *Token, err error) {
	if id == 0 || group == "" {
		return nil, errors.New("id or group is empty")
	}

	token = &Token{
		ID:      id,
		GroupID: group,
		Status:  update.Status,
	}

	defer func() {
		if err == nil {
			if err := CacheDeleteToken(token.Key); err != nil {
				log.Error("delete token from cache failed: " + err.Error())
			}
		}
	}()

	selects := []string{}
	if update.Name != nil && *update.Name != "" {
		token.Name = EmptyNullString(*update.Name)

		selects = append(selects, "name")
	}

	if update.Quota != nil {
		token.Quota = *update.Quota

		selects = append(selects, "quota")
	}

	if update.PeriodQuota != nil {
		token.PeriodQuota = *update.PeriodQuota

		selects = append(selects, "period_quota")
	}

	if update.PeriodType != nil {
		token.PeriodType = EmptyNullString(*update.PeriodType)

		selects = append(selects, "period_type")
	}

	if update.PeriodLastUpdateTime != nil {
		token.PeriodLastUpdateTime = time.UnixMilli(*update.PeriodLastUpdateTime)

		selects = append(selects, "period_last_update_time")
	}

	if update.Subnets != nil {
		token.Subnets = *update.Subnets

		selects = append(selects, "subnets")
	}

	if update.Models != nil {
		token.Models = *update.Models

		selects = append(selects, "models")
	}

	if update.Status != 0 {
		selects = append(selects, "status")
	}

	if len(selects) == 0 {
		return nil, errors.New("empty update request")
	}

	result := DB.
		Select(selects).
		Where("id = ? and group_id = ?", id, group).
		Clauses(clause.Returning{}).
		Updates(token)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return nil, errors.New("token name already exists in this group")
		}
	}

	return token, HandleUpdateResult(result, ErrTokenNotFound)
}

func UpdateTokenUsedAmount(id int, amount float64, requestCount int) (err error) {
	token := &Token{}
	defer func() {
		if amount > 0 && err == nil && (token.Quota > 0 || token.PeriodQuota > 0) {
			if err := CacheUpdateTokenUsedAmountOnlyIncrease(
				token.Key,
				token.UsedAmount,
			); err != nil {
				log.Error("update token used amount in cache failed: " + err.Error())
			}
		}
	}()

	result := DB.
		Model(token).
		Clauses(clause.Returning{
			Columns: []clause.Column{
				{Name: "key"},
				{Name: "quota"},
				{Name: "used_amount"},
				{Name: "period_quota"},
			},
		}).
		Where("id = ?", id).
		Updates(
			map[string]any{
				"used_amount":   gorm.Expr("used_amount + ?", amount),
				"request_count": gorm.Expr("request_count + ?", requestCount),
			},
		)

	return HandleUpdateResult(result, ErrTokenNotFound)
}

// calculateNextPeriodStartTime calculates the next period start time based on the last update time and period type
// This finds the most recent period boundary by incrementing from lastUpdateTime until we reach the current time
// This maintains period continuity - e.g., if reset was on Jan 15, next periods are Feb 15, Mar 15, etc.
func calculateNextPeriodStartTime(lastUpdateTime time.Time, periodType EmptyNullString) time.Time {
	if lastUpdateTime.IsZero() {
		// If never initialized, return current time
		return time.Now()
	}

	now := time.Now()

	// If we haven't passed the period yet, no reset needed
	if !now.After(lastUpdateTime) {
		return lastUpdateTime
	}

	switch periodType {
	case "", PeriodTypeMonthly:
		// Start from lastUpdateTime and keep adding months until we find the most recent period start
		nextPeriod := lastUpdateTime
		for {
			// Calculate next month period
			candidate := time.Date(
				nextPeriod.Year(),
				nextPeriod.Month()+1,
				nextPeriod.Day(),
				nextPeriod.Hour(),
				nextPeriod.Minute(),
				nextPeriod.Second(),
				nextPeriod.Nanosecond(),
				nextPeriod.Location(),
			)

			// If candidate is in the future, the current nextPeriod is the one we want
			if candidate.After(now) {
				return nextPeriod
			}

			nextPeriod = candidate
		}

	case PeriodTypeWeekly:
		// Calculate how many complete weeks have passed since lastUpdateTime
		daysSinceLastUpdate := now.Sub(lastUpdateTime).Hours() / 24
		weeksPassed := int(daysSinceLastUpdate / 7)

		if weeksPassed == 0 {
			// Still in the same week period, no reset needed
			return lastUpdateTime
		}

		// Return the start of the most recent week period
		// This is lastUpdateTime + (weeksPassed * 7 days)
		return lastUpdateTime.Add(time.Duration(weeksPassed*7*24) * time.Hour)

	case PeriodTypeDaily:
		// Calculate how many complete days have passed since lastUpdateTime
		daysSinceLastUpdate := int(now.Sub(lastUpdateTime).Hours() / 24)

		if daysSinceLastUpdate == 0 {
			// Still in the same day period, no reset needed
			return lastUpdateTime
		}

		// Return the start of the most recent day period
		// This is lastUpdateTime + (daysPassed * 1 day)
		return lastUpdateTime.Add(time.Duration(daysSinceLastUpdate*24) * time.Hour)

	default:
		// Fallback to current time for unknown period types
		return now
	}
}

// ResetTokenPeriodUsage resets the period usage for a token with concurrency safety
// This updates PeriodLastUpdateTime and PeriodLastUpdateAmount to current values
func ResetTokenPeriodUsage(id int) error {
	token := &Token{}

	var newPeriodStartTime time.Time

	// Use database transaction with optimistic locking to prevent concurrent resets
	err := DB.Transaction(func(tx *gorm.DB) error {
		// First, read the current state with FOR UPDATE lock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).
			First(token).Error; err != nil {
			return err
		}

		// Check if period still needs reset (another concurrent request might have already reset it)
		needsReset, err := token.NeedsPeriodReset()
		if err != nil {
			return err
		}

		// If period no longer needs reset, skip the update
		if !needsReset {
			return nil
		}

		// Calculate the correct next period start time based on period type
		newPeriodStartTime = calculateNextPeriodStartTime(
			token.PeriodLastUpdateTime,
			token.PeriodType,
		)

		if newPeriodStartTime.IsZero() {
			return errors.New("next period start time is zero")
		}

		// Perform the reset with the lock held - update period last update time and amount
		result := tx.
			Model(token).
			Clauses(clause.Returning{
				Columns: []clause.Column{
					{Name: "key"},
				},
			}).
			Where("id = ?", id).
			Updates(
				map[string]any{
					"period_last_update_time": newPeriodStartTime,
					"period_last_update_amount": gorm.Expr(
						"used_amount",
					), // Set to current total usage
				},
			)

		return HandleUpdateResult(result, ErrTokenNotFound)
	})

	// Update cache only if database update succeeded
	if err == nil && token.Key != "" && !newPeriodStartTime.IsZero() {
		if cacheErr := CacheResetTokenPeriodUsage(
			token.Key,
			newPeriodStartTime,
			token.UsedAmount,
		); cacheErr != nil {
			log.Error("reset token period usage in cache failed: " + cacheErr.Error())
		}
	}

	return err
}

func UpdateTokenName(id int, name string) (err error) {
	token := &Token{ID: id}
	defer func() {
		if err == nil {
			if err := CacheUpdateTokenName(token.Key, name); err != nil {
				log.Error("update token name in cache failed: " + err.Error())
			}
		}
	}()

	result := DB.
		Model(token).
		Clauses(clause.Returning{
			Columns: []clause.Column{
				{Name: "key"},
			},
		}).
		Where("id = ?", id).
		Update("name", name)
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return errors.New("token name already exists in this group")
	}

	return HandleUpdateResult(result, ErrTokenNotFound)
}

func UpdateGroupTokenName(group string, id int, name string) (err error) {
	token := &Token{ID: id, GroupID: group}
	defer func() {
		if err == nil {
			if err := CacheUpdateTokenName(token.Key, name); err != nil {
				log.Error("update token name in cache failed: " + err.Error())
			}
		}
	}()

	result := DB.
		Model(token).
		Clauses(clause.Returning{
			Columns: []clause.Column{
				{Name: "key"},
			},
		}).
		Where("id = ? and group_id = ?", id, group).
		Update("name", name)
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return errors.New("token name already exists in this group")
	}

	return HandleUpdateResult(result, ErrTokenNotFound)
}
