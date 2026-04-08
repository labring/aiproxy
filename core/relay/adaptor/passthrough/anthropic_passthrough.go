package passthrough

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
)

// headBufSize is the maximum bytes captured from the beginning of the response
// for Anthropic passthrough usage extraction. The message_start SSE event that
// carries input_tokens is the first event in the stream and is typically well
// under 512 bytes; 2KB provides a generous safety margin.
const headBufSize = 2 * 1024

// headBuffer captures only the first headBufSize bytes of a stream.
// Writes beyond the capacity are silently discarded. It always reports
// success so it can be used as an io.Writer in a MultiWriter chain.
type headBuffer struct {
	buf []byte
	cap int
}

func newHeadBuffer(size int) *headBuffer {
	return &headBuffer{buf: make([]byte, 0, size), cap: size}
}

func (h *headBuffer) Write(p []byte) (int, error) {
	if remaining := h.cap - len(h.buf); remaining > 0 {
		n := min(remaining, len(p))
		h.buf = append(h.buf, p[:n]...)
	}

	return len(p), nil
}

func (h *headBuffer) Bytes() []byte { return h.buf }

// DoAnthropicPassthrough pipes an Anthropic-protocol response verbatim to the
// client while accurately capturing token usage for billing.
//
// Anthropic streaming splits usage across two SSE events:
//
//	message_start (stream head) → input_tokens, cache_read_tokens, cache_creation_tokens
//	message_delta (stream tail) → output_tokens
//
// A head buffer captures the first headBufSize bytes; a ring buffer captures
// the last tailBufSize bytes. After the stream ends both are scanned and their
// usage figures are merged into a single model.Usage.
//
// For non-streaming responses the complete JSON body fits in the tail buffer,
// so the merge is a no-op (tail already has all fields).
func DoAnthropicPassthrough(
	m *meta.Meta,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, errorFromResponse(m, resp)
	}

	forwardResponseHeaders(c, resp.Header)
	c.Status(resp.StatusCode)

	head := newHeadBuffer(headBufSize)
	tail := newRingBuffer(tailBufSize)
	tee := io.TeeReader(resp.Body, io.MultiWriter(head, tail))

	_, copyErr := flushCopy(c.Writer, tee)
	if copyErr != nil {
		// Client disconnected; drain upstream so the final usage chunk reaches
		// the tail ring buffer (same logic as the standard passthrough DoResponse).
		drainCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		defer cancel()

		_, _ = io.Copy(discardWriter{drainCtx},
			io.TeeReader(resp.Body, io.MultiWriter(head, tail)))
	}

	usage := mergeAnthropicSSEUsage(
		extractUsageFromHead(head.Bytes()),
		extractUsageFromTail(tail.Bytes()),
	)

	return adaptor.DoResponseResult{
		Usage:      usage,
		UpstreamID: resp.Header.Get("x-request-id"),
	}, nil
}

// mergeAnthropicSSEUsage merges usage figures from the stream head and tail.
//
// In Anthropic streaming:
//   - head (message_start): input_tokens, cache_read_input_tokens, cache_creation_input_tokens
//   - tail (message_delta): output_tokens
//
// tail is used as the base; head fills any fields that tail leaves at zero.
// For non-streaming responses tail already contains all fields, so the merge
// is effectively a no-op.
func mergeAnthropicSSEUsage(head, tail model.Usage) model.Usage {
	merged := tail

	if merged.InputTokens == 0 {
		merged.InputTokens = head.InputTokens
	}

	if merged.CachedTokens == 0 {
		merged.CachedTokens = head.CachedTokens
	}

	if merged.CacheCreationTokens == 0 {
		merged.CacheCreationTokens = head.CacheCreationTokens
	}

	// Always recompute — head and tail each only have partial totals.
	merged.TotalTokens = merged.InputTokens + merged.OutputTokens

	return merged
}
