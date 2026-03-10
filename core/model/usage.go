package model

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type PriceCondition struct {
	InputTokenMin  int64  `json:"input_token_min,omitempty"`
	InputTokenMax  int64  `json:"input_token_max,omitempty"`
	OutputTokenMin int64  `json:"output_token_min,omitempty"`
	OutputTokenMax int64  `json:"output_token_max,omitempty"`
	StartTime      int64  `json:"start_time,omitempty"` // Unix timestamp, 0 means no start limit
	EndTime        int64  `json:"end_time,omitempty"`   // Unix timestamp, 0 means no end limit
	ServiceTier    string `json:"service_tier,omitempty"`
}

type ConditionalPrice struct {
	Condition PriceCondition `json:"condition"`
	Price     Price          `json:"price"`
}

type Price struct {
	PerRequestPrice ZeroNullFloat64 `json:"per_request_price,omitempty"`

	InputPrice     ZeroNullFloat64 `json:"input_price,omitempty"`
	InputPriceUnit ZeroNullInt64   `json:"input_price_unit,omitempty"`

	ImageInputPrice     ZeroNullFloat64 `json:"image_input_price,omitempty"`
	ImageInputPriceUnit ZeroNullInt64   `json:"image_input_price_unit,omitempty"`

	AudioInputPrice     ZeroNullFloat64 `json:"audio_input_price,omitempty"`
	AudioInputPriceUnit ZeroNullInt64   `json:"audio_input_price_unit,omitempty"`

	OutputPrice     ZeroNullFloat64 `json:"output_price,omitempty"`
	OutputPriceUnit ZeroNullInt64   `json:"output_price_unit,omitempty"`

	ImageOutputPrice     ZeroNullFloat64 `json:"image_output_price,omitempty"`
	ImageOutputPriceUnit ZeroNullInt64   `json:"image_output_price_unit,omitempty"`

	// when ThinkingModeOutputPrice and ReasoningTokens are not 0, OutputPrice and OutputPriceUnit
	// will be overwritten
	ThinkingModeOutputPrice     ZeroNullFloat64 `json:"thinking_mode_output_price,omitempty"`
	ThinkingModeOutputPriceUnit ZeroNullInt64   `json:"thinking_mode_output_price_unit,omitempty"`

	CachedPrice     ZeroNullFloat64 `json:"cached_price,omitempty"`
	CachedPriceUnit ZeroNullInt64   `json:"cached_price_unit,omitempty"`

	CacheCreationPrice     ZeroNullFloat64 `json:"cache_creation_price,omitempty"`
	CacheCreationPriceUnit ZeroNullInt64   `json:"cache_creation_price_unit,omitempty"`

	WebSearchPrice     ZeroNullFloat64 `json:"web_search_price,omitempty"`
	WebSearchPriceUnit ZeroNullInt64   `json:"web_search_price_unit,omitempty"`

	ConditionalPrices []ConditionalPrice `gorm:"serializer:fastjson;type:text" json:"conditional_prices,omitempty"`
}

func normalizeServiceTier(serviceTier string) string {
	return strings.ToLower(strings.TrimSpace(serviceTier))
}

func isAllowedServiceTier(serviceTier string) bool {
	switch normalizeServiceTier(serviceTier) {
	case "", "auto", "default", "flex", "scale", "priority":
		return true
	default:
		return false
	}
}

func serviceTierOverlap(serviceTier1, serviceTier2 string) bool {
	normalized1 := normalizeServiceTier(serviceTier1)
	normalized2 := normalizeServiceTier(serviceTier2)

	// Empty means wildcard (applies to any tier).
	if normalized1 == "" || normalized2 == "" {
		return true
	}

	return normalized1 == normalized2
}

