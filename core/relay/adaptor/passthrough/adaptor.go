// Package passthrough provides a zero-modification relay adaptor that pipes
// request bodies directly to upstream providers and response bytes back to
// clients, while tapping the response tail for usage data extraction.
//
// The adaptor is designed to be embedded by provider-specific adaptors
// (e.g. PPIO, Novita) that want transparent byte-level passthrough without
// the protocol re-serialization overhead of the standard OpenAI adaptor.
package passthrough

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
	"github.com/labring/aiproxy/core/relay/utils"
)

const (
	// tailBufSize is the ring buffer capacity used to capture response tail bytes
	// for usage extraction. 4KB is more than enough to hold the largest usage JSON
	// chunk observed in practice (~300 bytes including all detail fields).
	tailBufSize = 4 * 1024

	// drainTimeout is how long we continue draining the upstream response body
	// after a client-disconnect, to ensure the usage chunk is captured.
	drainTimeout = 2 * time.Second
)

// hopByHopHeaders lists HTTP/1.1 hop-by-hop headers that must not be forwarded
// between client and upstream when acting as a reverse proxy.
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

// forwardResponseHeaders copies upstream response headers to the Gin context,
// skipping hop-by-hop headers that must not be forwarded by a reverse proxy.
func forwardResponseHeaders(c *gin.Context, header http.Header) {
	for k, vs := range header {
		if hopByHopHeaders[k] {
			continue
		}

		for _, v := range vs {
			c.Header(k, v)
		}
	}
}

// clientHeadersToForward lists request headers from the client that should be
// forwarded verbatim to the upstream. Authorization is excluded because
// SetupRequestHeader replaces it with the channel key.
var clientHeadersToForward = []string{
	"Content-Type",
	"Content-Length",
	"anthropic-version",
	"anthropic-beta",
	"openai-beta",
}

// Adaptor is a zero-modification relay adaptor.
// Embed it in a provider adaptor and override DefaultBaseURL + Metadata.
type Adaptor struct{}

// SupportMode mirrors the OpenAI adaptor's supported mode set.
func (a *Adaptor) SupportMode(m mode.Mode) bool {
	switch m {
	case mode.ChatCompletions,
		mode.Completions,
		mode.Embeddings,
		mode.Moderations,
		mode.ImagesGenerations,
		mode.ImagesEdits,
		mode.AudioSpeech,
		mode.AudioTranscription,
		mode.AudioTranslation,
		mode.Rerank,
		mode.ParsePdf,
		mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
		mode.Anthropic,
		mode.Gemini,
		mode.Responses,
		mode.ResponsesGet,
		mode.ResponsesDelete,
		mode.ResponsesCancel,
		mode.ResponsesInputItems,
		mode.WebSearch:
		return true
	}

	return false
}

// NativeMode returns true for all modes except Anthropic and Gemini.
// Those protocols have dedicated channel types (ChannelTypeAnthropic,
// ChannelTypeGoogleGemini) and the passthrough adaptor should only be
// selected for them as a last-resort fallback when no native channel exists.
func (a *Adaptor) NativeMode(m mode.Mode) bool {
	if m == mode.Anthropic || m == mode.Gemini {
		return false
	}

	return a.SupportMode(m)
}

// ConvertRequest returns the original request body unchanged.
// Only protocol-relevant headers from the client request are forwarded;
// Authorization is replaced later by SetupRequestHeader.
func (a *Adaptor) ConvertRequest(
	_ *meta.Meta,
	_ adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	header := make(http.Header, len(clientHeadersToForward))

	for _, h := range clientHeadersToForward {
		if v := req.Header.Get(h); v != "" {
			header.Set(h, v)
		}
	}

	return adaptor.ConvertResult{
		Header: header,
		Body:   req.Body,
	}, nil
}

// SetupRequestHeader replaces Authorization with the channel key.
func (a *Adaptor) SetupRequestHeader(
	m *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) error {
	req.Header.Set("Authorization", "Bearer "+m.Channel.Key)

	return nil
}

// GetRequestURL builds the upstream URL using path-based passthrough logic.
//
// Priority order:
//  1. path_base_map in channel Configs — longest-prefix match selects an
//     alternative base URL (e.g. Responses API, web-search).
//  2. Default: strip the /v1 prefix from the client path and append to BaseURL.
//
// The HTTP method is taken directly from the incoming request so that GET,
// DELETE, and POST Responses API sub-endpoints work without special casing.
func (a *Adaptor) GetRequestURL(
	m *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	clientPath := c.Request.URL.Path
	base := m.Channel.BaseURL
	suffix := stripV1Prefix(clientPath)

	// path_base_map overrides the base URL for specific path prefixes.
	if pbm := GetPathBaseMap(m.ChannelConfigs); len(pbm) > 0 {
		if b, s, ok := matchPathBaseMap(pbm, clientPath); ok {
			base, suffix = b, s
		}
	}

	u, err := url.JoinPath(base, suffix)
	if err != nil {
		return adaptor.RequestURL{}, err
	}

	// Preserve query parameters (e.g. ?task_id=abc123 for async task polling).
	if q := c.Request.URL.RawQuery; q != "" {
		u += "?" + q
	}

	return adaptor.RequestURL{Method: c.Request.Method, URL: u}, nil
}

