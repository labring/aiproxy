package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"gorm.io/gorm"
)

type OneAPIChannel struct {
	Type         int               `gorm:"default:0"                              json:"type"`
	Key          string            `gorm:"type:text"                              json:"key"`
	Status       int               `gorm:"default:1"                              json:"status"`
	Name         string            `gorm:"index"                                  json:"name"`
	BaseURL      string            `gorm:"column:base_url;default:''"`
	ProxyURL     string            `gorm:"column:proxy_url;default:''"`
	Models       string            `                                              json:"models"`
	ModelMapping map[string]string `gorm:"type:varchar(1024);serializer:fastjson"`
	Priority     int32             `gorm:"bigint;default:0"`
	Config       ChannelConfig     `gorm:"serializer:fastjson"`
}

func (c *OneAPIChannel) TableName() string {
	return "channels"
}

type ChannelConfig struct {
	Region            string `json:"region,omitempty"`
	SK                string `json:"sk,omitempty"`
	AK                string `json:"ak,omitempty"`
	UserID            string `json:"user_id,omitempty"`
	APIVersion        string `json:"api_version,omitempty"`
	LibraryID         string `json:"library_id,omitempty"`
	VertexAIProjectID string `json:"vertex_ai_project_id,omitempty"`
	VertexAIADC       string `json:"vertex_ai_adc,omitempty"`
}

// https://github.com/songquanpeng/one-api/blob/main/relay/channeltype/define.go
const (
	OneAPIOpenAI = iota + 1
	OneAPIAPI2D
	OneAPIAzure
	OneAPICloseAI
	OneAPIOpenAISB
	OneAPIOpenAIMax
	OneAPIOhMyGPT
	OneAPICustom
	OneAPIAils
	OneAPIAIProxy
	OneAPIPaLM
	OneAPIAPI2GPT
	OneAPIAIGC2D
	OneAPIAnthropic
	OneAPIBaidu
	OneAPIZhipu
	OneAPIAli
	OneAPIXunfei
	OneAPIAI360
	OneAPIOpenRouter
	OneAPIAIProxyLibrary
	OneAPIFastGPT
	OneAPITencent
	OneAPIGemini
	OneAPIMoonshot
	OneAPIBaichuan
	OneAPIMinimax
	OneAPIMistral
	OneAPIGroq
	OneAPIOllama
	OneAPILingYiWanWu
	OneAPIStepFun
	OneAPIAwsClaude
	OneAPICoze
	OneAPICohere
	OneAPIDeepSeek
	OneAPICloudflare
	OneAPIDeepL
	OneAPITogetherAI
	OneAPIDoubao
	OneAPINovita
	OneAPIVertextAI
	OneAPIProxy
	OneAPISiliconFlow
	OneAPIXAI
	OneAPIReplicate
	OneAPIBaiduV2
	OneAPIXunfeiV2
	OneAPIAliBailian
	OneAPIOpenAICompatible
	OneAPIGeminiOpenAICompatible
)

var OneAPIChannelType2AIProxyMap = map[int]model.ChannelType{
	OneAPIOpenAI:                 model.ChannelTypeOpenAI,
	OneAPIAzure:                  model.ChannelTypeAzure,
	OneAPIAnthropic:              model.ChannelTypeAnthropic,
	OneAPIBaidu:                  model.ChannelTypeBaidu,
	OneAPIZhipu:                  model.ChannelTypeZhipu,
	OneAPIAli:                    model.ChannelTypeAli,
	OneAPIAI360:                  model.ChannelTypeAI360,
	OneAPIOpenRouter:             model.ChannelTypeOpenRouter,
	OneAPITencent:                model.ChannelTypeTencent,
	OneAPIGemini:                 model.ChannelTypeGoogleGemini,
	OneAPIMoonshot:               model.ChannelTypeMoonshot,
	OneAPIBaichuan:               model.ChannelTypeBaichuan,
	OneAPIMinimax:                model.ChannelTypeMinimax,
	OneAPIMistral:                model.ChannelTypeMistral,
	OneAPIGroq:                   model.ChannelTypeGroq,
	OneAPIOllama:                 model.ChannelTypeOllama,
	OneAPILingYiWanWu:            model.ChannelTypeLingyiwanwu,
	OneAPIStepFun:                model.ChannelTypeStepfun,
	OneAPIAwsClaude:              model.ChannelTypeAWS,
	OneAPICoze:                   model.ChannelTypeCoze,
	OneAPICohere:                 model.ChannelTypeCohere,
	OneAPIDeepSeek:               model.ChannelTypeDeepseek,
	OneAPICloudflare:             model.ChannelTypeCloudflare,
	OneAPIDoubao:                 model.ChannelTypeDoubao,
	OneAPINovita:                 model.ChannelTypeNovita,
	OneAPIVertextAI:              model.ChannelTypeVertexAI,
	OneAPISiliconFlow:            model.ChannelTypeSiliconflow,
	OneAPIBaiduV2:                model.ChannelTypeBaiduV2,
	OneAPIXunfeiV2:               model.ChannelTypeXunfei,
	OneAPIAliBailian:             model.ChannelTypeAli,
	OneAPIGeminiOpenAICompatible: model.ChannelTypeGoogleGeminiOpenAI,
	OneAPIXAI:                    model.ChannelTypeXAI,
}

type ImportChannelFromOneAPIRequest struct {
	DSN     string `json:"dsn"`
	GroupID string `json:"group_id"`
}

