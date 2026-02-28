package convert

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Options struct {
	OpenAPIFrom    string
	ServerName     string
	Version        string
	ToolNamePrefix string
	ServerAddr     string
	Authorization  string
}

// Converter represents an OpenAPI to MCP converter
type Converter struct {
	parser  *Parser
	options Options
}

// NewConverter creates a new OpenAPI to MCP converter
func NewConverter(parser *Parser, options Options) *Converter {
	return &Converter{
		parser:  parser,
		options: options,
	}
}

// Convert converts an OpenAPI document to an MCP configuration
func (c *Converter) Convert() (*server.MCPServer, error) {
	if c.parser.GetDocument() == nil {
		return nil, errors.New("no OpenAPI document loaded")
	}

	info := c.parser.GetInfo()
	if info == nil {
		return nil, errors.New("no info found in OpenAPI document")
	}

	if c.options.ServerName == "" {
		c.options.ServerName = info.Title
	}

	if c.options.Version == "" {
		c.options.Version = info.Version
	}

	// Create the MCP configuration
	mcpServer := server.NewMCPServer(
		c.options.ServerName,
		c.options.Version,
	)

	defaultServer := c.options.ServerAddr
	// Use custom server address if provided
	if defaultServer == "" {
		servers := c.parser.GetServers()
		if len(servers) == 1 {
			server, err := getServerURL(c.options.OpenAPIFrom, servers[0].URL)
			if err != nil {
				return nil, err
			}

			defaultServer = server
		} else if len(servers) == 0 {
			defaultServer, _ = getServerURL(c.options.OpenAPIFrom, "")
		}
	}

	// Process each path and operation
	for path, pathItem := range c.parser.GetPaths().Map() {
		operations := getOperations(pathItem)
		for method, operation := range operations {
			tool := c.convertOperation(path, method, operation)
			handler := newHandler(defaultServer, c.options.Authorization, path, method, operation)
			mcpServer.AddTool(*tool, handler)
		}
	}

	return mcpServer, nil
}

func getServerURL(from, dir string) (string, error) {
	if from == "" {
		return dir, nil
	}

	if strings.HasPrefix(dir, "http://") ||
		strings.HasPrefix(dir, "https://") {
		return dir, nil
	}

	if !strings.HasPrefix(from, "http://") &&
		!strings.HasPrefix(from, "https://") {
		return dir, nil
	}

	result, err := url.Parse(from)
	if err != nil {
		return "", err
	}

	result.Path = dir
	result.RawQuery = ""

	return result.String(), nil
}

// TODO: valid operation
func newHandler(
	defaultServer, authorization, path, method string,
	_ *openapi3.Operation,
) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		arg := getArgs(request.GetArguments())

		// Build the URL
		serverURL := arg.ServerAddr
		if serverURL == "" {
			serverURL = defaultServer
		}

		// Replace path parameters
		finalPath := path
		for paramName, paramValue := range arg.Path {
			finalPath = strings.ReplaceAll(
				finalPath,
				"{"+paramName+"}",
				fmt.Sprintf("%v", paramValue),
			)
		}

		// Build the full URL with query parameters
		fullURL, err := url.JoinPath(serverURL, finalPath)
		if err != nil {
			return nil, fmt.Errorf("failed to join URL path %s: %w", fullURL, err)
		}

		parsedURL, err := url.Parse(fullURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL %s: %w", fullURL, err)
		}

		// Add query parameters
		if len(arg.Query) > 0 {
			q := parsedURL.Query()
			for key, value := range arg.Query {
				q.Add(key, fmt.Sprintf("%v", value))
			}

			parsedURL.RawQuery = q.Encode()
		}

		// Create the request body if needed
		var reqBody io.Reader
		if arg.Body != nil {
			bodyBytes, err := sonic.Marshal(arg.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}

			reqBody = bytes.NewBuffer(bodyBytes)
		}

		// Create the HTTP request
		httpReq, err := http.NewRequestWithContext(
			ctx,
			strings.ToUpper(method),
			parsedURL.String(),
			reqBody,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}

		// Add headers
		for key, value := range arg.Headers {
			httpReq.Header.Add(key, fmt.Sprintf("%v", value))
		}

		// Set content type for requests with body
		if arg.BodyContentType != "" {
			httpReq.Header.Set("Content-Type", arg.BodyContentType)
		} else if arg.Body != nil {
			httpReq.Header.Set("Content-Type", "application/json")
		}

		// Add authentication if provided
		switch {
		case authorization != "":
			httpReq.Header.Set("Authorization", authorization)
		case arg.AuthToken != "":
			httpReq.Header.Set("Authorization", "Bearer "+arg.AuthToken)
		case arg.AuthUsername != "" && arg.AuthPassword != "":
			httpReq.SetBasicAuth(arg.AuthUsername, arg.AuthPassword)
		case arg.AuthOAuth2Token != "":
			httpReq.Header.Set("Authorization", "Bearer "+arg.AuthOAuth2Token)
		}

		// For form data
		if len(arg.Forms) > 0 {
			formData := url.Values{}
			for key, value := range arg.Forms {
				switch value := value.(type) {
				case map[string]any:
					jsonStr, err := sonic.Marshal(value)
					if err != nil {
						return nil, err
					}

					formData.Add(key, string(jsonStr))
				default:
					formData.Add(key, fmt.Sprintf("%v", value))
				}
			}

			httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			httpReq.Body = io.NopCloser(strings.NewReader(formData.Encode()))
		}

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		buf := bytes.NewBuffer(nil)

		err = resp.Write(buf)
		if err != nil {
			return nil, fmt.Errorf("read response error: %w", err)
		}

		return mcp.NewToolResultText(buf.String()), nil
	}
}

