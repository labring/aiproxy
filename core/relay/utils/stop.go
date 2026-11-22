package utils

// ParseOpenAIStopSequences maps the OpenAI stop field (string or array)
// onto a string slice for providers that take stop sequences as a list.
// Non-strings inside an array are ignored; absent or empty input yields nil.
func ParseOpenAIStopSequences(stop any) []string {
	switch v := stop.(type) {
	case string:
		return []string{v}
	case []string:
		if len(v) == 0 {
			return nil
		}

		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}

			out = append(out, s)
		}

		if len(out) == 0 {
			return nil
		}

		return out
	default:
		return nil
	}
}
