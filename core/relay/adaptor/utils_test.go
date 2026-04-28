package adaptor

import (
	"net/http"
	"testing"

	"github.com/labring/aiproxy/core/relay/mode"
)

func TestIsSuccessfulResponseStatus(t *testing.T) {
	tests := []struct {
		name       string
		mode       mode.Mode
		statusCode int
		want       bool
	}{
		{
			name:       "default accepts ok",
			mode:       mode.ChatCompletions,
			statusCode: http.StatusOK,
			want:       true,
		},
		{
			name:       "default rejects no content",
			mode:       mode.ChatCompletions,
			statusCode: http.StatusNoContent,
			want:       false,
		},
		{
			name:       "responses accepts created",
			mode:       mode.Responses,
			statusCode: http.StatusCreated,
			want:       true,
		},
		{
			name:       "responses delete accepts no content",
			mode:       mode.ResponsesDelete,
			statusCode: http.StatusNoContent,
			want:       true,
		},
		{
			name:       "responses delete accepts ok",
			mode:       mode.ResponsesDelete,
			statusCode: http.StatusOK,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSuccessfulResponseStatus(tt.mode, tt.statusCode)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
