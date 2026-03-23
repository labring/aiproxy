package convert

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestProcessSchemaPropertyCircularNoTitle verifies that circular references
// between schemas WITHOUT Title fields are handled gracefully (no stack overflow).
func TestProcessSchemaPropertyCircularNoTitle(t *testing.T) {
	c := &Converter{}

	// Build two schemas that reference each other with no Title
	schemaA := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
	}
	schemaB := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
	}

	// A.properties.b -> B, B.properties.a -> A (circular)
	schemaA.Properties = openapi3.Schemas{
		"b": &openapi3.SchemaRef{Value: schemaB},
	}
	schemaB.Properties = openapi3.Schemas{
		"a": &openapi3.SchemaRef{Value: schemaA},
	}

	// This previously caused a stack overflow; now it should return safely.
	result := c.processSchemaProperty(schemaA, newSchemaContext())
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	t.Logf("circular schema result: %v", result)
}

// TestProcessSchemaPropertyDeeplyNested verifies that very deep nesting
// is bounded by maxSchemaDepth instead of causing a stack overflow.
func TestProcessSchemaPropertyDeeplyNested(t *testing.T) {
	c := &Converter{}

	// Build a linear chain deeper than maxSchemaDepth
	depth := maxSchemaDepth + 10
	schemas := make([]*openapi3.Schema, depth)
	for i := range schemas {
		schemas[i] = &openapi3.Schema{
			Type: &openapi3.Types{"object"},
		}
	}

	for i := 0; i < depth-1; i++ {
		schemas[i].Properties = openapi3.Schemas{
			"child": &openapi3.SchemaRef{Value: schemas[i+1]},
		}
	}

	result := c.processSchemaProperty(schemas[0], newSchemaContext())
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	t.Logf("deep nesting result type: %v", result["type"])
}

// TestProcessSchemaPropertySelfRef verifies that a schema referencing itself
// is handled correctly.
func TestProcessSchemaPropertySelfRef(t *testing.T) {
	c := &Converter{}

	schema := &openapi3.Schema{
		Type:  &openapi3.Types{"object"},
		Title: "TreeNode",
	}
	schema.Properties = openapi3.Schemas{
		"children": &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Type: &openapi3.Types{"array"},
				Items: &openapi3.SchemaRef{
					Value: schema, // self-reference
				},
			},
		},
	}

	result := c.processSchemaProperty(schema, newSchemaContext())
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	t.Logf("self-ref result: %v", result)
}

// TestProcessSchemaItemsCircular verifies that processSchemaItems handles
// circular array item references.
func TestProcessSchemaItemsCircular(t *testing.T) {
	c := &Converter{}

	schema := &openapi3.Schema{
		Type: &openapi3.Types{"array"},
	}
	// Array whose items are itself
	schema.Items = &openapi3.SchemaRef{Value: schema}

	result := c.processSchemaItems(schema, newSchemaContext())
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	t.Logf("circular items result: %v", result)
}

// TestSchemaContextEnter verifies the enter method behavior.
func TestSchemaContextEnter(t *testing.T) {
	sc := newSchemaContext()
	schema := &openapi3.Schema{}

	// First entry should succeed
	next, ok := sc.enter(schema)
	if !ok {
		t.Fatal("first enter should succeed")
	}

	// Second entry with same schema should fail (already visited)
	_, ok = next.enter(schema)
	if ok {
		t.Fatal("entering same schema twice should fail")
	}

	// Original context should not be mutated
	_, ok = sc.enter(schema)
	if !ok {
		t.Fatal("original context should not be affected by next.enter")
	}

	// Depth limit test
	deep := newSchemaContext()
	for i := 0; i < maxSchemaDepth; i++ {
		s := &openapi3.Schema{Title: ""}
		var success bool
		deep, success = deep.enter(s)

		if !success {
			t.Fatalf("enter should succeed at depth %d", i)
		}
	}

	// One more should fail
	_, ok = deep.enter(&openapi3.Schema{})
	if ok {
		t.Fatal("should fail at maxSchemaDepth")
	}
}
