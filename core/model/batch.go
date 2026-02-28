package model

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/oncall"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
)

type batchUpdateData struct {
	Groups               map[string]*GroupUpdate
	Tokens               map[int]*TokenUpdate
	Channels             map[int]*ChannelUpdate
	Summaries            map[SummaryUnique]*SummaryUpdate
	GroupSummaries       map[GroupSummaryUnique]*GroupSummaryUpdate
	SummariesMinute      map[SummaryMinuteUnique]*SummaryMinuteUpdate
	GroupSummariesMinute map[GroupSummaryMinuteUnique]*GroupSummaryMinuteUpdate
	sync.Mutex
}

func (b *batchUpdateData) IsClean() bool {
	b.Lock()
	defer b.Unlock()

	return b.isCleanLocked()
}

func (b *batchUpdateData) isCleanLocked() bool {
	return len(b.Groups) == 0 &&
		len(b.Tokens) == 0 &&
		len(b.Channels) == 0 &&
		len(b.Summaries) == 0 &&
		len(b.GroupSummaries) == 0 &&
		len(b.SummariesMinute) == 0 &&
		len(b.GroupSummariesMinute) == 0
}

type GroupUpdate struct {
	Amount decimal.Decimal
	Count  int
}

type TokenUpdate struct {
	Amount decimal.Decimal
	Count  int
}

type ChannelUpdate struct {
	Amount     decimal.Decimal
	Count      int
	RetryCount int
}

type SummaryUpdate struct {
	SummaryUnique
	SummaryData
}

type SummaryMinuteUpdate struct {
	SummaryMinuteUnique
	SummaryData
}

type GroupSummaryUpdate struct {
	GroupSummaryUnique
	SummaryData
}

type GroupSummaryMinuteUpdate struct {
	GroupSummaryMinuteUnique
	SummaryData
}

var batchData batchUpdateData

func init() {
	batchData = batchUpdateData{
		Groups:               make(map[string]*GroupUpdate),
		Tokens:               make(map[int]*TokenUpdate),
		Channels:             make(map[int]*ChannelUpdate),
		Summaries:            make(map[SummaryUnique]*SummaryUpdate),
		GroupSummaries:       make(map[GroupSummaryUnique]*GroupSummaryUpdate),
		SummariesMinute:      make(map[SummaryMinuteUnique]*SummaryMinuteUpdate),
		GroupSummariesMinute: make(map[GroupSummaryMinuteUnique]*GroupSummaryMinuteUpdate),
	}
}

func StartBatchProcessorSummary(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ProcessBatchUpdatesSummary()
			return
		case <-ticker.C:
			ProcessBatchUpdatesSummary()
		}
	}
}

func CleanBatchUpdatesSummary(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			ProcessBatchUpdatesSummary()
			return
		default:
			if batchData.IsClean() {
				return
			}
		}

		ProcessBatchUpdatesSummary()
		time.Sleep(time.Second * 1)
	}
}

// batchErrors collects errors from batch processors
type batchErrors struct {
	mu     sync.Mutex
	errors []error
}

func (e *batchErrors) Add(err error) {
	if err == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.errors = append(e.errors, err)
}

func (e *batchErrors) HasDBConnectionError() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	return slices.ContainsFunc(e.errors, common.IsDBConnectionError)
}

func (e *batchErrors) FirstDBConnectionError() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, err := range e.errors {
		if common.IsDBConnectionError(err) {
			return err
		}
	}

	return nil
}

func ProcessBatchUpdatesSummary() {
	batchData.Lock()
	defer batchData.Unlock()

	errs := &batchErrors{}
	g := new(errgroup.Group)

	g.Go(func() error {
		processGroupUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processTokenUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processChannelUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processGroupSummaryUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processSummaryUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processSummaryMinuteUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processGroupSummaryMinuteUpdates(errs)
		return nil
	})

	_ = g.Wait()

	// Check for database connection errors after all processors complete
	if dbErr := errs.FirstDBConnectionError(); dbErr != nil {
		oncall.AlertDBError("BatchProcessor", dbErr)
	} else {
		oncall.ClearDBError("BatchProcessor")
	}
}

