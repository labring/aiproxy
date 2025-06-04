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
	DSN string `json:"dsn"`
}

func AddOneAPIChannel(ch OneAPIChannel) error {
	add := AddChannelRequest{
		Type:         model.ChannelType(ch.Type),
		Name:         ch.Name,
		Key:          ch.Key,
		BaseURL:      ch.BaseURL,
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
	chs, err := add.ToChannels()
	if err != nil {
		return err
	}
	return model.BatchInsertChannels(chs)
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

	if req.DSN == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "sql dsn is required")
		return
	}

	var db *gorm.DB
	var err error

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
		return
	}

	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	defer sqlDB.Close()

	allChannels := make([]*OneAPIChannel, 0)
	err = db.Model(&OneAPIChannel{}).Find(&allChannels).Error
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
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
