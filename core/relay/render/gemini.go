package render

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
)

type GeminiSSE struct {
	Data []byte
}

func (r *GeminiSSE) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)

	for _, bytes := range [][]byte{
		dataBytes,
		r.Data,
		nnBytes,
	} {
		// nosemgrep:
		// go.lang.security.audit.xss.no-direct-write-to-responsewriter.no-direct-write-to-responsewriter
		if _, err := w.Write(bytes); err != nil {
			return err
		}
	}

	return nil
}

func (r *GeminiSSE) WriteContentType(w http.ResponseWriter) {
	WriteSSEContentType(w)
}

func GeminiBytesData(c *gin.Context, data []byte) {
	if len(c.Errors) > 0 {
		return
	}

	if c.IsAborted() {
		return
	}

	c.Render(-1, &GeminiSSE{Data: data})
	c.Writer.Flush()
}

func GeminiObjectData(c *gin.Context, object any) error {
	if len(c.Errors) > 0 {
		return c.Errors.Last()
	}

	if c.IsAborted() {
		return errors.New("context aborted")
	}

	jsonData, err := sonic.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}

	c.Render(-1, &GeminiSSE{Data: jsonData})
	c.Writer.Flush()

	return nil
}
