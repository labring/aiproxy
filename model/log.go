package model

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/common"
	"github.com/labring/aiproxy/common/config"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type RequestDetail struct {
	CreatedAt             time.Time `gorm:"autoCreateTime;index"              json:"-"`
	RequestBody           string    `gorm:"type:text"                         json:"request_body,omitempty"`
	ResponseBody          string    `gorm:"type:text"                         json:"response_body,omitempty"`
	RequestBodyTruncated  bool      `json:"request_body_truncated,omitempty"`
	ResponseBodyTruncated bool      `json:"response_body_truncated,omitempty"`
	ID                    int       `gorm:"primaryKey"                        json:"id"`
	LogID                 int       `gorm:"index"                             json:"log_id"`
}

func (d *RequestDetail) BeforeSave(_ *gorm.DB) (err error) {
	if reqMax := config.GetLogDetailRequestBodyMaxSize(); reqMax > 0 && int64(len(d.RequestBody)) > reqMax {
		d.RequestBody = common.TruncateByRune(d.RequestBody, int(reqMax)) + "..."
		d.RequestBodyTruncated = true
	}
	if respMax := config.GetLogDetailResponseBodyMaxSize(); respMax > 0 && int64(len(d.ResponseBody)) > respMax {
		d.ResponseBody = common.TruncateByRune(d.ResponseBody, int(respMax)) + "..."
		d.ResponseBodyTruncated = true
	}
	return
}

type Price struct {
	InputPrice         float64 `json:"input_price,omitempty"`
	OutputPrice        float64 `json:"output_price,omitempty"`
	CachedPrice        float64 `json:"cached_price,omitempty"`
	CacheCreationPrice float64 `json:"cache_creation_price,omitempty"`
}

type Usage struct {
	InputTokens         int64 `json:"input_tokens,omitempty"`
	OutputTokens        int64 `json:"output_tokens,omitempty"`
	CachedTokens        int64 `json:"cached_tokens,omitempty"`
	CacheCreationTokens int64 `json:"cache_creation_tokens,omitempty"`
	TotalTokens         int64 `json:"total_tokens,omitempty"`
}

func (u *Usage) Add(other *Usage) {
	if other == nil {
		return
	}
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CachedTokens += other.CachedTokens
	u.CacheCreationTokens += other.CacheCreationTokens
	u.TotalTokens += other.TotalTokens
}

type Log struct {
	RequestDetail        *RequestDetail  `gorm:"foreignKey:LogID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"request_detail,omitempty"`
	RequestAt            time.Time       `json:"request_at"`
	RetryAt              time.Time       `json:"retry_at,omitempty"`
	TTFBMilliseconds     ZeroNullInt64   `json:"ttfb_milliseconds,omitempty"`
	TimestampTruncByHour int64           `json:"timestamp_trunc_by_hour"`
	CreatedAt            time.Time       `gorm:"autoCreateTime;index"                                           json:"created_at"`
	TokenName            string          `json:"token_name,omitempty"`
	Endpoint             EmptyNullString `json:"endpoint,omitempty"`
	Content              EmptyNullString `gorm:"type:text"                                                      json:"content,omitempty"`
	GroupID              string          `json:"group,omitempty"`
	Model                string          `json:"model"`
	RequestID            EmptyNullString `gorm:"index:,where:request_id is not null"                            json:"request_id"`
	ID                   int             `gorm:"primaryKey"                                                     json:"id"`
	TokenID              int             `gorm:"index"                                                          json:"token_id,omitempty"`
	ChannelID            int             `json:"channel,omitempty"`
	Code                 int             `gorm:"index"                                                          json:"code,omitempty"`
	Mode                 int             `json:"mode,omitempty"`
	IP                   EmptyNullString `gorm:"index:,where:ip is not null"                                    json:"ip,omitempty"`
	RetryTimes           ZeroNullInt64   `json:"retry_times,omitempty"`
	DownstreamResult     bool            `json:"downstream_result,omitempty"`
	Price                Price           `gorm:"embedded"                                                       json:"price,omitempty"`
	Usage                Usage           `gorm:"embedded"                                                       json:"usage,omitempty"`
	UsedAmount           float64         `json:"used_amount,omitempty"`
}

