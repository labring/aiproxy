package convert_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/labring/aiproxy/openapi-mcp/convert"
)

// newConverter parses the given OpenAPI document and returns a converter for it.
func newConverter(t *testing.T, document string) *convert.Converter {
	t.Helper()

	parser := convert.NewParser()
	if err := parser.Parse([]byte(document)); err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}

	return convert.NewConverter(parser, convert.Options{
		ServerAddr: "http://localhost:3000",
	})
}

// convertToolSchemas runs Convert and returns the input schema of every tool
// keyed by tool name. Convert must terminate for the test to reach this point.
func convertToolSchemas(t *testing.T, document string) map[string]map[string]any {
	t.Helper()

	mcpServer, err := newConverter(t, document).Convert()
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	tools := mcpServer.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected at least one converted tool")
	}

	schemas := make(map[string]map[string]any, len(tools))
	for name, tool := range tools {
		// Round trip through JSON so assertions work on the same representation
		// an MCP client receives in tools/list.
		raw, err := json.Marshal(tool.Tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal input schema of %s: %v", name, err)
		}

		schema := make(map[string]any)
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("unmarshal input schema of %s: %v", name, err)
		}

		schemas[name] = schema
	}

	return schemas
}

// toolSchema returns the JSON body schema of the single converted tool, failing
// the test when the tool or the body property is missing.
func toolSchema(t *testing.T, schemas map[string]map[string]any) map[string]any {
	t.Helper()

	// Every document built in this file declares exactly one operation.
	const toolName = "post_items"

	schema, ok := schemas[toolName]
	if !ok {
		t.Fatalf("tool %q not found, got %v", toolName, schemaNames(schemas))
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("tool %q has no properties: %v", toolName, schema)
	}

	body, ok := properties["body|application/json"].(map[string]any)
	if !ok {
		t.Fatalf("tool %q has no JSON body property: %v", toolName, properties)
	}

	return body
}

func schemaNames(schemas map[string]map[string]any) []string {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}

	return names
}

// bodyProperty walks to properties[path...] below the given schema map.
func bodyProperty(t *testing.T, schema map[string]any, path ...string) map[string]any {
	t.Helper()

	current := schema
	for _, name := range path {
		properties, ok := current["properties"].(map[string]any)
		if !ok {
			t.Fatalf("missing properties while looking for %q in %v", name, current)
		}

		next, ok := properties[name].(map[string]any)
		if !ok {
			t.Fatalf("missing property %q in %v", name, properties)
		}

		current = next
	}

	return current
}

// itemsOf returns the items schema of an array schema.
func itemsOf(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()

	items, ok := schema["items"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no items: %v", schema)
	}

	return items
}

func assertCircular(t *testing.T, schema map[string]any, ref string) {
	t.Helper()

	description, _ := schema["description"].(string)
	if !strings.Contains(description, "Circular reference to "+ref) {
		t.Fatalf("expected circular reference to %q, got %v", ref, schema)
	}

	if title, _ := schema["title"].(string); title != ref {
		t.Fatalf("expected circular reference title %q, got %v", ref, schema)
	}

	if _, ok := schema["properties"]; ok {
		t.Fatalf("circular reference must not be expanded: %v", schema)
	}
}

