package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"gorm.io/gorm"
)

type GroupChannelLog struct {
	RequestDetail    *GroupChannelRequestDetail `gorm:"foreignKey:LogID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"request_detail,omitempty"`
	RequestAt        time.Time                  `                                                                      json:"request_at"`
	RetryAt          time.Time                  `                                                                      json:"retry_at,omitempty"`
	TTFBMilliseconds ZeroNullInt64              `                                                                      json:"ttfb_milliseconds,omitempty"`
	CreatedAt        time.Time                  `gorm:"autoCreateTime;index"                                           json:"created_at"`
	TokenName        string                     `gorm:"size:32"                                                        json:"token_name,omitempty"`
	Endpoint         EmptyNullString            `gorm:"size:64"                                                        json:"endpoint,omitempty"`
	Content          EmptyNullString            `gorm:"type:text"                                                      json:"content,omitempty"`
	GroupID          string                     `gorm:"size:64;index"                                                  json:"group,omitempty"`
	Model            string                     `gorm:"size:128;index"                                                 json:"model"`
	RequestID        EmptyNullString            `gorm:"type:char(16);index:,where:request_id is not null"              json:"request_id"`
	UpstreamID       EmptyNullString            `gorm:"type:varchar(256)"                                              json:"upstream_id,omitempty"`
	AsyncUsageStatus AsyncUsageStatus           `                                                                      json:"async_usage_status,omitempty"`
	ID               int                        `gorm:"primaryKey"                                                     json:"id"`
	TokenID          int                        `gorm:"index"                                                          json:"token_id,omitempty"`
	GroupChannelID   int                        `gorm:"index"                                                          json:"group_channel_id,omitempty"`
	Code             int                        `gorm:"index"                                                          json:"code,omitempty"`
	Mode             int                        `                                                                      json:"mode,omitempty"`
	IP               EmptyNullString            `gorm:"size:45;index:,where:ip is not null"                            json:"ip,omitempty"`
	RetryTimes       ZeroNullInt64              `                                                                      json:"retry_times,omitempty"`
	Price            Price                      `gorm:"embedded"                                                       json:"price,omitempty"`
	Usage            Usage                      `gorm:"embedded"                                                       json:"usage,omitempty"`
	UsageContext     UsageContext               `gorm:"embedded"                                                       json:"usage_context,omitempty"`
	Amount           Amount                     `gorm:"embedded"                                                       json:"amount,omitempty"`
	PromptCacheKey   EmptyNullString            `gorm:"type:text"                                                      json:"prompt_cache_key,omitempty"`
	User             EmptyNullString            `gorm:"type:text"                                                      json:"user,omitempty"`
	Metadata         map[string]string          `gorm:"serializer:fastjson;type:text"                                  json:"metadata,omitempty"`
}

type GroupChannelRequestDetail struct {
	CreatedAt             time.Time `gorm:"autoCreateTime;index" json:"-"`
	RequestBody           string    `gorm:"type:text"            json:"request_body,omitempty"`
	ResponseBody          string    `gorm:"type:text"            json:"response_body,omitempty"`
	RequestBodyTruncated  bool      `                            json:"request_body_truncated,omitempty"`
	ResponseBodyTruncated bool      `                            json:"response_body_truncated,omitempty"`
	ID                    int       `gorm:"primaryKey"           json:"id"`
	LogID                 int       `gorm:"index"                json:"log_id"`
}

type GroupChannelRetryLog struct {
	RequestBody           string          `gorm:"type:text"                                         json:"request_body,omitempty"`
	ResponseBody          string          `gorm:"type:text"                                         json:"response_body,omitempty"`
	RequestBodyTruncated  bool            `                                                         json:"request_body_truncated,omitempty"`
	ResponseBodyTruncated bool            `                                                         json:"response_body_truncated,omitempty"`
	RequestAt             time.Time       `                                                         json:"request_at"`
	RetryAt               time.Time       `                                                         json:"retry_at,omitempty"`
	TTFBMilliseconds      ZeroNullInt64   `                                                         json:"ttfb_milliseconds,omitempty"`
	CreatedAt             time.Time       `gorm:"autoCreateTime;index"                              json:"created_at"`
	GroupID               string          `gorm:"size:64;index"                                     json:"group,omitempty"`
	Model                 string          `gorm:"size:128"                                          json:"model"`
	RequestID             EmptyNullString `gorm:"type:char(16);index:,where:request_id is not null" json:"request_id"`
	ID                    int             `gorm:"primaryKey"                                        json:"id"`
	GroupChannelID        int             `                                                         json:"group_channel_id,omitempty"`
	Code                  int             `gorm:"index"                                             json:"code,omitempty"`
	Mode                  int             `                                                         json:"mode,omitempty"`
	RetryTimes            ZeroNullInt64   `                                                         json:"retry_times,omitempty"`
}

func (l *GroupChannelLog) MarshalJSON() ([]byte, error) {
	type Alias GroupChannelLog

	a := &struct {
		*Alias
		CreatedAt  int64   `json:"created_at"`
		RequestAt  int64   `json:"request_at"`
		RetryAt    int64   `json:"retry_at,omitempty"`
		UsedAmount float64 `json:"used_amount,omitempty"`
	}{
		Alias:      (*Alias)(l),
		CreatedAt:  l.CreatedAt.UnixMilli(),
		RequestAt:  l.RequestAt.UnixMilli(),
		UsedAmount: l.Amount.UsedAmount,
	}
	if !l.RetryAt.IsZero() {
		a.RetryAt = l.RetryAt.UnixMilli()
	}

	return sonic.Marshal(a)
}

