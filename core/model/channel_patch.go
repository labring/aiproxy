package model

// ChannelPatch separates optional writes from stored channel values.
// Nil pointers leave columns unchanged; non-nil zero values are persisted.
type ChannelPatch struct {
	Type                    *ChannelType       `json:"type,omitempty"`
	Name                    *string            `json:"name,omitempty"`
	Remark                  *string            `json:"remark,omitempty"`
	Key                     *string            `json:"key,omitempty"`
	BaseURL                 *string            `json:"base_url,omitempty"`
	ProxyURL                *string            `json:"proxy_url,omitempty"`
	Models                  *[]string          `json:"models,omitempty"                     gorm:"serializer:fastjson;type:text"`
	ModelMapping            *map[string]string `json:"model_mapping,omitempty"              gorm:"serializer:fastjson;type:text"`
	Configs                 *ChannelConfigs    `json:"configs,omitempty"                    gorm:"serializer:fastjson;type:text"`
	Priority                *int32             `json:"priority,omitempty"`
	BackupOnly              *bool              `json:"backup_only,omitempty"`
	Sets                    *[]string          `json:"sets,omitempty"                       gorm:"serializer:fastjson;type:text"`
	EnabledAutoBalanceCheck *bool              `json:"enabled_auto_balance_check,omitempty"`
	SkipTLSVerify           *bool              `json:"skip_tls_verify,omitempty"`
	EnabledNoPermissionBan  *bool              `json:"enabled_no_permission_ban,omitempty"`
	WarnErrorRate           *float64           `json:"warn_error_rate,omitempty"`
	MaxErrorRate            *float64           `json:"max_error_rate,omitempty"`
	BalanceThreshold        *float64           `json:"balance_threshold,omitempty"`
}
