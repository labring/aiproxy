package model

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/shopspring/decimal"
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

func (d *RequestDetail) BeforeSave(_ *gorm.DB) (err error) {
	if reqMax := config.GetLogDetailRequestBodyMaxSize(); reqMax > 0 &&
		int64(len(d.RequestBody)) > reqMax {
		d.RequestBody = common.TruncateByRune(d.RequestBody, int(reqMax)) + "..."
		d.RequestBodyTruncated = true
	}
	if respMax := config.GetLogDetailResponseBodyMaxSize(); respMax > 0 &&
		int64(len(d.ResponseBody)) > respMax {
		d.ResponseBody = common.TruncateByRune(d.ResponseBody, int(respMax)) + "..."
		d.ResponseBodyTruncated = true
	}
	return
}

type Price struct {
	PerRequestPrice ZeroNullFloat64 `json:"per_request_price,omitempty"`

	InputPrice     ZeroNullFloat64 `json:"input_price,omitempty"`
	InputPriceUnit ZeroNullInt64   `json:"input_price_unit,omitempty"`

	ImageInputPrice     ZeroNullFloat64 `json:"image_input_price,omitempty"`
	ImageInputPriceUnit ZeroNullInt64   `json:"image_input_price_unit,omitempty"`

	OutputPrice     ZeroNullFloat64 `json:"output_price,omitempty"`
	OutputPriceUnit ZeroNullInt64   `json:"output_price_unit,omitempty"`

	// when ThinkingModeOutputPrice and ReasoningTokens are not 0, OutputPrice and OutputPriceUnit
	// will be overwritten
	ThinkingModeOutputPrice     ZeroNullFloat64 `json:"thinking_mode_output_price,omitempty"`
	ThinkingModeOutputPriceUnit ZeroNullInt64   `json:"thinking_mode_output_price_unit,omitempty"`

	CachedPrice     ZeroNullFloat64 `json:"cached_price,omitempty"`
	CachedPriceUnit ZeroNullInt64   `json:"cached_price_unit,omitempty"`

	CacheCreationPrice     ZeroNullFloat64 `json:"cache_creation_price,omitempty"`
	CacheCreationPriceUnit ZeroNullInt64   `json:"cache_creation_price_unit,omitempty"`

	WebSearchPrice     ZeroNullFloat64 `json:"web_search_price,omitempty"`
	WebSearchPriceUnit ZeroNullInt64   `json:"web_search_price_unit,omitempty"`
}

