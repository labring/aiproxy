package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/mode"
)

type ChannelTest struct {
	TestAt      time.Time   `json:"test_at"`
	Model       string      `json:"model"        gorm:"primaryKey"`
	ActualModel string      `json:"actual_model"`
	Response    string      `json:"response"     gorm:"type:text"`
	ChannelName string      `json:"channel_name"`
	ChannelType ChannelType `json:"channel_type"`
	ChannelID   int         `json:"channel_id"   gorm:"primaryKey"`
	Took        float64     `json:"took"`
	Success     bool        `json:"success"`
	Mode        mode.Mode   `json:"mode"`
	Code        int         `json:"code"`
}

func (ct *ChannelTest) MarshalJSON() ([]byte, error) {
	type Alias ChannelTest
	return sonic.Marshal(&struct {
		*Alias
		TestAt int64 `json:"test_at"`
	}{
		Alias:  (*Alias)(ct),
		TestAt: ct.TestAt.UnixMilli(),
	})
}
