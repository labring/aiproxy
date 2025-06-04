package model

import (
	"errors"
	"net/url"
	"regexp"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type PublicMCPStatus int

const (
	PublicMCPStatusEnabled PublicMCPStatus = iota + 1
	PublicMCPStatusDisabled
)

const (
	ErrPublicMCPNotFound       = "public mcp"
	ErrMCPReusingParamNotFound = "mcp reusing param"
)

type PublicMCPType string

const (
	PublicMCPTypeProxySSE        PublicMCPType = "mcp_proxy_sse"
	PublicMCPTypeProxyStreamable PublicMCPType = "mcp_proxy_streamable"
	PublicMCPTypeDocs            PublicMCPType = "mcp_docs" // read only
	PublicMCPTypeOpenAPI         PublicMCPType = "mcp_openapi"
	PublicMCPTypeEmbed           PublicMCPType = "mcp_embed"
)

type ParamType string

const (
	ParamTypeHeader ParamType = "header"
	ParamTypeQuery  ParamType = "query"
)

type ReusingParam struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        ParamType `json:"type"`
	Required    bool      `json:"required"`
}

type MCPPrice struct {
	DefaultToolsCallPrice float64            `json:"default_tools_call_price"`
	ToolsCallPrices       map[string]float64 `json:"tools_call_prices"        gorm:"serializer:fastjson;type:text"`
}

type PublicMCPProxyConfig struct {
	URL           string                  `json:"url"`
	Querys        map[string]string       `json:"querys"`
	Headers       map[string]string       `json:"headers"`
	ReusingParams map[string]ReusingParam `json:"reusing_params"`
}

type PublicMCPReusingParam struct {
	MCPID         string            `gorm:"primaryKey"                    json:"mcp_id"`
	GroupID       string            `gorm:"primaryKey"                    json:"group_id"`
	CreatedAt     time.Time         `gorm:"index"                         json:"created_at"`
	UpdateAt      time.Time         `gorm:"index"                         json:"update_at"`
	Group         *Group            `gorm:"foreignKey:GroupID"            json:"-"`
	ReusingParams map[string]string `gorm:"serializer:fastjson;type:text" json:"reusing_params"`
}

func (p *PublicMCPReusingParam) BeforeCreate(_ *gorm.DB) (err error) {
	if p.MCPID == "" {
		return errors.New("mcp id is empty")
	}
	if p.GroupID == "" {
		return errors.New("group is empty")
	}
	return
}

func (p *PublicMCPReusingParam) MarshalJSON() ([]byte, error) {
	type Alias PublicMCPReusingParam
	a := &struct {
		*Alias
		CreatedAt int64 `json:"created_at"`
		UpdateAt  int64 `json:"update_at"`
	}{
		Alias:     (*Alias)(p),
		CreatedAt: p.CreatedAt.UnixMilli(),
		UpdateAt:  p.UpdateAt.UnixMilli(),
	}
	return sonic.Marshal(a)
}

type MCPOpenAPIConfig struct {
	OpenAPISpec    string `json:"openapi_spec"`
	OpenAPIContent string `json:"openapi_content,omitempty"`
	V2             bool   `json:"v2"`
	ServerAddr     string `json:"server_addr,omitempty"`
	Authorization  string `json:"authorization,omitempty"`
}

type MCPEmbeddingReusingConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type MCPEmbeddingConfig struct {
	Init    map[string]string                    `json:"init"`
	Reusing map[string]MCPEmbeddingReusingConfig `json:"reusing"`
}

var validateMCPIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validateMCPID(id string) error {
	if id == "" {
		return errors.New("mcp id is empty")
	}
	if !validateMCPIDRegex.MatchString(id) {
		return errors.New("mcp id is invalid")
	}
	return nil
}

