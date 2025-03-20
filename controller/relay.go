package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/middleware"
	"github.com/labring/aiproxy/relay/mode"

	// relay model used by swagger
	_ "github.com/labring/aiproxy/relay/model"
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

func Edits() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.Edits),
		NewRelay(mode.Edits),
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
//	@Router			/v1/audio/transcription [post]
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
//	@Router			/v1/audio/translation [post]
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
//	@Router			/v1/parse-pdf [post]
func ParsePdf() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.ParsePdf),
		NewRelay(mode.ParsePdf),
	}
}
