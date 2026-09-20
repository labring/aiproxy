package model

// GroupChannelPatch keeps omitted fields unchanged while allowing explicit
// zero values to be persisted during group channel updates.
type GroupChannelPatch struct {
	Type                   *ChannelType       `json:"type,omitempty"`
	Name                   *string            `json:"name,omitempty"`
	Remark                 *string            `json:"remark,omitempty"`
	Key                    *string            `json:"key,omitempty"`
	BaseURL                *string            `json:"base_url,omitempty"`
	ProxyURL               *string            `json:"proxy_url,omitempty"`
	Models                 *[]string          `json:"models,omitempty"                    gorm:"serializer:fastjson;type:text"`
	ModelMapping           *map[string]string `json:"model_mapping,omitempty"             gorm:"serializer:fastjson;type:text"`
	Configs                *ChannelConfigs    `json:"configs,omitempty"                   gorm:"serializer:fastjson;type:text"`
	Priority               *int32             `json:"priority,omitempty"`
	BackupOnly             *bool              `json:"backup_only,omitempty"`
	Sets                   *[]string          `json:"sets,omitempty"                      gorm:"serializer:fastjson;type:text"`
	SkipTLSVerify          *bool              `json:"skip_tls_verify,omitempty"`
	EnabledNoPermissionBan *bool              `json:"enabled_no_permission_ban,omitempty"`
	WarnErrorRate          *float64           `json:"warn_error_rate,omitempty"`
	MaxErrorRate           *float64           `json:"max_error_rate,omitempty"`
}