type PublicMCP struct {
	ID                     string                  `gorm:"primaryKey"                    json:"id"`
	Status                 PublicMCPStatus         `gorm:"index;default:1"               json:"status"`
	CreatedAt              time.Time               `gorm:"index,autoCreateTime"          json:"created_at"`
	UpdateAt               time.Time               `gorm:"index,autoUpdateTime"          json:"update_at"`
	PublicMCPReusingParams []PublicMCPReusingParam `gorm:"foreignKey:MCPID"              json:"-"`
	Name                   string                  `                                     json:"name"`
	Type                   PublicMCPType           `gorm:"index"                         json:"type"`
	RepoURL                string                  `                                     json:"repo_url"`
	ReadmeURL              string                  `                                     json:"readme_url"`
	Readme                 string                  `gorm:"type:text"                     json:"readme"`
	Tags                   []string                `gorm:"serializer:fastjson;type:text" json:"tags,omitempty"`
	LogoURL                string                  `                                     json:"logo_url"`
	Price                  MCPPrice                `gorm:"embedded"                      json:"price"`
	ProxyConfig            *PublicMCPProxyConfig   `gorm:"serializer:fastjson;type:text" json:"proxy_config,omitempty"`
	OpenAPIConfig          *MCPOpenAPIConfig       `gorm:"serializer:fastjson;type:text" json:"openapi_config,omitempty"`
	EmbedConfig            *MCPEmbeddingConfig     `gorm:"serializer:fastjson;type:text" json:"embed_config,omitempty"`
}

func (p *PublicMCP) BeforeSave(_ *gorm.DB) error {
	if err := validateMCPID(p.ID); err != nil {
		return err
	}

	if p.Status == 0 {
		p.Status = PublicMCPStatusEnabled
	}

	if p.OpenAPIConfig != nil {
		config := p.OpenAPIConfig
		if config.OpenAPISpec != "" {
			return validateHTTPURL(config.OpenAPISpec)
		}
		if config.OpenAPIContent != "" {
			return nil
		}
		return errors.New("openapi spec and content is empty")
	}

	if p.ProxyConfig != nil {
		config := p.ProxyConfig
		return validateHTTPURL(config.URL)
	}
	return nil
}

func validateHTTPURL(str string) error {
	if str == "" {
		return errors.New("url is empty")
	}
	u, err := url.Parse(str)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url scheme not support")
	}
	return nil
}

func (p *PublicMCP) BeforeDelete(tx *gorm.DB) (err error) {
	return tx.Model(&PublicMCPReusingParam{}).
		Where("mcp_id = ?", p.ID).
		Delete(&PublicMCPReusingParam{}).
		Error
}

// CreatePublicMCP creates a new MCP
func CreatePublicMCP(mcp *PublicMCP) error {
	err := DB.Create(mcp).Error
	if err != nil && errors.Is(err, gorm.ErrDuplicatedKey) {
		return errors.New("mcp server already exist")
	}
	return err
}

func SavePublicMCP(mcp *PublicMCP) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeletePublicMCP(mcp.ID); err != nil {
				log.Error("cache delete public mcp error: " + err.Error())
			}
		}
	}()

	return DB.Save(mcp).Error
}

// UpdatePublicMCP updates an existing MCP
func UpdatePublicMCP(mcp *PublicMCP) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeletePublicMCP(mcp.ID); err != nil {
				log.Error("cache delete public mcp error: " + err.Error())
			}
		}
	}()

	selects := []string{
		"repo_url",
		"readme",
		"readme_url",
		"tags",
		"author",
		"logo_url",
		"proxy_config",
		"openapi_config",
		"embed_config",
	}
	if mcp.Status != 0 {
		selects = append(selects, "status")
	}
	if mcp.Name != "" {
		selects = append(selects, "name")
	}
	if mcp.Type != "" {
		selects = append(selects, "type")
	}
	if mcp.Price.DefaultToolsCallPrice != 0 ||
		len(mcp.Price.ToolsCallPrices) != 0 {
		selects = append(selects, "price")
	}
	result := DB.
		Select(selects).
		Where("id = ?", mcp.ID).
		Updates(mcp)
	return HandleUpdateResult(result, ErrPublicMCPNotFound)
}

func UpdatePublicMCPStatus(id string, status PublicMCPStatus) (err error) {
	defer func() {
		if err == nil {
			if err := CacheUpdatePublicMCPStatus(id, status); err != nil {
				log.Error("cache update public mcp status error: " + err.Error())
			}
		}
	}()

	result := DB.Model(&PublicMCP{}).Where("id = ?", id).Update("status", status)
	return HandleUpdateResult(result, ErrPublicMCPNotFound)
}

// DeletePublicMCP deletes an MCP by ID
func DeletePublicMCP(id string) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeletePublicMCP(id); err != nil {
				log.Error("cache delete public mcp error: " + err.Error())
			}
		}
	}()

	if id == "" {
		return errors.New("MCP id is empty")
	}
	result := DB.Delete(&PublicMCP{ID: id})
	return HandleUpdateResult(result, ErrPublicMCPNotFound)
}

