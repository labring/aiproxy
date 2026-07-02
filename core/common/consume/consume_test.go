package consume_test

import (
	"context"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

type trackingPostGroupConsumer struct {
	calls atomic.Int64
}

var _ balance.PostGroupConsumer = (*trackingPostGroupConsumer)(nil)

func (c *trackingPostGroupConsumer) PostGroupConsume(
	context.Context,
	string,
	float64,
) (float64, error) {
	c.calls.Add(1)
	return 0, nil
}

func TestCalculateAmount(t *testing.T) {
	tests := []struct {
		name        string
		code        int
		usage       model.Usage
		price       model.Price
		serviceTier string
		want        float64
	}{
		{
			name: "Per-Request Pricing (OK)",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			price: model.Price{
				PerRequestPrice: 2.5,
			},
			want: 2.5,
		},
		{
			name: "Per-Request Pricing (Non-OK)",
			code: http.StatusBadRequest,
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			price: model.Price{
				PerRequestPrice: 2.5,
			},
			want: 0,
		},
		{
			name: "Simple Pricing",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 2000,
			},
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
			},
			want: 0.005, // 0.001 * 1000/1000 + 0.002 * 2000/1000
		},
		{
			name: "Simple Pricing With Unit 1",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 2000,
			},
			price: model.Price{
				InputPrice:      0.001,
				InputPriceUnit:  1,
				OutputPrice:     0.002,
				OutputPriceUnit: 2,
			},
			want: 3, // 0.001 * 1000/1 + 0.002 * 2000/2
		},
		{
			name: "Images Pricing",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:      2000,
				ImageInputTokens: 1000,
				OutputTokens:     3000,
			},
			price: model.Price{
				InputPrice:      0.001,
				ImageInputPrice: 0.003,
				OutputPrice:     0.004,
			},
			want: 0.016, // 0.001 * (2000-1000)/1000 + 0.003 * 1000/1000 + 0.004 * 3000/1000
		},
		{
			name: "Image Output Pricing",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:       1000,
				OutputTokens:      3000,
				ImageOutputTokens: 1000,
			},
			price: model.Price{
				InputPrice:       0.001,
				OutputPrice:      0.004,
				ImageOutputPrice: 0.01,
			},
			want: 0.019, // 0.001 * 1000/1000 + 0.004 * (3000-1000)/1000 + 0.01 * 1000/1000
		},
		{
			name: "Audio Input and Output Pricing",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:       2000,
				AudioInputTokens:  500,
				OutputTokens:      3000,
				AudioOutputTokens: 1000,
			},
			price: model.Price{
				InputPrice:       0.001,
				AudioInputPrice:  0.003,
				OutputPrice:      0.004,
				AudioOutputPrice: 0.01,
			},
			want: 0.021, // text in 0.0015 + audio in 0.0015 + text out 0.008 + audio out 0.01
		},
		{
			name: "Video Input Pricing",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:      3000,
				ImageInputTokens: 500,
				AudioInputTokens: 600,
				VideoInputTokens: 1000,
				OutputTokens:     2000,
			},
			price: model.Price{
				InputPrice:      0.001,
				ImageInputPrice: 0.003,
				AudioInputPrice: 0.004,
				VideoInputPrice: 0.008,
				OutputPrice:     0.002,
			},
			want: 0.0168, // text in 0.0009 + image in 0.0015 + audio in 0.0024 + video in 0.008 + text out 0.004
		},
		{
			name: "Cached Token Pricing",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:         4000,
				CacheCreationTokens: 1000,
				CachedTokens:        2000,
			},
			price: model.Price{
				InputPrice:         0.01,
				CacheCreationPrice: 0.1,
				CachedPrice:        0.001,
			},
			want: 0.112, // 0.01 * (4000-1000-2000)/1000 + 0.1 * 1000/1000 + 0.001 * 2000/1000
		},
		{
			name: "Web Search Pricing",
			code: http.StatusOK,
			usage: model.Usage{
				WebSearchCount: 2,
			},
			price: model.Price{
				WebSearchPrice:     0.5,
				WebSearchPriceUnit: 1,
			},
			want: 1, // 0.5 * 2/1
		},
		{
			name: "Thinking Mode Output Pricing (ON)",
			code: http.StatusOK,
			usage: model.Usage{
				OutputTokens:    2000,
				ReasoningTokens: 1000,
			},
			price: model.Price{
				OutputPrice:             0.01,
				ThinkingModeOutputPrice: 0.03,
			},
			want: 0.06, // 0.03 * 2000/1000
		},
		{
			name: "Thinking Mode Output Pricing (OFF)",
			code: http.StatusOK,
			usage: model.Usage{
				OutputTokens: 2000,
			},
			price: model.Price{
				OutputPrice:             0.01,
				ThinkingModeOutputPrice: 0.03,
			},
			want: 0.02, // 0.01 * 2000/1000
		},
		{
			name: "Image Generation - With OutputTokensDetails (text + image output)",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:       1000, // total input (text 500 + image 500)
				ImageInputTokens:  500,
				OutputTokens:      2000, // total output (text 1000 + image 1000)
				ImageOutputTokens: 1000,
			},
			price: model.Price{
				InputPrice:       0.005, // $5 per 1M = $0.005 per 1K
				ImageInputPrice:  0.008, // $8 per 1M = $0.008 per 1K
				OutputPrice:      0.01,  // $10 per 1M = $0.01 per 1K
				ImageOutputPrice: 0.032, // $32 per 1M = $0.032 per 1K
			},
			// Text input: (1000 - 500) / 1000 * 0.005 = 0.0025
			// Image input: 500 / 1000 * 0.008 = 0.004
			// Text output: (2000 - 1000) / 1000 * 0.01 = 0.01
			// Image output: 1000 / 1000 * 0.032 = 0.032
			// Total: 0.0025 + 0.004 + 0.01 + 0.032 = 0.0485
			want: 0.0485,
		},
		{
			name: "Image Generation - Without OutputTokensDetails (all output is image)",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:       1000, // total input (text 500 + image 500)
				ImageInputTokens:  500,
				OutputTokens:      2000, // all image output
				ImageOutputTokens: 2000, // same as OutputTokens
			},
			price: model.Price{
				InputPrice:       0.005,
				ImageInputPrice:  0.008,
				OutputPrice:      0.01,
				ImageOutputPrice: 0.032,
			},
			// Text input: (1000 - 500) / 1000 * 0.005 = 0.0025
			// Image input: 500 / 1000 * 0.008 = 0.004
			// Text output: (2000 - 2000) / 1000 * 0.01 = 0
			// Image output: 2000 / 1000 * 0.032 = 0.064
			// Total: 0.0025 + 0.004 + 0 + 0.064 = 0.0705
			want: 0.0705,
		},
		{
			name: "Image Generation - Only image input and output",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:       1000, // all image input
				ImageInputTokens:  1000,
				OutputTokens:      1000, // all image output
				ImageOutputTokens: 1000,
			},
			price: model.Price{
				InputPrice:       0.005,
				ImageInputPrice:  0.008,
				OutputPrice:      0.01,
				ImageOutputPrice: 0.032,
			},
			// Text input: (1000 - 1000) / 1000 * 0.005 = 0
			// Image input: 1000 / 1000 * 0.008 = 0.008
			// Text output: (1000 - 1000) / 1000 * 0.01 = 0
			// Image output: 1000 / 1000 * 0.032 = 0.032
			// Total: 0 + 0.008 + 0 + 0.032 = 0.04
			want: 0.04,
		},
		{
			name: "Image Generation - Only text input with image output",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:       500, // all text input
				ImageInputTokens:  0,
				OutputTokens:      1000, // all image output
				ImageOutputTokens: 1000,
			},
			price: model.Price{
				InputPrice:       0.005,
				ImageInputPrice:  0.008,
				OutputPrice:      0.01,
				ImageOutputPrice: 0.032,
			},
			// Text input: 500 / 1000 * 0.005 = 0.0025
			// Image input: 0
			// Text output: (1000 - 1000) / 1000 * 0.01 = 0
			// Image output: 1000 / 1000 * 0.032 = 0.032
			// Total: 0.0025 + 0 + 0 + 0.032 = 0.0345
			want: 0.0345,
		},
	}

	for _, tt := range tests {
		got := consume.CalculateAmount(
			tt.code,
			tt.usage,
			model.UsageContext{ServiceTier: tt.serviceTier},
			tt.price,
		)
		if got != tt.want {
			t.Errorf("CalculateAmount()\n%s\n\tgot: %v\n\twant: %v\n\t", tt.name, got, tt.want)
		}
	}
}