func processGroupUpdates(errs *batchErrors) {
	for groupID, data := range batchData.Groups {
		err := UpdateGroupUsedAmountAndRequestCount(
			groupID,
			data.Amount.InexactFloat64(),
			data.Count,
		)
		if IgnoreNotFound(err) != nil {
			notify.ErrorThrottle(
				"batchUpdateGroupUsedAmountAndRequestCount",
				time.Minute*10,
				"failed to batch update group",
				err.Error(),
			)
			errs.Add(err)
		} else {
			delete(batchData.Groups, groupID)
		}
	}
}

func processTokenUpdates(errs *batchErrors) {
	for tokenID, data := range batchData.Tokens {
		err := UpdateTokenUsedAmount(tokenID, data.Amount.InexactFloat64(), data.Count)
		if IgnoreNotFound(err) != nil {
			notify.ErrorThrottle(
				"batchUpdateTokenUsedAmount",
				time.Minute*10,
				"failed to batch update token",
				err.Error(),
			)
			errs.Add(err)
		} else {
			delete(batchData.Tokens, tokenID)
		}
	}
}

func processChannelUpdates(errs *batchErrors) {
	for channelID, data := range batchData.Channels {
		err := UpdateChannelUsedAmount(
			channelID,
			data.Amount.InexactFloat64(),
			data.Count,
			data.RetryCount,
		)
		if IgnoreNotFound(err) != nil {
			notify.ErrorThrottle(
				"batchUpdateChannelUsedAmount",
				time.Minute*10,
				"failed to batch update channel",
				err.Error(),
			)
			errs.Add(err)
		} else {
			delete(batchData.Channels, channelID)
		}
	}
}

func processGroupSummaryUpdates(errs *batchErrors) {
	for key, data := range batchData.GroupSummaries {
		err := UpsertGroupSummary(data.GroupSummaryUnique, data.SummaryData)
		if err != nil {
			notify.ErrorThrottle(
				"batchUpdateGroupSummary",
				time.Minute*10,
				"failed to batch update group summary",
				err.Error(),
			)
			errs.Add(err)
		} else {
			delete(batchData.GroupSummaries, key)
		}
	}
}

func processGroupSummaryMinuteUpdates(errs *batchErrors) {
	for key, data := range batchData.GroupSummariesMinute {
		err := UpsertGroupSummaryMinute(data.GroupSummaryMinuteUnique, data.SummaryData)
		if err != nil {
			notify.ErrorThrottle(
				"batchUpdateGroupSummaryMinute",
				time.Minute*10,
				"failed to batch update group summary minute",
				err.Error(),
			)
			errs.Add(err)
		} else {
			delete(batchData.GroupSummariesMinute, key)
		}
	}
}

func processSummaryUpdates(errs *batchErrors) {
	for key, data := range batchData.Summaries {
		err := UpsertSummary(data.SummaryUnique, data.SummaryData)
		if err != nil {
			notify.ErrorThrottle(
				"batchUpdateSummary",
				time.Minute*10,
				"failed to batch update summary",
				err.Error(),
			)
			errs.Add(err)
		} else {
			delete(batchData.Summaries, key)
		}
	}
}

func processSummaryMinuteUpdates(errs *batchErrors) {
	for key, data := range batchData.SummariesMinute {
		err := UpsertSummaryMinute(data.SummaryMinuteUnique, data.SummaryData)
		if err != nil {
			notify.ErrorThrottle(
				"batchUpdateSummaryMinute",
				time.Minute*10,
				"failed to batch update summary minute",
				err.Error(),
			)
			errs.Add(err)
		} else {
			delete(batchData.SummariesMinute, key)
		}
	}
}

func BatchRecordLogs(
	now time.Time,
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
	user string,
	metadata map[string]string,
	upstreamID string,
) (err error) {
	if now.IsZero() {
		now = time.Now()
	}

	if code == http.StatusTooManyRequests ||
		config.GetLogDetailStorageHours() < 0 ||
		config.GetLogStorageHours() < 0 {
		requestDetail = nil
	}

	if downstreamResult {
		if config.GetLogStorageHours() >= 0 {
			err = RecordConsumeLog(
				requestID,
				now,
				requestAt,
				retryAt,
				firstByteAt,
				group,
				code,
				channelID,
				modelName,
				tokenID,
				tokenName,
				endpoint,
				content,
				mode,
				ip,
				retryTimes,
				requestDetail,
				usage,
				modelPrice,
				amount,
				user,
				metadata,
				upstreamID,
			)
		}
	} else {
		if code != http.StatusTooManyRequests &&
			config.GetLogStorageHours() >= 0 &&
			config.GetRetryLogStorageHours() > 0 {
			err = RecordRetryLog(
				requestID,
				now,
				requestAt,
				retryAt,
				firstByteAt,
				code,
				channelID,
				modelName,
				mode,
				retryTimes,
				requestDetail,
			)
		}
	}

	BatchUpdateSummary(
		now,
		requestAt,
		firstByteAt,
		group,
		code,
		channelID,
		modelName,
		tokenID,
		tokenName,
		downstreamResult,
		usage,
		amount,
	)

	return err
}

