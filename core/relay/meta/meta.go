package meta

import (
	"fmt"
	"strconv"
	"time"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

type ChannelMeta struct {
	Name                    string
	BaseURL                 string
	ProxyURL                string
	Key                     string
	GroupID                 string
	ID                      int
	Type                    model.ChannelType
	if m.Channel.Scope == "" {
		m.Channel.Scope = model.ChannelScopeGlobal
	}

