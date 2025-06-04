package mcpservers

import (
	"errors"
	"fmt"
)

type ConfigValueValidator func(value string) error

type ConfigRequiredType int

const (
	ConfigRequiredTypeInitOptional ConfigRequiredType = iota
	ConfigRequiredTypeReusingOptional
	ConfigRequiredTypeInitOnly
	ConfigRequiredTypeReusingOnly
	ConfigRequiredTypeInitOrReusingOnly
)

func (c ConfigRequiredType) Validate(config, reusingConfig string) error {
	switch c {
	case ConfigRequiredTypeInitOnly:
		if config == "" {
			return errors.New("config is required")
		}
	case ConfigRequiredTypeReusingOnly:
		if reusingConfig == "" {
			return errors.New("reusing config is required")
		}
	case ConfigRequiredTypeInitOrReusingOnly:
		if config == "" && reusingConfig == "" {
			return errors.New("config or reusing config is required")
		}
		if config != "" && reusingConfig != "" {
			return errors.New(
				"config and reusing config are both provided, but only one is allowed",
			)
		}
	}
	return nil
}

type ConfigTemplate struct {
	Name        string               `json:"name"`
	Required    ConfigRequiredType   `json:"required"`
	Example     string               `json:"example,omitempty"`
	Description string               `json:"description,omitempty"`
	Validator   ConfigValueValidator `json:"-"`
}

type ConfigTemplates = map[string]ConfigTemplate

func ValidateConfigTemplatesConfig(
	ct ConfigTemplates,
	config, reusingConfig map[string]string,
) error {
	if len(ct) == 0 {
		return nil
	}

	for key, template := range ct {
		c := config[key]
		rc := reusingConfig[key]
		if err := template.Required.Validate(c, rc); err != nil {
			return fmt.Errorf("config required %s is invalid: %w", key, err)
		}
		if template.Validator != nil {
			if c != "" {
				if err := template.Validator(c); err != nil {
					return fmt.Errorf("config %s is invalid: %w", key, err)
				}
			} else if rc != "" {
				if err := template.Validator(rc); err != nil {
					return fmt.Errorf("reusing config %s is invalid: %w", key, err)
				}
			}
		}
	}

	return nil
}

func CheckConfigTemplatesValidate(ct ConfigTemplates) error {
	for key, value := range ct {
		if value.Name == "" {
			return fmt.Errorf("config %s name is required", key)
		}
		if value.Description == "" {
			return fmt.Errorf("config %s description is required", key)
		}
		if value.Example == "" || value.Validator == nil {
			continue
		}
		if err := value.Validator(value.Example); err != nil {
			return fmt.Errorf("config %s example is invalid: %w", key, err)
		}
	}
	return nil
}

type NewServerFunc func(config, reusingConfig map[string]string) (Server, error)

type McpType string

const (
	McpTypeEmbed McpType = "embed"
	McpTypeDocs  McpType = "docs"
)

type McpServer struct {
	ID              string
	Name            string
	Type            McpType
	Readme          string
	LogoURL         string
	Tags            []string
	ConfigTemplates ConfigTemplates
	newServer       NewServerFunc
}

type McpConfig func(*McpServer)

func WithReadme(readme string) McpConfig {
	return func(e *McpServer) {
		e.Readme = readme
	}
}

func WithType(t McpType) McpConfig {
	return func(e *McpServer) {
		e.Type = t
	}
}

func WithLogoURL(logoURL string) McpConfig {
	return func(e *McpServer) {
		e.LogoURL = logoURL
	}
}

func WithTags(tags []string) McpConfig {
	return func(e *McpServer) {
		e.Tags = tags
	}
}

func WithConfigTemplates(configTemplates ConfigTemplates) McpConfig {
	return func(e *McpServer) {
		e.ConfigTemplates = configTemplates
	}
}

func WithNewServerFunc(newServer NewServerFunc) McpConfig {
	return func(e *McpServer) {
		e.newServer = newServer
	}
}

func NewMcp(id, name string, mcpType McpType, opts ...McpConfig) McpServer {
	e := McpServer{
		ID:   id,
		Name: name,
		Type: mcpType,
	}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

func (e *McpServer) NewServer(config, reusingConfig map[string]string) (Server, error) {
	if err := ValidateConfigTemplatesConfig(e.ConfigTemplates, config, reusingConfig); err != nil {
		return nil, fmt.Errorf("mcp %s config is invalid: %w", e.ID, err)
	}
	return e.newServer(config, reusingConfig)
}