func CreateLogIndexes(db *gorm.DB) error {
	var indexes []string
	if common.UsingSQLite {
		// not support INCLUDE
		indexes = []string{
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_model_reqat ON logs (model, request_at DESC)",
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_channel_reqat ON logs (channel_id, request_at DESC)",
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_channel_model_reqat ON logs (channel_id, model, request_at DESC)",

			// global hour indexes, used by global dashboard
			"CREATE INDEX IF NOT EXISTS idx_model_trunchour ON logs (model, timestamp_trunc_by_hour)",
			"CREATE INDEX IF NOT EXISTS idx_channel_trunchour ON logs (channel_id, timestamp_trunc_by_hour)",
			"CREATE INDEX IF NOT EXISTS idx_channel_model_trunchour ON logs (channel_id, model, timestamp_trunc_by_hour)",

			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_reqat ON logs (group_id, request_at DESC)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_token_reqat ON logs (group_id, token_name, request_at DESC)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_model_reqat ON logs (group_id, model, request_at DESC)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_token_model_reqat ON logs (group_id, token_name, model, request_at DESC)",

			// hour indexes, used by dashboard
			"CREATE INDEX IF NOT EXISTS idx_group_trunchour ON logs (group_id, timestamp_trunc_by_hour DESC)",
			"CREATE INDEX IF NOT EXISTS idx_group_model_trunchour ON logs (group_id, model, timestamp_trunc_by_hour DESC)",
			"CREATE INDEX IF NOT EXISTS idx_group_token_trunchour ON logs (group_id, token_name, timestamp_trunc_by_hour DESC)",
			"CREATE INDEX IF NOT EXISTS idx_group_model_token_trunchour ON logs (group_id, model, token_name, timestamp_trunc_by_hour DESC)",
		}
	} else {
		indexes = []string{
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_model_reqat ON logs (model, request_at DESC) INCLUDE (code, downstream_result)",
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_channel_reqat ON logs (channel_id, request_at DESC) INCLUDE (code, downstream_result)",
			// used by global search logs
			"CREATE INDEX IF NOT EXISTS idx_channel_model_reqat ON logs (channel_id, model, request_at DESC) INCLUDE (code, downstream_result)",

			// global hour indexes, used by global dashboard
			"CREATE INDEX IF NOT EXISTS idx_model_trunchour ON logs (model, timestamp_trunc_by_hour) INCLUDE (downstream_result)",
			"CREATE INDEX IF NOT EXISTS idx_channel_trunchour ON logs (channel_id, timestamp_trunc_by_hour) INCLUDE (downstream_result)",
			"CREATE INDEX IF NOT EXISTS idx_channel_model_trunchour ON logs (channel_id, model, timestamp_trunc_by_hour) INCLUDE (downstream_result)",

			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_reqat ON logs (group_id, request_at DESC) INCLUDE (code, downstream_result)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_token_reqat ON logs (group_id, token_name, request_at DESC) INCLUDE (code, downstream_result)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_model_reqat ON logs (group_id, model, request_at DESC) INCLUDE (code, downstream_result)",
			// used by search group logs
			"CREATE INDEX IF NOT EXISTS idx_group_token_model_reqat ON logs (group_id, token_name, model, request_at DESC) INCLUDE (code, downstream_result)",

			// hour indexes, used by dashboard
			"CREATE INDEX IF NOT EXISTS idx_group_trunchour ON logs (group_id, timestamp_trunc_by_hour DESC) INCLUDE (downstream_result)",
			"CREATE INDEX IF NOT EXISTS idx_group_model_trunchour ON logs (group_id, model, timestamp_trunc_by_hour DESC) INCLUDE (downstream_result)",
			"CREATE INDEX IF NOT EXISTS idx_group_token_trunchour ON logs (group_id, token_name, timestamp_trunc_by_hour DESC) INCLUDE (downstream_result)",
			"CREATE INDEX IF NOT EXISTS idx_group_model_token_trunchour ON logs (group_id, model, token_name, timestamp_trunc_by_hour DESC) INCLUDE (downstream_result)",
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
	contentMaxSize = 2 * 1024 // 2KB
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
	if l.TimestampTruncByHour == 0 {
		l.TimestampTruncByHour = l.RequestAt.Truncate(time.Hour).Unix()
	}
	return
}

func (l *Log) MarshalJSON() ([]byte, error) {
	type Alias Log
	a := &struct {
		*Alias
		CreatedAt int64 `json:"created_at"`
		RequestAt int64 `json:"request_at"`
		RetryAt   int64 `json:"retry_at,omitempty"`
	}{
		Alias:     (*Alias)(l),
		CreatedAt: l.CreatedAt.UnixMilli(),
		RequestAt: l.RequestAt.UnixMilli(),
	}
	if !l.RetryAt.IsZero() {
		a.RetryAt = l.RetryAt.UnixMilli()
	}
	return sonic.Marshal(a)
}

func GetLogDetail(logID int) (*RequestDetail, error) {
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
	if group == "" || group == "*" {
		return nil, errors.New("invalid group parameter")
	}
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

const defaultCleanLogBatchSize = 5000

func CleanLog(batchSize int, optimize bool) error {
	err := cleanLog(batchSize, optimize)
	if err != nil {
		return err
	}
	return cleanLogDetail(batchSize, optimize)
}

func cleanLog(batchSize int, optimize bool) error {
	if batchSize <= 0 {
		batchSize = defaultCleanLogBatchSize
	}
	logStorageHours := config.GetLogStorageHours()
	if logStorageHours > 0 {
		subQuery := LogDB.
			Model(&Log{}).
			Where(
				"created_at < ?",
				time.Now().Add(-time.Duration(logStorageHours)*time.Hour),
			).
			Limit(batchSize).
			Select("id")

		err := LogDB.
			Session(&gorm.Session{SkipDefaultTransaction: true}).
			Where("id IN (?)", subQuery).
			Delete(&Log{}).Error
		if err != nil {
			return err
		}
	}

	logContentStorageHours := config.GetLogContentStorageHours()
	if logContentStorageHours <= 0 ||
		logContentStorageHours <= logStorageHours {
		if optimize {
			return optimizeLog()
		}
		return nil
	}

	// Find the minimum ID that meets our criteria
	var id int64
	err := LogDB.
		Model(&Log{}).
		Where(
			"created_at < ?",
			time.Now().Truncate(time.Hour).Add(-time.Duration(logContentStorageHours)*time.Hour),
		).
		Where("content IS NOT NULL").
		Order("created_at DESC").
		Limit(1).
		Select("id").
		Scan(&id).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if id > 0 {
		// Process in batches based on ID range
		err = LogDB.
			Model(&Log{}).
			Session(&gorm.Session{SkipDefaultTransaction: true}).
			Where(
				"id BETWEEN ? AND ? AND content IS NOT NULL",
				id-int64(batchSize),
				id,
			).
			UpdateColumns(map[string]any{
				"content":           gorm.Expr("NULL"),
				"ip":                gorm.Expr("NULL"),
				"endpoint":          gorm.Expr("NULL"),
				"ttfb_milliseconds": gorm.Expr("NULL"),
			}).Error
	}
	if err != nil {
		return err
	}
	if !optimize {
		return nil
	}

	return optimizeLog()
}

func optimizeLog() error {
	switch {
	case common.UsingPostgreSQL:
		return LogDB.Exec("VACUUM ANALYZE logs").Error
	case common.UsingMySQL:
		return LogDB.Exec("OPTIMIZE TABLE logs").Error
	case common.UsingSQLite:
		return LogDB.Exec("VACUUM").Error
	}
	return nil
}

func cleanLogDetail(batchSize int, optimize bool) error {
	detailStorageHours := config.GetLogDetailStorageHours()
	if detailStorageHours <= 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = defaultCleanLogBatchSize
	}

	subQuery := LogDB.
		Model(&RequestDetail{}).
		Where(
			"created_at < ?",
			time.Now().Add(-time.Duration(detailStorageHours)*time.Hour),
		).
		Limit(batchSize).
		Select("id")

	err := LogDB.
		Session(&gorm.Session{SkipDefaultTransaction: true}).
		Where("id IN (?)", subQuery).
		Delete(&RequestDetail{}).Error
	if err != nil {
		return err
	}
	if !optimize {
		return nil
	}

	return optimizeLogDetail()
}

func optimizeLogDetail() error {
	switch {
	case common.UsingPostgreSQL:
		return LogDB.Exec("VACUUM ANALYZE request_details").Error
	case common.UsingMySQL:
		return LogDB.Exec("OPTIMIZE TABLE request_details").Error
	case common.UsingSQLite:
		return LogDB.Exec("VACUUM").Error
	}
	return nil
}

func RecordConsumeLog(
	requestID string,
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
	downstreamResult bool,
	usage Usage,
	modelPrice Price,
	amount float64,
) error {
	now := time.Now()
	if requestAt.IsZero() {
		requestAt = now
	}
	if firstByteAt.IsZero() || firstByteAt.Before(requestAt) {
		firstByteAt = requestAt
	}
	log := &Log{
		RequestID:        EmptyNullString(requestID),
		RequestAt:        requestAt,
		CreatedAt:        now,
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
		DownstreamResult: downstreamResult,
		Price:            modelPrice,
		Usage:            usage,
		UsedAmount:       amount,
	}
	return LogDB.Create(log).Error
}

func getLogOrder(order string) string {
	prefix, suffix, _ := strings.Cut(order, "-")
	switch prefix {
	case "request_at", "id", "created_at":
		switch suffix {
		case "asc":
			return prefix + " asc"
		default:
			return prefix + " desc"
		}
	default:
		return "request_at desc"
	}
}

type CodeType string

const (
	CodeTypeAll     CodeType = "all"
	CodeTypeSuccess CodeType = "success"
	CodeTypeError   CodeType = "error"
)

type GetLogsResult struct {
	Logs     []*Log `json:"logs"`
	Total    int64  `json:"total"`
	Channels []int  `json:"channels,omitempty"`
}

type GetGroupLogsResult struct {
	GetLogsResult
	Models     []string `json:"models"`
	TokenNames []string `json:"token_names"`
}

func buildGetLogsQuery(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	tokenID int,
	tokenName string,
	channelID int,
	codeType CodeType,
	ip string,
	resultOnly bool,
) *gorm.DB {
	tx := LogDB.Model(&Log{})

	if requestID != "" {
		tx = tx.Where("request_id = ?", requestID)
	}
	if ip != "" {
		tx = tx.Where("ip = ?", ip)
	}

	if group == "" {
		tx = tx.Where("group_id = ''")
	} else if group != "*" {
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
		tx = tx.Where("request_at BETWEEN ? AND ?", startTimestamp, endTimestamp)
	case !startTimestamp.IsZero():
		tx = tx.Where("request_at >= ?", startTimestamp)
	case !endTimestamp.IsZero():
		tx = tx.Where("request_at <= ?", endTimestamp)
	}

	if resultOnly {
		tx = tx.Where("downstream_result = true")
	}

	switch codeType {
	case CodeTypeSuccess:
		tx = tx.Where("code = 200")
	case CodeTypeError:
		tx = tx.Where("code != 200")
	}

	if tokenID != 0 {
		tx = tx.Where("token_id = ?", tokenID)
	}
	return tx
}

func getLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	withBody bool,
	ip string,
	page int,
	perPage int,
	resultOnly bool,
) (int64, []*Log, error) {
	var total int64
	var logs []*Log

	g := new(errgroup.Group)

	g.Go(func() error {
		return buildGetLogsQuery(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			ip,
			resultOnly,
		).Count(&total).Error
	})

	g.Go(func() error {
		query := buildGetLogsQuery(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
			tokenID,
			tokenName,
			channelID,
			codeType,
			ip,
			resultOnly,
		)
		if withBody {
			query = query.Preload("RequestDetail")
		} else {
			query = query.Preload("RequestDetail", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "log_id")
			})
		}

		limit, offset := toLimitOffset(page, perPage)
		return query.
			Order(getLogOrder(order)).
			Limit(limit).
			Offset(offset).
			Find(&logs).Error
	})

	if err := g.Wait(); err != nil {
		return 0, nil, err
	}

	return total, logs, nil
}