func TestCalculateAmountWithConditionalPricing(t *testing.T) {
	tests := []struct {
		name        string
		code        int
		usage       model.Usage
		price       model.Price
		serviceTier string
		want        float64
	}{
		{
			name: "Conditional Pricing - Small Input/Output",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  20000, // 20k tokens
				OutputTokens: 100,   // 0.1k tokens
			},
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin:  0,
							InputTokenMax:  32000,
							OutputTokenMin: 0,
							OutputTokenMax: 200,
						},
						Price: model.Price{
							InputPrice:  0.0008, // 0.80 per million tokens
							OutputPrice: 0.002,  // 2.00 per million tokens
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin:  0,
							InputTokenMax:  32000,
							OutputTokenMin: 201,
							OutputTokenMax: 16000,
						},
						Price: model.Price{
							InputPrice:  0.0008, // 0.80 per million tokens
							OutputPrice: 0.008,  // 8.00 per million tokens
						},
					},
				},
			},
			want: 0.0162, // 0.0008 * 20000/1000 + 0.002 * 100/1000
		},
		{
			name: "Conditional Pricing - Medium Input",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  80000, // 80k tokens
				OutputTokens: 5000,  // 5k tokens
			},
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 32001,
							InputTokenMax: 128000,
						},
						Price: model.Price{
							InputPrice:  0.0012, // 1.20 per million tokens
							OutputPrice: 0.016,  // 16.00 per million tokens
						},
					},
				},
			},
			want: 0.176, // 0.0012 * 80000/1000 + 0.016 * 5000/1000
		},
		{
			name: "Conditional Pricing - Large Input",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  200000, // 200k tokens
				OutputTokens: 10000,  // 10k tokens
			},
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 128001,
							InputTokenMax: 256000,
						},
						Price: model.Price{
							InputPrice:  0.0024, // 2.40 per million tokens
							OutputPrice: 0.024,  // 24.00 per million tokens
						},
					},
				},
			},
			want: 0.72, // 0.0024 * 200000/1000 + 0.024 * 10000/1000
		},
		{
			name: "Conditional Pricing with Cache",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:         50000, // 50k tokens
				OutputTokens:        2000,  // 2k tokens
				CachedTokens:        10000, // 10k cached tokens
				CacheCreationTokens: 5000,  // 5k cache creation tokens
			},
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 32001,
							InputTokenMax: 128000,
						},
						Price: model.Price{
							InputPrice:         0.0012,   // 1.20 per million tokens
							OutputPrice:        0.016,    // 16.00 per million tokens
							CachedPrice:        0.00016,  // 0.16 per million tokens
							CacheCreationPrice: 0.000017, // 0.017 per million tokens per hour
						},
					},
				},
			},
			want: 0.075685, // 0.0012 * (50000-10000-5000)/1000 + 0.016 * 2000/1000 + 0.00016 * 10000/1000 + 0.000017 * 5000/1000
		},
		{
			name: "Conditional Pricing Thinking",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:     30000, // 30k tokens
				OutputTokens:    3000,  // 3k tokens
				ReasoningTokens: 1000,  // 1k reasoning tokens (triggers thinking mode)
			},
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
						},
						Price: model.Price{
							InputPrice:  0.0008, // 0.80 per million tokens
							OutputPrice: 0.008,  // 8.00 per million tokens (thinking mode)
						},
					},
				},
			},
			want: 0.048, // 0.0008 * 30000/1000 + 0.008 * 3000/1000
		},
		{
			name: "Fallback to Base Price",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  500000, // 500k tokens (exceeds all conditional ranges)
				OutputTokens: 1000,
			},
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMax: 256000,
						},
						Price: model.Price{
							InputPrice:  0.0024,
							OutputPrice: 0.024,
						},
					},
				},
			},
			want: 0.502, // 0.001 * 500000/1000 + 0.002 * 1000/1000 (uses base price)
		},
		{
			name: "Conditional Prices - No Fallback to Base Price",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  500000, // 500k tokens (exceeds all conditional ranges)
				OutputTokens: 1000,
			},
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 256000,
						},
						Price: model.Price{
							InputPrice:  0.0024,
							OutputPrice: 0.024,
						},
					},
				},
			},
			want: 1.224, // 0.0024 * 500000/1000 + 0.024 * 1000/1000
		},
		{
			name: "No Conditional Prices - Use Base Price",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				// No conditional prices defined
			},
			want: 0.002, // 0.001 * 1000/1000 + 0.002 * 500/1000
		},
		{
			name: "Conditional Prices - Service Tier Priority",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			serviceTier: "priority",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							ServiceTier: "priority",
						},
						Price: model.Price{
							InputPrice:  0.003,
							OutputPrice: 0.006,
						},
					},
				},
			},
			want: 0.006, // 0.003 * 1000/1000 + 0.006 * 500/1000
		},
		{
			name: "Conditional Prices - More Specific Service Tier Wins",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			serviceTier: "priority",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMax: 32000,
						},
						Price: model.Price{
							InputPrice:  0.001,
							OutputPrice: 0.002,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMax: 32000,
							ServiceTier:   "priority",
						},
						Price: model.Price{
							InputPrice:  0.003,
							OutputPrice: 0.006,
						},
					},
				},
			},
			want: 0.006, // 0.003 * 1000/1000 + 0.006 * 500/1000
		},
		{
			name: "Conditional Prices - Service Tier Priority Before Wildcard",
			code: http.StatusOK,
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			serviceTier: "priority",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMax: 32000,
							ServiceTier:   "priority",
						},
						Price: model.Price{
							InputPrice:  0.003,
							OutputPrice: 0.006,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMax: 32000,
						},
						Price: model.Price{
							InputPrice:  0.001,
							OutputPrice: 0.002,
						},
					},
				},
			},
			want: 0.006, // 0.003 * 1000/1000 + 0.006 * 500/1000
		},
	}

	for _, tt := range tests {
		got := consume.CalculateAmount(
			tt.code,
			tt.usage,
			model.UsageContext{ServiceTier: tt.serviceTier},
			tt.price,
		)
		if got != tt.want {
			t.Errorf("CalculateAmount()\n%s\n\tgot: %v\n\twant: %v\n\t", tt.name, got, tt.want)
		}
	}
}

