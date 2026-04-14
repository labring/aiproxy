package controller

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller/utils"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
)

const (
	logExportMaxSpan              = 30 * 24 * time.Hour
	defaultLogExportChunkInterval = 30 * time.Minute
	minLogExportChunkInterval     = 10 * time.Minute
	maxLogExportChunkInterval     = 4 * time.Hour
)

type logExportParams struct {
	timezone      string
	location      *time.Location
	maxEntries    int
	includeCh     bool
	chunkInterval time.Duration
	startTime     time.Time
	endTime       time.Time
	group         string
	tokenName     string
	modelName     string
	channelID     int
	tokenID       int
	order         string
	requestID     string
	upstreamID    string
	codeType      string
	code          int
	includeDetail bool
	ip            string
	user          string
}

func parseLogExportParams(c *gin.Context) (logExportParams, error) {
	params := parseCommonParams(c)

	startTime, endTime := utils.ParseTimeRange(c, logExportMaxSpan)
	if !startTime.IsZero() && !endTime.IsZero() && startTime.After(endTime) {
		return logExportParams{}, errors.New("start_timestamp cannot be greater than end_timestamp")
	}

	timezone := c.DefaultQuery("timezone", "Local")

	location, err := time.LoadLocation(timezone)
	if err != nil {
		timezone = "Local"
		location = time.Local
	}

	maxEntries, _ := strconv.Atoi(c.Query("max_entries"))
	includeChannel, _ := strconv.ParseBool(c.Query("include_channel"))
	includeDetail, _ := strconv.ParseBool(c.Query("include_detail"))

	chunkInterval := defaultLogExportChunkInterval
	if raw := c.Query("chunk_interval"); raw != "" {
		chunkInterval, err = time.ParseDuration(raw)
		if err != nil {
			return logExportParams{}, errors.New(
				"chunk_interval must be a valid duration such as 30m or 1h",
			)
		}
	}

	if chunkInterval < minLogExportChunkInterval || chunkInterval > maxLogExportChunkInterval {
		return logExportParams{}, errors.New("chunk_interval must be between 10m and 4h")
	}

	order := c.Query("order")
	if order != "" && order != "desc" && order != "asc" {
		return logExportParams{}, errors.New("order must be asc or desc")
	}

	return logExportParams{
		timezone:      timezone,
		location:      location,
		maxEntries:    maxEntries,
		includeCh:     includeChannel,
		chunkInterval: chunkInterval,
		startTime:     startTime,
		endTime:       endTime,
		group:         params.group,
		tokenName:     params.tokenName,
		modelName:     params.modelName,
		channelID:     params.channelID,
		tokenID:       params.tokenID,
		order:         order,
		requestID:     params.requestID,
		upstreamID:    params.upstreamID,
		codeType:      params.codeType,
		code:          params.code,
		includeDetail: includeDetail,
		ip:            params.ip,
		user:          params.user,
	}, nil
}

// ExportLogs godoc
//
//	@Summary		Export global logs
//	@Description	Streams filtered global logs as a CSV table file
//	@Tags			logs
//	@Produce		text/csv
//	@Security		ApiKeyAuth
//	@Param			start_timestamp	query	int		false	"Start timestamp, max span 30 days"
//	@Param			end_timestamp	query	int		false	"End timestamp, max span 30 days"
//	@Param			model_name		query	string	false	"Model name"
//	@Param			channel			query	int		false	"Channel ID"
//	@Param			order			query	string	false	"Sort order for created_at, supports desc or asc"
//	@Param			request_id		query	string	false	"Request ID"
//	@Param			upstream_id		query	string	false	"Upstream ID"
//	@Param			code_type		query	string	false	"Status code type"
//	@Param			code			query	int		false	"Status code"
//	@Param			include_detail	query	bool	false	"Include request and response detail, default false"
//	@Param			ip				query	string	false	"IP"
//	@Param			user			query	string	false	"User"
//	@Param			timezone		query	string	false	"Timezone, default is Local"
//	@Param			max_entries		query	int		false	"Maximum exported rows; zero or negative means unlimited"
//	@Param			chunk_interval	query	string	false	"Chunk interval, default 30m, min 10m, max 4h, e.g. 10m, 30m, 1h"
//	@Router			/api/logs/export [get]
func ExportLogs(c *gin.Context) {
	params, err := parseLogExportParams(c)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	filename := buildLogExportFilename("global", "", params.location)
	streamCSV(
		c,
		filename,
		params,
		true,
		func(start, endExclusive time.Time, limit int) ([]*model.Log, error) {
			return model.ExportLogsRange(
				start,
				endExclusive,
				params.modelName,
				params.requestID,
				params.upstreamID,
				params.channelID,
				normalizeLogExportModelOrder(params.order),
				model.CodeType(params.codeType),
				params.code,
				params.includeDetail,
				params.ip,
				params.user,
				limit,
			)
		},
	)
}