type Args struct {
	ServerAddr      string
	AuthToken       string
	AuthUsername    string
	AuthPassword    string
	AuthOAuth2Token string
	Headers         map[string]any
	Body            any
	BodyContentType string
	Query           map[string]any
	Path            map[string]any
	Forms           map[string]any
}

func getArgs(args map[string]any) Args {
	arg := Args{
		Headers: make(map[string]any),
		Query:   make(map[string]any),
		Path:    make(map[string]any),
		Forms:   make(map[string]any),
	}
	for k, v := range args {
		switch {
		case strings.HasPrefix(k, "openapi|"):
			switch strings.TrimPrefix(k, "openapi|") {
			case "server_addr":
				arg.ServerAddr, _ = v.(string)
			case "auth_token":
				arg.AuthToken, _ = v.(string)
			case "auth_username":
				arg.AuthUsername, _ = v.(string)
			case "auth_password":
				arg.AuthPassword, _ = v.(string)
			case "auth_oauth2_token":
				arg.AuthOAuth2Token, _ = v.(string)
			}
		case k == "body":
			arg.Body = v
		case strings.HasPrefix(k, "body|"):
			arg.Body = v
			arg.BodyContentType = strings.TrimPrefix(k, "body|")
		case strings.HasPrefix(k, "query|"):
			arg.Query[strings.TrimPrefix(k, "query|")] = v
		case strings.HasPrefix(k, "path|"):
			arg.Path[strings.TrimPrefix(k, "path|")] = v
		case strings.HasPrefix(k, "header|"):
			arg.Headers[strings.TrimPrefix(k, "header|")] = v
		case strings.HasPrefix(k, "formData|"):
			arg.Forms[strings.TrimPrefix(k, "formData|")] = v
		}
	}

	return arg
}

// getOperations returns a map of HTTP method to operation
func getOperations(pathItem *openapi3.PathItem) map[string]*openapi3.Operation {
	operations := make(map[string]*openapi3.Operation)

	if pathItem.Get != nil {
		operations[http.MethodGet] = pathItem.Get
	}

	if pathItem.Post != nil {
		operations[http.MethodPost] = pathItem.Post
	}

	if pathItem.Put != nil {
		operations[http.MethodPut] = pathItem.Put
	}

	if pathItem.Delete != nil {
		operations[http.MethodDelete] = pathItem.Delete
	}

	if pathItem.Options != nil {
		operations[http.MethodOptions] = pathItem.Options
	}

	if pathItem.Head != nil {
		operations[http.MethodHead] = pathItem.Head
	}

	if pathItem.Patch != nil {
		operations[http.MethodPatch] = pathItem.Patch
	}

	if pathItem.Trace != nil {
		operations[http.MethodTrace] = pathItem.Trace
	}

	return operations
}

