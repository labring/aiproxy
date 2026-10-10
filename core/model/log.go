package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type RequestDetail struct {
	CreatedAt             time.Time `gorm:"autoCreateTime;index" json:"-"`
	RequestBody           string    `gorm:"type:text"            json:"request_body,omitempty"`
	ResponseBody          string    `gorm:"type:text"            json:"response_body,omitempty"`
	RequestBodyTruncated  bool      `                            json:"request_body_truncated,omitempty"`
	ResponseBodyTruncated bool      `                            json:"response_body_truncated,omitempty"`
	ID                    int       `gorm:"primaryKey"           json:"id"`
	LogID                 int       `gorm:"index"                json:"log_id"`
}

func truncateDetailBody(body string, maxSize int64) (string, bool) {
	switch {
	case maxSize < 0:
		return "", true
	case maxSize == 0:
		return body, false
	case int64(len(body)) <= maxSize:
		return body, false
	default:
		if maxSize <= 3 {
			return common.TruncateByRune(body, int(maxSize)), true
		}

		return common.TruncateByRune(body, int(maxSize)-3) + "...", true
	}
}

func (d *RequestDetail) ApplyBodySizeLimits(requestMaxSize, responseMaxSize int64) {
	d.RequestBody, d.RequestBodyTruncated = truncateDetailBody(d.RequestBody, requestMaxSize)
	d.ResponseBody, d.ResponseBodyTruncated = truncateDetailBody(d.ResponseBody, responseMaxSize)
}

func (d *RequestDetail) DropInvalidUTF8Bodies() {
	if d.RequestBody != "" && !utf8.ValidString(d.RequestBody) {
		d.RequestBody = ""
		d.RequestBodyTruncated = false
	}

	if d.ResponseBody != "" && !utf8.ValidString(d.ResponseBody) {
		d.ResponseBody = ""
		d.ResponseBodyTruncated = false
	}
}

type Log struct {
	RequestDetail    *RequestDetail   `gorm:"foreignKey:LogID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"request_detail,omitempty"`
	RequestAt        time.Time        `                                                                      json:"request_at"`
	RetryAt          time.Time        `                                                                      json:"retry_at,omitempty"`
	TTFBMilliseconds ZeroNullInt64    `                                                                      json:"ttfb_milliseconds,omitempty"`
	CreatedAt        time.Time        `gorm:"autoCreateTime;index"                                           json:"created_at"`
	TokenName        string           `gorm:"size:32"                                                        json:"token_name,omitempty"`
	Endpoint         EmptyNullString  `gorm:"size:64"                                                        json:"endpoint,omitempty"`
	Content          EmptyNullString  `gorm:"type:text"                                                      json:"content,omitempty"`
	GroupID          string           `gorm:"size:64"                                                        json:"group,omitempty"`
	Model            string           `gorm:"size:128"                                                       json:"model"`
	RequestID        EmptyNullString  `gorm:"type:char(16);index:,where:request_id is not null"              json:"request_id"`
	UpstreamID       EmptyNullString  `gorm:"type:varchar(256)"                                              json:"upstream_id,omitempty"`
	AsyncUsageStatus AsyncUsageStatus `                                                                      json:"async_usage_status,omitempty"`
	ID               int              `gorm:"primaryKey"                                                     json:"id"`
	TokenID          int              `gorm:"index"                                                          json:"token_id,omitempty"`
	ChannelID        int              `                                                                      json:"channel,omitempty"`
	Code             int              `gorm:"index"                                                          json:"code,omitempty"`
	Mode             int              `                                                                      json:"mode,omitempty"`
	IP               EmptyNullString  `gorm:"size:45;index:,where:ip is not null"                            json:"ip,omitempty"`
	RetryTimes       ZeroNullInt64    `                                                                      json:"retry_times,omitempty"`
	Price            Price            `gorm:"embedded"                                                       json:"price,omitempty"`
	Usage            Usage            `gorm:"embedded"                                                       json:"usage,omitempty"`
	UsageContext     UsageContext     `gorm:"embedded"                                                       json:"usage_context,omitempty"`
	Amount           Amount           `gorm:"embedded"                                                       json:"amount,omitempty"`
	PromptCacheKey   EmptyNullString  `gorm:"type:text"                                                      json:"prompt_cache_key,omitempty"`
	// https://platform.openai.com/docs/guides/safety-best-practices#end-user-ids
	User     EmptyNullString   `gorm:"type:text"                     json:"user,omitempty"`
	Metadata map[string]string `gorm:"serializer:fastjson;type:text" json:"metadata,omitempty"`
}

