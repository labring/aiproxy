package model

import (
	"errors"
	"time"

	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type GroupMCPStatus int

const (
	GroupMCPStatusEnabled GroupMCPStatus = iota + 1
	GroupMCPStatusDisabled
)

const (
	ErrGroupMCPNotFound = "group mcp"
)

type GroupMCPType string

const (
	GroupMCPTypeProxySSE        GroupMCPType = "mcp_proxy_sse"
	GroupMCPTypeProxyStreamable GroupMCPType = "mcp_proxy_streamable"
	GroupMCPTypeOpenAPI         GroupMCPType = "mcp_openapi"
)

type GroupMCPProxyConfig struct {
	URL     string            `json:"url"`
	Querys  map[string]string `json:"querys"`
	Headers map[string]string `json:"headers"`
}

type GroupMCP struct {
	ID            string               `gorm:"primaryKey"                    json:"id"`
	GroupID       string               `gorm:"primaryKey"                    json:"group_id"`
	Group         *Group               `gorm:"foreignKey:GroupID"            json:"-"`
	Status        GroupMCPStatus       `gorm:"index;default:1"               json:"status"`
	CreatedAt     time.Time            `gorm:"index,autoCreateTime"          json:"created_at"`
	UpdateAt      time.Time            `gorm:"index,autoUpdateTime"          json:"update_at"`
	Name          string               `                                     json:"name"`
	Type          GroupMCPType         `gorm:"index"                         json:"type"`
	ProxyConfig   *GroupMCPProxyConfig `gorm:"serializer:fastjson;type:text" json:"proxy_config,omitempty"`
	OpenAPIConfig *MCPOpenAPIConfig    `gorm:"serializer:fastjson;type:text" json:"openapi_config,omitempty"`
}

func (g *GroupMCP) BeforeSave(_ *gorm.DB) (err error) {
	if g.GroupID == "" {
		return errors.New("group id is empty")
	}
	if err := validateMCPID(g.ID); err != nil {
		return err
	}
	if g.UpdateAt.IsZero() {
		g.UpdateAt = time.Now()
	}

	if g.Status == 0 {
		g.Status = GroupMCPStatusEnabled
	}

	if g.OpenAPIConfig != nil {
		config := g.OpenAPIConfig
		if config.OpenAPISpec != "" {
			return validateHTTPURL(config.OpenAPISpec)
		}
		if config.OpenAPIContent != "" {
			return nil
		}
		return errors.New("openapi spec and content is empty")
	}

	if g.ProxyConfig != nil {
		config := g.ProxyConfig
		return validateHTTPURL(config.URL)
	}

	return err
}

// CreateGroupMCP creates a new GroupMCP
func CreateGroupMCP(mcp *GroupMCP) error {
	err := DB.Create(mcp).Error
	if err != nil && errors.Is(err, gorm.ErrDuplicatedKey) {
		return errors.New("group mcp already exists")
	}
	return err
}

// UpdateGroupMCP updates an existing GroupMCP
func UpdateGroupMCP(mcp *GroupMCP) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroupMCP(mcp.GroupID, mcp.ID); err != nil {
				log.Error("cache delete group mcp error: " + err.Error())
			}
		}
	}()

	selects := []string{
		"name",
		"proxy_config",
		"openapi_config",
	}
	if mcp.Type != "" {
		selects = append(selects, "type")
	}
	if mcp.Status != 0 {
		selects = append(selects, "status")
	}
	result := DB.
		Select(selects).
		Where("id = ? AND group_id = ?", mcp.ID, mcp.GroupID).
		Updates(mcp)
	return HandleUpdateResult(result, ErrGroupMCPNotFound)
}

func UpdateGroupMCPStatus(id, groupID string, status GroupMCPStatus) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroupMCP(groupID, id); err != nil {
				log.Error("cache delete group mcp error: " + err.Error())
			}
		}
	}()

	result := DB.Model(&GroupMCP{}).
		Where("id = ? AND group_id = ?", id, groupID).
		Update("status", status)
	return HandleUpdateResult(result, ErrGroupMCPNotFound)
}

func GetAllGroupMCPs(status GroupMCPStatus) ([]GroupMCP, error) {
	var mcps []GroupMCP
	tx := DB.Model(&GroupMCP{})
	if status != 0 {
		tx = tx.Where("status = ?", status)
	}
	err := tx.Find(&mcps).Error
	return mcps, err
}

// DeleteGroupMCP deletes a GroupMCP by ID and GroupID
func DeleteGroupMCP(id, groupID string) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeleteGroupMCP(groupID, id); err != nil {
				log.Error("cache delete group mcp error: " + err.Error())
			}
		}
	}()

	if id == "" || groupID == "" {
		return errors.New("group mcp id or group id is empty")
	}
	result := DB.Where("id = ? AND group_id = ?", id, groupID).Delete(&GroupMCP{})
	return HandleUpdateResult(result, ErrGroupMCPNotFound)
}

// GetGroupMCPByID retrieves a GroupMCP by ID and GroupID
func GetGroupMCPByID(id, groupID string) (GroupMCP, error) {
	var mcp GroupMCP
	if id == "" || groupID == "" {
		return mcp, errors.New("group mcp id or group id is empty")
	}
	err := DB.Where("id = ? AND group_id = ?", id, groupID).First(&mcp).Error
	return mcp, HandleNotFound(err, ErrGroupMCPNotFound)
}

// GetGroupMCPs retrieves GroupMCPs with pagination and filtering
func GetGroupMCPs(
	groupID string,
	page, perPage int,
	mcpType PublicMCPType,
	keyword string,
	status GroupMCPStatus,
) (mcps []GroupMCP, total int64, err error) {
	if groupID == "" {
		return nil, 0, errors.New("group id is empty")
	}

	tx := DB.Model(&GroupMCP{}).Where("group_id = ?", groupID)

	if mcpType != "" {
		tx = tx.Where("type = ?", mcpType)
	}

	if keyword != "" {
		keyword = "%" + keyword + "%"
		if common.UsingPostgreSQL {
			tx = tx.Where("name ILIKE ? OR id ILIKE ?", keyword, keyword)
		} else {
			tx = tx.Where("name LIKE ? OR id LIKE ?", keyword, keyword)
		}
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
	err = tx.
		Limit(limit).
		Offset(offset).
		Find(&mcps).
		Error

	return mcps, total, err
}
