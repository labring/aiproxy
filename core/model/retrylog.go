package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"gorm.io/gorm"
)

type RetryLog struct {
	RequestBody           string          `gorm:"type:text"                                         json:"request_body,omitempty"`
	ResponseBody          string          `gorm:"type:text"                                         json:"response_body,omitempty"`
	RequestBodyTruncated  bool            `                                                         json:"request_body_truncated,omitempty"`
	ResponseBodyTruncated bool            `                                                         json:"response_body_truncated,omitempty"`
	RequestAt             time.Time       `                                                         json:"request_at"`
	RetryAt               time.Time       `                                                         json:"retry_at,omitempty"`
	TTFBMilliseconds      ZeroNullInt64   `                                                         json:"ttfb_milliseconds,omitempty"`
	CreatedAt             time.Time       `gorm:"autoCreateTime;index"                              json:"created_at"`
	Model                 string          `gorm:"size:64"                                           json:"model"`
	RequestID             EmptyNullString `gorm:"type:char(16);index:,where:request_id is not null" json:"request_id"`
	ID                    int             `gorm:"primaryKey"                                        json:"id"`
	ChannelID             int             `                                                         json:"channel,omitempty"`
	Code                  int             `gorm:"index"                                             json:"code,omitempty"`
	Mode                  int             `                                                         json:"mode,omitempty"`
	RetryTimes            ZeroNullInt64   `                                                         json:"retry_times,omitempty"`
}

func (r *RetryLog) BeforeSave(_ *gorm.DB) (err error) {
	if reqMax := config.GetLogDetailRequestBodyMaxSize(); reqMax > 0 &&
		int64(len(r.RequestBody)) > reqMax {
		r.RequestBody = common.TruncateByRune(r.RequestBody, int(reqMax)) + "..."
		r.RequestBodyTruncated = true
	} else if reqMax < 0 {
		r.RequestBody = ""
		r.RequestBodyTruncated = true
	}

	if respMax := config.GetLogDetailResponseBodyMaxSize(); respMax > 0 &&
		int64(len(r.ResponseBody)) > respMax {
		r.ResponseBody = common.TruncateByRune(r.ResponseBody, int(respMax)) + "..."
		r.ResponseBodyTruncated = true
	} else if respMax < 0 {
		r.ResponseBody = ""
		r.ResponseBodyTruncated = true
	}

	return err
}

func (r *RetryLog) MarshalJSON() ([]byte, error) {
	type Alias RetryLog

	a := &struct {
		*Alias
		CreatedAt int64 `json:"created_at"`
		RequestAt int64 `json:"request_at"`
		RetryAt   int64 `json:"retry_at,omitempty"`
	}{
		Alias:     (*Alias)(r),
		CreatedAt: r.CreatedAt.UnixMilli(),
		RequestAt: r.RequestAt.UnixMilli(),
	}
	if !r.RetryAt.IsZero() {
		a.RetryAt = r.RetryAt.UnixMilli()
	}

	return sonic.Marshal(a)
}

func RecordRetryLog(
	requestID string,
	createAt time.Time,
	requestAt time.Time,
	retryAt time.Time,
	firstByteAt time.Time,
	code int,
	channelID int,
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

	log := &RetryLog{
		RequestID:        EmptyNullString(requestID),
		RequestAt:        requestAt,
		CreatedAt:        createAt,
		RetryAt:          retryAt,
		TTFBMilliseconds: ZeroNullInt64(firstByteAt.Sub(requestAt).Milliseconds()),
		Code:             code,
		Model:            modelName,
		Mode:             mode,
		ChannelID:        channelID,
		RetryTimes:       ZeroNullInt64(retryTimes),
		RequestBody:      requestDetail.RequestBody,
		ResponseBody:     requestDetail.ResponseBody,
	}

	return LogDB.Create(log).Error
}