// convertOperation converts an OpenAPI operation to an MCP tool
func (c *Converter) convertOperation(path, method string, operation *openapi3.Operation) *mcp.Tool {
	// Generate a tool name
	toolName := c.parser.GetOperationID(path, method, operation)
	if c.options.ToolNamePrefix != "" {
		toolName = c.options.ToolNamePrefix + toolName
	}

	args := c.convertParameters(operation.Parameters)

	// Handle request body if present
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		bodyArgs := c.convertRequestBody(operation.RequestBody.Value)
		args = append(args, bodyArgs...)
	}

	// Add server address parameter
	servers := c.parser.GetServers()
	switch {
	case c.options.ServerAddr != "":
		// Use custom server address from options
		args = append(args, mcp.WithString("openapi|server_addr",
			mcp.Description("Server address to connect to"),
			mcp.DefaultString(c.options.ServerAddr)))
	case len(servers) == 0:
		if c.options.OpenAPIFrom != "" {
			u, err := getServerURL(c.options.OpenAPIFrom, "")
			if err != nil {
				u = c.options.OpenAPIFrom
			}

			args = append(args, mcp.WithString("openapi|server_addr",
				mcp.Description("Server address to connect to, example: "+u),
				mcp.Required()))
		} else {
			args = append(args, mcp.WithString("openapi|server_addr",
				mcp.Description("Server address to connect to"),
				mcp.Required()))
		}
	case len(servers) == 1:
		u, err := getServerURL(c.options.OpenAPIFrom, servers[0].URL)
		if err != nil {
			u = servers[0].URL
		}

		args = append(args, mcp.WithString("openapi|server_addr",
			mcp.Description("Server address to connect to"),
			mcp.DefaultString(u)))
	default:
		serverUrls := make([]string, 0, len(servers))
		for _, server := range servers {
			u, err := getServerURL(c.options.OpenAPIFrom, server.URL)
			if err != nil {
				u = server.URL
			}

			serverUrls = append(serverUrls, u)
		}

		args = append(args, mcp.WithString("openapi|server_addr",
			mcp.Description("Server address to connect to"),
			mcp.Required(),
			mcp.Enum(serverUrls...)))
	}

	// Handle security requirements if present and enabled
	if c.options.Authorization != "" {
		// Use custom authorization from options
		args = append(args, mcp.WithString("header|Authorization",
			mcp.Description("Authorization header"),
			mcp.DefaultString(c.options.Authorization)))
	} else if operation.Security != nil && len(*operation.Security) > 0 {
		securityArgs := c.convertSecurityRequirements(*operation.Security)
		args = append(args, securityArgs...)
	}

	// Create description that includes summary, description, and response information
	description := getDescription(operation)

	// Add response information to description
	// if operation.Responses != nil {
	// 	responseDesc := c.generateResponseDescription(*operation.Responses)
	// 	if responseDesc != "" {
	// 		description += "\n\nResponses:\n\n" + responseDesc
	// 	}
	// }

	args = append(args, mcp.WithDescription(description))

	tool := mcp.NewTool(toolName,
		args...,
	)

	return &tool
}

// generateResponseDescription creates a human-readable description of possible responses
//
//nolint:unused
func (c *Converter) generateResponseDescription(responses openapi3.Responses) string {
	respMap := responses.Map()
	responseDescriptions := make([]string, 0, len(respMap))

	for code, responseRef := range respMap {
		if responseRef == nil || responseRef.Value == nil {
			continue
		}

		response := responseRef.Value

		var desc strings.Builder
		fmt.Fprintf(&desc, "- status: %s, description: %s", code, *response.Description)

		rawSchema, ok := response.Extensions["schema"].(map[string]any)
		if ok && len(rawSchema) > 0 {
			jsonStr, err := sonic.Marshal(rawSchema)
			if err != nil {
				continue
			}

			schema := openapi3.Schema{}

			err = schema.UnmarshalJSON(jsonStr)
			if err != nil {
				continue
			}

			property := c.processSchemaProperty(&schema, make(map[string]bool))

			str, err := sonic.Marshal(property)
			if err != nil {
				continue
			}

			fmt.Fprintf(&desc, ", schema: %s", str)
		}

		if len(response.Content) > 0 {
			for contentType, mediaType := range response.Content {
				if mediaType.Schema != nil && mediaType.Schema.Value != nil {
					property := c.processSchemaProperty(
						mediaType.Schema.Value,
						make(map[string]bool),
					)

					str, err := sonic.Marshal(property)
					if err != nil {
						continue
					}

					fmt.Fprintf(&desc, ", content type: %s, schema: %s", contentType, str)
				}
			}
		}

		responseDescriptions = append(responseDescriptions, desc.String())
	}

	return strings.Join(responseDescriptions, "\n\n")
}