// GetPublicMCPByID retrieves an MCP by ID
func GetPublicMCPByID(id string) (PublicMCP, error) {
	var mcp PublicMCP
	if id == "" {
		return mcp, errors.New("MCP id is empty")
	}
	err := DB.Where("id = ?", id).First(&mcp).Error
	return mcp, HandleNotFound(err, ErrPublicMCPNotFound)
}

// GetPublicMCPs retrieves MCPs with pagination and filtering
func GetPublicMCPs(
	page, perPage int,
	mcpType PublicMCPType,
	keyword string,
	status PublicMCPStatus,
) (mcps []PublicMCP, total int64, err error) {
	tx := DB.Model(&PublicMCP{})

	if mcpType != "" {
		tx = tx.Where("type = ?", mcpType)
	}

	if keyword != "" {
		keyword = "%" + keyword + "%"
		if common.UsingPostgreSQL {
			tx = tx.Where(
				"name ILIKE ? OR author ILIKE ? OR tags ILIKE ? OR id ILIKE ?",
				keyword,
				keyword,
				keyword,
				keyword,
			)
		} else {
			tx = tx.Where("name LIKE ? OR author LIKE ? OR tags LIKE ? OR id LIKE ?", keyword, keyword, keyword, keyword)
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

func GetAllPublicMCPs(status PublicMCPStatus) ([]PublicMCP, error) {
	var mcps []PublicMCP
	tx := DB.Model(&PublicMCP{})
	if status != 0 {
		tx = tx.Where("status = ?", status)
	}
	err := tx.Find(&mcps).Error
	return mcps, err
}

func GetPublicMCPsEnabled(ids []string) ([]string, error) {
	var mcpIDs []string
	err := DB.Model(&PublicMCP{}).
		Select("id").
		Where("id IN (?) AND status = ?", ids, PublicMCPStatusEnabled).
		Pluck("id", &mcpIDs).
		Error
	if err != nil {
		return nil, err
	}
	return mcpIDs, nil
}

func SavePublicMCPReusingParam(param *PublicMCPReusingParam) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeletePublicMCPReusingParam(param.MCPID, param.GroupID); err != nil {
				log.Error("cache delete public mcp reusing param error: " + err.Error())
			}
		}
	}()

	return DB.Save(param).Error
}

// UpdatePublicMCPReusingParam updates an existing GroupMCPReusingParam
func UpdatePublicMCPReusingParam(param *PublicMCPReusingParam) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeletePublicMCPReusingParam(param.MCPID, param.GroupID); err != nil {
				log.Error("cache delete public mcp reusing param error: " + err.Error())
			}
		}
	}()

	result := DB.
		Select([]string{
			"reusing_params",
		}).
		Where("mcp_id = ? AND group_id = ?", param.MCPID, param.GroupID).
		Updates(param)
	return HandleUpdateResult(result, ErrMCPReusingParamNotFound)
}

// DeletePublicMCPReusingParam deletes a GroupMCPReusingParam
func DeletePublicMCPReusingParam(mcpID, groupID string) (err error) {
	defer func() {
		if err == nil {
			if err := CacheDeletePublicMCPReusingParam(mcpID, groupID); err != nil {
				log.Error("cache delete public mcp reusing param error: " + err.Error())
			}
		}
	}()

	if mcpID == "" || groupID == "" {
		return errors.New("MCP ID or Group ID is empty")
	}
	result := DB.
		Where("mcp_id = ? AND group_id = ?", mcpID, groupID).
		Delete(&PublicMCPReusingParam{})
	return HandleUpdateResult(result, ErrMCPReusingParamNotFound)
}

// GetPublicMCPReusingParam retrieves a GroupMCPReusingParam by MCP ID and Group ID
func GetPublicMCPReusingParam(mcpID, groupID string) (*PublicMCPReusingParam, error) {
	if mcpID == "" || groupID == "" {
		return nil, errors.New("MCP ID or Group ID is empty")
	}
	var param PublicMCPReusingParam
	err := DB.Where("mcp_id = ? AND group_id = ?", mcpID, groupID).First(&param).Error
	return &param, HandleNotFound(err, ErrMCPReusingParamNotFound)
}
