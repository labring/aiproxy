package meta

import (
	"fmt"
	"time"

	"github.com/labring/aiproxy/model"
	"github.com/labring/aiproxy/relay/mode"
)

type ChannelMeta struct {
	Name     string
	BaseURL  string
	Key      string
	ID       int
	Type     int
	TypeName string
}

type Meta struct {
	values        map[string]any
	Channel       ChannelMeta
	ChannelConfig model.ChannelConfig
	Group         *model.GroupCache
	Token         *model.TokenCache
	ModelConfig   *model.ModelConfig

	Endpoint    string
	RequestAt   time.Time
	RetryAt     time.Time
	RequestID   string
	OriginModel string
	ActualModel string
	Mode        mode.Mode
	// TODO: remove this field
	InputTokens int64
}

type Option func(meta *Meta)

func WithEndpoint(endpoint string) Option {
	return func(meta *Meta) {
		meta.Endpoint = endpoint
	}
}

func WithRequestID(requestID string) Option {
	return func(meta *Meta) {
		meta.RequestID = requestID
	}
}

func WithRequestAt(requestAt time.Time) Option {
	return func(meta *Meta) {
		meta.RequestAt = requestAt
	}
}

func WithRetryAt(retryAt time.Time) Option {
	return func(meta *Meta) {
		meta.RetryAt = retryAt
	}
}

func WithGroup(group *model.GroupCache) Option {
	return func(meta *Meta) {
		meta.Group = group
	}
}

func WithToken(token *model.TokenCache) Option {
	return func(meta *Meta) {
		meta.Token = token
	}
}

func WithInputTokens(inputTokens int64) Option {
	return func(meta *Meta) {
		meta.InputTokens = inputTokens
	}
}

func WithChannelTypeName(typeName string) Option {
	return func(meta *Meta) {
		meta.Channel.TypeName = typeName
	}
}

func NewMeta(
	channel *model.Channel,
	mode mode.Mode,
	modelName string,
	modelConfig *model.ModelConfig,
	opts ...Option,
) *Meta {
	meta := Meta{
		values:      make(map[string]any),
		Mode:        mode,
		OriginModel: modelName,
		ActualModel: modelName,
		ModelConfig: modelConfig,
	}

	for _, opt := range opts {
		opt(&meta)
	}

	if meta.RequestAt.IsZero() {
		meta.RequestAt = time.Now()
	}

	if channel != nil {
		meta.Channel.Name = channel.Name
		meta.Channel.BaseURL = channel.BaseURL
		meta.Channel.Key = channel.Key
		meta.Channel.ID = channel.ID
		meta.Channel.Type = channel.Type
		if channel.Config != nil {
			meta.ChannelConfig = *channel.Config
		}
		meta.ActualModel, _ = GetMappedModelName(modelName, channel.ModelMapping)
	}

	return &meta
}

func (m *Meta) ClearValues() {
	clear(m.values)
}

func (m *Meta) Set(key string, value any) {
	m.values[key] = value
}

func (m *Meta) Get(key string) (any, bool) {
	v, ok := m.values[key]
	return v, ok
}

func (m *Meta) Delete(key string) {
	delete(m.values, key)
}

func (m *Meta) MustGet(key string) any {
	v, ok := m.Get(key)
	if !ok {
		panic(fmt.Sprintf("meta key %s not found", key))
	}
	return v
}

func (m *Meta) GetString(key string) string {
	v, ok := m.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (m *Meta) GetBool(key string) bool {
	v, ok := m.Get(key)
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func GetMappedModelName(modelName string, mapping map[string]string) (string, bool) {
	if len(modelName) == 0 {
		return modelName, false
	}
	mappedModelName := mapping[modelName]
	if mappedModelName != "" {
		return mappedModelName, true
	}
	return modelName, false
}