// ExportGroupLogs godoc
//
//	@Summary		Export group logs
//	@Description	Streams filtered group logs as a CSV table file
//	@Tags			log
//	@Produce		text/csv
//	@Security		ApiKeyAuth
//	@Param			group			path	string	true	"Group name"
//	@Param			start_timestamp	query	int		false	"Start timestamp, max span 30 days"
//	@Param			end_timestamp	query	int		false	"End timestamp, max span 30 days"
//	@Param			model_name		query	string	false	"Model name"
//	@Param			token_id		query	int		false	"Token ID"
//	@Param			token_name		query	string	false	"Token name"
//	@Param			order			query	string	false	"Sort order for created_at, supports desc or asc"
//	@Param			request_id		query	string	false	"Request ID"
//	@Param			upstream_id		query	string	false	"Upstream ID"
//	@Param			code_type		query	string	false	"Status code type"
//	@Param			code			query	int		false	"Status code"
//	@Param			include_detail	query	bool	false	"Include request and response detail, default false"
//	@Param			ip				query	string	false	"IP"
//	@Param			user			query	string	false	"User"
//	@Param			timezone		query	string	false	"Timezone, default is Local"
//	@Param			max_entries		query	int		false	"Maximum exported rows; zero or negative means unlimited"
//	@Param			include_channel	query	bool	false	"Include channel column, default false"
//	@Param			chunk_interval	query	string	false	"Chunk interval, default 30m, min 10m, max 4h, e.g. 10m, 30m, 1h"
//	@Router			/api/log/{group}/export [get]
func ExportGroupLogs(c *gin.Context) {
	group := c.Param("group")
	if group == "" {
		middleware.ErrorResponse(c, http.StatusBadRequest, "invalid group parameter")
		return
	}

	params, err := parseLogExportParams(c)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	filename := buildLogExportFilename("group_"+group, group, params.location)
	streamCSV(
		c,
		filename,
		params,
		params.includeCh,
		func(start, endExclusive time.Time, limit int) ([]*model.Log, error) {
			return model.ExportGroupLogsRange(
				group,
				start,
				endExclusive,
				params.modelName,
				params.requestID,
				params.upstreamID,
				params.tokenID,
				params.tokenName,
				normalizeLogExportModelOrder(params.order),
				model.CodeType(params.codeType),
				params.code,
				params.includeDetail,
				params.ip,
				params.user,
				limit,
			)
		},
	)
}

