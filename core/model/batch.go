package model

import (
	"context"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/shopspring/decimal"
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
	Amount decimal.Decimal
	Count  int
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

func ProcessBatchUpdatesSummary() {
	batchData.Lock()
	defer batchData.Unlock()

	var wg sync.WaitGroup

	wg.Add(1)

	go processGroupUpdates(&wg)

	wg.Add(1)

	go processTokenUpdates(&wg)

	wg.Add(1)

	go processChannelUpdates(&wg)

	wg.Add(1)

	go processGroupSummaryUpdates(&wg)

	wg.Add(1)

	go processSummaryUpdates(&wg)

	wg.Add(1)

	go processSummaryMinuteUpdates(&wg)

	wg.Add(1)

	go processGroupSummaryMinuteUpdates(&wg)

	wg.Wait()
}

func processGroupUpdates(wg *sync.WaitGroup) {
	defer wg.Done()

	for groupID, data := range batchData.Groups {
		err := UpdateGroupUsedAmountAndRequestCount(
			groupID,
			data.Amount.InexactFloat64(),
			data.Count,
		)
		if IgnoreNotFound(err) != nil {
			notify.ErrorThrottle(
				"batchUpdateGroupUsedAmountAndRequestCount",
				time.Minute,
				"failed to batch update group",
				err.Error(),
			)
		} else {
			delete(batchData.Groups, groupID)
		}
	}
}

func processTokenUpdates(wg *sync.WaitGroup) {
	defer wg.Done()

	for tokenID, data := range batchData.Tokens {
		err := UpdateTokenUsedAmount(tokenID, data.Amount.InexactFloat64(), data.Count)
		if IgnoreNotFound(err) != nil {
			notify.ErrorThrottle(
				"batchUpdateTokenUsedAmount",
				time.Minute,
				"failed to batch update token",
				err.Error(),
			)
		} else {
			delete(batchData.Tokens, tokenID)
		}
	}
}

func processChannelUpdates(wg *sync.WaitGroup) {
	defer wg.Done()

	for channelID, data := range batchData.Channels {
		err := UpdateChannelUsedAmount(channelID, data.Amount.InexactFloat64(), data.Count)
		if IgnoreNotFound(err) != nil {
			notify.ErrorThrottle(
				"batchUpdateChannelUsedAmount",
				time.Minute,
				"failed to batch update channel",
				err.Error(),
			)
		} else {
			delete(batchData.Channels, channelID)
		}
	}
}

func processGroupSummaryUpdates(wg *sync.WaitGroup) {
	defer wg.Done()

	for key, data := range batchData.GroupSummaries {
		err := UpsertGroupSummary(data.GroupSummaryUnique, data.SummaryData)
		if err != nil {
			notify.ErrorThrottle(
				"batchUpdateGroupSummary",
				time.Minute,
				"failed to batch update group summary",
				err.Error(),
			)
		} else {
			delete(batchData.GroupSummaries, key)
		}
	}
}

func processGroupSummaryMinuteUpdates(wg *sync.WaitGroup) {
	defer wg.Done()

	for key, data := range batchData.GroupSummariesMinute {
		err := UpsertGroupSummaryMinute(data.GroupSummaryMinuteUnique, data.SummaryData)
		if err != nil {
			notify.ErrorThrottle(
				"batchUpdateGroupSummary",
				time.Minute,
				"failed to batch update group summary",
				err.Error(),
			)
		} else {
			delete(batchData.GroupSummariesMinute, key)
		}
	}
}

func processSummaryUpdates(wg *sync.WaitGroup) {
	defer wg.Done()

	for key, data := range batchData.Summaries {
		err := UpsertSummary(data.SummaryUnique, data.SummaryData)
		if err != nil {
			notify.ErrorThrottle(
				"batchUpdateSummary",
				time.Minute,
				"failed to batch update summary",
				err.Error(),
			)
		} else {
			delete(batchData.Summaries, key)
		}
	}
}

func processSummaryMinuteUpdates(wg *sync.WaitGroup) {
	defer wg.Done()

	for key, data := range batchData.SummariesMinute {
		err := UpsertSummaryMinute(data.SummaryMinuteUnique, data.SummaryData)
		if err != nil {
			notify.ErrorThrottle(
				"batchUpdateSummaryMinute",
				time.Minute,
				"failed to batch update summary minute",
				err.Error(),
			)
		} else {
			delete(batchData.SummariesMinute, key)
		}
	}
}

func BatchRecordLogs(
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
) (err error) {
	now := time.Now()

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
			)
		}
	} else {
		if config.GetRetryLogStorageHours() >= 0 {
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

	amountDecimal := decimal.NewFromFloat(amount)

	batchData.Lock()
	defer batchData.Unlock()

	updateChannelData(channelID, amount, amountDecimal)

	if !downstreamResult {
		return err
	}

	updateGroupData(group, amount, amountDecimal)

	updateTokenData(tokenID, amount, amountDecimal)

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
		)
	}

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

	return err
}

func updateChannelData(channelID int, amount float64, amountDecimal decimal.Decimal) {
	if channelID > 0 {
		if _, ok := batchData.Channels[channelID]; !ok {
			batchData.Channels[channelID] = &ChannelUpdate{}
		}

		if amount > 0 {
			batchData.Channels[channelID].Amount = amountDecimal.
				Add(batchData.Channels[channelID].Amount)
		}

		batchData.Channels[channelID].Count++
	}
}

func updateGroupData(group string, amount float64, amountDecimal decimal.Decimal) {
	if group != "" {
		if _, ok := batchData.Groups[group]; !ok {
			batchData.Groups[group] = &GroupUpdate{}
		}

		if amount > 0 {
			batchData.Groups[group].Amount = amountDecimal.
				Add(batchData.Groups[group].Amount)
		}

		batchData.Groups[group].Count++
	}
}

func updateTokenData(tokenID int, amount float64, amountDecimal decimal.Decimal) {
	if tokenID > 0 {
		if _, ok := batchData.Tokens[tokenID]; !ok {
			batchData.Tokens[tokenID] = &TokenUpdate{}
		}

		if amount > 0 {
			batchData.Tokens[tokenID].Amount = amountDecimal.
				Add(batchData.Tokens[tokenID].Amount)
		}

		batchData.Tokens[tokenID].Count++
	}
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
	groupSummary.AddRequest(code)
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
	groupSummary.AddRequest(code)
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
	summary.AddRequest(code)
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
	summary.AddRequest(code)
}