// findCircular walks the generated schema and returns the first node that was
// truncated because of a circular reference, or nil when there is none.
func findCircular(node any) map[string]any {
	switch value := node.(type) {
	case map[string]any:
		if description, ok := value["description"].(string); ok &&
			strings.HasPrefix(description, "Circular reference to ") {
			return value
		}

		for _, child := range value {
			if found := findCircular(child); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range value {
			if found := findCircular(child); found != nil {
				return found
			}
		}
	}

	return nil
}

const documentHeader = `{
  "openapi": "3.0.0",
  "info": {"title": "test", "version": "1.0.0"},
  "servers": [{"url": "http://localhost:3000"}],
  "paths": {
    "/items": {
      "post": {
        "operationId": "post_items",
        "requestBody": {
          "content": {
            "application/json": {"schema": {"$ref": "#/components/schemas/REQUEST_SCHEMA"}}
          }
        },
        "responses": {"200": {"description": "ok"}}
      }
    }
  },
  "components": {"schemas": SCHEMAS}
}`

// buildDocument renders a document whose request body references requestSchema
// and whose components are the given raw JSON object entries.
func buildDocument(requestSchema, components string) string {
	document := strings.Replace(documentHeader, "REQUEST_SCHEMA", requestSchema, 1)

	return strings.Replace(document, "SCHEMAS", components, 1)
}

// TestConvertCircularReferences reproduces the OOM reported for the built-in
// aiproxy-openapi server: the Swagger document defines model.Price and
// model.ConditionalPrice in a cycle and none of the definitions has a title, so
// the title based detection never triggered.
func TestConvertCircularReferences(t *testing.T) {
	t.Parallel()

	document := buildDocument("Price", `{
      "Price": {
        "type": "object",
        "properties": {
          "input_price": {"type": "number"},
          "conditional_prices": {
            "type": "array",
            "items": {"$ref": "#/components/schemas/ConditionalPrice"}
          }
        }
      },
      "ConditionalPrice": {
        "type": "object",
        "properties": {
          "price": {"$ref": "#/components/schemas/Price"}
        }
      }
    }`)

	body := toolSchema(t, convertToolSchemas(t, document))

	// Non-circular fields must still be expanded.
	inputPrice := bodyProperty(t, body, "input_price")
	if inputPrice["type"] != "number" {
		t.Fatalf("expected input_price to be a number, got %v", inputPrice)
	}

	// Price -> conditional_prices[] -> ConditionalPrice -> price is the cycle.
	// Price is the request body root, so the nested ConditionalPrice.price is
	// truncated and the recursion stops there.
	conditionalPrice := itemsOf(t, bodyProperty(t, body, "conditional_prices"))
	price := bodyProperty(t, conditionalPrice, "price")
	assertCircular(t, price, "#/components/schemas/Price")
}

func TestConvertSelfReferencingSchema(t *testing.T) {
	t.Parallel()

	document := buildDocument("Node", `{
      "Node": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "parent": {"$ref": "#/components/schemas/Node"}
        }
      }
    }`)

	body := toolSchema(t, convertToolSchemas(t, document))

	if name := bodyProperty(t, body, "name"); name["type"] != "string" {
		t.Fatalf("expected name to be a string, got %v", name)
	}

	assertCircular(t, findCircular(body), "#/components/schemas/Node")
}

func TestConvertCircularArrayItems(t *testing.T) {
	t.Parallel()

	document := buildDocument("Tree", `{
      "Tree": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "label": {"type": "string"},
            "children": {
              "type": "array",
              "items": {"$ref": "#/components/schemas/Tree"}
            }
          }
        }
      }
    }`)

	body := toolSchema(t, convertToolSchemas(t, document))

	item := itemsOf(t, body)
	if label := bodyProperty(t, item, "label"); label["type"] != "string" {
		t.Fatalf("expected label to be a string, got %v", label)
	}

	// The recursion goes through processSchemaItems rather than
	// processSchemaProperty, so this asserts that path is covered too.
	children := bodyProperty(t, item, "children")
	assertCircular(t, findCircular(itemsOf(t, children)), "#/components/schemas/Tree")
}

func TestConvertCircularAdditionalProperties(t *testing.T) {
	t.Parallel()

	document := buildDocument("Dictionary", `{
      "Dictionary": {
        "type": "object",
        "properties": {
          "entries": {"$ref": "#/components/schemas/Entries"}
        }
      },
      "Entries": {
        "type": "object",
        "additionalProperties": {"$ref": "#/components/schemas/Entries"}
      }
    }`)

	body := toolSchema(t, convertToolSchemas(t, document))

	entries := bodyProperty(t, body, "entries")

	additional, ok := entries["additionalProperties"].(map[string]any)
	if !ok {
		t.Fatalf("expected additionalProperties schema, got %v", entries)
	}

	assertCircular(t, additional, "#/components/schemas/Entries")
}

func TestConvertCircularComposition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		components string
	}{
		{
			name: "allOf",
			components: `{
        "Wrapper": {
          "allOf": [
            {"$ref": "#/components/schemas/Base"},
            {"$ref": "#/components/schemas/Wrapper"}
          ]
        },
        "Base": {"type": "object", "properties": {"id": {"type": "string"}}}
      }`,
		},
		{
			name: "oneOf",
			components: `{
        "Wrapper": {
          "oneOf": [
            {"$ref": "#/components/schemas/Base"},
            {"$ref": "#/components/schemas/Wrapper"}
          ]
        },
        "Base": {"type": "object", "properties": {"id": {"type": "string"}}}
      }`,
		},
		{
			name: "anyOf",
			components: `{
        "Wrapper": {
          "anyOf": [
            {"$ref": "#/components/schemas/Base"},
            {"$ref": "#/components/schemas/Wrapper"}
          ]
        },
        "Base": {"type": "object", "properties": {"id": {"type": "string"}}}
      }`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			document := buildDocument("Wrapper", test.components)
			body := toolSchema(t, convertToolSchemas(t, document))

			// A composition-only body keeps its structure under
			// additionalProperties because it has neither a type nor properties.
			inner, ok := body["additionalProperties"].(map[string]any)
			if !ok {
				t.Fatalf("expected composition schema, got %v", body)
			}

			composed, ok := inner[test.name].([]any)
			if !ok || len(composed) != 2 {
				t.Fatalf("expected two %s entries, got %v", test.name, inner)
			}

			// The first branch is a regular schema and must stay expanded.
			base, ok := composed[0].(map[string]any)
			if !ok {
				t.Fatalf("expected %s entry to be an object, got %v", test.name, composed[0])
			}

			if id := bodyProperty(t, base, "id"); id["type"] != "string" {
				t.Fatalf("expected Base.id to be a string, got %v", id)
			}

			// The second branch points back at the composition root.
			circular, ok := composed[1].(map[string]any)
			if !ok {
				t.Fatalf("expected %s entry to be an object, got %v", test.name, composed[1])
			}

			assertCircular(t, circular, "#/components/schemas/Wrapper")
		})
	}
}