func GetLogs(
	group string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	modelName string,
	requestID string,
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	withBody bool,
	ip string,
	page int,
	perPage int,
	resultOnly bool,
) (*GetLogsResult, error) {
	var total int64
	var logs []*Log
	var channels []int

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error
		channels, err = GetUsedChannels(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error
		total, logs, err = getLogs(group, startTimestamp, endTimestamp, modelName, requestID, tokenID, tokenName, channelID, order, codeType, withBody, ip, page, perPage, resultOnly)
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
	tokenID int,
	tokenName string,
	channelID int,
	order string,
	codeType CodeType,
	withBody bool,
	ip string,
	page int,
	perPage int,
	resultOnly bool,
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
		total, logs, err = getLogs(group, startTimestamp, endTimestamp, modelName, requestID, tokenID, tokenName, channelID, order, codeType, withBody, ip, page, perPage, resultOnly)
		return err
	})

	g.Go(func() error {
		var err error
		tokenNames, err = GetUsedTokenNames(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error
		models, err = GetUsedModels(group, startTimestamp, endTimestamp)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &GetGroupLogsResult{
		GetLogsResult: GetLogsResult{
			Logs:  logs,
			Total: total,
		},
		Models:     models,
		TokenNames: tokenNames,
	}, nil
}

func buildSearchLogsQuery(
	group string,
	keyword string,
	requestID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	codeType CodeType,
	ip string,
	resultOnly bool,
) *gorm.DB {
	tx := LogDB.Model(&Log{})

	if requestID != "" {
		tx = tx.Where("request_id = ?", requestID)
	}
	if ip != "" {
		tx = tx.Where("ip = ?", ip)
	}

	if group == "" {
		tx = tx.Where("group_id = ''")
	} else if group != "*" {
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
		tx = tx.Where("request_at BETWEEN ? AND ?", startTimestamp, endTimestamp)
	case !startTimestamp.IsZero():
		tx = tx.Where("request_at >= ?", startTimestamp)
	case !endTimestamp.IsZero():
		tx = tx.Where("request_at <= ?", endTimestamp)
	}

	if resultOnly {
		tx = tx.Where("downstream_result = true")
	}

	switch codeType {
	case CodeTypeSuccess:
		tx = tx.Where("code = 200")
	case CodeTypeError:
		tx = tx.Where("code != 200")
	}

	if tokenID != 0 {
		tx = tx.Where("token_id = ?", tokenID)
	}

	// Handle keyword search for zero value fields
	if keyword != "" {
		var conditions []string
		var values []interface{}

		if requestID == "" {
			conditions = append(conditions, "request_id = ?")
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

func searchLogs(
	group string,
	keyword string,
	requestID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	withBody bool,
	ip string,
	page int,
	perPage int,
	resultOnly bool,
) (int64, []*Log, error) {
	var total int64
	var logs []*Log

	g := new(errgroup.Group)

	g.Go(func() error {
		return buildSearchLogsQuery(
			group,
			keyword,
			requestID,
			tokenID,
			tokenName,
			modelName,
			startTimestamp,
			endTimestamp,
			channelID,
			codeType,
			ip,
			resultOnly,
		).Count(&total).Error
	})

	g.Go(func() error {
		query := buildSearchLogsQuery(
			group,
			keyword,
			requestID,
			tokenID,
			tokenName,
			modelName,
			startTimestamp,
			endTimestamp,
			channelID,
			codeType,
			ip,
			resultOnly,
		)

		if withBody {
			query = query.Preload("RequestDetail")
		} else {
			query = query.Preload("RequestDetail", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "log_id")
			})
		}

		limit, offset := toLimitOffset(page, perPage)
		return query.
			Order(getLogOrder(order)).
			Limit(limit).
			Offset(offset).
			Find(&logs).Error
	})

	if err := g.Wait(); err != nil {
		return 0, nil, err
	}

	return total, logs, nil
}

func SearchLogs(
	group string,
	keyword string,
	requestID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	withBody bool,
	ip string,
	page int,
	perPage int,
	resultOnly bool,
) (*GetLogsResult, error) {
	var total int64
	var logs []*Log
	var channels []int

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error
		total, logs, err = searchLogs(group, keyword, requestID, tokenID, tokenName, modelName, startTimestamp, endTimestamp, channelID, order, codeType, withBody, ip, page, perPage, resultOnly)
		return err
	})

	g.Go(func() error {
		var err error
		channels, err = GetUsedChannels(group, startTimestamp, endTimestamp)
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

func SearchGroupLogs(
	group string,
	keyword string,
	requestID string,
	tokenID int,
	tokenName string,
	modelName string,
	startTimestamp time.Time,
	endTimestamp time.Time,
	channelID int,
	order string,
	codeType CodeType,
	withBody bool,
	ip string,
	page int,
	perPage int,
	resultOnly bool,
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
		total, logs, err = searchLogs(group, keyword, requestID, tokenID, tokenName, modelName, startTimestamp, endTimestamp, channelID, order, codeType, withBody, ip, page, perPage, resultOnly)
		return err
	})

	g.Go(func() error {
		var err error
		tokenNames, err = GetUsedTokenNames(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error
		models, err = GetUsedModels(group, startTimestamp, endTimestamp)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := &GetGroupLogsResult{
		GetLogsResult: GetLogsResult{
			Logs:  logs,
			Total: total,
		},
		Models:     models,
		TokenNames: tokenNames,
	}

	return result, nil
}

func DeleteOldLog(timestamp time.Time) (int64, error) {
	result := LogDB.Where("request_at < ?", timestamp).Delete(&Log{})
	return result.RowsAffected, result.Error
}

func DeleteGroupLogs(groupID string) (int64, error) {
	if groupID == "" {
		return 0, errors.New("group is required")
	}
	result := LogDB.Where("group_id = ?", groupID).Delete(&Log{})
	return result.RowsAffected, result.Error
}

type ChartData struct {
	Timestamp           int64   `json:"timestamp"`
	RequestCount        int64   `json:"request_count"`
	UsedAmount          float64 `json:"used_amount"`
	InputTokens         int64   `json:"input_tokens,omitempty"`
	OutputTokens        int64   `json:"output_tokens,omitempty"`
	CachedTokens        int64   `json:"cached_tokens,omitempty"`
	CacheCreationTokens int64   `json:"cache_creation_tokens,omitempty"`
	TotalTokens         int64   `json:"total_tokens,omitempty"`
	ExceptionCount      int64   `json:"exception_count"`
}

type DashboardResponse struct {
	ChartData      []*ChartData `json:"chart_data"`
	TotalCount     int64        `json:"total_count"`
	ExceptionCount int64        `json:"exception_count"`

	RPM int64 `json:"rpm"`
	TPM int64 `json:"tpm"`

	UsedAmount          float64 `json:"used_amount"`
	InputTokens         int64   `json:"input_tokens,omitempty"`
	OutputTokens        int64   `json:"output_tokens,omitempty"`
	TotalTokens         int64   `json:"total_tokens,omitempty"`
	CachedTokens        int64   `json:"cached_tokens,omitempty"`
	CacheCreationTokens int64   `json:"cache_creation_tokens,omitempty"`

	Channels []int `json:"channels,omitempty"`
}

type GroupDashboardResponse struct {
	DashboardResponse
	Models     []string `json:"models"`
	TokenNames []string `json:"token_names"`
}

type TimeSpanType string

const (
	TimeSpanDay  TimeSpanType = "day"
	TimeSpanHour TimeSpanType = "hour"
)

func getChartDataFromLog(
	group string,
	start, end time.Time,
	tokenName, modelName string,
	channelID int,
	timeSpan TimeSpanType,
	resultOnly bool, tokenUsage bool,
) ([]*ChartData, error) {
	var query *gorm.DB
	if tokenUsage {
		query = LogDB.Table("logs").
			Select("timestamp_trunc_by_hour as timestamp, count(*) as request_count, sum(used_amount) as used_amount, sum(case when code != 200 then 1 else 0 end) as exception_count, sum(input_tokens) as input_tokens, sum(output_tokens) as output_tokens, sum(cached_tokens) as cached_tokens, sum(cache_creation_tokens) as cache_creation_tokens, sum(total_tokens) as total_tokens").
			Group("timestamp").
			Order("timestamp ASC")
	} else {
		query = LogDB.Table("logs").
			Select("timestamp_trunc_by_hour as timestamp, count(*) as request_count, sum(used_amount) as used_amount, sum(case when code != 200 then 1 else 0 end) as exception_count").
			Group("timestamp").
			Order("timestamp ASC")
	}

	if group == "" {
		query = query.Where("group_id = ''")
	} else if group != "*" {
		query = query.Where("group_id = ?", group)
	}

	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
	}
	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}
	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("timestamp_trunc_by_hour BETWEEN ? AND ?", start.Unix(), end.Unix())
	case !start.IsZero():
		query = query.Where("timestamp_trunc_by_hour >= ?", start.Unix())
	case !end.IsZero():
		query = query.Where("timestamp_trunc_by_hour <= ?", end.Unix())
	}

	if resultOnly {
		query = query.Where("downstream_result = true")
	}

	var chartData []*ChartData
	err := query.Scan(&chartData).Error
	if err != nil {
		return nil, err
	}

	// If timeSpan is day, aggregate hour data into day data
	if timeSpan == TimeSpanDay && len(chartData) > 0 {
		return aggregateHourDataToDay(chartData), nil
	}

	return chartData, nil
}

// aggregateHourDataToDay converts hourly chart data into daily aggregated data
func aggregateHourDataToDay(hourlyData []*ChartData) []*ChartData {
	dayData := make(map[int64]*ChartData)
	for _, data := range hourlyData {
		dayTimestamp := data.Timestamp - (data.Timestamp % int64(24*time.Hour.Seconds()))

		if _, exists := dayData[dayTimestamp]; !exists {
			dayData[dayTimestamp] = &ChartData{
				Timestamp: dayTimestamp,
			}
		}

		day := dayData[dayTimestamp]
		day.RequestCount += data.RequestCount
		day.UsedAmount = decimal.
			NewFromFloat(data.UsedAmount).
			Add(decimal.NewFromFloat(day.UsedAmount)).
			InexactFloat64()
		day.ExceptionCount += data.ExceptionCount
		day.InputTokens += data.InputTokens
		day.OutputTokens += data.OutputTokens
		day.CachedTokens += data.CachedTokens
		day.CacheCreationTokens += data.CacheCreationTokens
		day.TotalTokens += data.TotalTokens
	}

	result := make([]*ChartData, 0, len(dayData))
	for _, data := range dayData {
		result = append(result, data)
	}

	slices.SortFunc(result, func(a, b *ChartData) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	return result
}

func GetUsedChannelsFromLog(group string, start, end time.Time) ([]int, error) {
	return getLogGroupByValuesFromLog[int]("channel_id", group, start, end)
}

func GetUsedModelsFromLog(group string, start, end time.Time) ([]string, error) {
	return getLogGroupByValuesFromLog[string]("model", group, start, end)
}

func GetUsedTokenNamesFromLog(group string, start, end time.Time) ([]string, error) {
	return getLogGroupByValuesFromLog[string]("token_name", group, start, end)
}

func getLogGroupByValuesFromLog[T cmp.Ordered](field string, group string, start, end time.Time) ([]T, error) {
	var values []T
	query := LogDB.
		Model(&Log{})

	if group == "" {
		query = query.Where("group_id = ''")
	} else if group != "*" {
		query = query.Where("group_id = ?", group)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("request_at BETWEEN ? AND ?", start, end)
	case !start.IsZero():
		query = query.Where("request_at >= ?", start)
	case !end.IsZero():
		query = query.Where("request_at <= ?", end)
	}

	err := query.
		Select(field).
		Group(field).
		Pluck(field, &values).Error
	if err != nil {
		return nil, err
	}
	slices.Sort(values)
	return values, nil
}

func sumDashboardResponse(chartData []*ChartData) DashboardResponse {
	dashboardResponse := DashboardResponse{
		ChartData: chartData,
	}
	usedAmount := decimal.NewFromFloat(0)
	for _, data := range chartData {
		dashboardResponse.TotalCount += data.RequestCount
		dashboardResponse.ExceptionCount += data.ExceptionCount

		usedAmount = usedAmount.Add(decimal.NewFromFloat(data.UsedAmount))
		dashboardResponse.InputTokens += data.InputTokens
		dashboardResponse.OutputTokens += data.OutputTokens
		dashboardResponse.TotalTokens += data.TotalTokens
		dashboardResponse.CachedTokens += data.CachedTokens
		dashboardResponse.CacheCreationTokens += data.CacheCreationTokens
	}
	dashboardResponse.UsedAmount = usedAmount.InexactFloat64()
	return dashboardResponse
}

func getRPM(group string, end time.Time, tokenName, modelName string, channelID int, resultOnly bool) (int64, error) {
	query := LogDB.Model(&Log{})

	if group == "" {
		query = query.Where("group_id = ''")
	} else if group != "*" {
		query = query.Where("group_id = ?", group)
	}
	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
	}
	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}
	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}
	if resultOnly {
		query = query.Where("downstream_result = true")
	}

	var count int64
	err := query.
		Where("request_at BETWEEN ? AND ?", end.Add(-time.Minute), end).
		Count(&count).Error
	return count, err
}

