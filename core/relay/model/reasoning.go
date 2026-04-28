package model

import "strings"

type ReasoningEffort = string

const (
	ReasoningEffortNone    ReasoningEffort = "none"
	ReasoningEffortMinimal ReasoningEffort = "minimal"
	ReasoningEffortLow     ReasoningEffort = "low"
	ReasoningEffortMedium  ReasoningEffort = "medium"
	ReasoningEffortHigh    ReasoningEffort = "high"
	ReasoningEffortXHigh   ReasoningEffort = "xhigh"
)

type NormalizedReasoning struct {
	Specified    bool
	Disabled     bool
	Effort       ReasoningEffort
	BudgetTokens *int
}

func NormalizeReasoningEffort(effort string) ReasoningEffort {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "none", "off", "disabled":
		return ReasoningEffortNone
	case "minimal":
		return ReasoningEffortMinimal
	case "low":
		return ReasoningEffortLow
	case "medium", "med":
		return ReasoningEffortMedium
	case "high":
		return ReasoningEffortHigh
	case "xhigh", "max", "maximum":
		return ReasoningEffortXHigh
	default:
		return ""
	}
}

func (r NormalizedReasoning) Enabled() bool {
	return r.Specified && !r.Disabled
}