func TestConsumePendingAsyncUsageDoesNotRecordPriceUsageOrAmount(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "pending_async_usage.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.Group{},
		&model.Token{},
		&model.Channel{},
		&model.Summary{},
		&model.SummaryMinute{},
		&model.GroupSummary{},
		&model.GroupSummaryMinute{},
	))

	oldLogDB := model.LogDB
	oldDB := model.DB
	model.LogDB = db
	model.DB = db
	t.Cleanup(func() {
		model.ProcessBatchUpdatesSummary()

		model.LogDB = oldLogDB
		model.DB = oldDB
	})

	requestMeta := &meta.Meta{
		RequestID:   "async_pending",
		RequestAt:   time.Now(),
		Group:       model.GroupCache{ID: "group"},
		Token:       model.TokenCache{ID: 1, Name: "token"},
		Channel:     meta.ChannelMeta{ID: 2},
		OriginModel: "video-model",
		Mode:        mode.VideoGenerationsJobs,
	}

	price := model.Price{
		OutputPrice:     0.1,
		OutputPriceUnit: 1,
		ConditionalPrices: []model.ConditionalPrice{
			{
				Condition: model.PriceCondition{Resolution: []string{"720p"}},
				Price: model.Price{
					OutputPrice:     0.4,
					OutputPriceUnit: 1,
				},
			},
		},
	}
	usage := model.Usage{
		OutputTokens: 5,
		TotalTokens:  5,
	}
	usageContext := model.UsageContext{
		Resolution: "720p",
	}

	consume.Consume(
		context.Background(),
		time.Now(),
		nil,
		time.Now(),
		http.StatusOK,
		requestMeta,
		usage,
		usageContext,
		price,
		"",
		"127.0.0.1",
		0,
		nil,
		true,
		nil,
		"upstream-id",
		model.AsyncUsageStatusPending,
	)

	var logEntry model.Log
	require.NoError(t, db.Where("request_id = ?", requestMeta.RequestID).First(&logEntry).Error)
	require.Equal(t, model.AsyncUsageStatusPending, logEntry.AsyncUsageStatus)
	require.Equal(t, model.ZeroNullInt64(0), logEntry.Usage.OutputTokens)
	require.Zero(t, logEntry.Amount.UsedAmount)
	require.Zero(t, logEntry.Price.OutputPrice)
	require.Empty(t, logEntry.Price.ConditionalPrices)
}