func getTPM(group string, end time.Time, tokenName, modelName string, channelID int, resultOnly bool) (int64, error) {
	query := LogDB.Model(&Log{}).
		Select("COALESCE(SUM(total_tokens), 0)")

	if group == "" {
		query = query.Where("group_id = ''")
	} else if group != "*" {
		query = query.Where("group_id = ?", group)
	}
	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
	}
	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}
	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}
	if resultOnly {
		query = query.Where("downstream_result = true")
	}

	var tpm int64
	err := query.
		Where("request_at BETWEEN ? AND ?", end.Add(-time.Minute), end).
		Scan(&tpm).Error
	return tpm, err
}

func GetDashboardData(
	group string,
	start,
	end time.Time,
	modelName string,
	channelID int,
	timeSpan TimeSpanType,
	resultOnly bool,
	needRPM bool,
	tokenUsage bool,
	fromLog bool,
) (*DashboardResponse, error) {
	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	var (
		chartData []*ChartData
		rpm       int64
		tpm       int64
		channels  []int
	)

	g := new(errgroup.Group)

	if fromLog {
		g.Go(func() error {
			var err error
			chartData, err = getChartDataFromLog(group, start, end, "", modelName, channelID, timeSpan, resultOnly, tokenUsage)
			return err
		})
	} else {
		g.Go(func() error {
			var err error
			chartData, err = getChartData(group, start, end, "", modelName, channelID, timeSpan)
			return err
		})
	}

	if needRPM {
		g.Go(func() error {
			var err error
			rpm, err = getRPM(group, end, "", modelName, channelID, resultOnly)
			return err
		})
	}

	g.Go(func() error {
		var err error
		tpm, err = getTPM(group, end, "", modelName, channelID, resultOnly)
		return err
	})

	g.Go(func() error {
		var err error
		channels, err = GetUsedChannels(group, start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Channels = channels
	dashboardResponse.RPM = rpm
	dashboardResponse.TPM = tpm

	return &dashboardResponse, nil
}

func GetGroupDashboardData(
	group string,
	start, end time.Time,
	tokenName string,
	modelName string,
	timeSpan TimeSpanType,
	resultOnly bool,
	needRPM bool,
	tokenUsage bool,
	fromLog bool,
) (*GroupDashboardResponse, error) {
	if group == "" || group == "*" {
		return nil, errors.New("group is required")
	}

	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	var (
		chartData  []*ChartData
		tokenNames []string
		models     []string
		rpm        int64
		tpm        int64
	)

	g := new(errgroup.Group)

	if fromLog {
		g.Go(func() error {
			var err error
			chartData, err = getChartDataFromLog(group, start, end, tokenName, modelName, 0, timeSpan, resultOnly, tokenUsage)
			return err
		})
	} else {
		g.Go(func() error {
			var err error
			chartData, err = getChartData(group, start, end, tokenName, modelName, 0, timeSpan)
			return err
		})
	}

	g.Go(func() error {
		var err error
		tokenNames, err = GetUsedTokenNames(group, start, end)
		return err
	})

	g.Go(func() error {
		var err error
		models, err = GetUsedModels(group, start, end)
		return err
	})

	if needRPM {
		g.Go(func() error {
			var err error
			rpm, err = getRPM(group, end, tokenName, modelName, 0, resultOnly)
			return err
		})
	}

	g.Go(func() error {
		var err error
		tpm, err = getTPM(group, end, tokenName, modelName, 0, resultOnly)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.RPM = rpm
	dashboardResponse.TPM = tpm

	return &GroupDashboardResponse{
		DashboardResponse: dashboardResponse,
		Models:            models,
		TokenNames:        tokenNames,
	}, nil
}

func GetGroupModelTPM(group string, model string) (int64, error) {
	end := time.Now()
	start := end.Add(-time.Minute)
	var tpm int64
	err := LogDB.
		Model(&Log{}).
		Where("group_id = ? AND request_at >= ? AND request_at <= ? AND model = ?", group, start, end, model).
		Select("COALESCE(SUM(total_tokens), 0)").
		Scan(&tpm).Error
	return tpm, err
}

//nolint:revive
type ModelCostRank struct {
	Model               string  `json:"model"`
	UsedAmount          float64 `json:"used_amount"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CachedTokens        int64   `json:"cached_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	Total               int64   `json:"total"`
}

func GetModelCostRank(group string, channelID int, start, end time.Time, tokenUsage bool) ([]*ModelCostRank, error) {
	var ranks []*ModelCostRank

	var query *gorm.DB
	if tokenUsage {
		query = LogDB.Model(&Log{}).
			Select("model, SUM(used_amount) as used_amount, SUM(input_tokens) as input_tokens, SUM(output_tokens) as output_tokens, SUM(cached_tokens) as cached_tokens, SUM(cache_creation_tokens) as cache_creation_tokens, SUM(total_tokens) as total_tokens, COUNT(*) as total").
			Group("model").
			Order("used_amount DESC")
	} else {
		query = LogDB.Model(&Log{}).
			Select("model, SUM(used_amount) as used_amount, COUNT(*) as total").
			Group("model").
			Order("used_amount DESC")
	}

	if group == "" {
		query = query.Where("group_id = ''")
	} else if group != "*" {
		query = query.Where("group_id = ?", group)
	}

	if channelID != 0 {
		query = query.Where("channel_id = ?", channelID)
	}

	switch {
	case !start.IsZero() && !end.IsZero():
		query = query.Where("request_at BETWEEN ? AND ?", start, end)
	case !start.IsZero():
		query = query.Where("request_at >= ?", start)
	case !end.IsZero():
		query = query.Where("request_at <= ?", end)
	}

	err := query.Scan(&ranks).Error
	if err != nil {
		return nil, err
	}

	return ranks, nil
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
		db = db.Where("request_at BETWEEN ? AND ?", start, end)
	case !start.IsZero():
		db = db.Where("request_at >= ?", start)
	case !end.IsZero():
		db = db.Where("request_at <= ?", end)
	}
	db.Where("ip IS NOT NULL AND ip != '' AND group_id != ''")

	result := make(map[string][]string)
	rows, err := db.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ip string
		var groups string
		err = rows.Scan(&ip, &groups)
		if err != nil {
			return nil, err
		}
		result[ip] = strings.Split(groups, ",")
	}
	return result, nil
}