func streamCSV(
	c *gin.Context,
	filename string,
	params logExportParams,
	includeChannel bool,
	fetch func(start, endExclusive time.Time, limit int) ([]*model.Log, error),
) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		middleware.ErrorResponse(c, http.StatusInternalServerError, "streaming not supported")
		return
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": filename,
	})

	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Trailer", "X-Export-Count")
	c.Status(http.StatusOK)

	if _, err := c.Writer.Write([]byte("\xEF\xBB\xBF")); err != nil {
		return
	}

	writer := csv.NewWriter(c.Writer)
	if err := writer.Write(buildLogExportHeader(includeChannel)); err != nil {
		return
	}

	writer.Flush()

	if writer.Error() != nil {
		return
	}

	flusher.Flush()

	totalWritten := 0
	unlimitedEntries := params.maxEntries <= 0
	chunkDuration := params.chunkInterval
	descending := !isAscendingLogExportOrder(params.order)
	endExclusive := params.endTime.Add(time.Nanosecond)

	for unlimitedEntries || totalWritten < params.maxEntries {
		chunkStart, chunkEndExclusive, done := nextLogExportChunk(
			params.startTime,
			endExclusive,
			chunkDuration,
			descending,
		)
		if done {
			break
		}

		remaining := -1
		if !unlimitedEntries {
			remaining = params.maxEntries - totalWritten
		}

		logs, err := fetch(chunkStart, chunkEndExclusive, remaining)
		if err != nil {
			_ = c.Error(err)
			break
		}

		var writeErr error
		for _, logItem := range logs {
			if err := writer.Write(
				buildLogExportRow(logItem, params.location, params.timezone, includeChannel),
			); err != nil {
				writeErr = err
				break
			}

			totalWritten++
			if !unlimitedEntries && totalWritten >= params.maxEntries {
				break
			}
		}

		if writeErr != nil {
			_ = c.Error(writeErr)
			break
		}

		writer.Flush()

		if err := writer.Error(); err != nil {
			_ = c.Error(err)
			break
		}

		flusher.Flush()

		if !unlimitedEntries && totalWritten >= params.maxEntries {
			break
		}

		if descending {
			endExclusive = chunkStart
		} else {
			params.startTime = chunkEndExclusive
		}
	}

	c.Writer.Header().Set("X-Export-Count", strconv.Itoa(totalWritten))
}

func nextLogExportChunk(
	start time.Time,
	endExclusive time.Time,
	chunkDuration time.Duration,
	descending bool,
) (time.Time, time.Time, bool) {
	if start.IsZero() || endExclusive.IsZero() || !start.Before(endExclusive) {
		return time.Time{}, time.Time{}, true
	}

	if descending {
		chunkStart := endExclusive.Add(-chunkDuration)
		if chunkStart.Before(start) {
			chunkStart = start
		}

		return chunkStart, endExclusive, false
	}

	chunkEndExclusive := start.Add(chunkDuration)
	if chunkEndExclusive.After(endExclusive) {
		chunkEndExclusive = endExclusive
	}

	return start, chunkEndExclusive, false
}

func isAscendingLogExportOrder(order string) bool {
	return order == "asc"
}

func normalizeLogExportModelOrder(order string) string {
	if order == "asc" {
		return "created_at-asc"
	}

	return "created_at-desc"
}