func TestConsumeGroupChannelRecordsAmountWithoutPostConsume(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "group_channel_consume.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Group{},
		&model.Token{},
		&model.GroupChannel{},
		&model.GroupChannelLog{},
		&model.GroupChannelSummary{},
		&model.GroupChannelSummaryMinute{},
		&model.GroupChannelTokenSummary{},
		&model.GroupChannelTokenSummaryMinute{},
	))

	oldLogDB := model.LogDB
	oldDB := model.DB
	model.LogDB = db
	model.DB = db
	t.Cleanup(func() {
		model.ProcessBatchUpdatesSummary()

		model.LogDB = oldLogDB
		model.DB = oldDB
	})

	require.NoError(t, db.Create(&model.Group{ID: "group"}).Error)
	require.NoError(t, db.Create(&model.Token{ID: 1, Name: "token", GroupID: "group"}).Error)
	require.NoError(t, db.Create(&model.GroupChannel{
		ID:      2,
		GroupID: "group",
		Type:    model.ChannelTypeOpenAI,
	}).Error)

	requestMeta := &meta.Meta{
		RequestID:   "group_channel_async_ignored",
		RequestAt:   time.Now(),
		Group:       model.GroupCache{ID: "group"},
		Token:       model.TokenCache{ID: 1, Name: "token"},
		Channel:     meta.ChannelMeta{ID: 2, Scope: model.ChannelScopeGroup, GroupID: "group"},
		OriginModel: "video-model",
		Mode:        mode.VideoGenerationsJobs,
	}
	consumer := &trackingPostGroupConsumer{}

	consume.Consume(
		context.Background(),
		time.Now(),
		consumer,
		time.Now(),
		http.StatusOK,
		requestMeta,
		model.Usage{OutputTokens: 5, TotalTokens: 5},
		model.UsageContext{},
		model.Price{OutputPrice: 1, OutputPriceUnit: 1},
		"",
		"127.0.0.1",
		0,
		nil,
		true,
		nil,
		"upstream-id",
		model.AsyncUsageStatusPending,
	)
	model.ProcessBatchUpdatesSummary()

	var logEntry model.GroupChannelLog
	require.NoError(t, db.Where("request_id = ?", requestMeta.RequestID).First(&logEntry).Error)
	require.Equal(t, model.AsyncUsageStatusNone, logEntry.AsyncUsageStatus)
	require.Equal(t, model.ZeroNullInt64(5), logEntry.Usage.OutputTokens)
	require.Equal(t, 5.0, logEntry.Amount.UsedAmount)
	require.Equal(t, float64(1), float64(logEntry.Price.OutputPrice))
	require.Empty(t, logEntry.Price.ConditionalPrices)
	require.Zero(t, consumer.calls.Load())
}

