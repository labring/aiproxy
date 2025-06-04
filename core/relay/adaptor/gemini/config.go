package gemini

import (
	"fmt"

	"github.com/labring/aiproxy/core/relay/adaptor"
)

type Config struct {
	Safety string `json:"safety"`
}

var ConfigTemplates = adaptor.ConfigTemplates{
	"safety": {
		Name:        "Safety",
		Description: "Safety settings: https://ai.google.dev/gemini-api/docs/safety-settings, default is BLOCK_NONE",
		Example:     "BLOCK_NONE",
		Type:        adaptor.ConfigTypeString,
		Validator: func(a any) error {
			s, ok := a.(string)
			if !ok {
				return fmt.Errorf("invalid safety settings type: %v, must be a string", a)
			}
			switch s {
			case "BLOCK_NONE",
				"BLOCK_ONLY_HIGH",
				"BLOCK_MEDIUM_AND_ABOVE",
				"BLOCK_LOW_AND_ABOVE",
				"HARM_BLOCK_THRESHOLD_UNSPECIFIED":
				return nil
			default:
				return fmt.Errorf(
					"invalid safety settings: %s, must be one of: BLOCK_NONE, BLOCK_ONLY_HIGH, BLOCK_MEDIUM_AND_ABOVE, BLOCK_LOW_AND_ABOVE, HARM_BLOCK_THRESHOLD_UNSPECIFIED",
					s,
				)
			}
		},
	},
}

func (a *Adaptor) ConfigTemplates() adaptor.ConfigTemplates {
	return ConfigTemplates
}
