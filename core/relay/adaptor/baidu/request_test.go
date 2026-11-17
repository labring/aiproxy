//nolint:testpackage
package baidu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/relay/meta"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func TestConvertRequestSystemMessage(t *testing.T) {
	user := relaymodel.Message{Role: "user", Content: "hello"}
	assistant := relaymodel.Message{Role: "assistant", Content: "hi"}
	system := relaymodel.Message{Role: "system", Content: "be helpful"}

	for _, tc := range []struct {
		name     string
		messages []relaymodel.Message
		system   string
		want     []relaymodel.Message
	}{
		{"leading system", []relaymodel.Message{system, user}, "be helpful", []relaymodel.Message{user}},
		{"middle system", []relaymodel.Message{user, system, assistant}, "be helpful", []relaymodel.Message{user, assistant}},
		{"trailing system", []relaymodel.Message{user, system}, "be helpful", []relaymodel.Message{user}},
		{"no system", []relaymodel.Message{user, assistant}, "", []relaymodel.Message{user, assistant}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(relaymodel.GeneralOpenAIRequest{Messages: tc.messages})
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/",
				strings.NewReader(string(body)),
			)

			result, err := ConvertRequest(&meta.Meta{ActualModel: "ernie"}, req)
			if err != nil {
				t.Fatal(err)
			}

			var got ChatRequest
			if err := json.NewDecoder(result.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}

			if got.System != tc.system {
				t.Errorf("system = %q, want %q", got.System, tc.system)
			}

			if !reflect.DeepEqual(got.Messages, tc.want) {
				t.Errorf("messages = %#v, want %#v", got.Messages, tc.want)
			}
		})
	}
}
