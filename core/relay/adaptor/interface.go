package adaptor

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

type StoreCache struct {
	ID        string
	GroupID   string
	TokenID   int
	ChannelID int
	Model     string
	ExpiresAt time.Time
}

type Store interface {
	GetStore(group string, tokenID int, id string) (StoreCache, error)
	SaveStore(store StoreCache) error
}

type Metadata struct {
	ConfigTemplates ConfigTemplates
	KeyHelp         string
	Features        []string
	Models          []model.ModelConfig
}

type RequestURL struct {
	Method string
	URL    string
}

type GetRequestURL interface {
	GetRequestURL(meta *meta.Meta, store Store) (RequestURL, error)
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

type DoResponse interface {
	DoResponse(
		meta *meta.Meta,
		store Store,
		c *gin.Context,
		resp *http.Response,
	) (model.Usage, Error)
}

type Adaptor interface {
	Metadata() Metadata
	SupportMode(mode mode.Mode) bool
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

var ErrGetBalanceNotImplemented = errors.New("get balance not implemented")

type Balancer interface {
	GetBalance(channel *model.Channel) (float64, error)
}

type KeyValidator interface {
	ValidateKey(key string) error
}

type ConfigTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example,omitempty"`
	Required    bool   `json:"required"`
}

type ConfigTemplates struct {
	Configs   map[string]ConfigTemplate
	Validator func(model.ChannelConfigs) error `json:"-"`
}