func BatchUpdateSummary(
	now time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	group string,
	code int,
	channelID int,
	modelName string,
	tokenID int,
	tokenName string,
	downstreamResult bool,
	usage Usage,
	amount float64,
) {
	if now.IsZero() {
		now = time.Now()
	}

	amountDecimal := decimal.NewFromFloat(amount)

	batchData.Lock()
	defer batchData.Unlock()

	updateChannelData(channelID, amount, amountDecimal, !downstreamResult)

	if channelID != 0 {
		updateSummaryData(
			channelID,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amountDecimal,
			usage,
			!downstreamResult,
		)

		updateSummaryDataMinute(
			channelID,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amountDecimal,
			usage,
			!downstreamResult,
		)
	}

	// group related data only records downstream result
	if !downstreamResult {
		return
	}

	updateGroupData(group, amount, amountDecimal)

	updateTokenData(tokenID, amount, amountDecimal)

	if group != "" {
		updateGroupSummaryData(
			group,
			tokenName,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amountDecimal,
			usage,
		)

		updateGroupSummaryDataMinute(
			group,
			tokenName,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amountDecimal,
			usage,
		)
	}
}

func updateChannelData(
	channelID int,
	amount float64,
	amountDecimal decimal.Decimal,
	isRetry bool,
) {
	if channelID <= 0 {
		return
	}

	if _, ok := batchData.Channels[channelID]; !ok {
		batchData.Channels[channelID] = &ChannelUpdate{}
	}

	if amount > 0 {
		batchData.Channels[channelID].Amount = amountDecimal.
			Add(batchData.Channels[channelID].Amount)
	}

	batchData.Channels[channelID].Count++
	if isRetry {
		batchData.Channels[channelID].RetryCount++
	}
}

func updateGroupData(group string, amount float64, amountDecimal decimal.Decimal) {
	if group == "" {
		return
	}

	if _, ok := batchData.Groups[group]; !ok {
		batchData.Groups[group] = &GroupUpdate{}
	}

	if amount > 0 {
		batchData.Groups[group].Amount = amountDecimal.
			Add(batchData.Groups[group].Amount)
	}

	batchData.Groups[group].Count++
}

func updateTokenData(tokenID int, amount float64, amountDecimal decimal.Decimal) {
	if tokenID <= 0 {
		return
	}

	if _, ok := batchData.Tokens[tokenID]; !ok {
		batchData.Tokens[tokenID] = &TokenUpdate{}
	}

	if amount > 0 {
		batchData.Tokens[tokenID].Amount = amountDecimal.
			Add(batchData.Tokens[tokenID].Amount)
	}

	batchData.Tokens[tokenID].Count++
}

func updateGroupSummaryData(
	group, tokenName, modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amountDecimal decimal.Decimal,
	usage Usage,
) {
	if createAt.IsZero() {
		createAt = time.Now()
	}

	if requestAt.IsZero() {
		requestAt = createAt
	}

	if firstByteAt.IsZero() || firstByteAt.Before(requestAt) {
		firstByteAt = requestAt
	}

	groupUnique := GroupSummaryUnique{
		GroupID:       group,
		TokenName:     tokenName,
		Model:         modelName,
		HourTimestamp: createAt.Truncate(time.Hour).Unix(),
	}

	groupSummary, ok := batchData.GroupSummaries[groupUnique]
	if !ok {
		groupSummary = &GroupSummaryUpdate{
			GroupSummaryUnique: groupUnique,
		}
		batchData.GroupSummaries[groupUnique] = groupSummary
	}

	groupSummary.UsedAmount = amountDecimal.
		Add(decimal.NewFromFloat(groupSummary.UsedAmount)).
		InexactFloat64()

	groupSummary.TotalTimeMilliseconds += createAt.Sub(requestAt).Milliseconds()
	groupSummary.TotalTTFBMilliseconds += firstByteAt.Sub(requestAt).Milliseconds()

	groupSummary.Usage.Add(usage)
	groupSummary.AddRequest(code, false)

	if usage.CachedTokens > 0 {
		groupSummary.CacheHitCount++
	}
}

