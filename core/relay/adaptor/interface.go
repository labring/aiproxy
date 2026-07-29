package adaptor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
)

type StoreCache struct {
	ID        string
	GroupID   string
	TokenID   int
	ChannelID int
	Model     string
	Metadata  string
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time
}

type SaveStoreOption struct {
	MinUpdateInterval time.Duration
}

type Store interface {
	GetStoreByScope(
		group string,
		tokenID int,
		id string,
		scope model.ChannelScope,
	) (StoreCache, error)
	SaveStore(store StoreCache, scope model.ChannelScope) error
	SaveStoreWithOption(store StoreCache, scope model.ChannelScope, opt SaveStoreOption) error
	SaveIfNotExistStore(store StoreCache, scope model.ChannelScope) error
}

type Metadata struct {
	ConfigSchema map[string]any
	KeyHelp      string
	Readme       string
	Models       []model.ModelConfig
}

type RequestURL struct {
	Method string
	URL    string
}

type GetRequestURL interface {
	GetRequestURL(meta *meta.Meta, store Store, c *gin.Context) (RequestURL, error)
}

type SetupRequestHeader interface {
	SetupRequestHeader(meta *meta.Meta, store Store, c *gin.Context, req *http.Request) error
}

type ConvertRequest interface {
	ConvertRequest(meta *meta.Meta, store Store, req *http.Request) (ConvertResult, error)
}

type DoRequest interface {
	DoRequest(
		meta *meta.Meta,
		store Store,
		c *gin.Context,
		req *http.Request,
	) (*http.Response, error)
}

// DoResponseResult contains the result of DoResponse
type DoResponseResult struct {
	Usage        model.Usage
	UsageContext model.UsageContext
	UpstreamID   string // ID from response body or x-request-id header
	AsyncUsage   bool   // usage will be fetched asynchronously by upstream ID
}

type DoResponse interface {
	DoResponse(
		meta *meta.Meta,
		store Store,
		c *gin.Context,
		resp *http.Response,
	) (DoResponseResult, Error)
}

type AsyncUsageRequest struct {
	Channel *model.Channel
	Info    *model.AsyncUsageInfo
	Store   Store
}

type AsyncUsageFetcher interface {
	FetchAsyncUsage(
		ctx context.Context,
		request AsyncUsageRequest,
	) (usage model.Usage, usageContext model.UsageContext, completed bool, err error)
}

type Adaptor interface {
	Metadata() Metadata
	SupportMode(meta *meta.Meta) bool
	DefaultBaseURL() string
	GetRequestURL
	SetupRequestHeader
	ConvertRequest
	DoRequest
	DoResponse
}

type ConvertResult struct {
	Header http.Header
	Body   io.Reader
}

type Error interface {
	json.Marshaler
	error
	StatusCode() int
}

type ErrorCodeProvider interface {
	ErrorCode() any
}

var ErrGetBalanceNotImplemented = errors.New("get balance not implemented")

type Balancer interface {
	GetBalance(channel *model.Channel) (float64, error)
}

type KeyValidator interface {
	ValidateKey(key string) error
}

type ConfigValidator func(model.ChannelConfigs) error
