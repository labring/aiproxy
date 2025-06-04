package render

import (
	"net/http"

	"github.com/labring/aiproxy/core/common/conv"
)

type OpenAISSE struct {
	Data string
}

const (
	nn   = "\n\n"
	data = "data: "
)

var (
	nnBytes   = conv.StringToBytes(nn)
	dataBytes = conv.StringToBytes(data)
)

func (r *OpenAISSE) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)

	for _, bytes := range [][]byte{
		dataBytes,
		conv.StringToBytes(r.Data),
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

func (r *OpenAISSE) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")
}