func updateGroupSummaryDataMinute(
	group, tokenName, modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amountDecimal decimal.Decimal,
	usage Usage,
) {
	if createAt.IsZero() {
		createAt = time.Now()
	}

	if requestAt.IsZero() {
		requestAt = createAt
	}

	if firstByteAt.IsZero() || firstByteAt.Before(requestAt) {
		firstByteAt = requestAt
	}

	groupUnique := GroupSummaryMinuteUnique{
		GroupID:         group,
		TokenName:       tokenName,
		Model:           modelName,
		MinuteTimestamp: createAt.Truncate(time.Minute).Unix(),
	}

	groupSummary, ok := batchData.GroupSummariesMinute[groupUnique]
	if !ok {
		groupSummary = &GroupSummaryMinuteUpdate{
			GroupSummaryMinuteUnique: groupUnique,
		}
		batchData.GroupSummariesMinute[groupUnique] = groupSummary
	}

	groupSummary.UsedAmount = amountDecimal.
		Add(decimal.NewFromFloat(groupSummary.UsedAmount)).
		InexactFloat64()

	groupSummary.TotalTimeMilliseconds += createAt.Sub(requestAt).Milliseconds()
	groupSummary.TotalTTFBMilliseconds += firstByteAt.Sub(requestAt).Milliseconds()

	groupSummary.Usage.Add(usage)
	groupSummary.AddRequest(code, false)

	if usage.CachedTokens > 0 {
		groupSummary.CacheHitCount++
	}
}

func updateSummaryData(
	channelID int,
	modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amountDecimal decimal.Decimal,
	usage Usage,
	isRetry bool,
) {
	if createAt.IsZero() {
		createAt = time.Now()
	}

	if requestAt.IsZero() {
		requestAt = createAt
	}

	if firstByteAt.IsZero() || firstByteAt.Before(requestAt) {
		firstByteAt = requestAt
	}

	summaryUnique := SummaryUnique{
		ChannelID:     channelID,
		Model:         modelName,
		HourTimestamp: createAt.Truncate(time.Hour).Unix(),
	}

	summary, ok := batchData.Summaries[summaryUnique]
	if !ok {
		summary = &SummaryUpdate{
			SummaryUnique: summaryUnique,
		}
		batchData.Summaries[summaryUnique] = summary
	}

	summary.UsedAmount = amountDecimal.
		Add(decimal.NewFromFloat(summary.UsedAmount)).
		InexactFloat64()

	summary.TotalTimeMilliseconds += createAt.Sub(requestAt).Milliseconds()
	summary.TotalTTFBMilliseconds += firstByteAt.Sub(requestAt).Milliseconds()

	summary.Usage.Add(usage)
	summary.AddRequest(code, isRetry)

	if usage.CachedTokens > 0 {
		summary.CacheHitCount++
	}
}

func updateSummaryDataMinute(
	channelID int,
	modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amountDecimal decimal.Decimal,
	usage Usage,
	isRetry bool,
) {
	if createAt.IsZero() {
		createAt = time.Now()
	}

	if requestAt.IsZero() {
		requestAt = createAt
	}

	if firstByteAt.IsZero() || firstByteAt.Before(requestAt) {
		firstByteAt = requestAt
	}

	summaryUnique := SummaryMinuteUnique{
		ChannelID:       channelID,
		Model:           modelName,
		MinuteTimestamp: createAt.Truncate(time.Minute).Unix(),
	}

	summary, ok := batchData.SummariesMinute[summaryUnique]
	if !ok {
		summary = &SummaryMinuteUpdate{
			SummaryMinuteUnique: summaryUnique,
		}
		batchData.SummariesMinute[summaryUnique] = summary
	}

	summary.UsedAmount = amountDecimal.
		Add(decimal.NewFromFloat(summary.UsedAmount)).
		InexactFloat64()

	summary.TotalTimeMilliseconds += createAt.Sub(requestAt).Milliseconds()
	summary.TotalTTFBMilliseconds += firstByteAt.Sub(requestAt).Milliseconds()

	summary.Usage.Add(usage)
	summary.AddRequest(code, isRetry)

	if usage.CachedTokens > 0 {
		summary.CacheHitCount++
	}
}
