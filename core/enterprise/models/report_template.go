//go:build enterprise

package models

import "time"

// ReportTemplate stores a saved custom report configuration.
type ReportTemplate struct {
	ID         int       `gorm:"primaryKey"                         json:"id"`
	Name       string    `gorm:"size:128;not null"                  json:"name"`
	CreatedBy  string    `gorm:"size:64;not null;index"             json:"created_by"`
	Dimensions string    `gorm:"type:text;not null"                 json:"dimensions"`
	Measures   string    `gorm:"type:text;not null"                 json:"measures"`
	ChartType  string    `gorm:"size:32"                            json:"chart_type"`
	ViewMode   string    `gorm:"size:16"                            json:"view_mode"`
	SortBy     string    `gorm:"size:64"                            json:"sort_by"`
	SortOrder  string    `gorm:"size:4"                             json:"sort_order"`
	CreatedAt  time.Time `gorm:"autoCreateTime"                     json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"                     json:"updated_at"`
}