func (p *Price) ValidateConditionalPrices() error {
	if len(p.ConditionalPrices) == 0 {
		return nil
	}

	for i, conditionalPrice := range p.ConditionalPrices {
		condition := conditionalPrice.Condition

		if !isAllowedServiceTier(condition.ServiceTier) {
			return fmt.Errorf(
				"conditional price %d: invalid service tier %q (allowed: auto, default, flex, scale, priority)",
				i,
				condition.ServiceTier,
			)
		}

		// Validate individual condition ranges
		if condition.InputTokenMin > 0 && condition.InputTokenMax > 0 {
			if condition.InputTokenMin > condition.InputTokenMax {
				return fmt.Errorf(
					"conditional price %d: input token min (%d) cannot be greater than max (%d)",
					i,
					condition.InputTokenMin,
					condition.InputTokenMax,
				)
			}
		}

		if condition.OutputTokenMin > 0 && condition.OutputTokenMax > 0 {
			if condition.OutputTokenMin > condition.OutputTokenMax {
				return fmt.Errorf(
					"conditional price %d: output token min (%d) cannot be greater than max (%d)",
					i,
					condition.OutputTokenMin,
					condition.OutputTokenMax,
				)
			}
		}

		// Validate time range
		if condition.StartTime > 0 && condition.EndTime > 0 {
			if condition.StartTime >= condition.EndTime {
				return fmt.Errorf(
					"conditional price %d: start time (%d) must be before end time (%d)",
					i,
					condition.StartTime,
					condition.EndTime,
				)
			}
		}

		// Check for overlaps with other conditions
		for j := i + 1; j < len(p.ConditionalPrices); j++ {
			otherCondition := p.ConditionalPrices[j].Condition
			if !serviceTierOverlap(condition.ServiceTier, otherCondition.ServiceTier) {
				continue
			}

			// Check input token range overlap
			if hasRangeOverlap(
				condition.InputTokenMin, condition.InputTokenMax,
				otherCondition.InputTokenMin, otherCondition.InputTokenMax,
			) {
				// If input ranges overlap, check if output ranges also overlap
				if hasRangeOverlap(
					condition.OutputTokenMin, condition.OutputTokenMax,
					otherCondition.OutputTokenMin, otherCondition.OutputTokenMax,
				) {
					// If both token ranges overlap, check if time ranges also overlap
					// If time ranges don't overlap, conditions are still valid
					if hasTimeRangeOverlap(
						condition.StartTime, condition.EndTime,
						otherCondition.StartTime, otherCondition.EndTime,
					) {
						return fmt.Errorf(
							"conditional prices %d and %d have overlapping conditions",
							i,
							j,
						)
					}
				}
			}
		}
	}

	// Check if conditions are sorted by input token ranges (optional ordering check)
	if err := p.validateConditionalPriceOrdering(); err != nil {
		return err
	}

	return nil
}

// hasRangeOverlap checks if two ranges overlap
// Range is defined by [min, max], where 0 means unbounded
func hasRangeOverlap(min1, max1, min2, max2 int64) bool {
	// Convert 0 to appropriate bounds for comparison
	actualMin1 := min1
	actualMax1 := max1
	actualMin2 := min2
	actualMax2 := max2

	if actualMin1 == 0 {
		actualMin1 = 0
	}

	if actualMax1 == 0 {
		actualMax1 = math.MaxInt64
	}

	if actualMin2 == 0 {
		actualMin2 = 0
	}

	if actualMax2 == 0 {
		actualMax2 = math.MaxInt64
	}

	// Check if ranges overlap: range1.max >= range2.min && range1.min <= range2.max
	return actualMax1 >= actualMin2 && actualMin1 <= actualMax2
}

// hasTimeRangeOverlap checks if two time ranges overlap
// Unlike hasRangeOverlap, this uses strict inequality to allow adjacent time ranges
// Time range is defined by [start, end], where 0 means unbounded
func hasTimeRangeOverlap(start1, end1, start2, end2 int64) bool {
	// Convert 0 to appropriate bounds for comparison
	actualStart1 := start1
	actualEnd1 := end1
	actualStart2 := start2
	actualEnd2 := end2

	if actualStart1 == 0 {
		actualStart1 = 0
	}

	if actualEnd1 == 0 {
		actualEnd1 = math.MaxInt64
	}

	if actualStart2 == 0 {
		actualStart2 = 0
	}

	if actualEnd2 == 0 {
		actualEnd2 = math.MaxInt64
	}

	// Check if ranges overlap with strict inequality: range1.end > range2.start && range1.start < range2.end
	// This allows adjacent ranges like [t1, t2] and [t2, t3] to be considered non-overlapping
	return actualEnd1 > actualStart2 && actualStart1 < actualEnd2
}

