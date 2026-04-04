//go:build enterprise

package synccommon

import "testing"

func TestInferToolChoice(t *testing.T) {
	tests := []struct {
		name      string
		modelType string
		features  []string
		want      bool
	}{
		{name: "chat no features", modelType: "chat", want: true},
		{name: "chat empty features", modelType: "chat", features: []string{}, want: true},
		{name: "chat tool_use", modelType: "chat", features: []string{"tool_use", "streaming"}, want: true},
		{name: "chat function_calling", modelType: "chat", features: []string{"function_calling"}, want: true},
		{name: "chat tools", modelType: "chat", features: []string{"tools"}, want: true},
		{name: "embedding no features", modelType: "embedding", want: false},
		{name: "image no features", modelType: "image", want: false},
		{name: "embedding tool_use", modelType: "embedding", features: []string{"tool_use"}, want: true},
		{name: "rerank no features", modelType: "rerank", features: []string{}, want: false},
		{name: "empty type no features", modelType: "", want: false},
		{name: "rerank function_calling", modelType: "rerank", features: []string{"function_calling"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferToolChoice(tt.modelType, tt.features)
			if got != tt.want {
				t.Errorf("InferToolChoice(%q, %v) = %v, want %v",
					tt.modelType, tt.features, got, tt.want)
			}
		})
	}
}

func TestAdjustTierBounds(t *testing.T) {
	tests := []struct {
		name      string
		min, max  int64
		prevMax   int64
		wantMin   int64
		wantMax   int64
	}{
		{
			name: "first tier no adjustment",
			min: 0, max: 128000, prevMax: 0,
			wantMin: 0, wantMax: 128000,
		},
		{
			name: "second tier overlapping boundary bumped",
			min: 128000, max: 0, prevMax: 128000,
			wantMin: 128001, wantMax: 0,
		},
		{
			name: "second tier non-overlapping unchanged",
			min: 128001, max: 0, prevMax: 128000,
			wantMin: 128001, wantMax: 0,
		},
		{
			name: "min zero not bumped",
			min: 0, max: 128000, prevMax: 128000,
			wantMin: 0, wantMax: 128000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := AdjustTierBounds(tt.min, tt.max, tt.prevMax)
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Errorf("AdjustTierBounds(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.min, tt.max, tt.prevMax, gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}