func TestSummaryGroupChannelRecordsScopedSummaryWithAmount(t *testing.T) {
	model.ProcessBatchUpdatesSummary()

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "group_channel_summary.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Group{},
		&model.Token{},
		&model.GroupChannel{},
		&model.GroupSummary{},
		&model.GroupSummaryMinute{},
		&model.GroupChannelSummary{},
		&model.GroupChannelSummaryMinute{},
		&model.GroupChannelTokenSummary{},
		&model.GroupChannelTokenSummaryMinute{},
	))

	oldLogDB := model.LogDB
	oldDB := model.DB
	model.LogDB = db
	model.DB = db
	t.Cleanup(func() {
		model.ProcessBatchUpdatesSummary()

		model.LogDB = oldLogDB
		model.DB = oldDB
	})

	require.NoError(t, db.Create(&model.Group{ID: "group"}).Error)
	require.NoError(t, db.Create(&model.Token{ID: 11, Name: "token", GroupID: "group"}).Error)
	require.NoError(t, db.Create(&model.GroupChannel{
		ID:      2,
		GroupID: "group",
		Type:    model.ChannelTypeOpenAI,
	}).Error)

	now := time.Now().Truncate(time.Minute)
	requestMeta := &meta.Meta{
		RequestID:   "group_channel_summary",
		RequestAt:   now,
		Group:       model.GroupCache{ID: "group"},
		Token:       model.TokenCache{ID: 11, Name: "token"},
		Channel:     meta.ChannelMeta{ID: 2, Scope: model.ChannelScopeGroup, GroupID: "group"},
		OriginModel: "gpt-5",
		Mode:        mode.ChatCompletions,
		ModelConfig: model.ModelConfig{Model: "gpt-5"},
	}

	consume.Summary(
		http.StatusOK,
		now,
		requestMeta,
		model.Usage{InputTokens: 5, OutputTokens: 7, TotalTokens: 12},
		model.UsageContext{},
		model.Price{InputPrice: 1, InputPriceUnit: 1, OutputPrice: 1, OutputPriceUnit: 1},
		true,
	)
	model.ProcessBatchUpdatesSummary()

	var groupSummaryCount int64
	require.NoError(t, db.Model(&model.GroupSummary{}).Count(&groupSummaryCount).Error)
	require.Zero(t, groupSummaryCount)

	var gotChannelSummary model.GroupChannelSummary
	require.NoError(t, db.Where(
		"group_id = ? AND group_channel_id = ? AND model = ?",
		"group",
		2,
		"gpt-5",
	).First(&gotChannelSummary).Error)
	require.Equal(t, int64(1), gotChannelSummary.Data.RequestCount)
	require.Equal(t, model.ZeroNullInt64(12), gotChannelSummary.Data.TotalTokens)
	require.Equal(t, 12.0, gotChannelSummary.Data.UsedAmount)

	var gotTokenSummary model.GroupChannelTokenSummary
	require.NoError(t, db.Where(
		"group_id = ? AND token_name = ? AND model = ?",
		"group",
		"token",
		"gpt-5",
	).First(&gotTokenSummary).Error)
	require.Equal(t, int64(1), gotTokenSummary.Data.RequestCount)
	require.Equal(t, model.ZeroNullInt64(12), gotTokenSummary.Data.TotalTokens)
	require.Equal(t, 12.0, gotTokenSummary.Data.UsedAmount)

	var gotTokenSummaryMinute model.GroupChannelTokenSummaryMinute
	require.NoError(t, db.Where(
		"group_id = ? AND token_name = ? AND model = ?",
		"group",
		"token",
		"gpt-5",
	).First(&gotTokenSummaryMinute).Error)
	require.Equal(t, int64(1), gotTokenSummaryMinute.Data.RequestCount)
	require.Equal(t, model.ZeroNullInt64(12), gotTokenSummaryMinute.Data.TotalTokens)
	require.Equal(t, 12.0, gotTokenSummaryMinute.Data.UsedAmount)

	var gotGroup model.Group
	require.NoError(t, db.First(&gotGroup, "id = ?", "group").Error)
	require.Zero(t, gotGroup.UsedAmount)
	require.Zero(t, gotGroup.RequestCount)
	require.Equal(t, 12.0, gotGroup.GroupChannelUsedAmount)
	require.Equal(t, 1, gotGroup.GroupChannelRequestCount)

	var gotToken model.Token
	require.NoError(t, db.First(&gotToken, "id = ?", 11).Error)
	require.Zero(t, gotToken.UsedAmount)
	require.Zero(t, gotToken.RequestCount)
	require.Equal(t, 12.0, gotToken.GroupChannelUsedAmount)
	require.Equal(t, 1, gotToken.GroupChannelRequestCount)

	var gotGroupChannel model.GroupChannel
	require.NoError(t, db.First(&gotGroupChannel, "group_id = ? AND id = ?", "group", 2).Error)
	require.Equal(t, 12.0, gotGroupChannel.UsedAmount)
	require.Equal(t, 1, gotGroupChannel.RequestCount)
}