// validateConditionalPriceOrdering checks if conditional prices are properly ordered
func (p *Price) validateConditionalPriceOrdering() error {
	if len(p.ConditionalPrices) <= 1 {
		return nil
	}

	for i := range len(p.ConditionalPrices) - 1 {
		current := p.ConditionalPrices[i].Condition

		next := p.ConditionalPrices[i+1].Condition
		if !serviceTierOverlap(current.ServiceTier, next.ServiceTier) {
			continue
		}

		// Check if input token ranges are in ascending order
		// Compare the starting points of ranges
		currentInputMin := current.InputTokenMin
		nextInputMin := next.InputTokenMin

		// If current range starts after next range, it's improperly ordered
		if currentInputMin > nextInputMin {
			return fmt.Errorf("conditional prices %d and %d are not properly ordered: "+
				"current min (%d) should not be greater than next min (%d)",
				i, i+1, currentInputMin, nextInputMin)
		}

		// If they have the same starting point, check the ending points
		if currentInputMin == nextInputMin {
			currentInputMax := current.InputTokenMax
			nextInputMax := next.InputTokenMax

			// Handle unbounded ranges (0 means unbounded)
			if currentInputMax == 0 {
				currentInputMax = math.MaxInt64
			}

			if nextInputMax == 0 {
				nextInputMax = math.MaxInt64
			}

			if currentInputMax > nextInputMax {
				return fmt.Errorf("conditional prices %d and %d are not properly ordered: "+
					"ranges with same min should be ordered by max",
					i, i+1)
			}
		}
	}

	return nil
}

func (p *Price) SelectConditionalPrice(usage Usage) Price {
	if len(p.ConditionalPrices) == 0 {
		return *p
	}

	inputTokens := int64(usage.InputTokens)
	outputTokens := int64(usage.OutputTokens)
	usageServiceTier := normalizeServiceTier(usage.ServiceTier)
	currentTime := time.Now().Unix()

	for _, conditionalPrice := range p.ConditionalPrices {
		condition := conditionalPrice.Condition
		conditionServiceTier := normalizeServiceTier(condition.ServiceTier)

		// If condition specifies service tier, it must match usage tier.
		if conditionServiceTier != "" && usageServiceTier != conditionServiceTier {
			continue
		}

		// Check time range
		if condition.StartTime > 0 && currentTime < condition.StartTime {
			continue
		}

		if condition.EndTime > 0 && currentTime > condition.EndTime {
			continue
		}

		// Check token ranges
		if condition.InputTokenMin > 0 && inputTokens < condition.InputTokenMin {
			continue
		}

		if condition.InputTokenMax > 0 && inputTokens > condition.InputTokenMax {
			continue
		}

		if condition.OutputTokenMin > 0 && outputTokens < condition.OutputTokenMin {
			continue
		}

		if condition.OutputTokenMax > 0 && outputTokens > condition.OutputTokenMax {
			continue
		}

		return conditionalPrice.Price
	}

	return *p
}