// DoRequest delegates to the standard HTTP client with per-channel proxy and
// timeout settings.
func (a *Adaptor) DoRequest(
	m *meta.Meta,
	_ adaptor.Store,
	_ *gin.Context,
	req *http.Request,
) (*http.Response, error) {
	return utils.DoRequestWithMeta(req, m)
}

// DoResponse pipes the upstream response bytes to the client verbatim while
// tapping the last tailBufSize bytes for usage extraction.
//
// On client disconnect, the upstream body is drained for up to drainTimeout so
// the usage chunk at the end of a streaming response is still captured.
func (a *Adaptor) DoResponse(
	m *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	if resp.StatusCode != http.StatusOK {
		return adaptor.DoResponseResult{}, errorFromResponse(m, resp)
	}

	forwardResponseHeaders(c, resp.Header)
	c.Status(resp.StatusCode)

	// Ring buffer captures the last tailBufSize bytes for usage extraction.
	tail := newRingBuffer(tailBufSize)
	tee := io.TeeReader(resp.Body, tail)

	_, copyErr := flushCopy(c.Writer, tee)
	if copyErr != nil {
		// Client disconnected before the usage chunk arrived (common for long
		// streaming responses). Continue draining the upstream body into the ring
		// buffer for up to drainTimeout so we can still record usage.
		drainCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		defer cancel()

		drainBody := io.TeeReader(resp.Body, tail)
		_, _ = io.Copy(discardWriter{drainCtx}, drainBody)
	}

	upstreamID := resp.Header.Get("x-request-id")
	usage := extractUsageFromTail(tail.Bytes())

	return adaptor.DoResponseResult{
		Usage:      usage,
		UpstreamID: upstreamID,
	}, nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

// stripV1Prefix removes the /v1 prefix from a path.
// /v1/chat/completions → /chat/completions
// /v1 → /
// /v2/... → /v2/... (unchanged)
func stripV1Prefix(path string) string {
	if strings.HasPrefix(path, "/v1/") {
		return path[3:] // keeps the leading /
	}

	if path == "/v1" {
		return "/"
	}

	return path
}

// GetPathBaseMap extracts the path_base_map entry from channel configs.
// The stored value may be map[string]string (typed) or map[string]interface{}
// (after JSON round-trip through GORM's fastjson serialiser).
func GetPathBaseMap(configs model.ChannelConfigs) map[string]string {
	v, ok := configs[model.ChannelConfigPathBaseMapKey]
	if !ok || v == nil {
		return nil
	}

	switch m := v.(type) {
	case map[string]string:
		return m
	case map[string]interface{}:
		result := make(map[string]string, len(m))

		for k, val := range m {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}

		return result
	}

	return nil
}

// matchPathBaseMap performs a longest-prefix match against pathBaseMap.
// On success it returns the mapped base URL, the /v1-stripped path suffix,
// and true.
func matchPathBaseMap(pathBaseMap map[string]string, path string) (base, suffix string, ok bool) {
	// Sort keys by length descending so the longest prefix wins.
	keys := make([]string, 0, len(pathBaseMap))

	for k := range pathBaseMap {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	for _, prefix := range keys {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return pathBaseMap[prefix], stripV1Prefix(path), true
		}
	}

	return "", "", false
}

// errorFromResponse reads the upstream error body and wraps it as an
// adaptor.Error appropriate for the current relay mode.
// The response body is fully consumed and closed before this function returns.
func errorFromResponse(m *meta.Meta, resp *http.Response) adaptor.Error {
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))

	return relaymodel.WrapperErrorWithMessage(
		m.Mode,
		resp.StatusCode,
		string(body),
		relaymodel.WithType(relaymodel.ErrorTypeUpstream),
	)
}

// flusherWriter is a writer that also supports explicit flushing.
// gin.ResponseWriter satisfies this interface, as does any net/http flusher.
type flusherWriter interface {
	io.Writer
	Flush()
}

// flushCopy copies src → w, flushing after every write.
//
// Plain io.Copy does not flush Gin's underlying bufio.Writer, which means SSE
// events can be held in a 4 KB transport buffer and not reach the client until
// the buffer fills or the connection closes. Flushing after each write ensures
// each upstream chunk is forwarded immediately.
func flushCopy(w flusherWriter, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)

	var written int64

	for {
		nr, readErr := src.Read(buf)
		if nr > 0 {
			nw, writeErr := w.Write(buf[:nr])
			written += int64(nw)
			w.Flush()

			if writeErr != nil {
				return written, writeErr
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}

			return written, readErr
		}
	}
}

// discardWriter discards all writes but returns a context error once its
// context is cancelled. Used to drain an upstream response body with a timeout
// without allocating a discard buffer.
type discardWriter struct {
	ctx context.Context
}

func (w discardWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
		return len(p), nil
	}
}
