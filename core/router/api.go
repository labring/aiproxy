package router

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/env"
	"github.com/labring/aiproxy/core/controller"
	mcp "github.com/labring/aiproxy/core/controller/mcp"
	"github.com/labring/aiproxy/core/middleware"
)

func SetAPIRouter(router *gin.Engine) {
	api := router.Group("/api")
	if env.Bool("GZIP_ENABLED", false) {
		api.Use(gzip.Gzip(gzip.DefaultCompression))
	}

	healthRouter := api.Group("")
	healthRouter.GET("/status", controller.GetStatus)

	apiRouter := api.Group("")
	apiRouter.Use(middleware.AdminAuth)
	{
		modelsRoute := apiRouter.Group("/models")
		{
			modelsRoute.GET("/builtin", controller.BuiltinModels)
			modelsRoute.GET("/builtin/channel", controller.ChannelBuiltinModels)
			modelsRoute.GET("/builtin/channel/:type", controller.ChannelBuiltinModelsByType)
			modelsRoute.GET("/enabled", controller.EnabledModels)
			modelsRoute.GET("/enabled/:set", controller.EnabledModelsSet)
			modelsRoute.GET("/channel", controller.EnabledModelChannels)
			modelsRoute.GET("/channel/:set", controller.EnabledModelChannelsSet)
			modelsRoute.GET("/default", controller.ChannelDefaultModelsAndMapping)
			modelsRoute.GET("/default/:type", controller.ChannelDefaultModelsAndMappingByType)
		}

		dashboardRoute := apiRouter.Group("/dashboard")
		{
			dashboardRoute.GET("/", controller.GetDashboard)
			dashboardRoute.GET("/:group", controller.GetGroupDashboard)
			dashboardRoute.GET("/:group/models", controller.GetGroupDashboardModels)
		}

		modelCostRankRoute := apiRouter.Group("/model_cost_rank")
		{
			modelCostRankRoute.GET("/", controller.GetModelCostRank)
			modelCostRankRoute.GET("/:group", controller.GetGroupModelCostRank)
		}

		groupsRoute := apiRouter.Group("/groups")
		{
			groupsRoute.GET("/", controller.GetGroups)
			groupsRoute.GET("/search", controller.SearchGroups)
			groupsRoute.POST("/batch_delete", controller.DeleteGroups)
			groupsRoute.POST("/batch_status", controller.UpdateGroupsStatus)
			groupsRoute.GET("/ip_groups", controller.GetIPGroupList)
		}
		groupRoute := apiRouter.Group("/group")
		{
			groupRoute.POST("/:group", controller.CreateGroup)
			groupRoute.PUT("/:group", controller.UpdateGroup)
			groupRoute.GET("/:group", controller.GetGroup)
			groupRoute.DELETE("/:group", controller.DeleteGroup)
			groupRoute.POST("/:group/status", controller.UpdateGroupStatus)
			groupRoute.POST("/:group/rpm_ratio", controller.UpdateGroupRPMRatio)
			groupRoute.POST("/:group/tpm_ratio", controller.UpdateGroupTPMRatio)

			groupModelConfigRoute := groupRoute.Group("/:group/model_config")
			{
				groupModelConfigRoute.GET("/", controller.GetGroupModelConfigs)
				groupModelConfigRoute.POST("/", controller.SaveGroupModelConfigs)
				groupModelConfigRoute.PUT("/", controller.UpdateGroupModelConfigs)
				groupModelConfigRoute.DELETE("/", controller.DeleteGroupModelConfigs)
				groupModelConfigRoute.POST("/:model", controller.SaveGroupModelConfig)
				groupModelConfigRoute.PUT("/:model", controller.UpdateGroupModelConfig)
				groupModelConfigRoute.DELETE("/:model", controller.DeleteGroupModelConfig)
				groupModelConfigRoute.GET("/:model", controller.GetGroupModelConfig)
			}
		}

		optionRoute := apiRouter.Group("/option")
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.GET("/:key", controller.GetOption)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.POST("/", controller.UpdateOption)
			optionRoute.PUT("/:key", controller.UpdateOptionByKey)
			optionRoute.POST("/batch", controller.UpdateOptions)
		}

		channelsRoute := apiRouter.Group("/channels")
		{
			channelsRoute.GET("/", controller.GetChannels)
			channelsRoute.GET("/all", controller.GetAllChannels)
			channelsRoute.GET("/type_metas", controller.ChannelTypeMetas)
			channelsRoute.POST("/", controller.AddChannels)
			channelsRoute.GET("/search", controller.SearchChannels)
			channelsRoute.GET("/update_balance", controller.UpdateAllChannelsBalance)
			channelsRoute.POST("/batch_delete", controller.DeleteChannels)
			channelsRoute.GET("/test", controller.TestAllChannels)

			importRoute := channelsRoute.Group("/import")
			{
				importRoute.POST("/oneapi", controller.ImportChannelFromOneAPI)
			}
		}
		channelRoute := apiRouter.Group("/channel")
		{
			channelRoute.GET("/:id", controller.GetChannel)
			channelRoute.POST("/", controller.AddChannel)
			channelRoute.PUT("/:id", controller.UpdateChannel)
			channelRoute.POST("/:id/status", controller.UpdateChannelStatus)
			channelRoute.DELETE("/:id", controller.DeleteChannel)
			channelRoute.GET("/:id/test", controller.TestChannelModels)
			channelRoute.GET("/:id/test/:model", controller.TestChannel)
			channelRoute.GET("/:id/update_balance", controller.UpdateChannelBalance)
		}

		tokensRoute := apiRouter.Group("/tokens")
		{
			tokensRoute.GET("/", controller.GetTokens)
			tokensRoute.GET("/:id", controller.GetToken)
			tokensRoute.PUT("/:id", controller.UpdateToken)
			tokensRoute.POST("/:id/status", controller.UpdateTokenStatus)
			tokensRoute.POST("/:id/name", controller.UpdateTokenName)
			tokensRoute.DELETE("/:id", controller.DeleteToken)
			tokensRoute.GET("/search", controller.SearchTokens)
			tokensRoute.POST("/batch_delete", controller.DeleteTokens)
		}
		tokenRoute := apiRouter.Group("/token")
		{
			tokenRoute.GET("/:group/search", controller.SearchGroupTokens)
			tokenRoute.POST("/:group/batch_delete", controller.DeleteGroupTokens)
			tokenRoute.GET("/:group", controller.GetGroupTokens)
			tokenRoute.GET("/:group/:id", controller.GetGroupToken)
			tokenRoute.POST("/:group", controller.AddGroupToken)
			tokenRoute.PUT("/:group/:id", controller.UpdateGroupToken)
			tokenRoute.POST("/:group/:id/status", controller.UpdateGroupTokenStatus)
			tokenRoute.POST("/:group/:id/name", controller.UpdateGroupTokenName)
			tokenRoute.DELETE("/:group/:id", controller.DeleteGroupToken)
		}

		logsRoute := apiRouter.Group("/logs")
		{
			logsRoute.GET("/", controller.GetLogs)
			logsRoute.DELETE("/", controller.DeleteHistoryLogs)
			logsRoute.GET("/search", controller.SearchLogs)
			logsRoute.GET("/consume_error", controller.SearchConsumeError)
			logsRoute.GET("/detail/:log_id", controller.GetLogDetail)
			logsRoute.GET("/used/models", controller.GetUsedModels)
			logsRoute.GET("/used/token_names", controller.GetUsedTokenNames)
		}
		logRoute := apiRouter.Group("/log")
		{
			logRoute.GET("/:group", controller.GetGroupLogs)
			logRoute.GET("/:group/search", controller.SearchGroupLogs)
			logRoute.GET("/:group/detail/:log_id", controller.GetGroupLogDetail)
			logRoute.GET("/:group/used/models", controller.GetGroupUsedModels)
			logRoute.GET("/:group/used/token_names", controller.GetGroupUsedTokenNames)
		}

		modelConfigsRoute := apiRouter.Group("/model_configs")
		{
			modelConfigsRoute.GET("/", controller.GetModelConfigs)
			modelConfigsRoute.GET("/search", controller.SearchModelConfigs)
			modelConfigsRoute.GET("/all", controller.GetAllModelConfigs)
			modelConfigsRoute.POST("/contains", controller.GetModelConfigsByModelsContains)
			modelConfigsRoute.POST("/", controller.SaveModelConfigs)
			modelConfigsRoute.POST("/batch_delete", controller.DeleteModelConfigs)
		}
		modelConfigRoute := apiRouter.Group("/model_config")
		{
			modelConfigRoute.GET("/:model", controller.GetModelConfig)
			modelConfigRoute.POST("/", controller.SaveModelConfig)
			modelConfigRoute.DELETE("/:model", controller.DeleteModelConfig)
		}

		monitorRoute := apiRouter.Group("/monitor")
		{
			monitorRoute.GET("/", controller.GetAllChannelModelErrorRates)
			monitorRoute.GET("/:id", controller.GetChannelModelErrorRates)
			monitorRoute.DELETE("/", controller.ClearAllModelErrors)
			monitorRoute.DELETE("/:id", controller.ClearChannelAllModelErrors)
			monitorRoute.DELETE("/:id/:model", controller.ClearChannelModelErrors)
			monitorRoute.GET("/models", controller.GetModelsErrorRate)
			monitorRoute.GET("/banned_channels", controller.GetAllBannedModelChannels)
		}
		publicMcpRoute := apiRouter.Group("/mcp/public")
		{
			publicMcpRoute.GET("/", mcp.GetPublicMCPs)
			publicMcpRoute.GET("/all", mcp.GetAllPublicMCPs)
			publicMcpRoute.GET("/:id", mcp.GetPublicMCPByIDHandler)
			publicMcpRoute.POST("/", mcp.CreatePublicMCP)
			publicMcpRoute.PUT("/:id", mcp.UpdatePublicMCP)
			publicMcpRoute.DELETE("/:id", mcp.DeletePublicMCP)
			publicMcpRoute.POST("/:id/status", mcp.UpdatePublicMCPStatus)
			publicMcpRoute.GET("/:id/group/:group/params", mcp.GetGroupPublicMCPReusingParam)
			publicMcpRoute.POST(
				"/:id/group/:group/params",
				mcp.SaveGroupPublicMCPReusingParam,
			)
		}

		groupMcpRoute := apiRouter.Group("/mcp/group")
		{
			groupMcpRoute.GET("/:group", mcp.GetGroupMCPs)
			groupMcpRoute.GET("/all", mcp.GetAllGroupMCPs)
			groupMcpRoute.GET("/:group/:id", mcp.GetGroupMCPByID)
			groupMcpRoute.POST("/:group", mcp.CreateGroupMCP)
			groupMcpRoute.PUT("/:group/:id", mcp.UpdateGroupMCP)
			groupMcpRoute.DELETE("/:group/:id", mcp.DeleteGroupMCP)
			groupMcpRoute.POST("/:group/:id/status", mcp.UpdateGroupMCPStatus)
		}

		embedMcpRoute := apiRouter.Group("/embedmcp")
		{
			embedMcpRoute.GET("/", mcp.GetEmbedMCPs)
			embedMcpRoute.POST("/", mcp.SaveEmbedMCP)
		}

		testEmbedMcpRoute := apiRouter.Group("/test-embedmcp")
		{
			testEmbedMcpRoute.GET("/:id/sse", mcp.TestEmbedMCPSseServer)
			testEmbedMcpRoute.POST("/message", mcp.TestEmbedMCPMessage)
			testEmbedMcpRoute.GET("/:id", mcp.TestEmbedMCPStreamable)
			testEmbedMcpRoute.POST("/:id", mcp.TestEmbedMCPStreamable)
			testEmbedMcpRoute.DELETE("/:id", mcp.TestEmbedMCPStreamable)
		}
	}
}
