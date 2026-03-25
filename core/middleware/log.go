package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/sirupsen/logrus"
)

func SetRequestAt(c *gin.Context, requestAt time.Time) {
	c.Set(RequestAt, requestAt)
}

func GetRequestAt(c *gin.Context) time.Time {
	return c.GetTime(RequestAt)
}

func NewLog(l *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		SetRequestAt(c, start)

		fields := common.GetLogFields()
		defer func() {
			common.PutLogFields(fields)
		}()

		entry := &logrus.Entry{
			Logger: l,
			Data:   fields,
		}
		common.SetLogger(c.Request, entry)

		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		param := gin.LogFormatterParams{
			Request: c.Request,
			Keys:    c.Keys,
		}

		// Stop timer
		param.Latency = time.Since(start)

		param.ClientIP = c.ClientIP()
		param.Method = c.Request.Method
		param.StatusCode = c.Writer.Status()
		param.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()

		param.BodySize = c.Writer.Size()

		if raw != "" {
			path = path + "?" + raw
		}

		param.Path = path

		logColor(entry, param)
	}
}

func logColor(log *logrus.Entry, p gin.LogFormatterParams) {
	str := formatter(p)

	switch {
	case p.StatusCode >= http.StatusInternalServerError:
		log.Error(str)
	case p.StatusCode >= http.StatusBadRequest:
		log.Warn(str)
	default:
		log.Info(str)
	}
}

func formatter(param gin.LogFormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	return fmt.Sprintf("[GIN] |%s %3d %s| %10v | %15s |%s %-7s %s %#v\n%s",
		statusColor, param.StatusCode, resetColor,
		common.TruncateDuration(param.Latency),
		param.ClientIP,
		methodColor, param.Method, resetColor,
		param.Path,
		param.ErrorMessage,
	)
}