func CreateLogIndexes(db *gorm.DB) error {
	var indexes []string
	if common.UsingSQLite {
		// not support INCLUDE
		indexes = []string{
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_model_creat ON logs (model, created_at DESC)",
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_channel_creat ON logs (channel_id, created_at DESC)",
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_channel_model_creat ON logs (channel_id, model, created_at DESC)",

			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_creat ON logs (group_id, created_at DESC)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_token_creat ON logs (group_id, token_name, created_at DESC)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_model_creat ON logs (group_id, model, created_at DESC)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_token_model_creat ON logs (group_id, token_name, model, created_at DESC)",
		}
	} else {
		indexes = []string{
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_model_creat ON logs (model, created_at DESC) INCLUDE (code)",
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_channel_creat ON logs (channel_id, created_at DESC) INCLUDE (code)",
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_channel_model_creat ON logs (channel_id, model, created_at DESC) INCLUDE (code)",

			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_creat ON logs (group_id, created_at DESC) INCLUDE (code)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_token_creat ON logs (group_id, token_name, created_at DESC) INCLUDE (code)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_model_creat ON logs (group_id, model, created_at DESC) INCLUDE (code)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_token_model_creat ON logs (group_id, token_name, model, created_at DESC) INCLUDE (code)",
		}
	}

	for _, index := range indexes {
		if err := db.Exec(index).Error; err != nil {
			return err
		}
	}

	return nil
}

const (
	contentMaxSize = 1024 // 1KB
)

func (l *Log) BeforeCreate(_ *gorm.DB) (err error) {
	if len(l.Content) > contentMaxSize {
		l.Content = common.TruncateByRune(l.Content, contentMaxSize) + "..."
	}

	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}

	if l.RequestAt.IsZero() {
		l.RequestAt = l.CreatedAt
	}

	return err
}

func (l *Log) MarshalJSON() ([]byte, error) {
	type Alias Log

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

func GetLogDetail(logID int) (*RequestDetail, error) {
	return getLogDetail(logID)
}

func getLogDetail(logID int) (*RequestDetail, error) {
	var detail RequestDetail

	err := LogDB.
		Model(&RequestDetail{}).
		Where("log_id = ?", logID).
		First(&detail).Error
	if err != nil {
		return nil, err
	}

	return &detail, nil
}

func GetGroupLogDetail(logID int, group string) (*RequestDetail, error) {
	if group == "" {
		return nil, errors.New("invalid group parameter")
	}

	return getLogDetailForGroup(logID, group)
}

func getLogDetailForGroup(logID int, group string) (*RequestDetail, error) {
	var detail RequestDetail

	err := LogDB.
		Model(&RequestDetail{}).
		Joins("JOIN logs ON logs.id = request_details.log_id").
		Where("logs.group_id = ?", group).
		Where("log_id = ?", logID).
		First(&detail).Error
	if err != nil {
		return nil, err
	}

	return &detail, nil
}

func GetGroupChannelLogDetailForGroup(logID int, group string) (*RequestDetail, error) {
	if group == "" {
		return nil, errors.New("invalid group parameter")
	}

	return getGroupChannelLogDetailByGroup(logID, group)
}

func GetGroupChannelLogDetail(logID int) (*RequestDetail, error) {
	return getGroupChannelLogDetailByGroup(logID, "")
}

func getGroupChannelLogDetailByGroup(logID int, group string) (*RequestDetail, error) {
	var detail GroupChannelRequestDetail

	query := LogDB.
		Model(&GroupChannelRequestDetail{}).
		Joins(
			"JOIN group_channel_logs ON group_channel_logs.id = group_channel_request_details.log_id",
		).
		Where("log_id = ?", logID)
	if group != "" {
		query = query.Where("group_channel_logs.group_id = ?", group)
	}

	err := query.First(&detail).Error
	if err != nil {
		return nil, err
	}

	return &RequestDetail{
		CreatedAt:             detail.CreatedAt,
		RequestBody:           detail.RequestBody,
		ResponseBody:          detail.ResponseBody,
		RequestBodyTruncated:  detail.RequestBodyTruncated,
		ResponseBodyTruncated: detail.ResponseBodyTruncated,
		ID:                    detail.ID,
		LogID:                 detail.LogID,
	}, nil
}

const defaultCleanLogBatchSize = 10000

func CleanLog(batchSize int, optimize bool) (err error) {
	err = cleanLog(batchSize)
	if err != nil {
		return err
	}

	err = cleanLogDetail(batchSize)
	if err != nil {
		return err
	}

	err = cleanAsyncUsageInfo(batchSize)
	if err != nil {
		return err
	}

	if optimize {
		return optimizeLog()
	}

	return nil
}

func cleanLog(batchSize int) error {
	if batchSize <= 0 {
		batchSize = defaultCleanLogBatchSize
	}

	logStorageHours := config.GetLogStorageHours()
	if logStorageHours != 0 {
		cutoff := time.Now().Add(-time.Duration(logStorageHours) * time.Hour)
		if err := cleanLogTable[Log](cutoff, batchSize); err != nil {
			return err
		}

		if err := cleanLogTable[GroupChannelLog](cutoff, batchSize); err != nil {
			return err
		}
	}

	retryLogStorageHours := config.GetRetryLogStorageHours()
	if retryLogStorageHours == 0 {
		retryLogStorageHours = logStorageHours
	}

	if retryLogStorageHours != 0 {
		cutoff := time.Now().Add(-time.Duration(retryLogStorageHours) * time.Hour)
		if err := cleanLogTable[RetryLog](cutoff, batchSize); err != nil {
			return err
		}

		if err := cleanLogTable[GroupChannelRetryLog](cutoff, batchSize); err != nil {
			return err
		}
	}

	if err := LogDB.
		Model(&StoreV2{}).
		Where("expires_at < ?", time.Now()).
		Delete(&StoreV2{}).
		Error; err != nil {
		return err
	}

	return LogDB.
		Model(&GroupChannelStoreV2{}).
		Where("expires_at < ?", time.Now()).
		Delete(&GroupChannelStoreV2{}).
		Error
}

func cleanLogTable[T any](cutoff time.Time, batchSize int) error {
	subQuery := LogDB.
		Model(new(T)).
		Where("created_at < ?", cutoff).
		Limit(batchSize).
		Select("id")

	return LogDB.
		Session(&gorm.Session{SkipDefaultTransaction: true}).
		Where("id IN (?)", subQuery).
		Delete(new(T)).
		Error
}

func cleanAsyncUsageInfo(batchSize int) error {
	logStorageHours := config.GetLogStorageHours()
	if logStorageHours == 0 {
		return nil
	}

	return CleanupFinishedAsyncUsages(time.Duration(logStorageHours)*time.Hour, batchSize)
}

func optimizeLog() error {
	switch {
	case common.UsingSQLite:
		return LogDB.Exec("VACUUM").Error
	default:
		return LogDB.Exec("VACUUM ANALYZE logs, group_channel_logs").Error
	}
}

func cleanLogDetail(batchSize int) error {
	detailStorageHours := config.GetLogDetailStorageHours()
	if detailStorageHours == 0 {
		detailStorageHours = config.GetLogStorageHours()
	}

	if detailStorageHours == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = defaultCleanLogBatchSize
	}

	cutoff := time.Now().Add(-time.Duration(detailStorageHours) * time.Hour)
	if err := cleanLogDetailTable[RequestDetail](cutoff, batchSize); err != nil {
		return err
	}

	return cleanLogDetailTable[GroupChannelRequestDetail](cutoff, batchSize)
}

