package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/mode"
)

type GroupChannelTest struct {
	TestAt         time.Time   `json:"test_at"`
	Model          string      `json:"model"            gorm:"size:128;primaryKey"`
	ActualModel    string      `json:"actual_model"     gorm:"size:128"`
	Response       string      `json:"response"         gorm:"type:text"`
	GroupID        string      `json:"group_id"         gorm:"size:64;primaryKey"`
	ChannelName    string      `json:"channel_name"     gorm:"size:64"`
	ChannelType    ChannelType `json:"channel_type"`
	GroupChannelID int         `json:"group_channel_id" gorm:"primaryKey"`
	Took           float64     `json:"took"`
	Success        bool        `json:"success"`
	Mode           mode.Mode   `json:"mode"`
	Code           int         `json:"code"`
}

func (ct *GroupChannelTest) MarshalJSON() ([]byte, error) {
	type Alias GroupChannelTest

	return sonic.Marshal(&struct {
		*Alias
		TestAt int64 `json:"test_at"`
	}{
		Alias:  (*Alias)(ct),
		TestAt: ct.TestAt.UnixMilli(),
	})
}
