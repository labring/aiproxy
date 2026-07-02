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
	Scope                   model.ChannelScope
	ModelMapping            map[string]string
	EnabledAutoBalanceCheck bool
	SkipTLSVerify           bool
	EnabledNoPermissionBan  bool
	WarnErrorRate           float64
	MaxErrorRate            float64
}

type Meta struct {
	values         map[string]any
	Channel        ChannelMeta
	ChannelConfigs model.ChannelConfigs
	Group          model.GroupCache
	Token          model.TokenCache
	ModelConfig    model.ModelConfig

	Endpoint    string
	RequestAt   time.Time
	RetryAt     time.Time
	RequestID   string
	OriginModel string
	ActualModel string
	Mode        mode.Mode

	RequestTimeout time.Duration

	RequestUsage        model.Usage
	RequestUsageContext model.UsageContext
	RequestServiceTier  string
	PromptCacheKey      string
	User                string

	JobID        string
	GenerationID string
	OperationID  string
	ResponseID   string
	VideoID      string
	FileID       string
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

func WithGroup(group model.GroupCache) Option {
	return func(meta *Meta) {
		meta.Group = group
	}
}

func WithToken(token model.TokenCache) Option {
	return func(meta *Meta) {
		meta.Token = token
	}
}

func WithModelConfig(modelConfig model.ModelConfig) Option {
	return func(meta *Meta) {
		meta.ModelConfig = modelConfig
	}
}

func WithRequestUsage(requestUsage model.Usage) Option {
	return func(meta *Meta) {
		meta.RequestUsage = requestUsage
	}
}

func WithRequestUsageContext(requestUsageContext model.UsageContext) Option {
	return func(meta *Meta) {
		meta.RequestUsageContext = requestUsageContext
	}
}

func WithRequestServiceTier(requestServiceTier string) Option {
	return func(meta *Meta) {
		meta.RequestServiceTier = requestServiceTier
	}
}

func WithJobID(jobID string) Option {
	return func(meta *Meta) {
		meta.JobID = jobID
	}
}

func WithGenerationID(generationID string) Option {
	return func(meta *Meta) {
		meta.GenerationID = generationID
	}
}

func WithOperationID(operationID string) Option {
	return func(meta *Meta) {
		meta.OperationID = operationID
	}
}

func WithResponseID(responseID string) Option {
	return func(meta *Meta) {
		meta.ResponseID = responseID
	}
}

func WithVideoID(videoID string) Option {
	return func(meta *Meta) {
		meta.VideoID = videoID
	}
}

func WithFileID(fileID string) Option {
	return func(meta *Meta) {
		meta.FileID = fileID
	}
}

func WithPromptCacheKey(promptCacheKey string) Option {
	return func(meta *Meta) {
		meta.PromptCacheKey = promptCacheKey
	}
}

func WithUser(user string) Option {
	return func(meta *Meta) {
		meta.User = user
	}
}

func WithChannelScope(scope model.ChannelScope, groupID string) Option {
	return func(meta *Meta) {
		meta.Channel.Scope = scope
		meta.Channel.GroupID = groupID
	}
}

func NewMeta(
	channel *model.Channel,
	mode mode.Mode,
	modelName string,
	modelConfig model.ModelConfig,
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
		meta.SetChannel(channel)
	}

	return &meta
}

func (m *Meta) SetChannel(channel *model.Channel) {
	m.Channel.Name = channel.Name
	m.Channel.BaseURL = channel.BaseURL
	m.Channel.ProxyURL = channel.ProxyURL
	m.Channel.Key = channel.Key
	m.Channel.ID = channel.ID

	m.Channel.Type = channel.Type
	if m.Channel.Scope == "" {
		m.Channel.Scope = model.ChannelScopeGlobal
	}

	m.Channel.EnabledAutoBalanceCheck = channel.EnabledAutoBalanceCheck
	m.Channel.SkipTLSVerify = channel.SkipTLSVerify
	m.Channel.EnabledNoPermissionBan = channel.EnabledNoPermissionBan
	m.Channel.WarnErrorRate = channel.WarnErrorRate
	m.Channel.MaxErrorRate = channel.MaxErrorRate

	m.Channel.ModelMapping = channel.ModelMapping
	m.ChannelConfigs = channel.Configs

	m.ActualModel, _ = GetMappedModelName(m.OriginModel, channel.ModelMapping)
}

func (m *Meta) ChannelMonitorKey() string {
	if m == nil {
		return "0"
	}

	if m.Channel.Scope == model.ChannelScopeGroup {
		return model.GroupChannelMonitorKey(m.Channel.GroupID, m.Channel.ID)
	}

	return strconv.Itoa(m.Channel.ID)
}

func (m *Meta) CopyChannelFromMeta(meta *Meta) {
	m.Channel = meta.Channel
	m.ChannelConfigs = meta.ChannelConfigs
	m.ActualModel, _ = GetMappedModelName(meta.OriginModel, meta.Channel.ModelMapping)
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

func (m *Meta) GetInt64(key string) int64 {
	v, ok := m.Get(key)
	if !ok {
		return 0
	}

	i, _ := v.(int64)

	return i
}

func (m *Meta) GetInt(key string) int {
	v, ok := m.Get(key)
	if !ok {
		return 0
	}

	i, _ := v.(int)

	return i
}

// PushToSlice appends an item to a slice stored under the given key
func (m *Meta) PushToSlice(key string, item any) {
	var slice []any
	if existing, ok := m.Get(key); ok {
		if existingSlice, ok := existing.([]any); ok {
			slice = existingSlice
		}
	}

	slice = append(slice, item)
	m.Set(key, slice)
}

// GetSlice retrieves a slice stored under the given key
func (m *Meta) GetSlice(key string) []any {
	if slice, ok := m.Get(key); ok {
		if typedSlice, ok := slice.([]any); ok {
			return typedSlice
		}
	}

	return nil
}

// ClearSlice removes the slice stored under the given key
func (m *Meta) ClearSlice(key string) {
	m.Delete(key)
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