// convertSecurityRequirements converts OpenAPI security requirements to MCP arguments
func (c *Converter) convertSecurityRequirements(
	securityRequirements openapi3.SecurityRequirements,
) []mcp.ToolOption {
	args := []mcp.ToolOption{}

	var securitySchemes openapi3.SecuritySchemes

	// Get security definitions from the document
	components := c.parser.GetDocument().Components
	if components != nil {
		securitySchemes = components.SecuritySchemes
	}

	// Process each security requirement
	for _, requirement := range securityRequirements {
		for schemeName, scopes := range requirement {
			var schemeRef *openapi3.SecuritySchemeRef

			if securitySchemes != nil {
				schemeRef = securitySchemes[schemeName]
			} else {
				args = append(args, mcp.WithString("header|Authorization",
					mcp.Description(fmt.Sprintf("API Key for %s authentication",
						schemeName)),
					mcp.Required()))
			}

			if schemeRef == nil || schemeRef.Value == nil {
				continue
			}

			scheme := schemeRef.Value
			switch scheme.Type {
			case "apiKey":
				args = append(args, mcp.WithString(scheme.In+"|"+scheme.Name,
					mcp.Description(fmt.Sprintf("API Key for %s authentication",
						schemeName)),
					mcp.Required()))
			case "http":
				switch scheme.Scheme {
				case "basic":
					args = append(args, mcp.WithString("openapi|auth_username",
						mcp.Description("Username for Basic authentication"),
						mcp.Required()))
					args = append(args, mcp.WithString("openapi|auth_password",
						mcp.Description("Password for Basic authentication"),
						mcp.Required()))
				case "bearer":
					args = append(args, mcp.WithString("openapi|auth_token",
						mcp.Description("Bearer token for authentication"),
						mcp.Required()))
				}
			case "oauth2":
				if len(scopes) > 0 {
					scopeDesc := "OAuth2 token with scopes: " + strings.Join(scopes, ", ")
					args = append(args, mcp.WithString("openapi|auth_oauth2_token",
						mcp.Description(scopeDesc),
						mcp.Required()))
				} else {
					args = append(args, mcp.WithString("openapi|auth_oauth2_token",
						mcp.Description("OAuth2 token for authentication"),
						mcp.Required()))
				}
			}
		}
	}

	return args
}

// convertRequestBody converts an OpenAPI request body to MCP arguments
func (c *Converter) convertRequestBody(requestBody *openapi3.RequestBody) []mcp.ToolOption {
	args := []mcp.ToolOption{}

	for contentType, mediaType := range requestBody.Content {
		if mediaType.Schema == nil || mediaType.Schema.Value == nil {
			continue
		}

		schema := mediaType.Schema.Value
		propertyOptions := []mcp.PropertyOption{}

		if requestBody.Description != "" {
			propertyOptions = append(propertyOptions, mcp.Description(requestBody.Description))
		}

		if requestBody.Required {
			propertyOptions = append(propertyOptions, mcp.Required())
		}

		t := PropertyTypeObject
		if schema.Type != nil {
			switch {
			case schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil:
				t = PropertyTypeArray
				item := c.processSchemaItems(schema.Items.Value, make(map[string]bool))
				propertyOptions = append(propertyOptions, mcp.Items(item))
			case schema.Type.Is("object") || len(schema.Properties) > 0:
				obj := c.processSchemaProperties(schema, make(map[string]bool))
				propertyOptions = append(propertyOptions, mcp.Properties(obj))
			case schema.Type.Is("string"):
				t = PropertyTypeString
			case schema.Type.Is("integer"):
				t = PropertyTypeInteger
			case schema.Type.Is("number"):
				t = PropertyTypeNumber
			case schema.Type.Is("boolean"):
				t = PropertyTypeBoolean
			}
		}

		// Add content type as part of the parameter name
		args = append(args, c.createToolOption(t, "body|"+contentType, propertyOptions...))
	}

	return args
}

type propertyType string

const (
	PropertyTypeString  propertyType = "string"
	PropertyTypeInteger propertyType = "integer"
	PropertyTypeNumber  propertyType = "number"
	PropertyTypeBoolean propertyType = "boolean"
	PropertyTypeObject  propertyType = "object"
	PropertyTypeArray   propertyType = "array"
)

