package aiproxyopenapi

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/labring/aiproxy/core/docs"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/labring/aiproxy/openapi-mcp/convert"
)

var configTemplates = map[string]mcpservers.ConfigTemplate{
	"host": {
		Name:        "Host",
		Required:    mcpservers.ConfigRequiredTypeInitOnly,
		Example:     "http://localhost:3000",
		Description: "The host of the OpenAPI server",
		Validator: func(value string) error {
			u, err := url.Parse(value)
			if err != nil {
				return err
			}

			if u.Scheme != "http" && u.Scheme != "https" {
				return fmt.Errorf("invalid scheme: %s", u.Scheme)
			}

			return nil
		},
	},

	"authorization": {
		Name:        "Authorization",
		Required:    mcpservers.ConfigRequiredTypeReusingOptional,
		Example:     "aiproxy-admin-key",
		Description: "The admin key of the OpenAPI server",
	},
}

var (
	parser    *convert.Parser
	parseOnce sync.Once
)

func getParser() *convert.Parser {
	parseOnce.Do(func() {
		parser = convert.NewParser()

		err := parser.Parse([]byte(docs.SwaggerInfo.ReadDoc()))
		if err != nil {
			panic(err)
		}
	})

	return parser
}

func NewServer(config, reusingConfig map[string]string) (mcpservers.Server, error) {
	converter := convert.NewConverter(getParser(), convert.Options{
		OpenAPIFrom:   config["host"],
		Authorization: reusingConfig["authorization"],
	})

	return converter.Convert()
}