// TestConvertSiblingReferencesAreNotCycles guards the branch scoping of the
// visited set: two sibling properties referencing the same schema are not a
// cycle and both must be expanded.
func TestConvertSiblingReferencesAreNotCycles(t *testing.T) {
	t.Parallel()

	document := buildDocument("Pair", `{
      "Pair": {
        "type": "object",
        "properties": {
          "left": {"$ref": "#/components/schemas/Leaf"},
          "right": {"$ref": "#/components/schemas/Leaf"}
        }
      },
      "Leaf": {
        "type": "object",
        "properties": {"value": {"type": "string"}}
      }
    }`)

	body := toolSchema(t, convertToolSchemas(t, document))

	for _, name := range []string{"left", "right"} {
		leaf := bodyProperty(t, body, name)
		if _, ok := leaf["description"]; ok {
			t.Fatalf("%s must be expanded, got %v", name, leaf)
		}

		if value := bodyProperty(t, leaf, "value"); value["type"] != "string" {
			t.Fatalf("expected %s.value to be a string, got %v", name, value)
		}
	}
}

func TestConvertPlainSchema(t *testing.T) {
	t.Parallel()

	document := buildDocument("Plain", `{
      "Plain": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {"type": "string", "description": "the name"},
          "count": {"type": "integer"},
          "tags": {"type": "array", "items": {"type": "string"}}
        }
      }
    }`)

	body := toolSchema(t, convertToolSchemas(t, document))

	if body["type"] != "object" {
		t.Fatalf("expected object body, got %v", body)
	}

	if name := bodyProperty(t, body, "name"); name["type"] != "string" ||
		name["description"] != "the name" {
		t.Fatalf("unexpected name schema: %v", name)
	}

	if count := bodyProperty(t, body, "count"); count["type"] != "integer" {
		t.Fatalf("unexpected count schema: %v", count)
	}

	tags := bodyProperty(t, body, "tags")
	if tags["type"] != "array" {
		t.Fatalf("unexpected tags schema: %v", tags)
	}

	if item := itemsOf(t, tags); item["type"] != "string" {
		t.Fatalf("unexpected tags items schema: %v", item)
	}
}
