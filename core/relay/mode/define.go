package mode

import "fmt"

type Mode int

func (m Mode) String() string {
	if name, ok := modeNames[m]; ok {
		return name
	}

	return fmt.Sprintf("Mode(%d)", m)
}

var modeNames = map[Mode]string{
	Unknown:                 "Unknown",
	ChatCompletions:         "ChatCompletions",
	Completions:             "Completions",
	Embeddings:              "Embeddings",
	Moderations:             "Moderations",
	ImagesGenerations:       "ImagesGenerations",
	ImagesEdits:             "ImagesEdits",
	AudioSpeech:             "AudioSpeech",
	AudioTranscription:      "AudioTranscription",
	AudioTranslation:        "AudioTranslation",
	Rerank:                  "Rerank",
	ParsePdf:                "ParsePdf",
	Anthropic:               "Anthropic",
	VideoGenerationsJobs:    "VideoGenerationsJobs",
	VideoGenerationsGetJobs: "VideoGenerationsGetJobs",
	VideoGenerationsContent: "VideoGenerationsContent",
	Videos:                  "Videos",
	VideosGet:               "VideosGet",
	VideosContent:           "VideosContent",
	VideosDelete:            "VideosDelete",
	VideosRemix:             "VideosRemix",
	VideosEdits:             "VideosEdits",
	VideosExtensions:        "VideosExtensions",
	GeminiVideo:             "GeminiVideo",
	GeminiVideoOperations:   "GeminiVideoOperations",
	AliVideo:                "AliVideo",
	AliVideoTasks:           "AliVideoTasks",
	DoubaoVideo:             "DoubaoVideo",
	DoubaoVideoTasks:        "DoubaoVideoTasks",
	DoubaoVideoTasksDelete:  "DoubaoVideoTasksDelete",
	GeminiTTS:               "GeminiTTS",
	GeminiImage:             "GeminiImage",
	GeminiFiles:             "GeminiFiles",
	Responses:               "Responses",
	ResponsesGet:            "ResponsesGet",
	ResponsesDelete:         "ResponsesDelete",
	ResponsesCancel:         "ResponsesCancel",
	ResponsesInputItems:     "ResponsesInputItems",
	ResponsesCompact:        "ResponsesCompact",
	AlphaSearch:             "AlphaSearch",
	SystemOne:               "SystemOne",
	Gemini:                  "Gemini",
}

const (
	Unknown Mode = iota
	ChatCompletions
	Completions
	Embeddings
	Moderations
	ImagesGenerations
	ImagesEdits
	AudioSpeech
	AudioTranscription
	AudioTranslation
	Rerank
	ParsePdf
	Anthropic
	VideoGenerationsJobs
	VideoGenerationsGetJobs
	VideoGenerationsContent
	Responses
	ResponsesGet
	ResponsesDelete
	ResponsesCancel
	ResponsesInputItems
	Gemini
	Videos
	VideosGet
	VideosContent
	VideosDelete
	VideosRemix
	GeminiVideo
	GeminiVideoOperations
	GeminiTTS
	GeminiImage
	GeminiFiles
	VideosEdits
	VideosExtensions
	AliVideo
	AliVideoTasks
	DoubaoVideo
	DoubaoVideoTasks
	DoubaoVideoTasksDelete
	ResponsesCompact
	AlphaSearch
	SystemOne
)