// convertParameters converts OpenAPI parameters to MCP arguments
func (c *Converter) convertParameters(parameters openapi3.Parameters) []mcp.ToolOption {
	args := []mcp.ToolOption{}

	for _, paramRef := range parameters {
		param := paramRef.Value
		if param == nil {
			continue
		}

		propertyOptions := []mcp.PropertyOption{
			mcp.Description(param.Description),
		}

		if param.Required {
			propertyOptions = append(propertyOptions, mcp.Required())
		}

		t := PropertyTypeString
		if param.Schema != nil && param.Schema.Value != nil {
			schema := param.Schema.Value

			// Determine property type and add specific options
			switch {
			case schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil:
				t = PropertyTypeArray
				item := c.processSchemaItems(schema.Items.Value, make(map[string]bool))
				propertyOptions = append(propertyOptions, mcp.Items(item))
			case schema.Type.Is("object") && len(schema.Properties) > 0:
				t = PropertyTypeObject
				obj := c.processSchemaProperties(schema, make(map[string]bool))
				propertyOptions = append(propertyOptions, mcp.Properties(obj))
			case schema.Type.Is("integer"):
				t = PropertyTypeInteger
			case schema.Type.Is("number"):
				t = PropertyTypeNumber
			case schema.Type.Is("boolean"):
				t = PropertyTypeBoolean
			}

			// Add enum values if present
			if len(schema.Enum) > 0 {
				enumValues := make([]string, 0, len(schema.Enum))
				for _, val := range schema.Enum {
					if strVal, ok := val.(string); ok {
						enumValues = append(enumValues, strVal)
					} else {
						// Convert non-string values to string
						enumValues = append(enumValues, fmt.Sprintf("%v", val))
					}
				}

				if len(enumValues) > 0 {
					propertyOptions = append(propertyOptions, mcp.Enum(enumValues...))
				}
			}

			// Add example if present
			if schema.Example != nil {
				propertyOptions = append(
					propertyOptions,
					mcp.DefaultString(fmt.Sprintf("%v", schema.Example)),
				)
			}
		}

		// Add the parameter based on its type
		if param.In == "body" {
			args = append(args, c.createToolOption(t, param.In, propertyOptions...))
		} else {
			args = append(args, c.createToolOption(t, param.In+"|"+param.Name, propertyOptions...))
		}
	}

	return args
}

// processSchemaItems processes schema items for array types
func (c *Converter) processSchemaItems(
	schema *openapi3.Schema,
	visited map[string]bool,
) map[string]any {
	item := make(map[string]any)

	if schema.Type != nil {
		item["type"] = schema.Type
	}

	if schema.Description != "" {
		item["description"] = schema.Description
	}

	// Process nested properties if this is an object
	if len(schema.Properties) > 0 {
		properties := make(map[string]any)
		for propName, propRef := range schema.Properties {
			if propRef.Value != nil {
				properties[propName] = c.processSchemaProperty(propRef.Value, visited)
			}
		}

		item["properties"] = properties
	}

	// Handle reference if this is a reference to another schema
	if schema.Items != nil && schema.Items.Value != nil {
		item["items"] = c.processSchemaItems(schema.Items.Value, visited)
	}

	return item
}

// processSchemaProperties processes schema properties for object types
func (c *Converter) processSchemaProperties(
	schema *openapi3.Schema,
	visited map[string]bool,
) map[string]any {
	obj := make(map[string]any)

	for propName, propRef := range schema.Properties {
		if propRef.Value != nil {
			obj[propName] = c.processSchemaProperty(propRef.Value, visited)
		}
	}

	return obj
}

// processSchemaProperty processes a single schema property
func (c *Converter) processSchemaProperty(
	schema *openapi3.Schema,
	visited map[string]bool,
) map[string]any {
	// Check for circular references
	if schema.Title != "" {
		refKey := schema.Title
		if visited[refKey] {
			// We've seen this schema before, return a simplified reference to avoid circular
			// references
			return map[string]any{
				"type":        "reference",
				"description": "Circular reference to " + refKey,
				"title":       refKey,
			}
		}

		visited[refKey] = true
		// Create a copy of the visited map to avoid cross-contamination between different branches
		visitedCopy := maps.Clone(visited)

		visited = visitedCopy
	}

	return c.buildPropertyMap(schema, visited)
}

