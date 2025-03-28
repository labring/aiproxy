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
	Groups    map[string]*GroupUpdate
	Tokens    map[int]*TokenUpdate
	Channels  map[int]*ChannelUpdate
	Summaries map[string]*SummaryUpdate
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
	return fmt.Sprintf("%s:%s:%s:%d:%d", unique.GroupID, unique.TokenName, unique.Model, unique.ChannelID, unique.HourTimestamp)
}

var batchData BatchUpdateData

func init() {
	batchData = BatchUpdateData{
		Groups:    make(map[string]*GroupUpdate),
		Tokens:    make(map[int]*TokenUpdate),
		Channels:  make(map[int]*ChannelUpdate),
		Summaries: make(map[string]*SummaryUpdate),
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
	go func() {
		defer wg.Done()
		if len(batchData.Groups) > 0 {
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
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if len(batchData.Tokens) > 0 {
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
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if len(batchData.Channels) > 0 {
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
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if len(batchData.Summaries) > 0 {
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
	}()

	wg.Wait()
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

	if !downstreamResult {
		return err
	}

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

	unique := SummaryUnique{
		GroupID:       group,
		TokenName:     tokenName,
		Model:         modelName,
		ChannelID:     channelID,
		HourTimestamp: requestAt.Truncate(time.Hour).Unix(),
	}

	summaryKey := summaryUniqueKey(unique)
	summary, ok := batchData.Summaries[summaryKey]
	if !ok {
		summary = &SummaryUpdate{
			SummaryUnique: unique,
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

	return err
}
