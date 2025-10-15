package model

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
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

type ProxyParamType string

const (
	ParamTypeURL    ProxyParamType = "url"
	ParamTypeHeader ProxyParamType = "header"
	ParamTypeQuery  ProxyParamType = "query"
)

type ReusingParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type MCPPrice struct {
	DefaultToolsCallPrice float64            `json:"default_tools_call_price"`
	ToolsCallPrices       map[string]float64 `json:"tools_call_prices"        gorm:"serializer:fastjson;type:text"`
}

type PublicMCPProxyReusingParam struct {
	ReusingParam
	Type ProxyParamType `json:"type"`
}

type PublicMCPProxyConfig struct {
	URL     string                                `json:"url"`
	Querys  map[string]string                     `json:"querys"`
	Headers map[string]string                     `json:"headers"`
	Reusing map[string]PublicMCPProxyReusingParam `json:"reusing"`
}

type Params = map[string]string

type PublicMCPReusingParam struct {
	MCPID     string    `gorm:"primaryKey"                    json:"mcp_id"`
	GroupID   string    `gorm:"primaryKey"                    json:"group_id"`
	CreatedAt time.Time `gorm:"index"                         json:"created_at"`
	UpdateAt  time.Time `gorm:"index"                         json:"update_at"`
	Group     *Group    `gorm:"foreignKey:GroupID"            json:"-"`
	Params    Params    `gorm:"serializer:fastjson;type:text" json:"params"`
}