func TestConsumeAdminGroupChannelRecordsLogAndSummaryWithoutTokenSummary(t *testing.T) {
	model.ProcessBatchUpdatesSummary()

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "group_channel_admin.db"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Group{},
		&model.Token{},
		&model.GroupChannel{},
		&model.GroupChannelLog{},
		&model.GroupChannelSummary{},
		&model.GroupChannelSummaryMinute{},
		&model.GroupChannelTokenSummary{},
		&model.GroupChannelTokenSummaryMinute{},
	))

	oldLogDB := model.LogDB
	oldDB := model.DB
	model.LogDB = db
	model.DB = db
	t.Cleanup(func() {
		model.ProcessBatchUpdatesSummary()

		model.LogDB = oldLogDB
		model.DB = oldDB
	})

	require.NoError(t, db.Create(&model.Group{ID: "group"}).Error)
	require.NoError(t, db.Create(&model.Token{ID: 11, Name: "token", GroupID: "group"}).Error)
	require.NoError(t, db.Create(&model.GroupChannel{
		ID:      2,
		GroupID: "group",
		Type:    model.ChannelTypeOpenAI,
	}).Error)

	now := time.Now().Truncate(time.Minute)
	requestMeta := &meta.Meta{
		RequestID:   "admin_group_channel",
		RequestAt:   now,
		Group:       model.GroupCache{ID: "group"},
		Token:       model.TokenCache{},
		Channel:     meta.ChannelMeta{ID: 2, Scope: model.ChannelScopeGroup, GroupID: "group"},
		OriginModel: "gpt-5",
		Mode:        mode.ChatCompletions,
		ModelConfig: model.ModelConfig{Model: "gpt-5"},
	}

	consume.Consume(
		context.Background(),
		now,
		nil,
		now,
		http.StatusOK,
		requestMeta,
		model.Usage{InputTokens: 5, OutputTokens: 7, TotalTokens: 12},
		model.UsageContext{},
		model.Price{InputPrice: 1, InputPriceUnit: 1, OutputPrice: 1, OutputPriceUnit: 1},
		"",
		"127.0.0.1",
		0,
		nil,
		true,
		nil,
		"upstream-id",
		model.AsyncUsageStatusNone,
	)
	model.ProcessBatchUpdatesSummary()

	var logEntry model.GroupChannelLog
	require.NoError(t, db.Where("request_id = ?", requestMeta.RequestID).First(&logEntry).Error)
	require.Zero(t, logEntry.TokenID)
	require.Empty(t, logEntry.TokenName)
	require.Equal(t, "group", logEntry.GroupID)
	require.Equal(t, 2, logEntry.GroupChannelID)
	require.Equal(t, 12.0, logEntry.Amount.UsedAmount)

	var gotChannelSummary model.GroupChannelSummary
	require.NoError(t, db.Where(
		"group_id = ? AND group_channel_id = ? AND model = ?",
		"group",
		2,
		"gpt-5",
	).First(&gotChannelSummary).Error)
	require.Equal(t, int64(1), gotChannelSummary.Data.RequestCount)
	require.Equal(t, model.ZeroNullInt64(12), gotChannelSummary.Data.TotalTokens)
	require.Equal(t, 12.0, gotChannelSummary.Data.UsedAmount)

	var tokenSummaryCount int64
	require.NoError(t, db.Model(&model.GroupChannelTokenSummary{}).Count(&tokenSummaryCount).Error)
	require.Zero(t, tokenSummaryCount)

	var tokenSummaryMinuteCount int64
	require.NoError(t, db.Model(&model.GroupChannelTokenSummaryMinute{}).
		Count(&tokenSummaryMinuteCount).Error)
	require.Zero(t, tokenSummaryMinuteCount)

	var gotGroup model.Group
	require.NoError(t, db.First(&gotGroup, "id = ?", "group").Error)
	require.Zero(t, gotGroup.UsedAmount)
	require.Zero(t, gotGroup.RequestCount)
	require.Zero(t, gotGroup.GroupChannelUsedAmount)
	require.Zero(t, gotGroup.GroupChannelRequestCount)

	var gotToken model.Token
	require.NoError(t, db.First(&gotToken, "id = ?", 11).Error)
	require.Zero(t, gotToken.UsedAmount)
	require.Zero(t, gotToken.RequestCount)
	require.Zero(t, gotToken.GroupChannelUsedAmount)
	require.Zero(t, gotToken.GroupChannelRequestCount)

	var gotGroupChannel model.GroupChannel
	require.NoError(t, db.First(&gotGroupChannel, "group_id = ? AND id = ?", "group", 2).Error)
	require.Equal(t, 12.0, gotGroupChannel.UsedAmount)
	require.Equal(t, 1, gotGroupChannel.RequestCount)
}
