package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/monitor"
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
	CreatedAt                time.Time               `json:"created_at"`
	ID                       string                  `json:"id"                          gorm:"size:64;primaryKey"`
	Tokens                   []Token                 `json:"-"                           gorm:"foreignKey:GroupID"`
	GroupModelConfigs        []GroupModelConfig      `json:"-"                           gorm:"foreignKey:GroupID"`
	PublicMCPReusingParams   []PublicMCPReusingParam `json:"-"                           gorm:"foreignKey:GroupID"`
	GroupMCPs                []GroupMCP              `json:"-"                           gorm:"foreignKey:GroupID"`
	Status                   int                     `json:"status"                      gorm:"default:1;index"`
	RPMRatio                 float64                 `json:"rpm_ratio,omitempty"         gorm:"index"`
	TPMRatio                 float64                 `json:"tpm_ratio,omitempty"         gorm:"index"`
	UsedAmount               float64                 `json:"used_amount"                 gorm:"index"`
	RequestCount             int                     `json:"request_count"               gorm:"index"`
	GroupChannelUsedAmount   float64                 `json:"group_channel_used_amount"   gorm:"index"`
	GroupChannelRequestCount int                     `json:"group_channel_request_count" gorm:"index"`
	AvailableSets            []string                `json:"available_sets,omitempty"    gorm:"serializer:fastjson;type:text"`

	BalanceAlertEnabled   bool    `gorm:"default:false" json:"balance_alert_enabled"`
	BalanceAlertThreshold float64 `gorm:"default:0"     json:"balance_alert_threshold"`
}

func (g *Group) BeforeSave(_ *gorm.DB) error {
	if len(g.ID) > 64 {
		return errors.New("group id length too long")
	}
	return nil
}

func (g *Group) BeforeDelete(tx *gorm.DB) (err error) {
	if g.ID == "" {
		return nil
	}
	return deleteGroupsOwnedState(tx, []string{g.ID})
}

func deleteGroupsOwnedState(tx *gorm.DB, groupIDs []string) error {
	if len(groupIDs) == 0 {
		return nil
	}

	err := tx.Model(&Token{}).Where("group_id IN ?", groupIDs).Delete(&Token{}).Error
	if err != nil {
		return err
	}

	err = tx.Model(&PublicMCPReusingParam{}).
		Where("group_id IN ?", groupIDs).
		Delete(&PublicMCPReusingParam{}).
		Error
	if err != nil {
		return err
	}

	err = tx.Model(&GroupMCP{}).Where("group_id IN ?", groupIDs).Delete(&GroupMCP{}).Error
	if err != nil {
		return err
	}

	err = tx.Model(&GroupModelConfig{}).
		Where("group_id IN ?", groupIDs).
		Delete(&GroupModelConfig{}).
		Error
	if err != nil {
		return err
	}

	err = tx.Model(&GroupChannelTest{}).
		Where("group_id IN ?", groupIDs).
		Delete(&GroupChannelTest{}).
		Error
	if err != nil {
		return err
	}

	err = tx.Session(&gorm.Session{SkipHooks: true}).
		Unscoped().
		Where("group_id IN ?", groupIDs).
		Delete(&GroupChannel{}).
		Error
	if err != nil {
		return err
	}

	return tx.Model(&GroupScopeModelConfig{}).
		Where("group_id IN ?", groupIDs).
		Delete(&GroupScopeModelConfig{}).
		Error
}

type groupChannelIDRow struct {
	GroupID string
	ID      int
}

func getGroupChannelIDsByGroups(tx *gorm.DB, groupIDs []string) ([]int, error) {
	rows, err := getGroupChannelIDRows(tx, groupIDs)
	if err != nil {
		return nil, err
	}

	channelIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		channelIDs = append(channelIDs, row.ID)
	}

	return channelIDs, nil
}

func getGroupChannelIDsByGroupMap(
	tx *gorm.DB,
	groupIDs []string,
) (map[string][]int, error) {
	rows, err := getGroupChannelIDRows(tx, groupIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]int, len(groupIDs))
	for _, row := range rows {
		result[row.GroupID] = append(result[row.GroupID], row.ID)
	}

	return result, nil
}

