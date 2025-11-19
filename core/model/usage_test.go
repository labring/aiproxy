package model_test

import (
	"testing"
	"time"

	"github.com/labring/aiproxy/core/model"
)

func TestPrice_ValidateConditionalPrices(t *testing.T) {
	tests := []struct {
		name    string
		price   model.Price
		wantErr bool
	}{
		{
			name: "Empty conditional prices",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{},
			},
			wantErr: false,
		},
		{
			name: "Nil conditional prices",
			price: model.Price{
				ConditionalPrices: nil,
			},
			wantErr: false,
		},
		{
			name: "Valid single condition",
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
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid multiple conditions - doubao-seed-1.6 example",
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
							InputPrice:  0.0008,
							OutputPrice: 0.002,
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
							InputPrice:  0.0008,
							OutputPrice: 0.008,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin: 32001,
							InputTokenMax: 128000,
						},
						Price: model.Price{
							InputPrice:  0.0012,
							OutputPrice: 0.016,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin: 128001,
							InputTokenMax: 256000,
						},
						Price: model.Price{
							InputPrice:  0.0024,
							OutputPrice: 0.024,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid input token range - min > max",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 32000,
							InputTokenMax: 1000, // min > max
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid output token range - min > max",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin:  0,
							InputTokenMax:  32000,
							OutputTokenMin: 1000,
							OutputTokenMax: 500, // min > max
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Overlapping input ranges with overlapping output ranges",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin:  0,
							InputTokenMax:  32000,
							OutputTokenMin: 0,
							OutputTokenMax: 500,
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin:  20000, // overlaps with previous
							InputTokenMax:  50000,
							OutputTokenMin: 200, // overlaps with previous
							OutputTokenMax: 1000,
						},
						Price: model.Price{
							InputPrice:  0.0012,
							OutputPrice: 0.008,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Overlapping input ranges but non-overlapping output ranges (valid)",
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
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin:  0,     // same input range
							InputTokenMax:  32000, // same input range
							OutputTokenMin: 201,   // non-overlapping output range
							OutputTokenMax: 16000,
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.008,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Improperly ordered conditions",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 32001,
							InputTokenMax: 128000,
						},
						Price: model.Price{
							InputPrice:  0.0012,
							OutputPrice: 0.016,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000, // should come before the previous one
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Valid consecutive ranges",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin: 32001, // consecutive with previous
							InputTokenMax: 128000,
						},
						Price: model.Price{
							InputPrice:  0.0012,
							OutputPrice: 0.016,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Gap between ranges (valid)",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin: 50000, // gap between 32000 and 50000
							InputTokenMax: 128000,
						},
						Price: model.Price{
							InputPrice:  0.0012,
							OutputPrice: 0.016,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Unbounded ranges (zero values)",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0, // unbounded min
							InputTokenMax: 0, // unbounded max
						},
						Price: model.Price{
							InputPrice:  0.001,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Mixed bounded and unbounded ranges",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin: 32001,
							InputTokenMax: 0, // unbounded max
						},
						Price: model.Price{
							InputPrice:  0.0012,
							OutputPrice: 0.016,
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.price.ValidateConditionalPrices()

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: ValidateConditionalPrices() expected error but got nil", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: ValidateConditionalPrices() unexpected error = %v", tt.name, err)
			}
		})
	}
}

func TestPrice_ValidateConditionalPrices_WithTime(t *testing.T) {
	now := time.Now().Unix()
	future := now + 3600 // 1 hour from now
	past := now - 3600   // 1 hour ago

	tests := []struct {
		name    string
		price   model.Price
		wantErr bool
	}{
		{
			name: "Valid time range - future time window",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     now,
							EndTime:       future,
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid time range - no end time",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     now,
							EndTime:       0, // no end limit
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid time range - no start time",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     0, // no start limit
							EndTime:       future,
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid time range - start time >= end time",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     future,
							EndTime:       now, // end before start
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid time range - start time equals end time",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     now,
							EndTime:       now, // same time
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Multiple conditions with different time ranges",
			price: model.Price{
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     past,
							EndTime:       now,
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.002,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     now,
							EndTime:       future,
						},
						Price: model.Price{
							InputPrice:  0.001,
							OutputPrice: 0.003,
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.price.ValidateConditionalPrices()

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: ValidateConditionalPrices() expected error but got nil", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: ValidateConditionalPrices() unexpected error = %v", tt.name, err)
			}
		})
	}
}

func TestPrice_SelectConditionalPrice_WithTime(t *testing.T) {
	now := time.Now().Unix()
	past := now - 3600      // 1 hour ago
	future := now + 3600    // 1 hour from now
	farFuture := now + 7200 // 2 hours from now

	tests := []struct {
		name           string
		price          model.Price
		usage          model.Usage
		expectedInput  float64
		expectedOutput float64
	}{
		{
			name: "Select price within active time range",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     past,
							EndTime:       future,
						},
						Price: model.Price{
							InputPrice:  0.0005,
							OutputPrice: 0.001,
						},
					},
				},
			},
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedInput:  0.0005,
			expectedOutput: 0.001,
		},
		{
			name: "Fallback to default price when time range not active (before start)",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     future,
							EndTime:       farFuture,
						},
						Price: model.Price{
							InputPrice:  0.0005,
							OutputPrice: 0.001,
						},
					},
				},
			},
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedInput:  0.001,
			expectedOutput: 0.002,
		},
		{
			name: "Fallback to default price when time range expired",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     past - 7200, // 3 hours ago
							EndTime:       past,        // 1 hour ago
						},
						Price: model.Price{
							InputPrice:  0.0005,
							OutputPrice: 0.001,
						},
					},
				},
			},
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedInput:  0.001,
			expectedOutput: 0.002,
		},
		{
			name: "Select first matching price with multiple time-based conditions",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     past,
							EndTime:       future,
						},
						Price: model.Price{
							InputPrice:  0.0005,
							OutputPrice: 0.001,
						},
					},
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     past,
							EndTime:       farFuture, // broader time range
						},
						Price: model.Price{
							InputPrice:  0.0008,
							OutputPrice: 0.0015,
						},
					},
				},
			},
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedInput:  0.0005,
			expectedOutput: 0.001,
		},
		{
			name: "Time range with no end time (ongoing promotion)",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     past,
							EndTime:       0, // no end time
						},
						Price: model.Price{
							InputPrice:  0.0005,
							OutputPrice: 0.001,
						},
					},
				},
			},
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedInput:  0.0005,
			expectedOutput: 0.001,
		},
		{
			name: "Time range with no start time (promotion until end)",
			price: model.Price{
				InputPrice:  0.001,
				OutputPrice: 0.002,
				ConditionalPrices: []model.ConditionalPrice{
					{
						Condition: model.PriceCondition{
							InputTokenMin: 0,
							InputTokenMax: 32000,
							StartTime:     0, // no start time
							EndTime:       future,
						},
						Price: model.Price{
							InputPrice:  0.0005,
							OutputPrice: 0.001,
						},
					},
				},
			},
			usage: model.Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			expectedInput:  0.0005,
			expectedOutput: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selectedPrice := tt.price.SelectConditionalPrice(tt.usage)

			if float64(selectedPrice.InputPrice) != tt.expectedInput {
				t.Errorf("%s: expected input price %v, got %v",
					tt.name, tt.expectedInput, float64(selectedPrice.InputPrice))
			}

			if float64(selectedPrice.OutputPrice) != tt.expectedOutput {
				t.Errorf("%s: expected output price %v, got %v",
					tt.name, tt.expectedOutput, float64(selectedPrice.OutputPrice))
			}
		})
	}
}