func cleanLogDetailTable[T any](cutoff time.Time, batchSize int) error {
	subQuery := LogDB.
		Model(new(T)).
		Where("created_at < ?", cutoff).
		Limit(batchSize).
		Select("id")

	return LogDB.
		Session(&gorm.Session{SkipDefaultTransaction: true}).
		Where("id IN (?)", subQuery).
		Delete(new(T)).Error
}

func RecordConsumeLog(
	requestID string,
	createAt time.Time,
	requestAt time.Time,
	retryAt time.Time,
	firstByteAt time.Time,
	group string,
	code int,
	channelID int,
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

	// Truncate upstreamID to max length
	const maxUpstreamIDLength = 256
	if len(upstreamID) > maxUpstreamIDLength {
		upstreamID = upstreamID[:maxUpstreamIDLength]
	}

	log := &Log{
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
		ChannelID:        channelID,
		Endpoint:         EmptyNullString(endpoint),
		Content:          EmptyNullString(content),
		RetryTimes:       ZeroNullInt64(retryTimes),
		RequestDetail:    requestDetail,
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

func getLogOrder(order string) string {
	prefix, suffix, _ := strings.Cut(order, "-")
	switch prefix {
	case "created_at", "request_at", "id":
		switch suffix {
		case "asc":
			return prefix + " asc"
		default:
			return prefix + " desc"
		}
	default:
		return "created_at desc"
	}
}

type CodeType string

const (
	CodeTypeAll     CodeType = "all"
	CodeTypeSuccess CodeType = "success"
	CodeTypeError   CodeType = "error"
)

type GetLogsResult struct {
	Logs     []*Log   `json:"logs"`
	Total    int64    `json:"total"`
	Channels []int    `json:"channels,omitempty"`
	Models   []string `json:"models,omitempty"`
}

type GetGroupLogsResult struct {
	GetLogsResult
	TokenNames []string `json:"token_names"`
}

type GetGroupChannelLogsResult struct {
	Logs       []*GroupChannelLog `json:"logs"`
	Total      int64              `json:"total"`
	Models     []string           `json:"models,omitempty"`
	TokenNames []string           `json:"token_names"`
}

func buildGetLogsQuery(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	codeType CodeType,
	code int,
	ip string,
	user string,
) *gorm.DB {
	tx := LogDB.Model(&Log{})

	if requestID != "" {
		tx = tx.Where("request_id = ?", requestID)
	}

	if upstreamID != "" {
		tx = tx.Where("upstream_id = ?", upstreamID)
	}

	if ip != "" {
		tx = tx.Where("ip = ?", ip)
	}

	if group != "" {
		tx = tx.Where("group_id = ?", group)
	}

	if modelName != "" {
		tx = tx.Where("model = ?", modelName)
	}

	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}

	if channelID != 0 {
		tx = tx.Where("channel_id = ?", channelID)
	}

	switch {
	case !startTimestamp.IsZero() && !endTimestamp.IsZero():
		tx = tx.Where("created_at BETWEEN ? AND ?", startTimestamp, endTimestamp)
	case !startTimestamp.IsZero():
		tx = tx.Where("created_at >= ?", startTimestamp)
	case !endTimestamp.IsZero():
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	switch codeType {
	case CodeTypeSuccess:
		tx = tx.Where("code = 200")
	case CodeTypeError:
		tx = tx.Where("code != 200")
	default:
		if code != 0 {
			tx = tx.Where("code = ?", code)
		}
	}

	if tokenID != 0 {
		tx = tx.Where("token_id = ?", tokenID)
	}

	if user != "" {
		tx = tx.Where("user = ?", user)
	}

	return tx
}

func buildGetGroupChannelLogsQuery(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	codeType CodeType,
	code int,
	ip string,
	user string,
) *gorm.DB {
	return applyGroupChannelLogFilters(
		LogDB.Model(&GroupChannelLog{}),
		group,
		true,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		codeType,
		code,
		ip,
		user,
	)
}

func buildGetGlobalGroupChannelLogsQuery(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	codeType CodeType,
	code int,
	ip string,
	user string,
) *gorm.DB {
	return applyGroupChannelLogFilters(
		LogDB.Model(&GroupChannelLog{}),
		group,
		false,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		codeType,
		code,
		ip,
		user,
	)
}

func applyGroupChannelLogFilters(
	tx *gorm.DB,
	group string,
	requireGroup bool,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	codeType CodeType,
	code int,
	ip string,
	user string,
) *gorm.DB {
	if requestID != "" {
		tx = tx.Where("request_id = ?", requestID)
	}

	if upstreamID != "" {
		tx = tx.Where("upstream_id = ?", upstreamID)
	}

	if ip != "" {
		tx = tx.Where("ip = ?", ip)
	}

	if requireGroup || group != "" {
		tx = tx.Where("group_id = ?", group)
	}

	if modelName != "" {
		tx = tx.Where("model = ?", modelName)
	}

	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}

	if channelID != 0 {
		tx = tx.Where("group_channel_id = ?", channelID)
	}

	switch {
	case !startTimestamp.IsZero() && !endTimestamp.IsZero():
		tx = tx.Where("created_at BETWEEN ? AND ?", startTimestamp, endTimestamp)
	case !startTimestamp.IsZero():
		tx = tx.Where("created_at >= ?", startTimestamp)
	case !endTimestamp.IsZero():
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	switch codeType {
	case CodeTypeSuccess:
		tx = tx.Where("code = 200")
	case CodeTypeError:
		tx = tx.Where("code != 200")
	default:
		if code != 0 {
			tx = tx.Where("code = ?", code)
		}
	}

	if tokenID != 0 {
		tx = tx.Where("token_id = ?", tokenID)
	}

	if user != "" {
		tx = tx.Where("user = ?", user)
	}

	return tx
}

func buildGroupChannelLogsQuery(
	group string,
	requireGroup bool,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	codeType CodeType,
	code int,
	ip string,
	user string,
) *gorm.DB {
	if requireGroup {
		return buildGetGroupChannelLogsQuery(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			code,
			ip,
			user,
		)
	}

	return buildGetGlobalGroupChannelLogsQuery(
		group,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		codeType,
		code,
		ip,
		user,
	)
}

func getGroupChannelLogsByScope(
	group string,
	requireGroup bool,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (int64, []*GroupChannelLog, error) {
	var (
		total            int64
		groupChannelLogs []*GroupChannelLog
	)

	g := new(errgroup.Group)
	g.Go(func() error {
		return buildGroupChannelLogsQuery(
			group,
			requireGroup,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			code,
			ip,
			user,
		).Count(&total).Error
	})

	g.Go(func() error {
		query := buildGroupChannelLogsQuery(
			group,
			requireGroup,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			code,
			ip,
			user,
		)
		if withBody {
			query = query.Preload("RequestDetail")
		} else {
			query = query.Preload("RequestDetail", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "log_id")
			})
		}

		query = query.Order(getLogOrder(order))
		limit, offset := toLimitOffset(page, perPage)
		query = query.Limit(limit).Offset(offset)

		return query.Find(&groupChannelLogs).Error
	})

	if err := g.Wait(); err != nil {
		return 0, nil, err
	}

	return total, groupChannelLogs, nil
}

func getGlobalGroupChannelLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (int64, []*GroupChannelLog, error) {
	return getGroupChannelLogsByScope(
		group,
		false,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		page,
		perPage,
	)
}

func getGroupChannelLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (int64, []*GroupChannelLog, error) {
	return getGroupChannelLogsByScope(
		group,
		true,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		page,
		perPage,
	)
}

func getLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (int64, []*Log, error) {
	var (
		total int64
		logs  []*Log
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		return buildGetLogsQuery(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			code,
			ip,
			user,
		).Count(&total).Error
	})

	g.Go(func() error {
		query := buildGetLogsQuery(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			code,
			ip,
			user,
		)
		if withBody {
			query = query.Preload("RequestDetail")
		} else {
			query = query.Preload("RequestDetail", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "log_id")
			})
		}

		query = query.Order(getLogOrder(order))
		limit, offset := toLimitOffset(page, perPage)
		query = query.Limit(limit).Offset(offset)

		return query.Find(&logs).Error
	})

	if err := g.Wait(); err != nil {
		return 0, nil, err
	}

	return total, logs, nil
}

