package mode_test

import (
	"testing"

	"github.com/labring/aiproxy/core/relay/mode"
)

func TestPersistedModeIDsStayStable(t *testing.T) {
	tests := map[mode.Mode]int{
		mode.Unknown:                 0,
		mode.ChatCompletions:         1,
		mode.Completions:             2,
		mode.Embeddings:              3,
		mode.Moderations:             4,
		mode.ImagesGenerations:       5,
		mode.ImagesEdits:             6,
		mode.AudioSpeech:             7,
		mode.AudioTranscription:      8,
		mode.AudioTranslation:        9,
		mode.Rerank:                  10,
		mode.ParsePdf:                11,
		mode.Anthropic:               12,
		mode.VideoGenerationsJobs:    13,
		mode.VideoGenerationsGetJobs: 14,
		mode.VideoGenerationsContent: 15,
		mode.Responses:               16,
		mode.ResponsesGet:            17,
		mode.ResponsesDelete:         18,
		mode.ResponsesCancel:         19,
		mode.ResponsesInputItems:     20,
		mode.Gemini:                  21,
		mode.Videos:                  22,
		mode.VideosGet:               23,
		mode.VideosContent:           24,
		mode.VideosDelete:            25,
		mode.VideosRemix:             26,
		mode.GeminiVideo:             27,
		mode.GeminiVideoOperations:   28,
		mode.GeminiTTS:               29,
		mode.GeminiImage:             30,
		mode.GeminiFiles:             31,
		mode.VideosEdits:             32,
		mode.VideosExtensions:        33,
		mode.AliVideo:                34,
		mode.AliVideoTasks:           35,
		mode.DoubaoVideo:             36,
		mode.DoubaoVideoTasks:        37,
		mode.DoubaoVideoTasksDelete:  38,
		mode.ResponsesCompact:        39,
		mode.AlphaSearch:             40,
		mode.SystemOne:               41,
	}

	for relayMode, want := range tests {
		if got := int(relayMode); got != want {
			t.Fatalf("mode %s id changed: got %d, want %d", relayMode, got, want)
		}
	}
}