func AddOneAPIChannel(ch OneAPIChannel) error {
	add := AddChannelRequest{
		Type:         model.ChannelType(ch.Type),
		Name:         ch.Name,
		Key:          ch.Key,
		BaseURL:      ch.BaseURL,
		ProxyURL:     ch.ProxyURL,
		Models:       strings.Split(ch.Models, ","),
		ModelMapping: ch.ModelMapping,
		Priority:     ch.Priority,
		Status:       ch.Status,
	}
	if t, ok := OneAPIChannelType2AIProxyMap[ch.Type]; ok {
		add.Type = t
	} else {
		add.Type = 1
	}

	if add.Type == 1 && add.BaseURL != "" {
		add.BaseURL += "/v1"
	}

	channel, err := add.ToChannel()
	if err != nil {
		return err
	}

	return model.BatchInsertChannels([]*model.Channel{channel})
}

func oneAPIChannelToGroupChannelRequest(ch OneAPIChannel) AddGroupChannelRequest {
	add := AddGroupChannelRequest{
		Type:         model.ChannelType(ch.Type),
		Name:         ch.Name,
		Key:          ch.Key,
		BaseURL:      ch.BaseURL,
		ProxyURL:     ch.ProxyURL,
		Models:       strings.Split(ch.Models, ","),
		ModelMapping: ch.ModelMapping,
		Priority:     ch.Priority,
		Status:       ch.Status,
	}
	if t, ok := OneAPIChannelType2AIProxyMap[ch.Type]; ok {
		add.Type = t
	} else {
		add.Type = 1
	}

	if add.Type == 1 && add.BaseURL != "" {
		add.BaseURL += "/v1"
	}

	return add
}

func AddOneAPIGroupChannel(group string, ch OneAPIChannel) error {
	add := oneAPIChannelToGroupChannelRequest(ch)

	channel, err := add.toGroupChannel(group)
	if err != nil {
		return err
	}

	return model.BatchInsertGroupChannels([]*model.GroupChannel{channel})
}

// ImportChannelFromOneAPI godoc
//
//	@Summary		Import channel from OneAPI
//	@Description	Imports channels from OneAPI
//	@Tags			channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request	body		ImportChannelFromOneAPIRequest	true	"Import channel from OneAPI request"
//	@Success		200		{object}	middleware.APIResponse{data=[]error}
//	@Router			/api/channels/import/oneapi [post]
func ImportChannelFromOneAPI(c *gin.Context) {
	var req ImportChannelFromOneAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	allChannels, ok := loadOneAPIChannels(c, req)
	if !ok {
		return
	}

	errs := make([]error, 0)
	for _, ch := range allChannels {
		err := AddOneAPIChannel(*ch)
		if err != nil {
			errs = append(errs, err)
		}
	}

	middleware.SuccessResponse(c, errs)
}

func loadOneAPIChannels(
	c *gin.Context,
	req ImportChannelFromOneAPIRequest,
) ([]*OneAPIChannel, bool) {
	if req.DSN == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "sql dsn is required")
		return nil, false
	}

	var (
		db  *gorm.DB
		err error
	)

	switch {
	case strings.HasPrefix(req.DSN, "mysql"):
		db, err = model.OpenMySQL(req.DSN)
	case strings.HasPrefix(req.DSN, "postgres"):
		db, err = model.OpenPostgreSQL(req.DSN)
	default:
		middleware.ErrorResponse(
			c,
			http.StatusBadRequest,
			"invalid dsn, only mysql and postgres are supported",
		)

		return nil, false
	}

	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return nil, false
	}

	sqlDB, err := db.DB()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return nil, false
	}
	defer sqlDB.Close()

	allChannels := make([]*OneAPIChannel, 0)
	if err := db.Model(&OneAPIChannel{}).Find(&allChannels).Error; err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return nil, false
	}

	return allChannels, true
}

func importGroupChannelsFromOneAPI(
	c *gin.Context,
	group string,
	req ImportChannelFromOneAPIRequest,
) {
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "group id is required")
		return
	}

	allChannels, ok := loadOneAPIChannels(c, req)
	if !ok {
		return
	}

	errs := make([]error, 0)
	for _, ch := range allChannels {
		if err := AddOneAPIGroupChannel(group, *ch); err != nil {
			errs = append(errs, err)
		}
	}

	middleware.SuccessResponse(c, errs)
}

// ImportGlobalGroupChannelFromOneAPI godoc
//
//	@Summary		Import group channel from OneAPI
//	@Description	Imports group channels from OneAPI from the global management view. The request body must include group_id.
//	@Tags			group_channels
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request	body		ImportChannelFromOneAPIRequest	true	"Import group channel from OneAPI request"
//	@Success		200		{object}	middleware.APIResponse{data=[]error}
//	@Router			/api/group_channels/import/oneapi [post]
func ImportGlobalGroupChannelFromOneAPI(c *gin.Context) {
	var req ImportChannelFromOneAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	importGroupChannelsFromOneAPI(c, req.GroupID, req)
}

// ImportGroupChannelFromOneAPI godoc
//
//	@Summary		Import group channel from OneAPI
//	@Description	Imports group channels from OneAPI into a group
//	@Tags			group-channel
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			group	path		string							true	"Group ID"
//	@Param			request	body		ImportChannelFromOneAPIRequest	true	"Import group channel from OneAPI request"
//	@Success		200		{object}	middleware.APIResponse{data=[]error}
//	@Router			/api/group/{group}/channels/import/oneapi [post]
func ImportGroupChannelFromOneAPI(c *gin.Context) {
	var req ImportChannelFromOneAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	importGroupChannelsFromOneAPI(c, c.Param("group"), req)
}
