package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/ali"
	relaycontroller "github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

// RealtimeSTTBrowserAuth adapts browser subprotocol credentials to the regular token middleware.
// Keys never enter URLs or the negotiated subprotocol, and normal authorization/scope checks apply.
func RealtimeSTTBrowserAuth(c *gin.Context) {
	if !websocket.IsWebSocketUpgrade(c.Request) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "WebSocket upgrade required"})
		return
	}

	var key string
	for _, protocol := range websocket.Subprotocols(c.Request) {
		if value, ok := strings.CutPrefix(protocol, "aiproxy-api-key."); ok {
			if key != "" || value == "" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			key = value
		}
	}

	if c.GetHeader("Authorization") == "" && key != "" {
		c.Request.Header.Set("Authorization", "Bearer "+key)
	}
	// Remove credential-bearing protocols before normal request logging/upgrade handling.
	c.Request.Header.Set("Sec-WebSocket-Protocol", "aiproxy-realtime")
}

// AudioRealtime uses the existing authorization, channel selection, rate limits and accounting.
// No whole-file upload or request-duration estimate is needed for a live audio session.
//
// @Summary Realtime speech transcription
// @Description WebSocket PCM16 session with native Qwen ASR partial and final transcription events.
// @Tags relay
// @Security ApiKeyAuth
// @Param model query string true "Registered realtime STT model"
// @Success 101 {string} string "WebSocket upgrade"
// @Router /v1/audio/realtime [get]
func AudioRealtime() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.NewDistribute(mode.AudioTranscription),
		func(c *gin.Context) {
			relay(c, mode.AudioTranscription, RelayController{
				GetRequestPrice: defaultPriceFunc,
				Handler: func(c *gin.Context, mt *meta.Meta) *relaycontroller.HandleResult {
					if mt.Channel.Type != model.ChannelTypeAli {
						return &relaycontroller.HandleResult{
							Error: relaymodel.WrapperOpenAIErrorWithMessage(
								"Realtime STT currently requires an Aliyun channel",
								"unsupported_realtime_channel",
								http.StatusBadRequest,
							),
						}
					}

					result, err := ali.RealtimeSTTHandler(mt, c)

					return &relaycontroller.HandleResult{
						Usage:      result.Usage,
						UpstreamID: result.UpstreamID,
						Error:      err,
					}
				},
			})
		},
	}
}