func getGroupChannelIDRows(tx *gorm.DB, groupIDs []string) ([]groupChannelIDRow, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	var rows []groupChannelIDRow

	err := tx.
		Model(&GroupChannel{}).
		Select("group_id", "id").
		Where("group_id IN ?", groupIDs).
		Find(&rows).
		Error

	return rows, err
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

	var channelIDs []int
	defer func() {
		if err == nil {
			if err := CacheDeleteGroup(id); err != nil {
				log.Error("cache delete group failed: " + err.Error())
			}

			if err := CacheDeleteGroupChannels(id); err != nil {
				log.Error("cache delete group channels failed: " + err.Error())
			}

			if err := CacheDeleteGroupScopeModelConfig(id); err != nil {
				log.Error("cache delete group scope model config failed: " + err.Error())
			}

			for _, channelID := range channelIDs {
				_ = monitor.ClearGroupChannelAllModelErrorsByKey(
					context.Background(),
					GroupChannelMonitorKey(id, channelID),
				)
			}

			if _, err := DeleteGroupLogs(id); err != nil {
				log.Error("delete group logs failed: " + err.Error())
			}

			if _, err := DeleteGroupChannelLogs(id); err != nil {
				log.Error("delete group channel logs failed: " + err.Error())
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Model(&Group{}).
			Select("id").
			Where("id = ?", id).
			First(&Group{}).
			Error; err != nil {
			return HandleNotFound(err, ErrGroupNotFound)
		}

		var err error

		channelIDs, err = getGroupChannelIDsByGroups(tx, []string{id})
		if err != nil {
			return err
		}

		if err := deleteGroupsOwnedState(tx, []string{id}); err != nil {
			return err
		}

		result := tx.Session(&gorm.Session{SkipHooks: true}).Delete(&Group{ID: id})

		return HandleUpdateResult(result, ErrGroupNotFound)
	})
}

func DeleteGroupsByIDs(ids []string) (err error) {
	if len(ids) == 0 {
		return nil
	}

	var (
		groupIDs          []string
		channelIDsByGroup map[string][]int
	)
	defer func() {
		if err == nil {
			for _, groupID := range groupIDs {
				if err := CacheDeleteGroup(groupID); err != nil {
					log.Error("cache delete group failed: " + err.Error())
				}

				if err := CacheDeleteGroupChannels(groupID); err != nil {
					log.Error("cache delete group channels failed: " + err.Error())
				}

				if err := CacheDeleteGroupScopeModelConfig(groupID); err != nil {
					log.Error("cache delete group scope model config failed: " + err.Error())
				}

				for _, channelID := range channelIDsByGroup[groupID] {
					_ = monitor.ClearGroupChannelAllModelErrorsByKey(
						context.Background(),
						GroupChannelMonitorKey(groupID, channelID),
					)
				}

				if _, err := DeleteGroupLogs(groupID); err != nil {
					log.Error("delete group logs failed: " + err.Error())
				}

				if _, err := DeleteGroupChannelLogs(groupID); err != nil {
					log.Error("delete group channel logs failed: " + err.Error())
				}
			}
		}
	}()

	return DB.Transaction(func(tx *gorm.DB) error {
		var groups []Group
		if err := tx.
			Model(&Group{}).
			Select("id").
			Where("id IN ?", ids).
			Find(&groups).
			Error; err != nil {
			return err
		}

		if len(groups) == 0 {
			return nil
		}

		groupIDs = make([]string, 0, len(groups))
		for _, group := range groups {
			groupIDs = append(groupIDs, group.ID)
		}

		var err error

		channelIDsByGroup, err = getGroupChannelIDsByGroupMap(tx, groupIDs)
		if err != nil {
			return err
		}

		if err := deleteGroupsOwnedState(tx, groupIDs); err != nil {
			return err
		}

		return tx.Session(&gorm.Session{SkipHooks: true}).
			Where("id IN ?", groupIDs).
			Delete(&Group{}).
			Error
	})
}

type UpdateGroupRequest struct {
	Status                int       `json:"status"`
	RPMRatio              *float64  `json:"rpm_ratio,omitempty"`
	TPMRatio              *float64  `json:"tpm_ratio,omitempty"`
	AvailableSets         *[]string `json:"available_sets,omitempty"`
	BalanceAlertEnabled   *bool     `json:"balance_alert_enabled"`
	BalanceAlertThreshold *float64  `json:"balance_alert_threshold"`
}

func UpdateGroup(id string, update UpdateGroupRequest) (group *Group, err error) {
	if id == "" {
		return nil, errors.New("group id is empty")
	}

	group = &Group{
		ID:     id,
		Status: update.Status,
	}

	defer func() {
		if err == nil {
			if err := CacheDeleteGroup(id); err != nil {
				log.Error("cache delete group failed: " + err.Error())
			}
		}
	}()

	selects := []string{}
	if update.RPMRatio != nil {
		group.RPMRatio = *update.RPMRatio

		selects = append(selects, "rpm_ratio")
	}

	if update.TPMRatio != nil {
		group.TPMRatio = *update.TPMRatio

		selects = append(selects, "tpm_ratio")
	}

	if update.AvailableSets != nil {
		group.AvailableSets = *update.AvailableSets

		selects = append(selects, "available_sets")
	}

	if update.BalanceAlertEnabled != nil {
		group.BalanceAlertEnabled = *update.BalanceAlertEnabled

		selects = append(selects, "balance_alert_enabled")
	}

	if update.BalanceAlertThreshold != nil {
		group.BalanceAlertThreshold = *update.BalanceAlertThreshold

		selects = append(selects, "balance_alert_threshold")
	}

	if group.Status != 0 {
		selects = append(selects, "status")
	}

	result := DB.
		Clauses(clause.Returning{}).
		Where("id = ?", id).
		Select(selects).
		Updates(group)

	return group, HandleUpdateResult(result, ErrGroupNotFound)
}

func UpdateGroupUsedAmountAndRequestCount(id string, amount float64, count int) (err error) {
	group := &Group{}
	defer func() {
		if amount > 0 && err == nil {
			if err := CacheUpdateGroupUsedAmountOnlyIncrease(
				group.ID,
				group.UsedAmount,
			); err != nil {
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

func UpdateGroupChannelGroupUsedAmountAndRequestCount(
	id string,
	amount float64,
	count int,
) error {
	result := DB.
		Model(&Group{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"group_channel_used_amount":   gorm.Expr("group_channel_used_amount + ?", amount),
			"group_channel_request_count": gorm.Expr("group_channel_request_count + ?", count),
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

	if !common.UsingSQLite {
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
