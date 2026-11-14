package model

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// NormalizeSTTTokenUsage exposes actual upstream input/output counters under the STT
// contract. Canonical fields, including explicit zero, take precedence over chat
// aliases. Missing counters are never inferred from total_tokens, seconds or text.
// Raw messages preserve unrelated fields and numeric precision; total_tokens is unchanged.
func NormalizeSTTTokenUsage(data []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("parse STT usage: %w", err)
	}

	if fields == nil {
		return data, nil
	}

	var kind string

	_ = json.Unmarshal(fields["type"], &kind)
	if kind == SttUsageTypeDuration {
		return data, nil
	}

	for _, names := range [][2]string{{"input_tokens", "prompt_tokens"}, {"output_tokens", "completion_tokens"}} {
		raw, exists := fields[names[0]]
		if !exists {
			raw, exists = fields[names[1]]
		}

		if !exists {
			continue
		}

		var value int64
		if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &value) != nil || value < 0 ||
			value > 9007199254740991 {
			value = 0
		}

		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}

		fields[names[0]] = encoded
	}

	if _, exists := fields["input_token_details"]; !exists {
		for _, name := range []string{"input_tokens_details", "prompt_tokens_details"} {
			if value, ok := fields[name]; ok {
				fields["input_token_details"] = value
				break
			}
		}
	}

	return json.Marshal(fields)
}
