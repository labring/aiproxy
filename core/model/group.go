package model

import (
	"errors"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ErrGroupNotFound = "group"
)

const (
	GroupStatusEnabled  = 1
	GroupStatusDisabled = 2
	GroupStatusInternal = 3
)

type Group struct {
	CreatedAt              time.Time               `json:"created_at"`
	ID                     string                  `json:"id"                       gorm:"primaryKey"`
	Tokens                 []Token                 `json:"-"                        gorm:"foreignKey:GroupID"`
	GroupModelConfigs      []GroupModelConfig      `json:"-"                        gorm:"foreignKey:GroupID"`
	PublicMCPReusingParams []PublicMCPReusingParam `json:"-"                        gorm:"foreignKey:GroupID"`
	GroupMCPs              []GroupMCP              `json:"-"                        gorm:"foreignKey:GroupID"`
	Status                 int                     `json:"status"                   gorm:"default:1;index"`
	RPMRatio               float64                 `json:"rpm_ratio,omitempty"      gorm:"index"`
	TPMRatio               float64                 `json:"tpm_ratio,omitempty"      gorm:"index"`
	UsedAmount             float64                 `json:"used_amount"              gorm:"index"`
	RequestCount           int                     `json:"request_count"            gorm:"index"`
	AvailableSets          []string                `json:"available_sets,omitempty" gorm:"serializer:fastjson;type:text"`

	BalanceAlertEnabled   bool    `gorm:"default:false" json:"balance_alert_enabled"`
	BalanceAlertThreshold float64 `gorm:"default:0"     json:"balance_alert_threshold"`
}

func (g *Group) BeforeDelete(tx *gorm.DB) (err error) {
	err = tx.Model(&Token{}).Where("group_id = ?", g.ID).Delete(&Token{}).Error
	if err != nil {
		return err
	}
	err = tx.Model(&PublicMCPReusingParam{}).
		Where("group_id = ?", g.ID).
		Delete(&PublicMCPReusingParam{}).
		Error
	if err != nil {
		return err
	}
	err = tx.Model(&GroupMCP{}).Where("group_id = ?", g.ID).Delete(&GroupMCP{}).Error
	if err != nil {
		return err
	}
	return tx.Model(&GroupModelConfig{}).
		Where("group_id = ?", g.ID).
		Delete(&GroupModelConfig{}).
		Error
}