func newGroupChannelRequestDetail(detail *RequestDetail) *GroupChannelRequestDetail {
	if detail == nil {
		return nil
	}

	return &GroupChannelRequestDetail{
		CreatedAt:             detail.CreatedAt,
		RequestBody:           detail.RequestBody,
		ResponseBody:          detail.ResponseBody,
		RequestBodyTruncated:  detail.RequestBodyTruncated,
		ResponseBodyTruncated: detail.ResponseBodyTruncated,
	}
}

func CreateGroupChannelLogIndexes(db *gorm.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_group_channel_logs_group_creat ON group_channel_logs (group_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_logs_channel_creat ON group_channel_logs (group_id, group_channel_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_group_channel_logs_model_creat ON group_channel_logs (group_id, model, created_at DESC)",
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

func (l *GroupChannelLog) BeforeCreate(_ *gorm.DB) error {
	if len(l.Content) > contentMaxSize {
		l.Content = EmptyNullString(
			common.TruncateByRune(string(l.Content), contentMaxSize) + "...",
		)
	}

	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}

	if l.RequestAt.IsZero() {
		l.RequestAt = l.CreatedAt
	}

	return nil
}

func RecordGroupChannelRetryLog(
	requestID string,
	createAt time.Time,
	requestAt time.Time,
	retryAt time.Time,
	firstByteAt time.Time,
	group string,
	code int,
	groupChannelID int,
	modelName string,
	mode int,
	retryTimes int,
	requestDetail *RequestDetail,
) error {
	if createAt.IsZero() {
		createAt = time.Now()
	}

	if requestAt.IsZero() {
		requestAt = createAt
	}

	if firstByteAt.IsZero() || firstByteAt.Before(requestAt) {
		firstByteAt = requestAt
	}

	log := &GroupChannelRetryLog{
		RequestID:        EmptyNullString(requestID),
		RequestAt:        requestAt,
		CreatedAt:        createAt,
		RetryAt:          retryAt,
		TTFBMilliseconds: ZeroNullInt64(firstByteAt.Sub(requestAt).Milliseconds()),
		GroupID:          group,
		Code:             code,
		Model:            modelName,
		Mode:             mode,
		GroupChannelID:   groupChannelID,
		RetryTimes:       ZeroNullInt64(retryTimes),
	}
	if requestDetail != nil {
		log.RequestBody = requestDetail.RequestBody
		log.ResponseBody = requestDetail.ResponseBody
		log.RequestBodyTruncated = requestDetail.RequestBodyTruncated
		log.ResponseBodyTruncated = requestDetail.ResponseBodyTruncated
	}

	return LogDB.Create(log).Error
}

func RecordGroupChannelConsumeLog(
	requestID string,
	createAt time.Time,
	requestAt time.Time,
	retryAt time.Time,
	firstByteAt time.Time,
	group string,
	code int,
	groupChannelID int,
	modelName string,
	tokenID int,
	tokenName string,
	endpoint string,
	content string,
	mode int,
	ip string,
	retryTimes int,
	requestDetail *RequestDetail,
	usage Usage,
	usageContext UsageContext,
	modelPrice Price,
	amountDetail Amount,
	user string,
	metadata map[string]string,
	promptCacheKey string,
	upstreamID string,
	asyncUsageStatus AsyncUsageStatus,
) error {
	if createAt.IsZero() {
		createAt = time.Now()
	}

	if requestAt.IsZero() {
		requestAt = createAt
	}

	if firstByteAt.IsZero() || firstByteAt.Before(requestAt) {
		firstByteAt = requestAt
	}

	const maxUpstreamIDLength = 256
	if len(upstreamID) > maxUpstreamIDLength {
		upstreamID = upstreamID[:maxUpstreamIDLength]
	}

	log := &GroupChannelLog{
		RequestID:        EmptyNullString(requestID),
		RequestAt:        requestAt,
		CreatedAt:        createAt,
		RetryAt:          retryAt,
		TTFBMilliseconds: ZeroNullInt64(firstByteAt.Sub(requestAt).Milliseconds()),
		GroupID:          group,
		Code:             code,
		TokenID:          tokenID,
		TokenName:        tokenName,
		Model:            modelName,
		Mode:             mode,
		IP:               EmptyNullString(ip),
		GroupChannelID:   groupChannelID,
		Endpoint:         EmptyNullString(endpoint),
		Content:          EmptyNullString(content),
		RetryTimes:       ZeroNullInt64(retryTimes),
		RequestDetail:    newGroupChannelRequestDetail(requestDetail),
		Price:            modelPrice,
		Usage:            usage,
		UsageContext:     usageContext,
		Amount:           amountDetail,
		User:             EmptyNullString(user),
		Metadata:         metadata,
		PromptCacheKey:   EmptyNullString(promptCacheKey),
		UpstreamID:       EmptyNullString(upstreamID),
		AsyncUsageStatus: asyncUsageStatus,
	}

	return LogDB.Create(log).Error
}