func (p *PublicMCPReusingParam) BeforeCreate(_ *gorm.DB) (err error) {
	if p.MCPID == "" {
		return errors.New("mcp id is empty")
	}

	if p.GroupID == "" {
		return errors.New("group is empty")
	}

	return err
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

type MCPEmbeddingConfig struct {
	Init    map[string]string       `json:"init"`
	Reusing map[string]ReusingParam `json:"reusing"`
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

type TestConfig struct {
	Enabled bool   `json:"enabled"`
	Params  Params `json:"params"`
}

type PublicMCP struct {
	ID                     string                  `gorm:"primaryKey"           json:"id"`
	CreatedAt              time.Time               `gorm:"index,autoCreateTime" json:"created_at"`
	UpdateAt               time.Time               `gorm:"index,autoUpdateTime" json:"update_at"`
	PublicMCPReusingParams []PublicMCPReusingParam `gorm:"foreignKey:MCPID"     json:"-"`

	Name          string          `json:"name"`
	NameCN        string          `json:"name_cn,omitempty"`
	Status        PublicMCPStatus `json:"status"                   gorm:"index;default:1"`
	Type          PublicMCPType   `json:"type,omitempty"           gorm:"index"`
	Description   string          `json:"description"`
	DescriptionCN string          `json:"description_cn,omitempty"`
	GitHubURL     string          `json:"github_url"`
	Readme        string          `json:"readme,omitempty"         gorm:"type:text"`
	ReadmeCN      string          `json:"readme_cn,omitempty"      gorm:"type:text"`
	ReadmeURL     string          `json:"readme_url,omitempty"`
	ReadmeCNURL   string          `json:"readme_cn_url,omitempty"`
	Tags          []string        `json:"tags,omitempty"           gorm:"serializer:fastjson;type:text"`
	LogoURL       string          `json:"logo_url,omitempty"`
	Price         MCPPrice        `json:"price"                    gorm:"embedded"`

	ProxyConfig   *PublicMCPProxyConfig `gorm:"serializer:fastjson;type:text" json:"proxy_config,omitempty"`
	OpenAPIConfig *MCPOpenAPIConfig     `gorm:"serializer:fastjson;type:text" json:"openapi_config,omitempty"`
	EmbedConfig   *MCPEmbeddingConfig   `gorm:"serializer:fastjson;type:text" json:"embed_config,omitempty"`
	// only used by list tools
	TestConfig *TestConfig `gorm:"serializer:fastjson;type:text" json:"test_config,omitempty"`
}

func (p *PublicMCP) BeforeCreate(_ *gorm.DB) error {
	if err := validateMCPID(p.ID); err != nil {
		return err
	}

	if p.Status == 0 {
		p.Status = PublicMCPStatusEnabled
	}

	return nil
}

func (p *PublicMCP) BeforeSave(_ *gorm.DB) error {
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

	return DB.
		Omit(
			"created_at",
			"update_at",
		).
		Save(mcp).Error
}

func SavePublicMCPs(msps []PublicMCP) (err error) {
	defer func() {
		if err == nil {
			for _, mcp := range msps {
				if err := CacheDeletePublicMCP(mcp.ID); err != nil {
					log.Error("cache delete public mcp error: " + err.Error())
				}
			}
		}
	}()

	return DB.
		Omit(
			"created_at",
			"update_at",
		).
		Save(msps).Error
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
		"github_url",
		"description",
		"description_cn",
		"readme",
		"readme_cn",
		"readme_url",
		"readme_cn_url",
		"tags",
		"logo_url",
		"proxy_config",
		"openapi_config",
		"embed_config",
		"test_config",
	}
	if mcp.Status != 0 {
		selects = append(selects, "status")
	}

	if mcp.Name != "" {
		selects = append(selects, "name")
	}

	if mcp.NameCN != "" {
		selects = append(selects, "name_cn")
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
	id string,
	mcpType []PublicMCPType,
	keyword string,
	status PublicMCPStatus,
) (mcps []PublicMCP, total int64, err error) {
	tx := DB.Model(&PublicMCP{})

	if id != "" {
		tx = tx.Where("id = ?", id)
	}

	if status != 0 {
		tx = tx.Where("status = ?", status)
	}

	if len(mcpType) > 0 {
		tx = tx.Where("type IN (?)", mcpType)
	}

	if keyword != "" {
		var (
			conditions []string
			values     []any
		)

		if id == "" {
			if !common.UsingSQLite {
				conditions = append(conditions, "id ILIKE ?")
				values = append(values, "%"+keyword+"%")
			} else {
				conditions = append(conditions, "id LIKE ?")
				values = append(values, "%"+keyword+"%")
			}
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "name ILIKE ?")
			values = append(values, "%"+keyword+"%")
		} else {
			conditions = append(conditions, "name LIKE ?")
			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "name_cn ILIKE ?")
			values = append(values, "%"+keyword+"%")
		} else {
			conditions = append(conditions, "name_cn LIKE ?")
			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "description ILIKE ?")
			values = append(values, "%"+keyword+"%")
		} else {
			conditions = append(conditions, "description LIKE ?")
			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "description_cn ILIKE ?")
			values = append(values, "%"+keyword+"%")
		} else {
			conditions = append(conditions, "description_cn LIKE ?")
			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "readme ILIKE ?")
			values = append(values, "%"+keyword+"%")
		} else {
			conditions = append(conditions, "readme LIKE ?")
			values = append(values, "%"+keyword+"%")
		}

		if !common.UsingSQLite {
			conditions = append(conditions, "readme_cn ILIKE ?")
			values = append(values, "%"+keyword+"%")
		} else {
			conditions = append(conditions, "readme_cn LIKE ?")
			values = append(values, "%"+keyword+"%")
		}

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

func GetPublicMCPsEmbedConfig(ids []string) (map[string]MCPEmbeddingConfig, error) {
	var configs []struct {
		ID          string
		EmbedConfig MCPEmbeddingConfig `gorm:"serializer:fastjson;type:text"`
	}

	err := DB.Model(&PublicMCP{}).
		Select("id, embed_config").
		Where("id IN (?)", ids).
		Find(&configs).Error
	if err != nil {
		return nil, err
	}

	configsMap := make(map[string]MCPEmbeddingConfig)
	for _, config := range configs {
		configsMap[config.ID] = config.EmbedConfig
	}

	return configsMap, nil
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
			"params",
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
func GetPublicMCPReusingParam(mcpID, groupID string) (PublicMCPReusingParam, error) {
	if mcpID == "" || groupID == "" {
		return PublicMCPReusingParam{}, errors.New("MCP ID or Group ID is empty")
	}

	var param PublicMCPReusingParam

	err := DB.Where("mcp_id = ? AND group_id = ?", mcpID, groupID).First(&param).Error

	return param, HandleNotFound(err, ErrMCPReusingParamNotFound)
}