func getGroupOrder(order string) string {
	prefix, suffix, _ := strings.Cut(order, "-")
	switch prefix {
	case "id", "request_count", "status", "created_at", "used_amount":
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

func GetGroups(
	page, perPage int,
	order string,
	onlyDisabled bool,
) (groups []*Group, total int64, err error) {
	tx := DB.Model(&Group{})
	if onlyDisabled {
		tx = tx.Where("status = ?", GroupStatusDisabled)
	}

	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	if total <= 0 {
		return nil, 0, nil
	}
	limit, offset := toLimitOffset(page, perPage)
	err = tx.
		Order(getGroupOrder(order)).
		Limit(limit).
		Offset(offset).
		Find(&groups).
		Error
	return groups, total, err
}

func GetGroupByID(id string, preloadGroupModelConfigs bool) (*Group, error) {
	if id == "" {
		return nil, errors.New("group id is empty")
	}
	group := Group{}
	tx := DB.Where("id = ?", id)
	if preloadGroupModelConfigs {
		tx = tx.Preload("GroupModelConfigs")
	}
	err := tx.First(&group).Error
	return &group, HandleNotFound(err, ErrGroupNotFound)
}

func DeleteGroupByID(id string) (err error) {
	if id == "" {
		return errors.New("group id is empty")
	}
	defer func() {
		if err == nil {
			if err := CacheDeleteGroup(id); err != nil {
				log.Error("cache delete group failed: " + err.Error())
			}
			if _, err := DeleteGroupLogs(id); err != nil {
				log.Error("delete group logs failed: " + err.Error())
			}
		}
	}()
	result := DB.Delete(&Group{ID: id})
	return HandleUpdateResult(result, ErrGroupNotFound)
}

func DeleteGroupsByIDs(ids []string) (err error) {
	if len(ids) == 0 {
		return nil
	}
	groups := make([]Group, len(ids))
	defer func() {
		if err == nil {
			for _, group := range groups {
				if err := CacheDeleteGroup(group.ID); err != nil {
					log.Error("cache delete group failed: " + err.Error())
				}
				if _, err := DeleteGroupLogs(group.ID); err != nil {
					log.Error("delete group logs failed: " + err.Error())
				}
			}
		}
	}()
	return DB.Transaction(func(tx *gorm.DB) error {
		return tx.
			Clauses(clause.Returning{
				Columns: []clause.Column{
					{Name: "id"},
				},
			}).
			Where("id IN (?)", ids).
			Delete(&groups).
			Error
	})
}

func UpdateGroup(id string, group *Group) (err error) {
	if id == "" {
		return errors.New("group id is empty")
	}
	defer func() {
		if err == nil {
			if err := CacheDeleteGroup(id); err != nil {
				log.Error("cache delete group failed: " + err.Error())
			}
		}
	}()
	selects := []string{
		"rpm_ratio",
		"tpm_ratio",
		"available_sets",
		"balance_alert_enabled",
		"balance_alert_threshold",
	}
	if group.Status != 0 {
		selects = append(selects, "status")
	}
	result := DB.
		Clauses(clause.Returning{}).
		Where("id = ?", id).
		Select(selects).
		Updates(group)
	return HandleUpdateResult(result, ErrGroupNotFound)
}

func UpdateGroupUsedAmountAndRequestCount(id string, amount float64, count int) (err error) {
	group := &Group{}
	defer func() {
		if amount > 0 && err == nil {
			if err := CacheUpdateGroupUsedAmountOnlyIncrease(group.ID, group.UsedAmount); err != nil {
				log.Error("update group used amount in cache failed: " + err.Error())
			}
		}
	}()
	result := DB.
		Model(group).
		Clauses(clause.Returning{
			Columns: []clause.Column{
				{Name: "used_amount"},
			},
		}).
		Where("id = ?", id).
		Updates(map[string]any{
			"used_amount":   gorm.Expr("used_amount + ?", amount),
			"request_count": gorm.Expr("request_count + ?", count),
		})
	return HandleUpdateResult(result, ErrGroupNotFound)
}

func UpdateGroupRPMRatio(id string, rpmRatio float64) (err error) {
	defer func() {
		if err == nil {
			if err := CacheUpdateGroupRPMRatio(id, rpmRatio); err != nil {
				log.Error("cache update group rpm failed: " + err.Error())
			}
		}
	}()
	result := DB.Model(&Group{}).Where("id = ?", id).Update("rpm_ratio", rpmRatio)
	return HandleUpdateResult(result, ErrGroupNotFound)
}

func UpdateGroupTPMRatio(id string, tpmRatio float64) (err error) {
	defer func() {
		if err == nil {
			if err := CacheUpdateGroupTPMRatio(id, tpmRatio); err != nil {
				log.Error("cache update group tpm ratio failed: " + err.Error())
			}
		}
	}()
	result := DB.Model(&Group{}).Where("id = ?", id).Update("tpm_ratio", tpmRatio)
	return HandleUpdateResult(result, ErrGroupNotFound)
}

func UpdateGroupStatus(id string, status int) (err error) {
	defer func() {
		if err == nil {
			if err := CacheUpdateGroupStatus(id, status); err != nil {
				log.Error("cache update group status failed: " + err.Error())
			}
		}
	}()
	result := DB.Model(&Group{}).Where("id = ?", id).Update("status", status)
	return HandleUpdateResult(result, ErrGroupNotFound)
}

func UpdateGroupsStatus(ids []string, status int) (rowsAffected int64, err error) {
	defer func() {
		if err == nil {
			for _, id := range ids {
				if err := CacheUpdateGroupStatus(id, status); err != nil {
					log.Error("cache update group status failed: " + err.Error())
				}
			}
		}
	}()

	result := DB.Model(&Group{}).
		Where("id IN (?) AND status != ?", ids, status).
		Update("status", status)
	return result.RowsAffected, result.Error
}

func SearchGroup(
	keyword string,
	page, perPage int,
	order string,
	status int,
) (groups []*Group, total int64, err error) {
	tx := DB.Model(&Group{})
	if status != 0 {
		tx = tx.Where("status = ?", status)
	}
	if common.UsingPostgreSQL {
		tx = tx.Where("id ILIKE ? OR available_sets ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	} else {
		tx = tx.Where("id LIKE ? OR available_sets LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	if total <= 0 {
		return nil, 0, nil
	}
	limit, offset := toLimitOffset(page, perPage)
	err = tx.
		Order(getGroupOrder(order)).
		Limit(limit).
		Offset(offset).
		Find(&groups).
		Error
	return groups, total, err
}

func CreateGroup(group *Group) error {
	return DB.Create(group).Error
}
