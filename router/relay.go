package router

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/controller"
	"github.com/labring/aiproxy/middleware"
)

func SetRelayRouter(router *gin.Engine) {
	// https://platform.openai.com/docs/api-reference/introduction
	v1Router := router.Group("/v1")
	v1Router.Use(middleware.TokenAuth)

	modelsRouter := v1Router.Group("/models")
	{
		modelsRouter.GET("", controller.ListModels)
		modelsRouter.GET("/:model", controller.RetrieveModel)
	}
	dashboardRouter := v1Router.Group("/dashboard")
	{
		dashboardRouter.GET("/billing/subscription", controller.GetSubscription)
		dashboardRouter.GET("/billing/usage", controller.GetUsage)
	}
	relayRouter := v1Router.Group("")
	{
		relayRouter.POST(
			"/completions",
			controller.Completions()...,
		)

		relayRouter.POST(
			"/chat/completions",
			controller.ChatCompletions()...,
		)
		relayRouter.POST(
			"/edits",
			controller.Edits()...,
		)
		relayRouter.POST(
			"/images/generations",
			controller.ImagesGenerations()...,
		)
		relayRouter.POST(
			"/embeddings",
			controller.Embeddings()...,
		)
		relayRouter.POST(
			"/engines/:model/embeddings",
			controller.Embeddings()...,
		)
		relayRouter.POST(
			"/audio/transcriptions",
			controller.AudioTranscription()...,
		)
		relayRouter.POST(
			"/audio/translations",
			controller.AudioTranslation()...,
		)
		relayRouter.POST(
			"/audio/speech",
			controller.AudioSpeech()...,
		)
		relayRouter.POST(
			"/rerank",
			controller.Rerank()...,
		)
		relayRouter.POST(
			"/moderations",
			controller.Moderations()...,
		)
		relayRouter.POST(
			"/parse/pdf",
			controller.ParsePdf()...,
		)

		relayRouter.POST("/images/edits", controller.RelayNotImplemented)
		relayRouter.POST("/images/variations", controller.RelayNotImplemented)
		relayRouter.GET("/files", controller.RelayNotImplemented)
		relayRouter.POST("/files", controller.RelayNotImplemented)
		relayRouter.DELETE("/files/:id", controller.RelayNotImplemented)
		relayRouter.GET("/files/:id", controller.RelayNotImplemented)
		relayRouter.GET("/files/:id/content", controller.RelayNotImplemented)
		relayRouter.POST("/fine_tuning/jobs", controller.RelayNotImplemented)
		relayRouter.GET("/fine_tuning/jobs", controller.RelayNotImplemented)
		relayRouter.GET("/fine_tuning/jobs/:id", controller.RelayNotImplemented)
		relayRouter.POST("/fine_tuning/jobs/:id/cancel", controller.RelayNotImplemented)
		relayRouter.GET("/fine_tuning/jobs/:id/events", controller.RelayNotImplemented)
		relayRouter.DELETE("/models/:model", controller.RelayNotImplemented)
		relayRouter.POST("/assistants", controller.RelayNotImplemented)
		relayRouter.GET("/assistants/:id", controller.RelayNotImplemented)
		relayRouter.POST("/assistants/:id", controller.RelayNotImplemented)
		relayRouter.DELETE("/assistants/:id", controller.RelayNotImplemented)
		relayRouter.GET("/assistants", controller.RelayNotImplemented)
		relayRouter.POST("/assistants/:id/files", controller.RelayNotImplemented)
		relayRouter.GET("/assistants/:id/files/:fileId", controller.RelayNotImplemented)
		relayRouter.DELETE("/assistants/:id/files/:fileId", controller.RelayNotImplemented)
		relayRouter.GET("/assistants/:id/files", controller.RelayNotImplemented)
		relayRouter.POST("/threads", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id", controller.RelayNotImplemented)
		relayRouter.DELETE("/threads/:id", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id/messages", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/messages/:messageId", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id/messages/:messageId", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/messages/:messageId/files/:filesId", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/messages/:messageId/files", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id/runs", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/runs/:runsId", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id/runs/:runsId", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/runs", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id/runs/:runsId/submit_tool_outputs", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id/runs/:runsId/cancel", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/runs/:runsId/steps/:stepId", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/runs/:runsId/steps", controller.RelayNotImplemented)
	}
}
