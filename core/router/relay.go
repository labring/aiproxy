package router

import (
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/controller"
	"github.com/labring/aiproxy/core/middleware"
)

func SetRelayRouter(router *gin.Engine) {
	// https://platform.openai.com/docs/api-reference/introduction
	v1Router := router.Group("/v1")
	v1Router.Use(middleware.IPBlock, middleware.TokenAuth)

	v1betaRouter := router.Group("/v1beta")
	v1betaRouter.Use(middleware.IPBlock, middleware.TokenAuth)

	aliRouter := router.Group("/api/v1")
	aliRouter.Use(middleware.IPBlock, middleware.TokenAuth)

	doubaoRouter := router.Group("/api/v3")
	doubaoRouter.Use(middleware.IPBlock, middleware.TokenAuth)

	modelsRouter := v1Router.Group("/models")
	{
		modelsRouter.GET("", controller.ListModels)
		modelsRouter.GET("/:model", controller.RetrieveModel)
	}

	// provider-native video APIs
	{
		aliRouter.POST(
			"/services/aigc/video-generation/video-synthesis",
			controller.AliVideo()...,
		)
		aliRouter.GET(
			"/tasks/:task_id",
			controller.AliVideoTask()...,
		)
		doubaoRouter.POST(
			"/contents/generations/tasks",
			controller.DoubaoVideo()...,
		)
		doubaoRouter.GET(
			"/contents/generations/tasks/:task_id",
			controller.DoubaoVideoTask()...,
		)
		doubaoRouter.DELETE(
			"/contents/generations/tasks/:task_id",
			controller.DeleteDoubaoVideoTask()...,
		)
	}

	// gemini
	{
		v1Router.POST(
			"/models/*model",
			controller.GeminiByPath()...,
		)
		v1Router.GET(
			"/operations/*operation_id",
			controller.GeminiOperation()...,
		)
		v1Router.GET(
			"/models/:model/operations/*operation_id",
			controller.GeminiOperation()...,
		)
		v1betaRouter.POST(
			"/models/*model",
			controller.GeminiByPath()...,
		)
		v1betaRouter.GET(
			"/operations/*operation_id",
			controller.GeminiOperation()...,
		)
		v1betaRouter.GET(
			"/models/:model/operations/*operation_id",
			controller.GeminiOperation()...,
		)
		v1betaRouter.GET(
			"/files/*model",
			controller.GeminiByPath()...,
		)
	}

	dashboardRouter := v1Router.Group("/dashboard")
	{
		dashboardRouter.GET("/billing/subscription", controller.GetSubscription)
		dashboardRouter.GET("/billing/usage", controller.GetUsage)
		dashboardRouter.GET("/billing/quota", controller.GetQuota)
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
			"/messages",
			controller.Anthropic()...,
		)
		relayRouter.POST(
			"/images/edits",
			controller.ImagesEdits()...,
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
		relayRouter.POST(
			"/video/generations/jobs",
			controller.VideoGenerationsJobs()...,
		)
		relayRouter.GET(
			"/video/generations/jobs/:id",
			controller.VideoGenerationsGetJobs()...,
		)
		relayRouter.GET(
			"/video/generations/:id/content/video",
			controller.VideoGenerationsContent()...,
		)
		relayRouter.POST(
			"/videos",
			controller.Videos()...,
		)
		relayRouter.POST(
			"/videos/edits",
			controller.EditVideo()...,
		)
		relayRouter.POST(
			"/videos/extensions",
			controller.ExtendVideo()...,
		)
		relayRouter.GET(
			"/videos/:video_id",
			controller.GetVideo()...,
		)
		relayRouter.GET(
			"/videos/:video_id/content",
			controller.GetVideoContent()...,
		)
		relayRouter.DELETE(
			"/videos/:video_id",
			controller.DeleteVideo()...,
		)
		relayRouter.POST(
			"/videos/:video_id/remix",
			controller.RemixVideo()...,
		)
		relayRouter.POST("/responses",
			controller.CreateResponse()...)
		relayRouter.POST("/responses/compact",
			controller.CompactResponse()...)
		relayRouter.POST("/alpha/search",
			controller.AlphaSearch()...)
		relayRouter.POST("/systemone",
			controller.SystemOne()...)
		relayRouter.GET("/responses/:response_id",
			controller.GetResponse()...)
		relayRouter.DELETE("/responses/:response_id",
			controller.DeleteResponse()...)
		relayRouter.POST("/responses/:response_id/cancel",
			controller.CancelResponse()...)
		relayRouter.GET(
			"/responses/:response_id/input_items",
			controller.GetResponseInputItems()...,
		)

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
		relayRouter.GET(
			"/threads/:id/messages/:messageId/files/:filesId",
			controller.RelayNotImplemented,
		)
		relayRouter.GET("/threads/:id/messages/:messageId/files", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id/runs", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/runs/:runsId", controller.RelayNotImplemented)
		relayRouter.POST("/threads/:id/runs/:runsId", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/runs", controller.RelayNotImplemented)
		relayRouter.POST(
			"/threads/:id/runs/:runsId/submit_tool_outputs",
			controller.RelayNotImplemented,
		)
		relayRouter.POST("/threads/:id/runs/:runsId/cancel", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/runs/:runsId/steps/:stepId", controller.RelayNotImplemented)
		relayRouter.GET("/threads/:id/runs/:runsId/steps", controller.RelayNotImplemented)
	}
}
