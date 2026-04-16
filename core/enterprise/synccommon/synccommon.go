//go:build enterprise

// Package synccommon provides provider-agnostic utilities shared by the PPIO
// and Novita sync implementations.
package synccommon

import (
	"slices"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

// SyncProgressEvent represents a progress event sent via SSE during a model sync.
// Shared by all provider sync implementations to ensure a consistent wire format.
type SyncProgressEvent struct {
	Type     string `json:"type"`              // "progress", "success", "error"
	Step     string `json:"step"`              // "fetching", "comparing", "syncing", "complete"
	Message  string `json:"message"`           // Human-readable message
	Progress int    `json:"progress,omitempty"`
	Data     any    `json:"data,omitempty"` // Additional data (e.g., SyncResult on success)
}

// SendProgress delivers a progress event to callback if one is registered.
// step == "complete" upgrades the event type to "success".
func SendProgress(
	callback func(event SyncProgressEvent),
	step, message string,
	progress int,
	data any,
) {
	if callback == nil {
		return
	}

	eventType := "progress"
	if step == "complete" {
		eventType = "success"
	}

	callback(SyncProgressEvent{
		Type:     eventType,
		Step:     step,
		Message:  message,
		Progress: progress,
		Data:     data,
	})
}

// InferToolChoice returns true when a model is likely to support tool_choice.
// Feature-list signals ("tool_use", "function_calling", "tools") take highest
// priority; failing that, all "chat" models are assumed to support tool calling.
func InferToolChoice(modelType string, features []string) bool {
	for _, f := range features {
		switch f {
		case "tool_use", "function_calling", "tools":
			return true
		}
	}

	return modelType == "chat"
}

// ToModelConfigKeys converts map[string]any to map[model.ModelConfigKey]any
// without a JSON round-trip.
func ToModelConfigKeys(m map[string]any) map[model.ModelConfigKey]any {
	out := make(map[model.ModelConfigKey]any, len(m))

	for k, v := range m {
		out[model.ModelConfigKey(k)] = v
	}

	return out
}

// MergeModels returns the sorted union of existing and incoming model lists.
// Existing models are preserved even if absent from incoming, preventing
// accidental removal when upstream temporarily delists a model.
func MergeModels(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	for _, m := range existing {
		seen[m] = struct{}{}
	}
	for _, m := range incoming {
		seen[m] = struct{}{}
	}

	merged := make([]string, 0, len(seen))
	for m := range seen {
		merged = append(merged, m)
	}

	slices.Sort(merged)

	return merged
}

// ownerPriority defines sync priority: higher value = higher priority.
// PPIO is the primary domestic provider (CNY prices), so it takes precedence
// over Novita (USD→CNY converted prices) for shared models.
var ownerPriority = map[model.ModelOwner]int{
	model.ModelOwnerPPIO:   2,
	model.ModelOwnerNovita: 1,
}

// CanClaimOwnership returns true if newOwner can take ownership from existingOwner.
// A provider can claim if it has strictly higher priority than the current owner.
// Note: unknown owners have implicit priority 0 (map zero-value).
func CanClaimOwnership(existingOwner, newOwner model.ModelOwner) bool {
	return ownerPriority[newOwner] > ownerPriority[existingOwner]
}

// ShouldSkipOwnership returns true when the caller (newOwner) must NOT modify an
// existing model because it belongs to a higher-priority provider.
func ShouldSkipOwnership(existingOwner, newOwner model.ModelOwner) bool {
	return existingOwner != newOwner && !CanClaimOwnership(existingOwner, newOwner)
}

// IsLocalOnlyMode returns true for model types that are generated locally and
// not sourced from the standard V1/V2 remote model list API. These must be
// excluded from delete detection during sync diagnostics to avoid false positives.
func IsLocalOnlyMode(t mode.Mode) bool {
	return t == mode.WebSearch || t == mode.PPIONative
}

// AdjustTierBounds returns the effective [minTokens, maxTokens] for a tiered-billing
// tier, bumping minTokens by 1 when it overlaps with prevMax (the previous tier's max).
//
// Providers use inclusive boundaries like [0,128000],[128000,∞] but aiproxy requires
// non-overlapping ranges. Pass prevMax=0 for the first tier (index 0).
func AdjustTierBounds(minTokens, maxTokens, prevMax int64) (int64, int64) {
	if minTokens > 0 && prevMax > 0 && minTokens <= prevMax {
		minTokens = prevMax + 1
	}

	return minTokens, maxTokens
}
