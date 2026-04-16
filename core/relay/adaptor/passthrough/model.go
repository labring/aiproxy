package passthrough

import (
	"bytes"
	"io"
	"net/http"

	"github.com/bytedance/sonic/ast"
	"github.com/labring/aiproxy/core/relay/meta"
)

// ReplaceModelInBody replaces the "model" field in a JSON request body with
// meta.ActualModel when model mapping is active (ActualModel != OriginModel).
//
// Returns the original body unchanged when no mapping is needed, body is
// empty/nil, body is not valid JSON, or the JSON has no "model" field.
// The bool return indicates whether the body was modified, so callers can
// invalidate stale Content-Length headers.
func ReplaceModelInBody(m *meta.Meta, body io.ReadCloser) (io.ReadCloser, bool, error) {
	if m.ActualModel == m.OriginModel {
		return body, false, nil
	}

	if body == nil || body == http.NoBody {
		return http.NoBody, false, nil
	}

	raw, err := io.ReadAll(body)
	_ = body.Close()

	if err != nil {
		return nil, false, err
	}

	if len(raw) == 0 {
		return http.NoBody, false, nil
	}

	unchanged := io.NopCloser(bytes.NewReader(raw))

	node, parseErr := ast.NewParser(string(raw)).Parse()
	if parseErr != 0 {
		return unchanged, false, nil
	}

	modelNode := node.Get("model")
	if modelNode == nil || modelNode.Check() != nil {
		return unchanged, false, nil
	}

	if _, setErr := node.Set("model", ast.NewString(m.ActualModel)); setErr != nil {
		return unchanged, false, nil
	}

	out, marshalErr := node.MarshalJSON()
	if marshalErr != nil {
		return unchanged, false, nil
	}

	return io.NopCloser(bytes.NewReader(out)), true, nil
}