// buildPropertyMap builds the property map for a schema
// This function was extracted to reduce cyclomatic complexity
func (c *Converter) buildPropertyMap(
	schema *openapi3.Schema,
	visited map[string]bool,
) map[string]any {
	property := make(map[string]any)

	// Add basic schema information
	c.addBasicSchemaInfo(schema, property)

	// Add schema validations
	c.addSchemaValidations(schema, property)

	// Add schema composition
	c.addSchemaComposition(schema, property, visited)

	// Add object properties
	c.addObjectProperties(schema, property, visited)

	// Add array items
	c.addArrayItems(schema, property, visited)

	// Add additional schema metadata
	c.addAdditionalMetadata(schema, property)

	return property
}

// addBasicSchemaInfo adds basic schema information to the property map
func (c *Converter) addBasicSchemaInfo(schema *openapi3.Schema, property map[string]any) {
	if schema.Type != nil {
		property["type"] = schema.Type
	}

	// Basic metadata
	if schema.Title != "" {
		property["title"] = schema.Title
	}

	if schema.Description != "" {
		property["description"] = schema.Description
	}

	if schema.Default != nil {
		property["default"] = schema.Default
	}

	if schema.Example != nil {
		property["example"] = schema.Example
	}

	if len(schema.Enum) > 0 {
		property["enum"] = schema.Enum
	}

	if schema.Format != "" {
		property["format"] = schema.Format
	}
}

// addSchemaValidations adds schema validations to the property map
func (c *Converter) addSchemaValidations(schema *openapi3.Schema, property map[string]any) {
	// Boolean flags
	if schema.Nullable {
		property["nullable"] = schema.Nullable
	}

	if schema.ReadOnly {
		property["readOnly"] = schema.ReadOnly
	}

	if schema.WriteOnly {
		property["writeOnly"] = schema.WriteOnly
	}

	if schema.Deprecated {
		property["deprecated"] = schema.Deprecated
	}

	if schema.AllowEmptyValue {
		property["allowEmptyValue"] = schema.AllowEmptyValue
	}

	if schema.UniqueItems {
		property["uniqueItems"] = schema.UniqueItems
	}

	if schema.ExclusiveMin {
		property["exclusiveMinimum"] = schema.ExclusiveMin
	}

	if schema.ExclusiveMax {
		property["exclusiveMaximum"] = schema.ExclusiveMax
	}

	// Number validations
	if schema.Min != nil {
		property["minimum"] = *schema.Min
	}

	if schema.Max != nil {
		property["maximum"] = *schema.Max
	}

	if schema.MultipleOf != nil {
		property["multipleOf"] = *schema.MultipleOf
	}

	// String validations
	if schema.MinLength != 0 {
		property["minLength"] = schema.MinLength
	}

	if schema.MaxLength != nil {
		property["maxLength"] = *schema.MaxLength
	}

	if schema.Pattern != "" {
		property["pattern"] = schema.Pattern
	}

	// Array validations
	if schema.MinItems != 0 {
		property["minItems"] = schema.MinItems
	}

	if schema.MaxItems != nil {
		property["maxItems"] = *schema.MaxItems
	}

	// Object validations
	if schema.MinProps != 0 {
		property["minProperties"] = schema.MinProps
	}

	if schema.MaxProps != nil {
		property["maxProperties"] = *schema.MaxProps
	}

	if len(schema.Required) > 0 {
		property["required"] = schema.Required
	}
}

// addSchemaComposition adds schema composition to the property map
func (c *Converter) addSchemaComposition(
	schema *openapi3.Schema,
	property map[string]any,
	visited map[string]bool,
) {
	// Schema composition
	if len(schema.OneOf) > 0 {
		oneOf := make([]any, 0, len(schema.OneOf))
		for _, schemaRef := range schema.OneOf {
			if schemaRef.Value != nil {
				oneOf = append(oneOf, c.processSchemaProperty(schemaRef.Value, visited))
			}
		}

		if len(oneOf) > 0 {
			property["oneOf"] = oneOf
		}
	}

	if len(schema.AnyOf) > 0 {
		anyOf := make([]any, 0, len(schema.AnyOf))
		for _, schemaRef := range schema.AnyOf {
			if schemaRef.Value != nil {
				anyOf = append(anyOf, c.processSchemaProperty(schemaRef.Value, visited))
			}
		}

		if len(anyOf) > 0 {
			property["anyOf"] = anyOf
		}
	}

	if len(schema.AllOf) > 0 {
		allOf := make([]any, 0, len(schema.AllOf))
		for _, schemaRef := range schema.AllOf {
			if schemaRef.Value != nil {
				allOf = append(allOf, c.processSchemaProperty(schemaRef.Value, visited))
			}
		}

		if len(allOf) > 0 {
			property["allOf"] = allOf
		}
	}

	if schema.Not != nil && schema.Not.Value != nil {
		property["not"] = c.processSchemaProperty(schema.Not.Value, visited)
	}
}

