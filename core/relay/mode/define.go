package mode

import "fmt"

type Mode int

func (m Mode) String() string {
	switch m {
	case Unknown:
		return "Unknown"
	case ChatCompletions:
		return "ChatCompletions"
	case Completions:
		return "Completions"
	case Embeddings:
		return "Embeddings"
	case Moderations:
		return "Moderations"
	case ImagesGenerations:
		return "ImagesGenerations"
	case ImagesEdits:
		return "ImagesEdits"
	case AudioSpeech:
		return "AudioSpeech"
	case AudioTranscription:
		return "AudioTranscription"
	case AudioTranslation:
		return "AudioTranslation"
	case Rerank:
		return "Rerank"
	case ParsePdf:
		return "ParsePdf"
	case Anthropic:
		return "Anthropic"
	case VideoGenerationsJobs:
		return "VideoGenerationsJobs"
	case VideoGenerationsGetJobs:
		return "VideoGenerationsGetJobs"
	case VideoGenerationsContent:
		return "VideoGenerationsContent"
	default:
		return fmt.Sprintf("Mode(%d)", m)
	}
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
)
