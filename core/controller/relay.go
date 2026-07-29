package controller

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/relay/mode"
	// relay model used by swagger
	_ "github.com/labring/aiproxy/core/relay/model"
)

// Completions godoc
//
//	@Summary		Completions
//	@Description	Completions
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.GeneralOpenAIRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string						false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string						false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string						false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.TextResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/completions [post]
func Completions() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.Completions),
		NewRelay(mode.Completions),
	}
}

// Anthropic godoc
//
//	@Summary		Anthropic
//	@Description	Anthropic
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.AnthropicMessageRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string							false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string							false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string							false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.TextResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/messages [post]
func Anthropic() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.Anthropic),
		NewRelay(mode.Anthropic),
	}
}

// ChatCompletions godoc
//
//	@Summary		ChatCompletions
//	@Description	ChatCompletions
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.GeneralOpenAIRequest		true	"Request"
//	@Param			Aiproxy-Channel					header		string							false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string							false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string							false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.TextResponse				|		model.ChatCompletionsStreamResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/chat/completions [post]
func ChatCompletions() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ChatCompletions),
		NewRelay(mode.ChatCompletions),
	}
}

// Embeddings godoc
//
//	@Summary		Embeddings
//	@Description	Embeddings
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.EmbeddingRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string					false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string					false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string					false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.EmbeddingResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/embeddings [post]
func Embeddings() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.Embeddings),
		NewRelay(mode.Embeddings),
	}
}

// ImagesEdits godoc
//
//	@Summary		ImagesEdits
//	@Description	ImagesEdits
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			prompt							formData	string	true	"Prompt"
//	@Param			model							formData	string	true	"Model"
//	@Param			image							formData	file	true	"Images"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.SttJSONResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/images/edits [post]
func ImagesEdits() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ImagesEdits),
		NewRelay(mode.ImagesEdits),
	}
}

// ImagesGenerations godoc
//
//	@Summary		ImagesGenerations
//	@Description	ImagesGenerations
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.ImageRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string				false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string				false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string				false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.ImageResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/images/generations [post]
func ImagesGenerations() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ImagesGenerations),
		NewRelay(mode.ImagesGenerations),
	}
}

// AudioSpeech godoc
//
//	@Summary		AudioSpeech
//	@Description	AudioSpeech
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.TextToSpeechRequest		true	"Request"
//	@Param			Aiproxy-Channel					header		string							false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string							false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string							false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{file}		file							"audio binary"
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/audio/speech [post]
func AudioSpeech() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.AudioSpeech),
		NewRelay(mode.AudioSpeech),
	}
}

// AudioTranscription godoc
//
//	@Summary		AudioTranscription
//	@Description	AudioTranscription
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			model							formData	string	true	"Model"
//	@Param			file							formData	file	true	"File"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.SttJSONResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/audio/transcriptions [post]
func AudioTranscription() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.AudioTranscription),
		NewRelay(mode.AudioTranscription),
	}
}

// AudioTranslation godoc
//
//	@Summary		AudioTranslation
//	@Description	AudioTranslation
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			model							formData	string	true	"Model"
//	@Param			file							formData	file	true	"File"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.SttJSONResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/audio/translations [post]
func AudioTranslation() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.AudioTranslation),
		NewRelay(mode.AudioTranslation),
	}
}

func Moderations() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.Moderations),
		NewRelay(mode.Moderations),
	}
}

// Rerank godoc
//
//	@Summary		Rerank
//	@Description	Rerank
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.RerankRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string				false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string				false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string				false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.RerankResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/rerank [post]
func Rerank() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.Rerank),
		NewRelay(mode.Rerank),
	}
}

// ParsePdf godoc
//
//	@Summary		ParsePdf
//	@Description	ParsePdf
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			model							formData	string	true	"Model"
//	@Param			file							formData	file	true	"File"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.ParsePdfResponse
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/parse/pdf [post]
func ParsePdf() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ParsePdf),
		NewRelay(mode.ParsePdf),
	}
}

// VideoGenerationsJobs godoc
//
//	@Summary		VideoGenerationsJobs
//	@Description	VideoGenerationsJobs
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.VideoGenerationJobRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string							false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string							false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string							false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.VideoGenerationJob
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/video/generations/jobs [post]
func VideoGenerationsJobs() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideoGenerationsJobs),
		NewRelay(mode.VideoGenerationsJobs),
	}
}

// AliVideo godoc
//
//	@Summary		Create Ali native video task
//	@Description	Create an Ali DashScope native video generation task.
//	@Tags			relay
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		object	true	"Ali DashScope video synthesis request"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	object
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/api/v1/services/aigc/video-generation/video-synthesis [post]
func AliVideo() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.AliVideo),
		NewRelay(mode.AliVideo),
	}
}