func buildLogExportCSV(
	logs []*model.Log,
	location *time.Location,
	timezone string,
	includeChannel bool,
) ([]byte, error) {
	if location == nil {
		location = time.Local
	}

	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(&buffer)
	if err := writer.Write(buildLogExportHeader(includeChannel)); err != nil {
		return nil, err
	}

	for _, logItem := range logs {
		if err := writer.Write(
			buildLogExportRow(logItem, location, timezone, includeChannel),
		); err != nil {
			return nil, err
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func buildLogExportHeader(includeChannel bool) []string {
	header := []string{
		"id",
		"created_at",
		"request_at",
		"retry_at",
		"timezone",
		"group",
		"token_id",
		"token_name",
	}
	if includeChannel {
		header = append(header, "channel")
	}

	return append(header,
		"model",
		"endpoint",
		"code",
		"mode",
		"request_id",
		"upstream_id",
		"ip",
		"user",
		"service_tier",
		"ttfb_milliseconds",
		"retry_times",
		"input_tokens",
		"image_input_tokens",
		"audio_input_tokens",
		"output_tokens",
		"image_output_tokens",
		"cached_tokens",
		"cache_creation_tokens",
		"reasoning_tokens",
		"total_tokens",
		"web_search_count",
		"input_amount",
		"image_input_amount",
		"audio_input_amount",
		"output_amount",
		"image_output_amount",
		"cached_amount",
		"cache_creation_amount",
		"web_search_amount",
		"used_amount",
		"content",
		"prompt_cache_key",
		"metadata",
		"request_body",
		"response_body",
	)
}

func buildLogExportRow(
	logItem *model.Log,
	location *time.Location,
	timezone string,
	includeChannel bool,
) []string {
	requestBody := ""

	responseBody := ""
	if logItem.RequestDetail != nil {
		requestBody = logItem.RequestDetail.RequestBody
		responseBody = logItem.RequestDetail.ResponseBody
	}

	metadata := ""
	if len(logItem.Metadata) > 0 {
		metadata, _ = sonic.MarshalString(logItem.Metadata)
	}

	row := []string{
		strconv.Itoa(logItem.ID),
		formatTimeForExport(logItem.CreatedAt, location),
		formatTimeForExport(logItem.RequestAt, location),
		formatTimeForExport(logItem.RetryAt, location),
		timezone,
		sanitizeCSVCell(logItem.GroupID),
		strconv.Itoa(logItem.TokenID),
		sanitizeCSVCell(logItem.TokenName),
	}
	if includeChannel {
		row = append(row, strconv.Itoa(logItem.ChannelID))
	}

	return append(row,
		sanitizeCSVCell(logItem.Model),
		sanitizeCSVCell(logItem.Endpoint.String()),
		strconv.Itoa(logItem.Code),
		strconv.Itoa(logItem.Mode),
		sanitizeCSVCell(logItem.RequestID.String()),
		sanitizeCSVCell(logItem.UpstreamID.String()),
		sanitizeCSVCell(logItem.IP.String()),
		sanitizeCSVCell(logItem.User.String()),
		sanitizeCSVCell(logItem.ServiceTier),
		strconv.FormatInt(int64(logItem.TTFBMilliseconds), 10),
		strconv.FormatInt(int64(logItem.RetryTimes), 10),
		strconv.FormatInt(int64(logItem.Usage.InputTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.ImageInputTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.AudioInputTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.OutputTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.ImageOutputTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.CachedTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.CacheCreationTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.ReasoningTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.TotalTokens), 10),
		strconv.FormatInt(int64(logItem.Usage.WebSearchCount), 10),
		formatFloatForExport(logItem.Amount.InputAmount),
		formatFloatForExport(logItem.Amount.ImageInputAmount),
		formatFloatForExport(logItem.Amount.AudioInputAmount),
		formatFloatForExport(logItem.Amount.OutputAmount),
		formatFloatForExport(logItem.Amount.ImageOutputAmount),
		formatFloatForExport(logItem.Amount.CachedAmount),
		formatFloatForExport(logItem.Amount.CacheCreationAmount),
		formatFloatForExport(logItem.Amount.WebSearchAmount),
		formatFloatForExport(logItem.Amount.UsedAmount),
		sanitizeCSVCell(logItem.Content.String()),
		sanitizeCSVCell(logItem.PromptCacheKey.String()),
		sanitizeCSVCell(metadata),
		sanitizeCSVCell(requestBody),
		sanitizeCSVCell(responseBody),
	)
}

func formatTimeForExport(t time.Time, location *time.Location) string {
	if t.IsZero() {
		return ""
	}

	return t.In(location).Format("2006-01-02 15:04:05.000 MST")
}

func formatFloatForExport(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func sanitizeCSVCell(value string) string {
	if value == "" {
		return ""
	}

	switch value[0] {
	case '=', '+', '-', '@', '\t':
		return "'" + value
	default:
		return value
	}
}

func buildLogExportFilename(prefix, group string, location *time.Location) string {
	now := time.Now()
	if location != nil {
		now = now.In(location)
	}

	filename := fmt.Sprintf("%s_logs_%s.csv", prefix, now.Format("20060102_150405"))
	if group != "" {
		filename = fmt.Sprintf(
			"%s_logs_%s_%s.csv",
			sanitizeFilename(group),
			now.Format("20060102_150405"),
			now.Format("MST"),
		)
	}

	return sanitizeFilename(filename)
}

func sanitizeFilename(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))

	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}

	result := strings.Trim(builder.String(), "._")
	if result == "" {
		return "logs.csv"
	}

	return result
}
