package model

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labring/aiproxy/common/notify"
	"github.com/shopspring/decimal"
)

type BatchUpdateData struct {
	Groups         map[string]*GroupUpdate
	Tokens         map[int]*TokenUpdate
	Channels       map[int]*ChannelUpdate
	Summaries      map[string]*SummaryUpdate
	GroupSummaries map[string]*GroupSummaryUpdate
	sync.Mutex
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

func summaryUniqueKey(unique SummaryUnique) string {
	return fmt.Sprintf("%d:%s:%d", unique.ChannelID, unique.Model, unique.HourTimestamp)
}

type GroupSummaryUpdate struct {
	GroupSummaryUnique
	SummaryData
}

func groupSummaryUniqueKey(unique GroupSummaryUnique) string {
	return fmt.Sprintf("%s:%s:%s:%d", unique.GroupID, unique.TokenName, unique.Model, unique.HourTimestamp)
}

var batchData BatchUpdateData

func init() {
	batchData = BatchUpdateData{
		Groups:         make(map[string]*GroupUpdate),
		Tokens:         make(map[int]*TokenUpdate),
		Channels:       make(map[int]*ChannelUpdate),
		Summaries:      make(map[string]*SummaryUpdate),
		GroupSummaries: make(map[string]*GroupSummaryUpdate),
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

	wg.Wait()
}

func processGroupUpdates(wg *sync.WaitGroup) {
	defer wg.Done()
	for groupID, data := range batchData.Groups {
		err := UpdateGroupUsedAmountAndRequestCount(groupID, data.Amount.InexactFloat64(), data.Count)
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

func BatchRecordConsume(
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
	err := RecordConsumeLog(
		requestID,
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
		downstreamResult,
		usage,
		modelPrice,
		amount,
	)

	amountDecimal := decimal.NewFromFloat(amount)

	batchData.Lock()
	defer batchData.Unlock()

	updateChannelData(channelID, amount, amountDecimal)

	if !downstreamResult {
		return err
	}

	updateGroupData(group, amount, amountDecimal)

	updateTokenData(tokenID, amount, amountDecimal)

	updateSummaryData(channelID, modelName, requestAt, code, amountDecimal, usage)

	updateGroupSummaryData(group, tokenName, modelName, requestAt, code, amountDecimal, usage)

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

func updateGroupSummaryData(group string, tokenName string, modelName string, requestAt time.Time, code int, amountDecimal decimal.Decimal, usage Usage) {
	groupUnique := GroupSummaryUnique{
		GroupID:       group,
		TokenName:     tokenName,
		Model:         modelName,
		HourTimestamp: requestAt.Truncate(time.Hour).Unix(),
	}

	groupSummaryKey := groupSummaryUniqueKey(groupUnique)
	groupSummary, ok := batchData.GroupSummaries[groupSummaryKey]
	if !ok {
		groupSummary = &GroupSummaryUpdate{
			GroupSummaryUnique: groupUnique,
		}
		batchData.GroupSummaries[groupSummaryKey] = groupSummary
	}

	groupSummary.RequestCount++
	groupSummary.UsedAmount = amountDecimal.
		Add(decimal.NewFromFloat(groupSummary.UsedAmount)).
		InexactFloat64()
	groupSummary.Usage.Add(&usage)
	if code != http.StatusOK {
		groupSummary.ExceptionCount++
	}
}

func updateSummaryData(channelID int, modelName string, requestAt time.Time, code int, amountDecimal decimal.Decimal, usage Usage) {
	summaryUnique := SummaryUnique{
		ChannelID:     channelID,
		Model:         modelName,
		HourTimestamp: requestAt.Truncate(time.Hour).Unix(),
	}

	summaryKey := summaryUniqueKey(summaryUnique)
	summary, ok := batchData.Summaries[summaryKey]
	if !ok {
		summary = &SummaryUpdate{
			SummaryUnique: summaryUnique,
		}
		batchData.Summaries[summaryKey] = summary
	}

	summary.RequestCount++
	summary.UsedAmount = amountDecimal.
		Add(decimal.NewFromFloat(summary.UsedAmount)).
		InexactFloat64()
	summary.Usage.Add(&usage)
	if code != http.StatusOK {
		summary.ExceptionCount++
	}
}
