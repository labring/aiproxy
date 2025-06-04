package anthropic

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/conv"
)

type Anthropic struct {
	Event string
	Data  []byte
}

const (
	n     = "\n"
	nn    = "\n\n"
	event = "event: "
	data  = "data: "
)

var (
	nBytes     = conv.StringToBytes(n)
	nnBytes    = conv.StringToBytes(nn)
	eventBytes = conv.StringToBytes(event)
	dataBytes  = conv.StringToBytes(data)
)

func (r *Anthropic) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)

	event := r.Event

	if event == "" {
		eventNode, err := sonic.Get(r.Data, "type")
		if err != nil {
			return err
		}
		event, err = eventNode.String()
		if err != nil {
			return err
		}
	}

	for _, bytes := range [][]byte{
		eventBytes,
		conv.StringToBytes(event),
		nBytes,
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

func (r *Anthropic) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")
}
