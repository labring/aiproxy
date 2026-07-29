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
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type batchUpdateData struct {
	Groups                           map[string]*GroupUpdate
	Tokens                           map[int]*TokenUpdate
	Channels                         map[int]*ChannelUpdate
	GroupChannels                    map[groupChannelUpdateKey]*ChannelUpdate
	GroupChannelGroups               map[string]*GroupUpdate
	GroupChannelTokens               map[int]*TokenUpdate
	Summaries                        map[SummaryUnique]*SummaryUpdate
	GroupSummaries                   map[GroupSummaryUnique]*GroupSummaryUpdate
	GroupChannelSummaries            map[GroupChannelSummaryUnique]*GroupChannelSummaryUpdate
	GroupChannelTokenSummaries       map[GroupChannelTokenSummaryUnique]*GroupChannelTokenSummaryUpdate
	SummariesMinute                  map[SummaryMinuteUnique]*SummaryMinuteUpdate
	GroupSummariesMinute             map[GroupSummaryMinuteUnique]*GroupSummaryMinuteUpdate
	GroupChannelSummariesMinute      map[GroupChannelSummaryMinuteUnique]*GroupChannelSummaryMinuteUpdate
	GroupChannelTokenSummariesMinute map[GroupChannelTokenSummaryMinuteUnique]*GroupChannelTokenSummaryMinuteUpdate
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
		len(b.GroupChannels) == 0 &&
		len(b.GroupChannelGroups) == 0 &&
		len(b.GroupChannelTokens) == 0 &&
		len(b.Summaries) == 0 &&
		len(b.GroupSummaries) == 0 &&
		len(b.GroupChannelSummaries) == 0 &&
		len(b.GroupChannelTokenSummaries) == 0 &&
		len(b.SummariesMinute) == 0 &&
		len(b.GroupSummariesMinute) == 0 &&
		len(b.GroupChannelSummariesMinute) == 0 &&
		len(b.GroupChannelTokenSummariesMinute) == 0
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

type groupChannelUpdateKey struct {
	GroupID string
	ID      int
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

type GroupChannelSummaryUpdate struct {
	GroupChannelSummaryUnique
	SummaryData
}

type GroupChannelSummaryMinuteUpdate struct {
	GroupChannelSummaryMinuteUnique
	SummaryData
}

type GroupChannelTokenSummaryUpdate struct {
	GroupChannelTokenSummaryUnique
	SummaryData
}

type GroupChannelTokenSummaryMinuteUpdate struct {
	GroupChannelTokenSummaryMinuteUnique
	SummaryData
}

var batchData batchUpdateData

func init() {
	batchData = batchUpdateData{
		Groups:             make(map[string]*GroupUpdate),
		Tokens:             make(map[int]*TokenUpdate),
		Channels:           make(map[int]*ChannelUpdate),
		GroupChannels:      make(map[groupChannelUpdateKey]*ChannelUpdate),
		GroupChannelGroups: make(map[string]*GroupUpdate),
		GroupChannelTokens: make(map[int]*TokenUpdate),
		Summaries:          make(map[SummaryUnique]*SummaryUpdate),
		GroupSummaries:     make(map[GroupSummaryUnique]*GroupSummaryUpdate),
		GroupChannelSummaries: make(
			map[GroupChannelSummaryUnique]*GroupChannelSummaryUpdate,
		),
		GroupChannelTokenSummaries: make(
			map[GroupChannelTokenSummaryUnique]*GroupChannelTokenSummaryUpdate,
		),
		SummariesMinute: make(map[SummaryMinuteUnique]*SummaryMinuteUpdate),
		GroupSummariesMinute: make(
			map[GroupSummaryMinuteUnique]*GroupSummaryMinuteUpdate,
		),
		GroupChannelSummariesMinute: make(
			map[GroupChannelSummaryMinuteUnique]*GroupChannelSummaryMinuteUpdate,
		),
		GroupChannelTokenSummariesMinute: make(
			map[GroupChannelTokenSummaryMinuteUnique]*GroupChannelTokenSummaryMinuteUpdate,
		),
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
		processGroupChannelUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processGroupChannelGroupUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processGroupChannelTokenUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processGroupSummaryUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processGroupChannelSummaryUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processGroupChannelTokenSummaryUpdates(errs)
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
	g.Go(func() error {
		processGroupChannelSummaryMinuteUpdates(errs)
		return nil
	})
	g.Go(func() error {
		processGroupChannelTokenSummaryMinuteUpdates(errs)
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

func logBatchGroupChannelUpdateError(message string, err error) {
	log.WithError(err).Error(message)
}

func processGroupChannelUpdates(errs *batchErrors) {
	for key, data := range batchData.GroupChannels {
		err := UpdateGroupChannelUsedAmount(
			key.GroupID,
			key.ID,
			data.Amount.InexactFloat64(),
			data.Count,
			data.RetryCount,
		)
		if IgnoreNotFound(err) != nil {
			logBatchGroupChannelUpdateError("failed to batch update group channel", err)
			errs.Add(err)
		} else {
			delete(batchData.GroupChannels, key)
		}
	}
}

func processGroupChannelGroupUpdates(errs *batchErrors) {
	for groupID, data := range batchData.GroupChannelGroups {
		err := UpdateGroupChannelGroupUsedAmountAndRequestCount(
			groupID,
			data.Amount.InexactFloat64(),
			data.Count,
		)
		if IgnoreNotFound(err) != nil {
			logBatchGroupChannelUpdateError("failed to batch update group channel group", err)
			errs.Add(err)
		} else {
			delete(batchData.GroupChannelGroups, groupID)
		}
	}
}

func processGroupChannelTokenUpdates(errs *batchErrors) {
	for tokenID, data := range batchData.GroupChannelTokens {
		err := UpdateGroupChannelTokenUsedAmount(
			tokenID,
			data.Amount.InexactFloat64(),
			data.Count,
		)
		if IgnoreNotFound(err) != nil {
			logBatchGroupChannelUpdateError("failed to batch update group channel token", err)
			errs.Add(err)
		} else {
			delete(batchData.GroupChannelTokens, tokenID)
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

func processGroupChannelSummaryUpdates(errs *batchErrors) {
	for key, data := range batchData.GroupChannelSummaries {
		err := UpsertGroupChannelSummary(data.GroupChannelSummaryUnique, data.SummaryData)
		if err != nil {
			logBatchGroupChannelUpdateError("failed to batch update group channel summary", err)
			errs.Add(err)
		} else {
			delete(batchData.GroupChannelSummaries, key)
		}
	}
}

func processGroupChannelTokenSummaryUpdates(errs *batchErrors) {
	for key, data := range batchData.GroupChannelTokenSummaries {
		err := UpsertGroupChannelTokenSummary(
			data.GroupChannelTokenSummaryUnique,
			data.SummaryData,
		)
		if err != nil {
			logBatchGroupChannelUpdateError(
				"failed to batch update group channel token summary",
				err,
			)
			errs.Add(err)
		} else {
			delete(batchData.GroupChannelTokenSummaries, key)
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

func processGroupChannelSummaryMinuteUpdates(errs *batchErrors) {
	for key, data := range batchData.GroupChannelSummariesMinute {
		err := UpsertGroupChannelSummaryMinute(
			data.GroupChannelSummaryMinuteUnique,
			data.SummaryData,
		)
		if err != nil {
			logBatchGroupChannelUpdateError(
				"failed to batch update group channel summary minute",
				err,
			)
			errs.Add(err)
		} else {
			delete(batchData.GroupChannelSummariesMinute, key)
		}
	}
}

func processGroupChannelTokenSummaryMinuteUpdates(errs *batchErrors) {
	for key, data := range batchData.GroupChannelTokenSummariesMinute {
		err := UpsertGroupChannelTokenSummaryMinute(
			data.GroupChannelTokenSummaryMinuteUnique,
			data.SummaryData,
		)
		if err != nil {
			logBatchGroupChannelUpdateError(
				"failed to batch update group channel token summary minute",
				err,
			)
			errs.Add(err)
		} else {
			delete(batchData.GroupChannelTokenSummariesMinute, key)
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
	channelScope ChannelScope,
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
	usageContext UsageContext,
	modelPrice Price,
	amount Amount,
	user string,
	metadata map[string]string,
	promptCacheKey string,
	upstreamID string,
	asyncUsageStatus AsyncUsageStatus,
	summaryServiceTier string,
	summaryClaudeLongContext bool,
) (err error) {
	if now.IsZero() {
		now = time.Now()
	}

	if channelScope == ChannelScopeGroup {
		asyncUsageStatus = AsyncUsageStatusNone
	}

	if code == http.StatusTooManyRequests ||
		config.GetLogDetailStorageHours() < 0 ||
		config.GetLogStorageHours() < 0 {
		requestDetail = nil
	}

	if downstreamResult {
		if config.GetLogStorageHours() >= 0 {
			if channelScope == ChannelScopeGroup {
				err = RecordGroupChannelConsumeLog(
					requestID, now, requestAt, retryAt, firstByteAt, group, code, channelID,
					modelName, tokenID, tokenName, endpoint, content, mode, ip, retryTimes,
					requestDetail, usage, usageContext, modelPrice, amount, user, metadata,
					promptCacheKey, upstreamID, asyncUsageStatus,
				)
			} else {
				err = RecordConsumeLog(
					requestID, now, requestAt, retryAt, firstByteAt, group, code, channelID,
					modelName, tokenID, tokenName, endpoint, content, mode, ip, retryTimes,
					requestDetail, usage, usageContext, modelPrice, amount, user, metadata,
					promptCacheKey, upstreamID, asyncUsageStatus,
				)
			}
		}
	} else {
		if code != http.StatusTooManyRequests &&
			config.GetLogStorageHours() >= 0 &&
			config.GetRetryLogStorageHours() > 0 {
			if channelScope == ChannelScopeGroup {
				err = RecordGroupChannelRetryLog(
					requestID,
					now,
					requestAt,
					retryAt,
					firstByteAt,
					group,
					code,
					channelID,
					modelName,
					mode,
					retryTimes,
					requestDetail,
				)
			} else {
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
	}

	BatchUpdateSummary(
		now,
		requestAt,
		firstByteAt,
		group,
		code,
		channelScope,
		channelID,
		modelName,
		tokenID,
		tokenName,
		downstreamResult,
		usage,
		amount,
		summaryServiceTier,
		summaryClaudeLongContext,
	)

	return err
}

func BatchUpdateSummary(
	now time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	group string,
	code int,
	channelScope ChannelScope,
	channelID int,
	modelName string,
	tokenID int,
	tokenName string,
	downstreamResult bool,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if now.IsZero() {
		now = time.Now()
	}

	amountDecimal := decimal.NewFromFloat(amount.UsedAmount)

	batchData.Lock()
	defer batchData.Unlock()

	if channelScope == ChannelScopeGroup {
		updateGroupChannelData(
			group,
			channelID,
			amount.UsedAmount,
			amountDecimal,
			!downstreamResult,
		)
	} else {
		updateChannelData(channelID, amount.UsedAmount, amountDecimal, !downstreamResult)
	}

	if channelID != 0 && channelScope != ChannelScopeGroup {
		updateSummaryData(
			channelID,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amount,
			usage,
			!downstreamResult,
			serviceTier,
			summaryClaudeLongContext,
		)

		updateSummaryDataMinute(
			channelID,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amount,
			usage,
			!downstreamResult,
			serviceTier,
			summaryClaudeLongContext,
		)
	}

	if channelID != 0 && channelScope == ChannelScopeGroup && group != "" {
		updateGroupChannelSummaryData(
			group,
			channelID,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amount,
			usage,
			!downstreamResult,
			serviceTier,
			summaryClaudeLongContext,
		)

		updateGroupChannelSummaryDataMinute(
			group,
			channelID,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amount,
			usage,
			!downstreamResult,
			serviceTier,
			summaryClaudeLongContext,
		)
	}

	// group related data only records downstream result
	if !downstreamResult {
		return
	}

	if channelScope == ChannelScopeGroup {
		if group != "" && tokenID > 0 {
			updateGroupChannelGroupData(group, amount.UsedAmount, amountDecimal)
			updateGroupChannelTokenData(tokenID, amount.UsedAmount, amountDecimal)
		}

		if group != "" && tokenName != "" {
			updateGroupChannelTokenSummaryData(
				group,
				tokenName,
				modelName,
				now,
				requestAt,
				firstByteAt,
				code,
				amount,
				usage,
				serviceTier,
				summaryClaudeLongContext,
			)

			updateGroupChannelTokenSummaryDataMinute(
				group,
				tokenName,
				modelName,
				now,
				requestAt,
				firstByteAt,
				code,
				amount,
				usage,
				serviceTier,
				summaryClaudeLongContext,
			)
		}

		return
	}

	updateGroupData(group, amount.UsedAmount, amountDecimal)

	updateTokenData(tokenID, amount.UsedAmount, amountDecimal)

	if group != "" {
		updateGroupSummaryData(
			group,
			tokenName,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amount,
			usage,
			serviceTier,
			summaryClaudeLongContext,
		)

		updateGroupSummaryDataMinute(
			group,
			tokenName,
			modelName,
			now,
			requestAt,
			firstByteAt,
			code,
			amount,
			usage,
			serviceTier,
			summaryClaudeLongContext,
		)
	}
}

func BatchUpdateSummaryOnlyUsage(
	now time.Time,
	requestAt time.Time,
	group string,
	channelScope ChannelScope,
	channelID int,
	modelName string,
	tokenID int,
	tokenName string,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if now.IsZero() {
		now = time.Now()
	}

	summaryAt := now
	if !requestAt.IsZero() {
		summaryAt = requestAt
	}

	amountDecimal := decimal.NewFromFloat(amount.UsedAmount)

	batchData.Lock()
	defer batchData.Unlock()

	if channelScope == ChannelScopeGroup {
		updateGroupChannelSummaryUsageData(
			group,
			channelID,
			modelName,
			summaryAt,
			usage,
			amount,
			serviceTier,
			summaryClaudeLongContext,
		)
		updateGroupChannelSummaryUsageDataMinute(
			group,
			channelID,
			modelName,
			summaryAt,
			usage,
			amount,
			serviceTier,
			summaryClaudeLongContext,
		)
	} else {
		updateChannelAmountData(channelID, amount.UsedAmount, amountDecimal)
		updateSummaryUsageData(
			channelID,
			modelName,
			summaryAt,
			usage,
			amount,
			serviceTier,
			summaryClaudeLongContext,
		)
		updateSummaryUsageDataMinute(
			channelID,
			modelName,
			summaryAt,
			usage,
			amount,
			serviceTier,
			summaryClaudeLongContext,
		)
	}

	if channelScope == ChannelScopeGroup {
		if group != "" && tokenID > 0 && tokenName != "" {
			updateGroupChannelTokenSummaryUsageData(
				group,
				tokenName,
				modelName,
				summaryAt,
				usage,
				amount,
				serviceTier,
				summaryClaudeLongContext,
			)
			updateGroupChannelTokenSummaryUsageDataMinute(
				group,
				tokenName,
				modelName,
				summaryAt,
				usage,
				amount,
				serviceTier,
				summaryClaudeLongContext,
			)
		}

		return
	}

	updateGroupAmountData(group, amount.UsedAmount, amountDecimal)
	updateTokenAmountData(tokenID, amount.UsedAmount, amountDecimal)
	updateGroupSummaryUsageData(
		group,
		tokenName,
		modelName,
		summaryAt,
		usage,
		amount,
		serviceTier,
		summaryClaudeLongContext,
	)
	updateGroupSummaryUsageDataMinute(
		group,
		tokenName,
		modelName,
		summaryAt,
		usage,
		amount,
		serviceTier,
		summaryClaudeLongContext,
	)
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

func updateChannelAmountData(channelID int, amount float64, amountDecimal decimal.Decimal) {
	if channelID <= 0 || amount <= 0 {
		return
	}

	if _, ok := batchData.Channels[channelID]; !ok {
		batchData.Channels[channelID] = &ChannelUpdate{}
	}

	batchData.Channels[channelID].Amount = amountDecimal.
		Add(batchData.Channels[channelID].Amount)
}

func updateGroupChannelData(
	group string,
	channelID int,
	amount float64,
	amountDecimal decimal.Decimal,
	isRetry bool,
) {
	if group == "" || channelID <= 0 {
		return
	}

	key := groupChannelUpdateKey{GroupID: group, ID: channelID}
	if _, ok := batchData.GroupChannels[key]; !ok {
		batchData.GroupChannels[key] = &ChannelUpdate{}
	}

	if amount > 0 {
		batchData.GroupChannels[key].Amount = amountDecimal.
			Add(batchData.GroupChannels[key].Amount)
	}

	batchData.GroupChannels[key].Count++
	if isRetry {
		batchData.GroupChannels[key].RetryCount++
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

func updateGroupAmountData(group string, amount float64, amountDecimal decimal.Decimal) {
	if group == "" || amount <= 0 {
		return
	}

	if _, ok := batchData.Groups[group]; !ok {
		batchData.Groups[group] = &GroupUpdate{}
	}

	batchData.Groups[group].Amount = amountDecimal.
		Add(batchData.Groups[group].Amount)
}

func updateGroupChannelGroupData(group string, amount float64, amountDecimal decimal.Decimal) {
	if group == "" {
		return
	}

	if _, ok := batchData.GroupChannelGroups[group]; !ok {
		batchData.GroupChannelGroups[group] = &GroupUpdate{}
	}

	if amount > 0 {
		batchData.GroupChannelGroups[group].Amount = amountDecimal.
			Add(batchData.GroupChannelGroups[group].Amount)
	}

	batchData.GroupChannelGroups[group].Count++
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

func updateTokenAmountData(tokenID int, amount float64, amountDecimal decimal.Decimal) {
	if tokenID <= 0 || amount <= 0 {
		return
	}

	if _, ok := batchData.Tokens[tokenID]; !ok {
		batchData.Tokens[tokenID] = &TokenUpdate{}
	}

	batchData.Tokens[tokenID].Amount = amountDecimal.
		Add(batchData.Tokens[tokenID].Amount)
}

func updateGroupChannelTokenData(tokenID int, amount float64, amountDecimal decimal.Decimal) {
	if tokenID <= 0 {
		return
	}

	if _, ok := batchData.GroupChannelTokens[tokenID]; !ok {
		batchData.GroupChannelTokens[tokenID] = &TokenUpdate{}
	}

	if amount > 0 {
		batchData.GroupChannelTokens[tokenID].Amount = amountDecimal.
			Add(batchData.GroupChannelTokens[tokenID].Amount)
	}

	batchData.GroupChannelTokens[tokenID].Count++
}

func updateGroupSummaryData(
	group, tokenName, modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if createAt.IsZero() {
		createAt = time.Now()
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

	addRequestSummaryData(
		&groupSummary.SummaryData,
		createAt,
		requestAt,
		firstByteAt,
		code,
		amount,
		usage,
		false,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func addRequestSummaryData(
	data *SummaryData,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	isRetry bool,
	serviceTier string,
	summaryClaudeLongContext bool,
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

	totalTimeMilliseconds, totalTTFBMilliseconds := getSummaryLatencyMetrics(
		createAt,
		requestAt,
		firstByteAt,
	)

	data.Amount.Add(amount)
	data.TotalTimeMilliseconds += totalTimeMilliseconds
	data.TotalTTFBMilliseconds += totalTTFBMilliseconds
	data.Usage.Add(usage)
	data.AddRequest(code, isRetry)
	data.AddServiceTierBreakdown(
		serviceTier,
		usage,
		amount,
		totalTimeMilliseconds,
		totalTTFBMilliseconds,
		isRetry,
		code,
	)

	if summaryClaudeLongContext {
		data.AddClaudeLongContextBreakdown(usage, amount, isRetry, code)
		data.ClaudeLongContext.TotalTimeMilliseconds += totalTimeMilliseconds
		data.ClaudeLongContext.TotalTTFBMilliseconds += totalTTFBMilliseconds
	}

	if usage.CachedTokens > 0 {
		data.CacheHitCount++
	}

	if usage.CacheCreationTokens > 0 {
		data.CacheCreationCount++
	}
}

func updateGroupChannelSummaryData(
	group string,
	groupChannelID int,
	modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	isRetry bool,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" || groupChannelID <= 0 {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
	}

	unique := GroupChannelSummaryUnique{
		GroupID:        group,
		GroupChannelID: groupChannelID,
		Model:          modelName,
		HourTimestamp:  createAt.Truncate(time.Hour).Unix(),
	}

	summary, ok := batchData.GroupChannelSummaries[unique]
	if !ok {
		summary = &GroupChannelSummaryUpdate{GroupChannelSummaryUnique: unique}
		batchData.GroupChannelSummaries[unique] = summary
	}

	addRequestSummaryData(
		&summary.SummaryData,
		createAt,
		requestAt,
		firstByteAt,
		code,
		amount,
		usage,
		isRetry,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateGroupChannelSummaryDataMinute(
	group string,
	groupChannelID int,
	modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	isRetry bool,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" || groupChannelID <= 0 {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
	}

	unique := GroupChannelSummaryMinuteUnique{
		GroupID:         group,
		GroupChannelID:  groupChannelID,
		Model:           modelName,
		MinuteTimestamp: createAt.Truncate(time.Minute).Unix(),
	}

	summary, ok := batchData.GroupChannelSummariesMinute[unique]
	if !ok {
		summary = &GroupChannelSummaryMinuteUpdate{GroupChannelSummaryMinuteUnique: unique}
		batchData.GroupChannelSummariesMinute[unique] = summary
	}

	addRequestSummaryData(
		&summary.SummaryData,
		createAt,
		requestAt,
		firstByteAt,
		code,
		amount,
		usage,
		isRetry,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateGroupChannelTokenSummaryData(
	group, tokenName, modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" || tokenName == "" {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
	}

	unique := GroupChannelTokenSummaryUnique{
		GroupID:       group,
		TokenName:     tokenName,
		Model:         modelName,
		HourTimestamp: createAt.Truncate(time.Hour).Unix(),
	}

	summary, ok := batchData.GroupChannelTokenSummaries[unique]
	if !ok {
		summary = &GroupChannelTokenSummaryUpdate{
			GroupChannelTokenSummaryUnique: unique,
		}
		batchData.GroupChannelTokenSummaries[unique] = summary
	}

	addRequestSummaryData(
		&summary.SummaryData,
		createAt,
		requestAt,
		firstByteAt,
		code,
		amount,
		usage,
		false,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateGroupChannelTokenSummaryDataMinute(
	group, tokenName, modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" || tokenName == "" {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
	}

	unique := GroupChannelTokenSummaryMinuteUnique{
		GroupID:         group,
		TokenName:       tokenName,
		Model:           modelName,
		MinuteTimestamp: createAt.Truncate(time.Minute).Unix(),
	}

	summary, ok := batchData.GroupChannelTokenSummariesMinute[unique]
	if !ok {
		summary = &GroupChannelTokenSummaryMinuteUpdate{
			GroupChannelTokenSummaryMinuteUnique: unique,
		}
		batchData.GroupChannelTokenSummariesMinute[unique] = summary
	}

	addRequestSummaryData(
		&summary.SummaryData,
		createAt,
		requestAt,
		firstByteAt,
		code,
		amount,
		usage,
		false,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateSummaryUsageData(
	channelID int,
	modelName string,
	createAt time.Time,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if channelID <= 0 {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
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

	addSummaryUsageOnly(&summary.SummaryData, usage, amount, serviceTier, summaryClaudeLongContext)
}

func updateSummaryUsageDataMinute(
	channelID int,
	modelName string,
	createAt time.Time,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if channelID <= 0 {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
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

	addSummaryUsageOnly(&summary.SummaryData, usage, amount, serviceTier, summaryClaudeLongContext)
}

func updateGroupChannelSummaryUsageData(
	group string,
	groupChannelID int,
	modelName string,
	createAt time.Time,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" || groupChannelID <= 0 {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
	}

	unique := GroupChannelSummaryUnique{
		GroupID:        group,
		GroupChannelID: groupChannelID,
		Model:          modelName,
		HourTimestamp:  createAt.Truncate(time.Hour).Unix(),
	}

	summary, ok := batchData.GroupChannelSummaries[unique]
	if !ok {
		summary = &GroupChannelSummaryUpdate{GroupChannelSummaryUnique: unique}
		batchData.GroupChannelSummaries[unique] = summary
	}

	addSummaryUsageOnly(&summary.SummaryData, usage, amount, serviceTier, summaryClaudeLongContext)
}

func updateGroupChannelSummaryUsageDataMinute(
	group string,
	groupChannelID int,
	modelName string,
	createAt time.Time,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" || groupChannelID <= 0 {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
	}

	unique := GroupChannelSummaryMinuteUnique{
		GroupID:         group,
		GroupChannelID:  groupChannelID,
		Model:           modelName,
		MinuteTimestamp: createAt.Truncate(time.Minute).Unix(),
	}

	summary, ok := batchData.GroupChannelSummariesMinute[unique]
	if !ok {
		summary = &GroupChannelSummaryMinuteUpdate{GroupChannelSummaryMinuteUnique: unique}
		batchData.GroupChannelSummariesMinute[unique] = summary
	}

	addSummaryUsageOnly(&summary.SummaryData, usage, amount, serviceTier, summaryClaudeLongContext)
}

func updateGroupChannelTokenSummaryUsageData(
	group, tokenName, modelName string,
	createAt time.Time,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" || tokenName == "" {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
	}

	unique := GroupChannelTokenSummaryUnique{
		GroupID:       group,
		TokenName:     tokenName,
		Model:         modelName,
		HourTimestamp: createAt.Truncate(time.Hour).Unix(),
	}

	summary, ok := batchData.GroupChannelTokenSummaries[unique]
	if !ok {
		summary = &GroupChannelTokenSummaryUpdate{
			GroupChannelTokenSummaryUnique: unique,
		}
		batchData.GroupChannelTokenSummaries[unique] = summary
	}

	addSummaryUsageOnly(
		&summary.SummaryData,
		usage,
		amount,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateGroupChannelTokenSummaryUsageDataMinute(
	group, tokenName, modelName string,
	createAt time.Time,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" || tokenName == "" {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
	}

	unique := GroupChannelTokenSummaryMinuteUnique{
		GroupID:         group,
		TokenName:       tokenName,
		Model:           modelName,
		MinuteTimestamp: createAt.Truncate(time.Minute).Unix(),
	}

	summary, ok := batchData.GroupChannelTokenSummariesMinute[unique]
	if !ok {
		summary = &GroupChannelTokenSummaryMinuteUpdate{
			GroupChannelTokenSummaryMinuteUnique: unique,
		}
		batchData.GroupChannelTokenSummariesMinute[unique] = summary
	}

	addSummaryUsageOnly(
		&summary.SummaryData,
		usage,
		amount,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateGroupSummaryUsageData(
	group, tokenName, modelName string,
	createAt time.Time,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
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

	addSummaryUsageOnly(
		&groupSummary.SummaryData,
		usage,
		amount,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateGroupSummaryUsageDataMinute(
	group, tokenName, modelName string,
	createAt time.Time,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if group == "" {
		return
	}

	if createAt.IsZero() {
		createAt = time.Now()
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

	addSummaryUsageOnly(
		&groupSummary.SummaryData,
		usage,
		amount,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func addSummaryUsageOnly(
	summary *SummaryData,
	usage Usage,
	amount Amount,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	summary.Usage.Add(usage)
	summary.Amount.Add(amount)

	if usage.CachedTokens > 0 {
		summary.CacheHitCount++
	}

	if usage.CacheCreationTokens > 0 {
		summary.CacheCreationCount++
	}

	switch normalizeSummaryServiceTier(serviceTier) {
	case "flex":
		summary.ServiceTierFlex.Usage.Add(usage)
		summary.ServiceTierFlex.Amount.Add(amount)

		if usage.CachedTokens > 0 {
			summary.ServiceTierFlex.CacheHitCount++
		}

		if usage.CacheCreationTokens > 0 {
			summary.ServiceTierFlex.CacheCreationCount++
		}
	case "priority":
		summary.ServiceTierPriority.Usage.Add(usage)
		summary.ServiceTierPriority.Amount.Add(amount)

		if usage.CachedTokens > 0 {
			summary.ServiceTierPriority.CacheHitCount++
		}

		if usage.CacheCreationTokens > 0 {
			summary.ServiceTierPriority.CacheCreationCount++
		}
	}

	if summaryClaudeLongContext {
		summary.ClaudeLongContext.Usage.Add(usage)
		summary.ClaudeLongContext.Amount.Add(amount)

		if usage.CachedTokens > 0 {
			summary.ClaudeLongContext.CacheHitCount++
		}

		if usage.CacheCreationTokens > 0 {
			summary.ClaudeLongContext.CacheCreationCount++
		}
	}
}

func updateGroupSummaryDataMinute(
	group, tokenName, modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if createAt.IsZero() {
		createAt = time.Now()
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

	addRequestSummaryData(
		&groupSummary.SummaryData,
		createAt,
		requestAt,
		firstByteAt,
		code,
		amount,
		usage,
		false,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateSummaryData(
	channelID int,
	modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	isRetry bool,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if createAt.IsZero() {
		createAt = time.Now()
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

	addRequestSummaryData(
		&summary.SummaryData,
		createAt,
		requestAt,
		firstByteAt,
		code,
		amount,
		usage,
		isRetry,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func updateSummaryDataMinute(
	channelID int,
	modelName string,
	createAt time.Time,
	requestAt time.Time,
	firstByteAt time.Time,
	code int,
	amount Amount,
	usage Usage,
	isRetry bool,
	serviceTier string,
	summaryClaudeLongContext bool,
) {
	if createAt.IsZero() {
		createAt = time.Now()
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

	addRequestSummaryData(
		&summary.SummaryData,
		createAt,
		requestAt,
		firstByteAt,
		code,
		amount,
		usage,
		isRetry,
		serviceTier,
		summaryClaudeLongContext,
	)
}

func getSummaryLatencyMetrics(
	createAt, requestAt, firstByteAt time.Time,
) (totalTimeMilliseconds, totalTTFBMilliseconds int64) {
	return createAt.Sub(requestAt).Milliseconds(), firstByteAt.Sub(requestAt).Milliseconds()
}