func GetLogs(
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetLogsResult, error) {
	var (
		total    int64
		logs     []*Log
		channels []int
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		channels, err = GetUsedChannels(startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error

		total, logs, err = getLogs(
			"",
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			0,
			"",
			channelID,
			order,
			codeType,
			code,
			withBody,
			ip,
			user,
			page,
			perPage,
		)

		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := &GetLogsResult{
		Logs:     logs,
		Total:    total,
		Channels: channels,
	}

	return result, nil
}

func GetGroupLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetGroupLogsResult, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	var (
		total      int64
		logs       []*Log
		tokenNames []string
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		total, logs, err = getLogs(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			order,
			codeType,
			code,
			withBody,
			ip,
			user,
			page,
			perPage,
		)

		return err
	})

	g.Go(func() error {
		var err error

		tokenNames, err = GetGroupUsedTokenNames(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupUsedModels(group, tokenName, startTimestamp, endTimestamp)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &GetGroupLogsResult{
		GetLogsResult: GetLogsResult{
			Logs:   logs,
			Total:  total,
			Models: models,
		},
		TokenNames: tokenNames,
	}, nil
}

func GetGroupChannelLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetGroupChannelLogsResult, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	var (
		total      int64
		logs       []*GroupChannelLog
		tokenNames []string
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		total, logs, err = getGroupChannelLogs(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			order,
			codeType,
			code,
			withBody,
			ip,
			user,
			page,
			perPage,
		)

		return err
	})

	g.Go(func() error {
		var err error

		tokenNames, err = GetGroupChannelTokenUsedTokenNames(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupChannelTokenUsedModels(group, tokenName, startTimestamp, endTimestamp)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &GetGroupChannelLogsResult{
		Logs:       logs,
		Total:      total,
		Models:     models,
		TokenNames: tokenNames,
	}, nil
}

func GetGlobalGroupChannelLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetGroupChannelLogsResult, error) {
	var (
		total      int64
		logs       []*GroupChannelLog
		tokenNames []string
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		total, logs, err = getGlobalGroupChannelLogs(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			order,
			codeType,
			code,
			withBody,
			ip,
			user,
			page,
			perPage,
		)

		return err
	})

	g.Go(func() error {
		var err error

		tokenNames, err = GetGlobalGroupChannelTokenUsedTokenNames(
			group,
			startTimestamp,
			endTimestamp,
		)

		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGlobalGroupChannelTokenUsedModels(
			group,
			tokenName,
			startTimestamp,
			endTimestamp,
		)

		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &GetGroupChannelLogsResult{
		Logs:       logs,
		Total:      total,
		Models:     models,
		TokenNames: tokenNames,
	}, nil
}

func exportLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*Log, error) {
	var logs []*Log

	query := buildGetLogsQuery(
		group,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		codeType,
		code,
		ip,
		user,
	)

	if withBody {
		query = query.Preload("RequestDetail")
	}

	query = query.Order(getLogOrder(order))
	if maxEntries > 0 {
		query = query.Limit(maxEntries)
	}

	return logs, query.Find(&logs).Error
}

func exportLogsRange(
	group string,
	startTimestamp time.Time,
	endExclusive time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*Log, error) {
	var logs []*Log

	query := buildGetLogsQuery(
		group,
		time.Time{},
		time.Time{},
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		codeType,
		code,
		ip,
		user,
	)

	if !startTimestamp.IsZero() {
		query = query.Where("created_at >= ?", startTimestamp)
	}

	if !endExclusive.IsZero() {
		query = query.Where("created_at < ?", endExclusive)
	}

	if withBody {
		query = query.Preload("RequestDetail")
	}

	query = query.Order(getLogOrder(order))
	if maxEntries > 0 {
		query = query.Limit(maxEntries)
	}

	return logs, query.Find(&logs).Error
}

func exportGroupChannelLogsByScope(
	group string,
	requireGroup bool,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*GroupChannelLog, error) {
	var groupChannelLogs []*GroupChannelLog

	query := buildGroupChannelLogsQuery(
		group,
		requireGroup,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		codeType,
		code,
		ip,
		user,
	)

	if withBody {
		query = query.Preload("RequestDetail")
	}

	query = query.Order(getLogOrder(order))
	if maxEntries > 0 {
		query = query.Limit(maxEntries)
	}

	return groupChannelLogs, query.Find(&groupChannelLogs).Error
}

func exportGroupChannelLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*GroupChannelLog, error) {
	return exportGroupChannelLogsByScope(
		group,
		true,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func exportGroupChannelLogsRangeByScope(
	group string,
	requireGroup bool,
	startTimestamp time.Time,
	endExclusive time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*GroupChannelLog, error) {
	var groupChannelLogs []*GroupChannelLog

	query := buildGroupChannelLogsQuery(
		group,
		requireGroup,
		time.Time{},
		time.Time{},
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		codeType,
		code,
		ip,
		user,
	)

	if !startTimestamp.IsZero() {
		query = query.Where("created_at >= ?", startTimestamp)
	}

	if !endExclusive.IsZero() {
		query = query.Where("created_at < ?", endExclusive)
	}

	if withBody {
		query = query.Preload("RequestDetail")
	}

	query = query.Order(getLogOrder(order))
	if maxEntries > 0 {
		query = query.Limit(maxEntries)
	}

	return groupChannelLogs, query.Find(&groupChannelLogs).Error
}

func exportGroupChannelLogsRange(
	group string,
	startTimestamp time.Time,
	endExclusive time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*GroupChannelLog, error) {
	return exportGroupChannelLogsRangeByScope(
		group,
		true,
		startTimestamp,
		endExclusive,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func ExportLogs(
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*Log, error) {
	return exportLogs(
		"",
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		0,
		"",
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func ExportGroupLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*Log, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return exportLogs(
		group,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func ExportGroupChannelLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*GroupChannelLog, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return exportGroupChannelLogs(
		group,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func ExportGlobalGroupChannelLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*GroupChannelLog, error) {
	return exportGroupChannelLogsByScope(
		group,
		false,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func ExportLogsRange(
	startTimestamp time.Time,
	endExclusive time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*Log, error) {
	return exportLogsRange(
		"",
		startTimestamp,
		endExclusive,
		modelName,
		requestID,
		upstreamID,
		0,
		"",
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func ExportGroupLogsRange(
	group string,
	startTimestamp time.Time,
	endExclusive time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*Log, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return exportLogsRange(
		group,
		startTimestamp,
		endExclusive,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func ExportGroupChannelLogsRange(
	group string,
	startTimestamp time.Time,
	endExclusive time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*GroupChannelLog, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	return exportGroupChannelLogsRange(
		group,
		startTimestamp,
		endExclusive,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func ExportGlobalGroupChannelLogsRange(
	group string,
	startTimestamp time.Time,
	endExclusive time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	maxEntries int,
) ([]*GroupChannelLog, error) {
	return exportGroupChannelLogsRangeByScope(
		group,
		false,
		startTimestamp,
		endExclusive,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		maxEntries,
	)
}

func buildSearchLogsQuery(
	group string,
	keyword string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	codeType CodeType,
	code int,
	ip string,
	user string,
) *gorm.DB {
	tx := LogDB.Model(&Log{})

	if requestID != "" {
		tx = tx.Where("request_id = ?", requestID)
	}

	if upstreamID != "" {
		tx = tx.Where("upstream_id = ?", upstreamID)
	}

	if ip != "" {
		tx = tx.Where("ip = ?", ip)
	}

	if group != "" {
		tx = tx.Where("group_id = ?", group)
	}

	if modelName != "" {
		tx = tx.Where("model = ?", modelName)
	}

	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}

	if channelID != 0 {
		tx = tx.Where("channel_id = ?", channelID)
	}

	switch {
	case !startTimestamp.IsZero() && !endTimestamp.IsZero():
		tx = tx.Where("created_at BETWEEN ? AND ?", startTimestamp, endTimestamp)
	case !startTimestamp.IsZero():
		tx = tx.Where("created_at >= ?", startTimestamp)
	case !endTimestamp.IsZero():
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	switch codeType {
	case CodeTypeSuccess:
		tx = tx.Where("code = 200")
	case CodeTypeError:
		tx = tx.Where("code != 200")
	default:
		if code != 0 {
			tx = tx.Where("code = ?", code)
		}
	}

	if tokenID != 0 {
		tx = tx.Where("token_id = ?", tokenID)
	}

	if user != "" {
		tx = tx.Where("user = ?", user)
	}

	// Handle keyword search for zero value fields
	if keyword != "" {
		var (
			conditions []string
			values     []any
		)

		if requestID == "" {
			conditions = append(conditions, "request_id = ?")
			values = append(values, keyword)
		}

		if upstreamID == "" {
			conditions = append(conditions, "upstream_id = ?")
			values = append(values, keyword)
		}

		if group == "" {
			conditions = append(conditions, "group_id = ?")
			values = append(values, keyword)
		}

		if modelName == "" {
			conditions = append(conditions, "model = ?")
			values = append(values, keyword)
		}

		if tokenName == "" {
			conditions = append(conditions, "token_name = ?")
			values = append(values, keyword)
		}

		// if num := String2Int(keyword); num != 0 {
		// 	if channelID == 0 {
		// 		conditions = append(conditions, "channel_id = ?")
		// 		values = append(values, num)
		// 	}
		// }

		// if ip != "" {
		// 	conditions = append(conditions, "ip = ?")
		// 	values = append(values, ip)
		// }

		// slow query
		// if common.UsingPostgreSQL {
		// 	conditions = append(conditions, "content ILIKE ?")
		// } else {
		// 	conditions = append(conditions, "content LIKE ?")
		// }
		// values = append(values, "%"+keyword+"%")

		if len(conditions) > 0 {
			tx = tx.Where(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), values...)
		}
	}

	return tx
}

func buildSearchGroupChannelLogsQueryByScope(
	group string,
	requireGroup bool,
	keyword string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	channelID int,
	codeType CodeType,
	code int,
	ip string,
	user string,
) *gorm.DB {
	tx := buildGroupChannelLogsQuery(
		group,
		requireGroup,
		startTimestamp,
		endTimestamp,
		modelName,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		channelID,
		codeType,
		code,
		ip,
		user,
	)

	if keyword == "" {
		return tx
	}

	var (
		conditions []string
		values     []any
	)

	if requestID == "" {
		conditions = append(conditions, "request_id = ?")
		values = append(values, keyword)
	}

	if upstreamID == "" {
		conditions = append(conditions, "upstream_id = ?")
		values = append(values, keyword)
	}

	if group == "" {
		conditions = append(conditions, "group_id = ?")
		values = append(values, keyword)
	}

	if modelName == "" {
		conditions = append(conditions, "model = ?")
		values = append(values, keyword)
	}

	if tokenName == "" {
		conditions = append(conditions, "token_name = ?")
		values = append(values, keyword)
	}

	if len(conditions) > 0 {
		tx = tx.Where(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), values...)
	}

	return tx
}

func searchLogs(
	group string,
	keyword string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (int64, []*Log, error) {
	var (
		total int64
		logs  []*Log
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		return buildSearchLogsQuery(
			group,
			keyword,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			modelName,
			startTimestamp,
			endTimestamp,
			channelID,
			codeType,
			code,
			ip,
			user,
		).Count(&total).Error
	})

	g.Go(func() error {
		query := buildSearchLogsQuery(
			group,
			keyword,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			modelName,
			startTimestamp,
			endTimestamp,
			channelID,
			codeType,
			code,
			ip,
			user,
		)

		if withBody {
			query = query.Preload("RequestDetail")
		} else {
			query = query.Preload("RequestDetail", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "log_id")
			})
		}

		query = query.Order(getLogOrder(order))
		limit, offset := toLimitOffset(page, perPage)
		query = query.Limit(limit).Offset(offset)

		return query.Find(&logs).Error
	})

	if err := g.Wait(); err != nil {
		return 0, nil, err
	}

	return total, logs, nil
}

func searchGroupChannelLogsByScope(
	group string,
	requireGroup bool,
	keyword string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (int64, []*GroupChannelLog, error) {
	var (
		total            int64
		groupChannelLogs []*GroupChannelLog
	)

	g := new(errgroup.Group)
	g.Go(func() error {
		return buildSearchGroupChannelLogsQueryByScope(
			group,
			requireGroup,
			keyword,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			code,
			ip,
			user,
		).Count(&total).Error
	})

	g.Go(func() error {
		query := buildSearchGroupChannelLogsQueryByScope(
			group,
			requireGroup,
			keyword,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			code,
			ip,
			user,
		)

		if withBody {
			query = query.Preload("RequestDetail")
		} else {
			query = query.Preload("RequestDetail", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "log_id")
			})
		}

		query = query.Order(getLogOrder(order))
		limit, offset := toLimitOffset(page, perPage)
		query = query.Limit(limit).Offset(offset)

		return query.Find(&groupChannelLogs).Error
	})

	if err := g.Wait(); err != nil {
		return 0, nil, err
	}

	return total, groupChannelLogs, nil
}

func searchGlobalGroupChannelLogs(
	group string,
	keyword string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (int64, []*GroupChannelLog, error) {
	return searchGroupChannelLogsByScope(
		group,
		false,
		keyword,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		modelName,
		startTimestamp,
		endTimestamp,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		page,
		perPage,
	)
}

func searchGroupChannelLogs(
	group string,
	keyword string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (int64, []*GroupChannelLog, error) {
	return searchGroupChannelLogsByScope(
		group,
		true,
		keyword,
		requestID,
		upstreamID,
		tokenID,
		tokenName,
		modelName,
		startTimestamp,
		endTimestamp,
		channelID,
		order,
		codeType,
		code,
		withBody,
		ip,
		user,
		page,
		perPage,
	)
}

func SearchLogs(
	keyword string,
	requestID string,
	upstreamID string,
	group string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetLogsResult, error) {
	var (
		total    int64
		logs     []*Log
		channels []int
		models   []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		total, logs, err = searchLogs(
			group,
			keyword,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			modelName,
			startTimestamp,
			endTimestamp,
			channelID,
			order,
			codeType,
			code,
			withBody,
			ip,
			user,
			page,
			perPage,
		)

		return err
	})

	g.Go(func() error {
		var err error

		channels, err = GetUsedChannels(startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetUsedModels(channelID, startTimestamp, endTimestamp)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := &GetLogsResult{
		Logs:     logs,
		Total:    total,
		Channels: channels,
		Models:   models,
	}

	return result, nil
}

func SearchGroupLogs(
	group string,
	keyword string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetGroupLogsResult, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	var (
		total      int64
		logs       []*Log
		tokenNames []string
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		total, logs, err = searchLogs(
			group,
			keyword,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			modelName,
			startTimestamp,
			endTimestamp,
			channelID,
			order,
			codeType,
			code,
			withBody,
			ip,
			user,
			page,
			perPage,
		)

		return err
	})

	g.Go(func() error {
		var err error

		tokenNames, err = GetGroupUsedTokenNames(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupUsedModels(group, tokenName, startTimestamp, endTimestamp)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := &GetGroupLogsResult{
		GetLogsResult: GetLogsResult{
			Logs:   logs,
			Total:  total,
			Models: models,
		},
		TokenNames: tokenNames,
	}

	return result, nil
}

func SearchGroupChannelLogs(
	group string,
	keyword string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetGroupChannelLogsResult, error) {
	if group == "" {
		return nil, errors.New("group is required")
	}

	var (
		total      int64
		logs       []*GroupChannelLog
		tokenNames []string
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		total, logs, err = searchGroupChannelLogs(
			group,
			keyword,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			modelName,
			startTimestamp,
			endTimestamp,
			channelID,
			order,
			codeType,
			code,
			withBody,
			ip,
			user,
			page,
			perPage,
		)

		return err
	})

	g.Go(func() error {
		var err error

		tokenNames, err = GetGroupChannelTokenUsedTokenNames(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGroupChannelTokenUsedModels(group, tokenName, startTimestamp, endTimestamp)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := &GetGroupChannelLogsResult{
		Logs:       logs,
		Total:      total,
		Models:     models,
		TokenNames: tokenNames,
	}

	return result, nil
}

func SearchGlobalGroupChannelLogs(
	group string,
	keyword string,
	requestID string,
	upstreamID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetGroupChannelLogsResult, error) {
	var (
		total      int64
		logs       []*GroupChannelLog
		tokenNames []string
		models     []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error

		total, logs, err = searchGlobalGroupChannelLogs(
			group,
			keyword,
			requestID,
			upstreamID,
			tokenID,
			tokenName,
			modelName,
			startTimestamp,
			endTimestamp,
			channelID,
			order,
			codeType,
			code,
			withBody,
			ip,
			user,
			page,
			perPage,
		)

		return err
	})

	g.Go(func() error {
		var err error

		tokenNames, err = GetGlobalGroupChannelTokenUsedTokenNames(
			group,
			startTimestamp,
			endTimestamp,
		)

		return err
	})

	g.Go(func() error {
		var err error

		models, err = GetGlobalGroupChannelTokenUsedModels(
			group,
			tokenName,
			startTimestamp,
			endTimestamp,
		)

		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := &GetGroupChannelLogsResult{
		Logs:       logs,
		Total:      total,
		Models:     models,
		TokenNames: tokenNames,
	}

	return result, nil
}

func DeleteOldLog(timestamp time.Time) (int64, error) {
	return deleteOldLogsFromTable[Log](timestamp)
}

func DeleteOldGroupChannelLog(timestamp time.Time) (int64, error) {
	return deleteOldLogsFromTable[GroupChannelLog](timestamp)
}

func DeleteOldGroupChannelLogForGroup(groupID string, timestamp time.Time) (int64, error) {
	if groupID == "" {
		return 0, errors.New("group is required")
	}

	result := LogDB.
		Where("group_id = ? AND created_at < ?", groupID, timestamp).
		Delete(&GroupChannelLog{})

	return result.RowsAffected, result.Error
}

func DeleteGroupLogs(groupID string) (int64, error) {
	if groupID == "" {
		return 0, errors.New("group is required")
	}

	return deleteGroupLogsFromTable[Log](groupID)
}

func DeleteGroupChannelLogs(groupID string) (int64, error) {
	if groupID == "" {
		return 0, errors.New("group is required")
	}

	return deleteGroupLogsFromTable[GroupChannelLog](groupID)
}

func deleteOldLogsFromTable[T any](timestamp time.Time) (int64, error) {
	result := LogDB.Where("created_at < ?", timestamp).Delete(new(T))
	return result.RowsAffected, result.Error
}

func deleteGroupLogsFromTable[T any](groupID string) (int64, error) {
	result := LogDB.Where("group_id = ?", groupID).Delete(new(T))
	return result.RowsAffected, result.Error
}

func GetIPGroups(threshold int, start, end time.Time) (map[string][]string, error) {
	if threshold < 1 {
		threshold = 1
	}

	var selectClause string
	if common.UsingSQLite {
		selectClause = "ip, GROUP_CONCAT(DISTINCT group_id) as groups"
	} else {
		selectClause = "ip, STRING_AGG(DISTINCT group_id, ',') as groups"
	}

	db := LogDB.Model(&Log{}).
		Select(selectClause).
		Group("ip").
		Having("COUNT(DISTINCT group_id) >= ?", threshold)

	switch {
	case !start.IsZero() && !end.IsZero():
		db = db.Where("created_at BETWEEN ? AND ?", start, end)
	case !start.IsZero():
		db = db.Where("created_at >= ?", start)
	case !end.IsZero():
		db = db.Where("created_at <= ?", end)
	}

	db.Where("ip IS NOT NULL AND ip != '' AND group_id != ''")

	result := make(map[string][]string)

	rows, err := db.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			ip     string
			groups string
		)

		err = rows.Scan(&ip, &groups)
		if err != nil {
			return nil, err
		}

		result[ip] = strings.Split(groups, ",")
	}

	return result, nil
}
