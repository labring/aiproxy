package consume_test

import (
	"net/http"
	"testing"

	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/model"
)

func TestCalculateAmount(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		usage model.Usage
		price model.Price
		want  float64
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
	}

	for _, tt := range tests {
		got := consume.CalculateAmount(tt.code, tt.usage, tt.price)
		if got != tt.want {
			t.Errorf("CalculateAmount()\n%s\n\tgot: %v\n\twant: %v\n\t", tt.name, got, tt.want)
		}
	}
}

func TestCalculateAmountWithConditionalPricing(t *testing.T) {
	tests := []struct {
		name  string
		code  int
		usage model.Usage
		price model.Price
		want  float64
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
	}

	for _, tt := range tests {
		got := consume.CalculateAmount(tt.code, tt.usage, tt.price)
		if got != tt.want {
			t.Errorf("CalculateAmount()\n%s\n\tgot: %v\n\twant: %v\n\t", tt.name, got, tt.want)
		}
	}
}