// addObjectProperties adds object properties to the property map
func (c *Converter) addObjectProperties(
	schema *openapi3.Schema,
	property map[string]any,
	visited map[string]bool,
) {
	// Handle AdditionalProperties
	if schema.AdditionalProperties.Has != nil {
		property["additionalProperties"] = *schema.AdditionalProperties.Has
	} else if schema.AdditionalProperties.Schema != nil && schema.AdditionalProperties.Schema.Value != nil {
		property["additionalProperties"] = c.processSchemaProperty(
			schema.AdditionalProperties.Schema.Value,
			visited,
		)
	}

	// Handle discriminator
	if schema.Discriminator != nil {
		discriminator := make(map[string]any)

		discriminator["propertyName"] = schema.Discriminator.PropertyName
		if len(schema.Discriminator.Mapping) > 0 {
			discriminator["mapping"] = schema.Discriminator.Mapping
		}

		property["discriminator"] = discriminator
	}

	// Recursively process nested objects
	if schema.Type != nil && schema.Type.Is("object") && len(schema.Properties) > 0 {
		nestedProps := make(map[string]any)
		for propName, propRef := range schema.Properties {
			if propRef.Value != nil {
				nestedProps[propName] = c.processSchemaProperty(propRef.Value, visited)
			}
		}

		property["properties"] = nestedProps
	}
}

func (c *Converter) addArrayItems(
	schema *openapi3.Schema,
	property map[string]any,
	visited map[string]bool,
) {
	// Recursively process array items
	if schema.Type != nil && schema.Type.Is("array") && schema.Items != nil &&
		schema.Items.Value != nil {
		property["items"] = c.processSchemaItems(schema.Items.Value, visited)
	}
}

func (c *Converter) addAdditionalMetadata(schema *openapi3.Schema, property map[string]any) {
	// Handle external docs if present
	if schema.ExternalDocs != nil {
		externalDocs := make(map[string]any)
		if schema.ExternalDocs.Description != "" {
			externalDocs["description"] = schema.ExternalDocs.Description
		}

		if schema.ExternalDocs.URL != "" {
			externalDocs["url"] = schema.ExternalDocs.URL
		}

		property["externalDocs"] = externalDocs
	}

	// Handle XML object if present
	if schema.XML != nil {
		xml := make(map[string]any)
		if schema.XML.Name != "" {
			xml["name"] = schema.XML.Name
		}

		if schema.XML.Namespace != "" {
			xml["namespace"] = schema.XML.Namespace
		}

		if schema.XML.Prefix != "" {
			xml["prefix"] = schema.XML.Prefix
		}

		xml["attribute"] = schema.XML.Attribute
		xml["wrapped"] = schema.XML.Wrapped
		property["xml"] = xml
	}
}

// createToolOption creates the appropriate tool option based on property type
func (c *Converter) createToolOption(
	t propertyType,
	name string,
	options ...mcp.PropertyOption,
) mcp.ToolOption {
	switch t {
	case PropertyTypeString:
		return mcp.WithString(name, options...)
	case PropertyTypeInteger:
		return mcp.WithNumber(name, options...)
	case PropertyTypeNumber:
		return mcp.WithNumber(name, options...)
	case PropertyTypeBoolean:
		return mcp.WithBoolean(name, options...)
	case PropertyTypeObject:
		return mcp.WithObject(name, options...)
	case PropertyTypeArray:
		return mcp.WithArray(name, options...)
	default:
		return mcp.WithString(name, options...)
	}
}

// getDescription returns a description for an operation
func getDescription(operation *openapi3.Operation) string {
	var parts []string

	if operation.Summary != "" {
		parts = append(parts, operation.Summary)
	}

	if operation.Description != "" {
		parts = append(parts, operation.Description)
	}

	// Add deprecated notice if applicable
	if operation.Deprecated {
		parts = append(parts, "WARNING: This operation is deprecated.")
	}

	return strings.Join(parts, "\n\n")
}
