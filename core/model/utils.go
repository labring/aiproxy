package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NotFoundError(errMsg ...string) error {
	return fmt.Errorf("%s %w", strings.Join(errMsg, " "), gorm.ErrRecordNotFound)
}

func HandleNotFound(err error, errMsg ...string) error {
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return NotFoundError(strings.Join(errMsg, " "))
	}
	return err
}

func HandleUpdateResult(result *gorm.DB, entityName string) error {
	if result.Error != nil {
		return HandleNotFound(result.Error, entityName)
	}
	if result.RowsAffected == 0 {
		return NotFoundError(entityName)
	}
	return nil
}

func OnConflictDoNothing() *gorm.DB {
	return DB.Clauses(clause.OnConflict{
		DoNothing: true,
	})
}

func IgnoreNotFound(err error) error {
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

type ZeroNullFloat64 float64

func (znf ZeroNullFloat64) Value() (driver.Value, error) {
	if znf == 0 {
		return nil, nil
	}
	return float64(znf), nil
}

func (znf *ZeroNullFloat64) Scan(value any) error {
	if value == nil {
		*znf = 0
		return nil
	}
	switch v := value.(type) {
	case string:
		vf, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		*znf = ZeroNullFloat64(vf)
	case int:
		*znf = ZeroNullFloat64(v)
	case int64:
		*znf = ZeroNullFloat64(v)
	case float32:
		*znf = ZeroNullFloat64(v)
	case float64:
		*znf = ZeroNullFloat64(v)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

type ZeroNullInt64 int64

func (zni ZeroNullInt64) Value() (driver.Value, error) {
	if zni == 0 {
		return nil, nil
	}
	return int64(zni), nil
}

func (zni *ZeroNullInt64) Scan(value any) error {
	if value == nil {
		*zni = 0
		return nil
	}
	switch v := value.(type) {
	case int:
		*zni = ZeroNullInt64(v)
	case int64:
		*zni = ZeroNullInt64(v)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

type EmptyNullString string

func (ns EmptyNullString) String() string {
	return string(ns)
}

// Scan implements the [Scanner] interface.
func (ns *EmptyNullString) Scan(value any) error {
	if value == nil {
		*ns = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*ns = EmptyNullString(v)
	case string:
		*ns = EmptyNullString(v)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

// Value implements the [driver.Valuer] interface.
func (ns EmptyNullString) Value() (driver.Value, error) {
	if ns == "" {
		return nil, nil
	}
	return string(ns), nil
}

func String2Int(keyword string) int {
	if keyword == "" {
		return 0
	}
	i, err := strconv.Atoi(keyword)
	if err != nil {
		return 0
	}
	return i
}

func toLimitOffset(page, perPage int) (limit, offset int) {
	page--
	if page < 0 {
		page = 0
	}
	if perPage <= 0 {
		perPage = 10
	} else if perPage > 100 {
		perPage = 100
	}
	return perPage, page * perPage
}
