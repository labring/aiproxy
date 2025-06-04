package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func AbortLogWithMessageWithMode(
	m mode.Mode,
	c *gin.Context,
	statusCode int,
	message string,
	typ ...string,
) {
	GetLogger(c).Error(message)
	AbortWithMessageWithMode(m, c, statusCode, message, typ...)
}

func AbortWithMessageWithMode(
	m mode.Mode,
	c *gin.Context,
	statusCode int,
	message string,
	typ ...string,
) {
	c.JSON(statusCode,
		relaymodel.WrapperErrorWithMessage(m, statusCode, message, typ...),
	)
	c.Abort()
}

func AbortLogWithMessage(c *gin.Context, statusCode int, message string, typ ...string) {
	GetLogger(c).Error(message)
	AbortWithMessage(c, statusCode, message, typ...)
}

func AbortWithMessage(c *gin.Context, statusCode int, message string, typ ...string) {
	c.JSON(statusCode,
		relaymodel.WrapperErrorWithMessage(GetMode(c), statusCode, message, typ...),
	)
	c.Abort()
}

func GetMode(c *gin.Context) mode.Mode {
	m, exists := c.Get(Mode)
	if !exists {
		return mode.Unknown
	}
	v, ok := m.(mode.Mode)
	if !ok {
		panic(fmt.Sprintf("mode type error: %T, %v", v, v))
	}
	return v
}
