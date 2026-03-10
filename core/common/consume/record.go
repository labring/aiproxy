package consume

import (
	"time"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
)

func recordConsume(
	now time.Time,
	meta *meta.Meta,
	code int,
	firstByteAt time.Time,
	usage model.Usage,
	modelPrice model.Price,
	content string,
	ip string,
	requestDetail *model.RequestDetail,
	amount model.Amount,
	retryTimes int,
	downstreamResult bool,
	user string,
	metadata map[string]string,
	upstreamID string,
	serviceTier string,
) error {
	summaryServiceTier := serviceTier
	if !meta.ModelConfig.ShouldSummaryServiceTier() {
		summaryServiceTier = ""
	}

	summaryClaudeLongContext := meta.ModelConfig.ShouldSummaryClaudeLongContext() &&
		model.IsClaudeLongContextSummary(meta.OriginModel, usage)

	return model.BatchRecordLogs(
		now,
		meta.RequestID,
		meta.RequestAt,
		meta.RetryAt,
		firstByteAt,
		meta.Group.ID,
		code,
		meta.Channel.ID,
		meta.OriginModel,
		meta.Token.ID,
		meta.Token.Name,
		meta.Endpoint,
		content,
		int(meta.Mode),
		ip,
		retryTimes,
		requestDetail,
		downstreamResult,
		usage,
		modelPrice,
		amount,
		user,
		metadata,
		upstreamID,
		serviceTier,
		summaryServiceTier,
		summaryClaudeLongContext,
	)
}

func recordSummary(
	now time.Time,
	meta *meta.Meta,
	code int,
	firstByteAt time.Time,
	usage model.Usage,
	amount model.Amount,
	downstreamResult bool,
	serviceTier string,
) {
	if !meta.ModelConfig.ShouldSummaryServiceTier() {
		serviceTier = ""
	}

	summaryClaudeLongContext := meta.ModelConfig.ShouldSummaryClaudeLongContext() &&
		model.IsClaudeLongContextSummary(meta.OriginModel, usage)

	model.BatchUpdateSummary(
		now,
		meta.RequestAt,
		firstByteAt,
		meta.Group.ID,
		code,
		meta.Channel.ID,
		meta.OriginModel,
		meta.Token.ID,
		meta.Token.Name,
		downstreamResult,
		usage,
		amount,
		serviceTier,
		summaryClaudeLongContext,
	)
}
