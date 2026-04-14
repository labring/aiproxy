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

type logExportParams struct {
	timezone   string
	location   *time.Location
	maxEntries int
	includeCh  bool
	startTime  time.Time
	endTime    time.Time
	group      string
	tokenName  string
	modelName  string
	channelID  int
	tokenID    int
	order      string
	requestID  string
	upstreamID string
	codeType   string
	code       int
	withBody   bool
	ip         string
	user       string
}

func parseLogExportParams(c *gin.Context) (logExportParams, error) {
	params := parseCommonParams(c)

	startTime, endTime := utils.ParseTimeRange(c, -1)
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

	return logExportParams{
		timezone:   timezone,
		location:   location,
		maxEntries: model.NormalizeLogExportLimit(maxEntries),
		includeCh:  includeChannel,
		startTime:  startTime,
		endTime:    endTime,
		group:      params.group,
		tokenName:  params.tokenName,
		modelName:  params.modelName,
		channelID:  params.channelID,
		tokenID:    params.tokenID,
		order:      params.order,
		requestID:  params.requestID,
		upstreamID: params.upstreamID,
		codeType:   params.codeType,
		code:       params.code,
		withBody:   params.withBody,
		ip:         params.ip,
		user:       params.user,
	}, nil
}

// ExportLogs godoc
//
//	@Summary		Export global logs
//	@Description	Exports filtered global logs as a CSV table file
//	@Tags			logs
//	@Produce		text/csv
//	@Security		ApiKeyAuth
//	@Param			start_timestamp	query		int		false	"Start timestamp"
//	@Param			end_timestamp	query		int		false	"End timestamp"
//	@Param			model_name		query		string	false	"Model name"
//	@Param			channel			query		int		false	"Channel ID"
//	@Param			order			query		string	false	"Order"
//	@Param			request_id		query		string	false	"Request ID"
//	@Param			upstream_id		query		string	false	"Upstream ID"
//	@Param			code_type		query		string	false	"Status code type"
//	@Param			code			query		int		false	"Status code"
//	@Param			with_body		query		bool	false	"With request and response body"
//	@Param			ip				query		string	false	"IP"
//	@Param			user			query		string	false	"User"
//	@Param			timezone		query		string	false	"Timezone, default is Local"
//	@Param			max_entries		query		int		false	"Maximum exported rows, default 1000, max 10000"
//	@Router			/api/logs/export [get]
func ExportLogs(c *gin.Context) {
	params, err := parseLogExportParams(c)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	logs, err := model.ExportLogs(
		params.startTime,
		params.endTime,
		params.modelName,
		params.requestID,
		params.upstreamID,
		params.channelID,
		params.order,
		model.CodeType(params.codeType),
		params.code,
		params.withBody,
		params.ip,
		params.user,
		params.maxEntries,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	filename := buildLogExportFilename(
		"global",
		"",
		params.location,
	)
	downloadCSV(c, filename, logs, params.location, params.timezone, true)
}

// ExportGroupLogs godoc
//
//	@Summary		Export group logs
//	@Description	Exports filtered group logs as a CSV table file
//	@Tags			log
//	@Produce		text/csv
//	@Security		ApiKeyAuth
//	@Param			group			path		string	true	"Group name"
//	@Param			start_timestamp	query		int		false	"Start timestamp"
//	@Param			end_timestamp	query		int		false	"End timestamp"
//	@Param			model_name		query		string	false	"Model name"
//	@Param			token_id		query		int		false	"Token ID"
//	@Param			token_name		query		string	false	"Token name"
//	@Param			order			query		string	false	"Order"
//	@Param			request_id		query		string	false	"Request ID"
//	@Param			upstream_id		query		string	false	"Upstream ID"
//	@Param			code_type		query		string	false	"Status code type"
//	@Param			code			query		int		false	"Status code"
//	@Param			with_body		query		bool	false	"With request and response body"
//	@Param			ip				query		string	false	"IP"
//	@Param			user			query		string	false	"User"
//	@Param			timezone		query		string	false	"Timezone, default is Local"
//	@Param			max_entries		query		int		false	"Maximum exported rows, default 1000, max 10000"
//	@Param			include_channel	query		bool	false	"Include channel column, default false"
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

	logs, err := model.ExportGroupLogs(
		group,
		params.startTime,
		params.endTime,
		params.modelName,
		params.requestID,
		params.upstreamID,
		params.tokenID,
		params.tokenName,
		params.order,
		model.CodeType(params.codeType),
		params.code,
		params.withBody,
		params.ip,
		params.user,
		params.maxEntries,
	)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	filename := buildLogExportFilename(
		"group_"+group,
		group,
		params.location,
	)
	downloadCSV(c, filename, logs, params.location, params.timezone, params.includeCh)
}

func downloadCSV(
	c *gin.Context,
	filename string,
	logs []*model.Log,
	location *time.Location,
	timezone string,
	includeChannel bool,
) {
	content, err := buildLogExportCSV(logs, location, timezone, includeChannel)
	if err != nil {
		middleware.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": filename,
	})

	c.Header("Content-Disposition", disposition)
	c.Header("X-Export-Count", strconv.Itoa(len(logs)))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", content)
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

	// BOM improves CSV compatibility with spreadsheet tools.
	buffer.WriteString("\xEF\xBB\xBF")

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

	header = append(header,
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

	writer := csv.NewWriter(&buffer)
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, logItem := range logs {
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

		row = append(row,
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

		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
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