func (p *Price) GetInputPriceUnit() int64 {
	if p.InputPriceUnit > 0 {
		return int64(p.InputPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetImageInputPriceUnit() int64 {
	if p.ImageInputPriceUnit > 0 {
		return int64(p.ImageInputPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetAudioInputPriceUnit() int64 {
	if p.AudioInputPriceUnit > 0 {
		return int64(p.AudioInputPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetOutputPriceUnit() int64 {
	if p.OutputPriceUnit > 0 {
		return int64(p.OutputPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetImageOutputPriceUnit() int64 {
	if p.ImageOutputPriceUnit > 0 {
		return int64(p.ImageOutputPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetCachedPriceUnit() int64 {
	if p.CachedPriceUnit > 0 {
		return int64(p.CachedPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetCacheCreationPriceUnit() int64 {
	if p.CacheCreationPriceUnit > 0 {
		return int64(p.CacheCreationPriceUnit)
	}
	return PriceUnit
}

func (p *Price) GetWebSearchPriceUnit() int64 {
	if p.WebSearchPriceUnit > 0 {
		return int64(p.WebSearchPriceUnit)
	}
	return PriceUnit
}

type Usage struct {
	InputTokens         ZeroNullInt64 `json:"input_tokens,omitempty"`
	ImageInputTokens    ZeroNullInt64 `json:"image_input_tokens,omitempty"`
	AudioInputTokens    ZeroNullInt64 `json:"audio_input_tokens,omitempty"`
	OutputTokens        ZeroNullInt64 `json:"output_tokens,omitempty"`
	ImageOutputTokens   ZeroNullInt64 `json:"image_output_tokens,omitempty"`
	CachedTokens        ZeroNullInt64 `json:"cached_tokens,omitempty"`
	CacheCreationTokens ZeroNullInt64 `json:"cache_creation_tokens,omitempty"`
	ReasoningTokens     ZeroNullInt64 `json:"reasoning_tokens,omitempty"`
	TotalTokens         ZeroNullInt64 `json:"total_tokens,omitempty"`
	WebSearchCount      ZeroNullInt64 `json:"web_search_count,omitempty"`
	ServiceTier         string        `json:"service_tier,omitempty"          gorm:"-"`
}

func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.ImageInputTokens += other.ImageInputTokens
	u.AudioInputTokens += other.AudioInputTokens
	u.OutputTokens += other.OutputTokens
	u.ImageOutputTokens += other.ImageOutputTokens
	u.CachedTokens += other.CachedTokens
	u.CacheCreationTokens += other.CacheCreationTokens
	u.TotalTokens += other.TotalTokens
	u.WebSearchCount += other.WebSearchCount

	if u.ServiceTier == "" {
		u.ServiceTier = other.ServiceTier
	}
}

type Amount struct {
	InputAmount         float64 `json:"input_amount,omitempty"`
	ImageInputAmount    float64 `json:"image_input_amount,omitempty"`
	AudioInputAmount    float64 `json:"audio_input_amount,omitempty"`
	OutputAmount        float64 `json:"output_amount,omitempty"`
	ImageOutputAmount   float64 `json:"image_output_amount,omitempty"`
	CachedAmount        float64 `json:"cached_amount,omitempty"`
	CacheCreationAmount float64 `json:"cache_creation_amount,omitempty"`
	WebSearchAmount     float64 `json:"web_search_amount,omitempty"`
	UsedAmount          float64 `json:"used_amount,omitempty"`
}

func (a *Amount) Add(other Amount) {
	a.InputAmount = decimal.NewFromFloat(a.InputAmount).
		Add(decimal.NewFromFloat(other.InputAmount)).
		InexactFloat64()
	a.ImageInputAmount = decimal.NewFromFloat(a.ImageInputAmount).
		Add(decimal.NewFromFloat(other.ImageInputAmount)).
		InexactFloat64()
	a.AudioInputAmount = decimal.NewFromFloat(a.AudioInputAmount).
		Add(decimal.NewFromFloat(other.AudioInputAmount)).
		InexactFloat64()
	a.OutputAmount = decimal.NewFromFloat(a.OutputAmount).
		Add(decimal.NewFromFloat(other.OutputAmount)).
		InexactFloat64()
	a.ImageOutputAmount = decimal.NewFromFloat(a.ImageOutputAmount).
		Add(decimal.NewFromFloat(other.ImageOutputAmount)).
		InexactFloat64()
	a.CachedAmount = decimal.NewFromFloat(a.CachedAmount).
		Add(decimal.NewFromFloat(other.CachedAmount)).
		InexactFloat64()
	a.CacheCreationAmount = decimal.NewFromFloat(a.CacheCreationAmount).
		Add(decimal.NewFromFloat(other.CacheCreationAmount)).
		InexactFloat64()
	a.WebSearchAmount = decimal.NewFromFloat(a.WebSearchAmount).
		Add(decimal.NewFromFloat(other.WebSearchAmount)).
		InexactFloat64()
	a.UsedAmount = decimal.NewFromFloat(a.UsedAmount).
		Add(decimal.NewFromFloat(other.UsedAmount)).
		InexactFloat64()
}