func (p *Price) GetInputPriceUnit() int64 {
	if p.InputPriceUnit > 0 {
		return int64(p.InputPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetImageInputPriceUnit() int64 {
	if p.ImageInputPriceUnit > 0 {
		return int64(p.ImageInputPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetOutputPriceUnit() int64 {
	if p.OutputPriceUnit > 0 {
		return int64(p.OutputPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetCachedPriceUnit() int64 {
	if p.CachedPriceUnit > 0 {
		return int64(p.CachedPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetCacheCreationPriceUnit() int64 {
	if p.CacheCreationPriceUnit > 0 {
		return int64(p.CacheCreationPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetWebSearchPriceUnit() int64 {
	if p.WebSearchPriceUnit > 0 {
		return int64(p.WebSearchPriceUnit)
	}
	return PriceUnit
}

type Usage struct {
	InputTokens         ZeroNullInt64 `json:"input_tokens,omitempty"`
	ImageInputTokens    ZeroNullInt64 `json:"image_input_tokens,omitempty"`
	OutputTokens        ZeroNullInt64 `json:"output_tokens,omitempty"`
	CachedTokens        ZeroNullInt64 `json:"cached_tokens,omitempty"`
	CacheCreationTokens ZeroNullInt64 `json:"cache_creation_tokens,omitempty"`
	ReasoningTokens     ZeroNullInt64 `json:"reasoning_tokens,omitempty"`
	TotalTokens         ZeroNullInt64 `json:"total_tokens,omitempty"`
	WebSearchCount      ZeroNullInt64 `json:"web_search_count,omitempty"`
}

func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.ImageInputTokens += other.ImageInputTokens
	u.OutputTokens += other.OutputTokens
	u.CachedTokens += other.CachedTokens
	u.CacheCreationTokens += other.CacheCreationTokens
	u.TotalTokens += other.TotalTokens
	u.WebSearchCount += other.WebSearchCount
}

type Log struct {
	RequestDetail    *RequestDetail  `gorm:"foreignKey:LogID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"request_detail,omitempty"`
	RequestAt        time.Time       `                                                                      json:"request_at"`
	RetryAt          time.Time       `                                                                      json:"retry_at,omitempty"`
	TTFBMilliseconds ZeroNullInt64   `                                                                      json:"ttfb_milliseconds,omitempty"`
	CreatedAt        time.Time       `gorm:"autoCreateTime;index"                                           json:"created_at"`
	TokenName        string          `                                                                      json:"token_name,omitempty"`
	Endpoint         EmptyNullString `                                                                      json:"endpoint,omitempty"`
	Content          EmptyNullString `gorm:"type:text"                                                      json:"content,omitempty"`
	GroupID          string          `                                                                      json:"group,omitempty"`
	Model            string          `                                                                      json:"model"`
	RequestID        EmptyNullString `gorm:"index:,where:request_id is not null"                            json:"request_id"`
	ID               int             `gorm:"primaryKey"                                                     json:"id"`
	TokenID          int             `gorm:"index"                                                          json:"token_id,omitempty"`
	ChannelID        int             `                                                                      json:"channel,omitempty"`
	Code             int             `gorm:"index"                                                          json:"code,omitempty"`
	Mode             int             `                                                                      json:"mode,omitempty"`
	IP               EmptyNullString `gorm:"index:,where:ip is not null"                                    json:"ip,omitempty"`
	RetryTimes       ZeroNullInt64   `                                                                      json:"retry_times,omitempty"`
	Price            Price           `gorm:"embedded"                                                       json:"price,omitempty"`
	Usage            Usage           `gorm:"embedded"                                                       json:"usage,omitempty"`
	UsedAmount       float64         `                                                                      json:"used_amount,omitempty"`
	// https://platform.openai.com/docs/guides/safety-best-practices#end-user-ids
	User     EmptyNullString   `                                                                      json:"user,omitempty"`
	Metadata map[string]string `gorm:"serializer:fastjson;type:text"                                  json:"metadata,omitempty"`
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

func CleanLog(batchSize int, optimize bool) (err error) {
	err = cleanLog(batchSize)
	if err != nil {
		return err
	}
	err = cleanLogDetail(batchSize)
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

	retryLogStorageHours := config.GetRetryLogStorageHours()
	if retryLogStorageHours != 0 {
		subQuery := LogDB.
			Model(&RetryLog{}).
			Where(
				"created_at < ?",
				time.Now().Add(-time.Duration(retryLogStorageHours)*time.Hour),
			).
			Limit(batchSize).
			Select("id")

		err := LogDB.
			Session(&gorm.Session{SkipDefaultTransaction: true}).
			Where("id IN (?)", subQuery).
			Delete(&RetryLog{}).Error
		if err != nil {
			return err
		}
	}

	return LogDB.
		Model(&Store{}).
		Where("expires_at < ?", time.Now()).
		Delete(&Store{}).
		Error
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

func cleanLogDetail(batchSize int) error {
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

	return nil
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
	modelPrice Price,
	amount float64,
	user string,
	metadata map[string]string,
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
		UsedAmount:       amount,
		User:             EmptyNullString(user),
		Metadata:         metadata,
	}
	return LogDB.Create(log).Error
}

func getLogOrder(order string) string {
	prefix, suffix, _ := strings.Cut(order, "-")
	switch prefix {
	case "request_at", "id":
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
	code int,
	ip string,
	user string,
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
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
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
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
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
		total, logs, err = getLogs(
			group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
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
		total, logs, err = getLogs(group,
			startTimestamp,
			endTimestamp,
			modelName,
			requestID,
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
		tokenNames, err = GetUsedTokenNames(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error
		models, err = GetUsedModels(group, tokenName, startTimestamp, endTimestamp)
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
	code int,
	ip string,
	user string,
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
		var conditions []string
		var values []any

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
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
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
	code int,
	withBody bool,
	ip string,
	user string,
	page int,
	perPage int,
) (*GetLogsResult, error) {
	var total int64
	var logs []*Log
	var channels []int

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error
		total, logs, err = searchLogs(
			group,
			keyword,
			requestID,
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
		total, logs, err = searchLogs(group,
			keyword,
			requestID,
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
		tokenNames, err = GetUsedTokenNames(group, startTimestamp, endTimestamp)
		return err
	})

	g.Go(func() error {
		var err error
		models, err = GetUsedModels(group, tokenName, startTimestamp, endTimestamp)
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
	result := LogDB.Where("created_at < ?", timestamp).Delete(&Log{})
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
	WebSearchCount      int64   `json:"web_search_count,omitempty"`

	MaxRPM int64 `json:"max_rpm,omitempty"`
	MaxTPM int64 `json:"max_tpm,omitempty"`
	MaxRPS int64 `json:"max_rps,omitempty"`
	MaxTPS int64 `json:"max_tps,omitempty"`
}

type DashboardResponse struct {
	ChartData      []*ChartData `json:"chart_data"`
	TotalCount     int64        `json:"total_count"`
	ExceptionCount int64        `json:"exception_count"`

	RPM int64 `json:"rpm"`
	TPM int64 `json:"tpm"`

	MaxRPM int64 `json:"max_rpm,omitempty"`
	MaxTPM int64 `json:"max_tpm,omitempty"`
	MaxRPS int64 `json:"max_rps,omitempty"`
	MaxTPS int64 `json:"max_tps,omitempty"`

	UsedAmount          float64 `json:"used_amount"`
	InputTokens         int64   `json:"input_tokens,omitempty"`
	OutputTokens        int64   `json:"output_tokens,omitempty"`
	TotalTokens         int64   `json:"total_tokens,omitempty"`
	CachedTokens        int64   `json:"cached_tokens,omitempty"`
	CacheCreationTokens int64   `json:"cache_creation_tokens,omitempty"`
	WebSearchCount      int64   `json:"web_search_count,omitempty"`

	Channels []int    `json:"channels,omitempty"`
	Models   []string `json:"models,omitempty"`
}

type GroupDashboardResponse struct {
	DashboardResponse
	TokenNames []string `json:"token_names"`
}

type TimeSpanType string

const (
	TimeSpanDay  TimeSpanType = "day"
	TimeSpanHour TimeSpanType = "hour"
)

// aggregateHourDataToDay converts hourly chart data into daily aggregated data
func aggregateHourDataToDay(hourlyData []*ChartData, timezone *time.Location) []*ChartData {
	dayData := make(map[int64]*ChartData)
	if timezone == nil {
		timezone = time.Local
	}

	for _, data := range hourlyData {
		// Convert timestamp to time in the specified timezone
		t := time.Unix(data.Timestamp, 0).In(timezone)
		// Get the start of the day in the specified timezone
		startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timezone)
		dayTimestamp := startOfDay.Unix()

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
		day.WebSearchCount += data.WebSearchCount

		if data.MaxRPM > day.MaxRPM {
			day.MaxRPM = data.MaxRPM
		}
		if data.MaxTPM > day.MaxTPM {
			day.MaxTPM = data.MaxTPM
		}
		if data.MaxRPS > day.MaxRPS {
			day.MaxRPS = data.MaxRPS
		}
		if data.MaxTPS > day.MaxTPS {
			day.MaxTPS = data.MaxTPS
		}
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

func getLogGroupByValuesFromLog[T cmp.Ordered](
	field, group string,
	start, end time.Time,
) ([]T, error) {
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
		query = query.Where("created_at BETWEEN ? AND ?", start, end)
	case !start.IsZero():
		query = query.Where("created_at >= ?", start)
	case !end.IsZero():
		query = query.Where("created_at <= ?", end)
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
		dashboardResponse.WebSearchCount += data.WebSearchCount

		if data.MaxRPM > dashboardResponse.MaxRPM {
			dashboardResponse.MaxRPM = data.MaxRPM
		}
		if data.MaxTPM > dashboardResponse.MaxTPM {
			dashboardResponse.MaxTPM = data.MaxTPM
		}
		if data.MaxRPS > dashboardResponse.MaxRPS {
			dashboardResponse.MaxRPS = data.MaxRPS
		}
		if data.MaxTPS > dashboardResponse.MaxTPS {
			dashboardResponse.MaxTPS = data.MaxTPS
		}
	}
	dashboardResponse.UsedAmount = usedAmount.InexactFloat64()
	return dashboardResponse
}

func GetDashboardData(
	start,
	end time.Time,
	modelName string,
	channelID int,
	timeSpan TimeSpanType,
	timezone *time.Location,
) (*DashboardResponse, error) {
	if end.IsZero() {
		end = time.Now()
	} else if end.Before(start) {
		return nil, errors.New("end time is before start time")
	}

	var (
		chartData []*ChartData
		channels  []int
		models    []string
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error
		chartData, err = getChartData("*", start, end, "", modelName, channelID, timeSpan, timezone)
		return err
	})

	g.Go(func() error {
		var err error
		channels, err = GetUsedChannels("*", start, end)
		return err
	})

	g.Go(func() error {
		var err error
		models, err = GetUsedModels("*", "", start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Channels = channels
	dashboardResponse.Models = models

	return &dashboardResponse, nil
}

func GetGroupDashboardData(
	group string,
	start, end time.Time,
	tokenName string,
	modelName string,
	timeSpan TimeSpanType,
	timezone *time.Location,
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
	)

	g := new(errgroup.Group)

	g.Go(func() error {
		var err error
		chartData, err = getChartData(
			group,
			start,
			end,
			tokenName,
			modelName,
			0,
			timeSpan,
			timezone,
		)
		return err
	})

	g.Go(func() error {
		var err error
		tokenNames, err = GetUsedTokenNames(group, start, end)
		return err
	})

	g.Go(func() error {
		var err error
		models, err = GetUsedModels(group, tokenName, start, end)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dashboardResponse := sumDashboardResponse(chartData)
	dashboardResponse.Models = models

	return &GroupDashboardResponse{
		DashboardResponse: dashboardResponse,
		TokenNames:        tokenNames,
	}, nil
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