// AliVideoTask godoc
//
//	@Summary		Get Ali native video task
//	@Description	Get an Ali DashScope native video generation task.
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			task_id							path		string	true	"Task ID"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	object
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/api/v1/tasks/{task_id} [get]
func AliVideoTask() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.AliVideoTasks),
		NewRelay(mode.AliVideoTasks),
	}
}

// DoubaoVideo godoc
//
//	@Summary		Create Doubao native video task
//	@Description	Create a Doubao native video generation task.
//	@Tags			relay
//	@Accept			json
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		object	true	"Doubao video generation task request"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	object
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/api/v3/contents/generations/tasks [post]
func DoubaoVideo() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.DoubaoVideo),
		NewRelay(mode.DoubaoVideo),
	}
}

// DoubaoVideoTask godoc
//
//	@Summary		Get Doubao native video task
//	@Description	Get a Doubao native video generation task.
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			task_id							path		string	true	"Task ID"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	object
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/api/v3/contents/generations/tasks/{task_id} [get]
func DoubaoVideoTask() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.DoubaoVideoTasks),
		NewRelay(mode.DoubaoVideoTasks),
	}
}

// DeleteDoubaoVideoTask godoc
//
//	@Summary		Delete Doubao native video task
//	@Description	Delete a Doubao native video generation task.
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			task_id							path		string	true	"Task ID"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	object
//	@Success		204								{object}	nil
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/api/v3/contents/generations/tasks/{task_id} [delete]
func DeleteDoubaoVideoTask() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.DoubaoVideoTasksDelete),
		NewRelay(mode.DoubaoVideoTasksDelete),
	}
}

// VideoGenerationsGetJobs godoc
//
//	@Summary		VideoGenerationsGetJobs
//	@Description	VideoGenerationsGetJobs
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.VideoGenerationJobRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string							false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string							false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string							false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.VideoGenerationJob
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/video/generations/jobs/{id} [get]
func VideoGenerationsGetJobs() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideoGenerationsGetJobs),
		NewRelay(mode.VideoGenerationsGetJobs),
	}
}

// VideoGenerationsContent godoc
//
//	@Summary		VideoGenerationsContent
//	@Description	VideoGenerationsContent
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.VideoGenerationJobRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string							false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string							false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string							false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{file}		file							"video binary"
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/video/generations/{id}/content/video [get]
func VideoGenerationsContent() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideoGenerationsContent),
		NewRelay(mode.VideoGenerationsContent),
	}
}

// Videos godoc
//
//	@Summary		Create video
//	@Description	Create a video generation job
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.VideosRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string				false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string				false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string				false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.Video
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/videos [post]
func Videos() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.Videos),
		NewRelay(mode.Videos),
	}
}

// EditVideo godoc
//
//	@Summary		Edit video
//	@Description	Create a new video by editing a source video
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.VideosEditRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string					false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string					false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string					false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.Video
//	@Router			/v1/videos/edits [post]
func EditVideo() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideosEdits),
		NewRelay(mode.VideosEdits),
	}
}

// ExtendVideo godoc
//
//	@Summary		Extend video
//	@Description	Create an extension of a completed video
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.VideosExtensionRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string							false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string							false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string							false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.Video
//	@Router			/v1/videos/extensions [post]
func ExtendVideo() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideosExtensions),
		NewRelay(mode.VideosExtensions),
	}
}

// GetVideo godoc
//
//	@Summary		Get video
//	@Description	Get a video by ID
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			video_id						path		string	true	"Video ID"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.Video
//	@Router			/v1/videos/{video_id} [get]
func GetVideo() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideosGet),
		NewRelay(mode.VideosGet),
	}
}

// GetVideoContent godoc
//
//	@Summary		Get video content
//	@Description	Get generated video binary content
//	@Tags			relay
//	@Produce		application/octet-stream
//	@Security		ApiKeyAuth
//	@Param			video_id						path	string	true	"Video ID"
//	@Param			Aiproxy-Channel					header	string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header	string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header	string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{file}	file	"video binary"
//	@Router			/v1/videos/{video_id}/content [get]
func GetVideoContent() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideosContent),
		NewRelay(mode.VideosContent),
	}
}

// DeleteVideo godoc
//
//	@Summary		Delete video
//	@Description	Delete a video by ID
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			video_id						path	string	true	"Video ID"
//	@Param			Aiproxy-Channel					header	string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header	string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header	string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		204
//	@Router			/v1/videos/{video_id} [delete]
func DeleteVideo() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideosDelete),
		NewRelay(mode.VideosDelete),
	}
}

