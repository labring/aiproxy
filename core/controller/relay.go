package controller

import (
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
//	@Param			request			body		model.GeneralOpenAIRequest	true	"Request"
//	@Param			Aiproxy-Channel	header		string						false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.TextResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			request			body		model.AnthropicMessageRequest	true	"Request"
//	@Param			Aiproxy-Channel	header		string							false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.TextResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			request			body		model.GeneralOpenAIRequest		true	"Request"
//	@Param			Aiproxy-Channel	header		string							false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.TextResponse				|		model.ChatCompletionsStreamResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			request			body		model.EmbeddingRequest	true	"Request"
//	@Param			Aiproxy-Channel	header		string					false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.EmbeddingResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			prompt			formData	string	true	"Prompt"
//	@Param			model			formData	string	true	"Model"
//	@Param			image			formData	file	true	"Images"
//	@Param			Aiproxy-Channel	header		string	false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.SttJSONResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			request			body		model.ImageRequest	true	"Request"
//	@Param			Aiproxy-Channel	header		string				false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.ImageResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			request			body		model.TextToSpeechRequest		true	"Request"
//	@Param			Aiproxy-Channel	header		string							false	"Optional Aiproxy-Channel header"
//	@Success		200				{file}		file							"audio binary"
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			model			formData	string	true	"Model"
//	@Param			file			formData	file	true	"File"
//	@Param			Aiproxy-Channel	header		string	false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.SttJSONResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			model			formData	string	true	"Model"
//	@Param			file			formData	file	true	"File"
//	@Param			Aiproxy-Channel	header		string	false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.SttJSONResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			request			body		model.RerankRequest	true	"Request"
//	@Param			Aiproxy-Channel	header		string				false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.RerankResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			model			formData	string	true	"Model"
//	@Param			file			formData	file	true	"File"
//	@Param			Aiproxy-Channel	header		string	false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.ParsePdfResponse
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			request			body		model.VideoGenerationJobRequest	true	"Request"
//	@Param			Aiproxy-Channel	header		string							false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.VideoGenerationJob
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/video/generations/jobs [post]
func VideoGenerationsJobs() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideoGenerationsJobs),
		NewRelay(mode.VideoGenerationsJobs),
	}
}

// VideoGenerationsGetJobs godoc
//
//	@Summary		VideoGenerationsGetJobs
//	@Description	VideoGenerationsGetJobs
//	@Tags			relay
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Param			request			body		model.VideoGenerationJobRequest	true	"Request"
//	@Param			Aiproxy-Channel	header		string							false	"Optional Aiproxy-Channel header"
//	@Success		200				{object}	model.VideoGenerationJob
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
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
//	@Param			request			body		model.VideoGenerationJobRequest	true	"Request"
//	@Param			Aiproxy-Channel	header		string							false	"Optional Aiproxy-Channel header"
//	@Success		200				{file}		file							"video binary"
//	@Header			all				{integer}	X-RateLimit-Limit-Requests		"X-RateLimit-Limit-Requests"
//	@Header			all				{integer}	X-RateLimit-Limit-Tokens		"X-RateLimit-Limit-Tokens"
//	@Header			all				{integer}	X-RateLimit-Remaining-Requests	"X-RateLimit-Remaining-Requests"
//	@Header			all				{integer}	X-RateLimit-Remaining-Tokens	"X-RateLimit-Remaining-Tokens"
//	@Header			all				{string}	X-RateLimit-Reset-Requests		"X-RateLimit-Reset-Requests"
//	@Header			all				{string}	X-RateLimit-Reset-Tokens		"X-RateLimit-Reset-Tokens"
//	@Router			/v1/video/generations/{id}/content/video [get]
func VideoGenerationsContent() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.VideoGenerationsContent),
		NewRelay(mode.VideoGenerationsContent),
	}
}