// RemixVideo godoc
//
//	@Summary		Remix video
//	@Description	Create a new video from an existing video
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			video_id						path		string						true	"Video ID"
//	@Param			request							body		model.VideosRemixRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string						false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string						false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string						false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.Video
//	@Router			/v1/videos/{video_id}/remix [post]
func RemixVideo() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideosRemix),
		NewRelay(mode.VideosRemix),
	}
}

// CreateResponse godoc
//
//	@Summary		Create response
//	@Description	Create a new response
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request							body		model.CreateResponseRequest	true	"Request"
//	@Param			Aiproxy-Channel					header		string						false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string						false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string						false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.Response
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/responses [post]
func CreateResponse() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.Responses),
		NewRelay(mode.Responses),
	}
}

// GetResponse godoc
//
//	@Summary		Get response
//	@Description	Get a response by ID
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			response_id						path		string	true	"Response ID"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.Response
//	@Router			/v1/responses/{response_id} [get]
func GetResponse() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ResponsesGet),
		NewRelay(mode.ResponsesGet),
	}
}

// DeleteResponse godoc
//
//	@Summary		Delete response
//	@Description	Delete a response by ID
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			response_id						path	string	true	"Response ID"
//	@Param			Aiproxy-Channel					header	string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header	string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header	string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		204
//	@Router			/v1/responses/{response_id} [delete]
func DeleteResponse() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ResponsesDelete),
		NewRelay(mode.ResponsesDelete),
	}
}

// CancelResponse godoc
//
//	@Summary		Cancel response
//	@Description	Cancel a response by ID
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			response_id						path		string	true	"Response ID"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.Response
//	@Router			/v1/responses/{response_id}/cancel [post]
func CancelResponse() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ResponsesCancel),
		NewRelay(mode.ResponsesCancel),
	}
}

// GetResponseInputItems godoc
//
//	@Summary		Get response input items
//	@Description	Get input items for a response
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			response_id						path		string	true	"Response ID"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	model.InputItemList
//	@Router			/v1/responses/{response_id}/input_items [get]
func GetResponseInputItems() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ResponsesInputItems),
		NewRelay(mode.ResponsesInputItems),
	}
}

// Gemini godoc
//
//	@Summary		Gemini Native API
//	@Description	Gemini Native API
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			version							path		string	true	"API Version (v1 or v1beta)"
//	@Param			model							path		string	true	"Model name with action (e.g., gemini-2.0-flash:generateContent)"
//	@Param			request							body		object	true	"Request"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	object
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/{version}/models/{model} [post]
func GeminiByPath() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		func(c *gin.Context) {
			relayMode := mode.Gemini
			modelPath := c.Param("model")
			action := geminiPathAction(modelPath)

			switch {
			case geminiPathIsFile(modelPath):
				relayMode = mode.GeminiFiles
			case action == "predictLongRunning":
				relayMode = mode.GeminiVideo
			}

			middleware.NewDistribute(relayMode)(c)

			if c.IsAborted() {
				return
			}

			NewRelay(relayMode)(c)
		},
	}
}

// GeminiOperation godoc
//
//	@Summary		Gemini Operation API
//	@Description	Get a Gemini long-running operation, including Gemini video generation operations.
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			version							path		string	true	"API Version (v1 or v1beta)"
//	@Param			operation_id					path		string	true	"Operation ID"
//	@Param			Aiproxy-Channel					header		string	false	"Optional Aiproxy-Channel header"
//	@Param			X-Aiproxy-Group					header		string	false	"Optional group ID for internal token requests"
//	@Param			X-Aiproxy-Group-Channel-Mode	header		string	false	"Optional group channel mode for internal token requests. Values: global, own"	Enums(global, own)
//	@Success		200								{object}	object
//	@Header			all								{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all								{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all								{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all								{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all								{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all								{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/{version}/operations/{operation_id} [get]
func GeminiOperation() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.GeminiVideoOperations),
		NewRelay(mode.GeminiVideoOperations),
	}
}

func geminiPathAction(modelPath string) string {
	modelPath = strings.TrimPrefix(modelPath, "/")

	_, action, ok := strings.Cut(modelPath, ":")
	if !ok {
		return ""
	}

	return action
}

func geminiPathIsFile(modelPath string) bool {
	modelPath = strings.TrimPrefix(modelPath, "/")
	fileID, ok := strings.CutSuffix(modelPath, ":download")

	return ok && fileID != "" && !strings.Contains(fileID, "/")
}
