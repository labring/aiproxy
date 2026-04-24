// Package patch provides high-performance JSON request patching functionality using sonic.
// It allows automatic modification of API requests based on conditions and rules.
package patch

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/noop"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ plugin.Plugin = (*Plugin)(nil)

const PluginName = "patch"

// LazyPatchData represents data to be applied by patch plugin later
type LazyPatchData struct {
	Source string `json:"source"` // Source plugin name
	Data   any    `json:"data"`   // Data to be patched
}

const lazyPatchesKey = "_lazy_patches"

// Plugin implements JSON request patching functionality
type Plugin struct {
	noop.Noop
	configCache utils.PluginConfigCache[Config]
}

// NewPatchPlugin creates a new patch plugin instance
func NewPatchPlugin() *Plugin {
	return &Plugin{}
}

// AddLazyPatch adds data to the lazy patch queue in meta
func AddLazyPatch(meta *meta.Meta, patch PatchOperation) {
	meta.PushToSlice(lazyPatchesKey, patch)
}

// GetLazyPatches retrieves all lazy patch data from meta
func GetLazyPatches(meta *meta.Meta) []PatchOperation {
	slice := meta.GetSlice(lazyPatchesKey)
	if slice == nil {
		return nil
	}

	patches := make([]PatchOperation, 0, len(slice))
	for _, item := range slice {
		if patch, ok := item.(PatchOperation); ok {
			patches = append(patches, patch)
		}
	}

	return patches
}

// ConvertRequest applies JSON patches to the request body
func (p *Plugin) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
	do adaptor.ConvertRequest,
) (adaptor.ConvertResult, error) {
	// Load patch configuration from model config
	config := p.loadConfig(meta)

	bodyBytes, err := common.GetRequestBodyReusable(req)
	if err != nil {
		return do.ConvertRequest(meta, store, req)
	}

	// Apply patches
	patchedBody, modified, err := p.ApplyPatches(bodyBytes, meta, config)
	if err != nil {
		return do.ConvertRequest(meta, store, req)
	}

	// If no modifications were made, return original
	if !modified {
		return do.ConvertRequest(meta, store, req)
	}

	common.SetRequestBody(req, patchedBody)
	defer func() {
		common.SetRequestBody(req, bodyBytes)
	}()

	return do.ConvertRequest(meta, store, req)
}

// loadConfig loads patch configuration from model config
func (p *Plugin) loadConfig(meta *meta.Meta) *Config {
	config, err := p.configCache.Load(meta, PluginName, Config{})
	if err != nil {
		return &Config{}
	}

	return &config
}

// ApplyPatches applies all applicable patches to the JSON body
func (p *Plugin) ApplyPatches(
	bodyBytes []byte,
	meta *meta.Meta,
	config *Config,
) ([]byte, bool, error) {
	// Parse JSON using sonic AST
	node, err := sonic.Get(bodyBytes)
	if err != nil {
		// If it's not valid JSON, return as is
		return bodyBytes, false, nil
	}

	modified := false

	// Apply predefined patches (always enabled)
	for _, patch := range DefaultPredefinedPatches {
		if p.shouldApplyPatch(&patch, &node, meta) {
			if p.applyPatch(&patch, &node) {
				modified = true
			}
		}
	}

	// Apply lazy patches from meta
	if p.applyLazyPatches(&node, meta) {
		modified = true
	}

	// Apply user-defined patches
	for _, patch := range config.UserPatches {
		if p.shouldApplyPatch(&patch, &node, meta) {
			if p.applyPatch(&patch, &node) {
				modified = true
			}
		}
	}

	if !modified {
		return bodyBytes, false, nil
	}

	// Marshal back to JSON using sonic
	patchedBytes, err := node.MarshalJSON()
	if err != nil {
		return bodyBytes, false, fmt.Errorf("failed to marshal patched JSON: %w", err)
	}

	return patchedBytes, true, nil
}

// shouldApplyPatch determines if a patch should be applied based on conditions
func (p *Plugin) shouldApplyPatch(patch *PatchRule, root *ast.Node, meta *meta.Meta) bool {
	// Check if the patch has conditions
	if len(patch.Conditions) == 0 {
		return true // No conditions means always apply
	}

	// Default to "and" logic if not specified
	logic := patch.ConditionLogic
	if logic == "" {
		logic = LogicAnd
	}

	switch logic {
	case LogicOr:
		// At least one condition must be satisfied
		for _, condition := range patch.Conditions {
			if p.evaluateCondition(&condition, root, meta) {
				return true
			}
		}

		return false
	case LogicAnd:
		fallthrough
	default:
		// All conditions must be satisfied
		for _, condition := range patch.Conditions {
			if !p.evaluateCondition(&condition, root, meta) {
				return false
			}
		}

		return true
	}
}

// evaluateCondition evaluates a single condition
func (p *Plugin) evaluateCondition(
	condition *PatchCondition,
	root *ast.Node,
	meta *meta.Meta,
) bool {
	var actualValue any

	// Get the value to check
	switch condition.Key {
	case "model":
		actualValue = meta.ActualModel
	case "original_model":
		actualValue = meta.OriginModel
	default:
		// Look in JSON data
		actualValue = p.getNestedValueAST(root, condition.Key)
	}

	// Convert to string for comparison
	actualStr := fmt.Sprintf("%v", actualValue)

	var result bool

	// Apply the operator
	switch condition.Operator {
	case OperatorEquals:
		result = actualStr == condition.Value
	case OperatorNotEquals:
		result = actualStr != condition.Value
	case OperatorContains:
		result = strings.Contains(actualStr, condition.Value)
	case OperatorNotContains:
		result = !strings.Contains(actualStr, condition.Value)
	case OperatorHasPrefix:
		result = strings.HasPrefix(actualStr, condition.Value)
	case OperatorHasSuffix:
		result = strings.HasSuffix(actualStr, condition.Value)
	case OperatorRegex:
		matched, err := regexp.MatchString(condition.Value, actualStr)
		result = err == nil && matched
	case OperatorExists:
		result = actualValue != nil
	case OperatorNotExists:
		result = actualValue == nil
	case OperatorGreaterThan:
		result = p.compareNumeric(actualValue, condition.Value, ">")
	case OperatorLessThan:
		result = p.compareNumeric(actualValue, condition.Value, "<")
	case OperatorGreaterEq:
		result = p.compareNumeric(actualValue, condition.Value, ">=")
	case OperatorLessEq:
		result = p.compareNumeric(actualValue, condition.Value, "<=")
	case OperatorIn:
		result = p.stringInSlice(actualStr, condition.Values)
	case OperatorNotIn:
		result = !p.stringInSlice(actualStr, condition.Values)
	default:
		result = false
	}

	// Apply negation if specified
	if condition.Negate {
		result = !result
	}

	return result
}

// applyPatch applies a single patch to the JSON data
func (p *Plugin) applyPatch(patch *PatchRule, root *ast.Node) bool {
	modified := false

	for _, operation := range patch.Operations {
		operationModified, err := p.applyOperation(&operation, root)
		if err == nil && operationModified {
			modified = true
		}
	}

	return modified
}

// applyOperation applies a single operation
func (p *Plugin) applyOperation(operation *PatchOperation, root *ast.Node) (bool, error) {
	// Resolve placeholders in the value
	resolvedValue := p.resolvePlaceholdersAST(operation.Value, root)

	switch operation.Op {
	case OpSet:
		return p.setValueAST(root, operation.Key, resolvedValue), nil
	case OpDelete:
		return p.deleteValueAST(root, operation.Key), nil
	case OpAdd:
		// For add, we only set if the key doesn't exist
		if p.getNestedValueAST(root, operation.Key) == nil {
			return p.setValueAST(root, operation.Key, resolvedValue), nil
		}
		return false, nil
	case OpLimit:
		return p.limitValueAST(root, operation.Key, resolvedValue), nil
	case OpIncrement:
		return p.incrementValueAST(root, operation.Key, resolvedValue), nil
	case OpDecrement:
		return p.decrementValueAST(root, operation.Key, resolvedValue), nil
	case OpMultiply:
		return p.multiplyValueAST(root, operation.Key, resolvedValue), nil
	case OpDivide:
		return p.divideValueAST(root, operation.Key, resolvedValue), nil
	case OpAppend:
		return p.appendValueAST(root, operation.Key, resolvedValue), nil
	case OpPrepend:
		return p.prependValueAST(root, operation.Key, resolvedValue), nil
	case OpFunction:
		return operation.Function(root)
	default:
		return false, nil
	}
}

// getNestedValueAST retrieves a value from nested JSON structure using AST
func (p *Plugin) getNestedValueAST(root *ast.Node, key string) any {
	keys := strings.Split(key, ".")
	current := root

	for _, k := range keys {
		if current.TypeSafe() != ast.V_OBJECT {
			return nil
		}

		next := current.Get(k)
		if !next.Valid() {
			return nil
		}

		current = next
	}

	// Convert AST node to interface{}
	val, _ := current.Interface()

	return val
}

// setValueAST sets a value in nested JSON structure using AST
func (p *Plugin) setValueAST(root *ast.Node, key string, value any) bool {
	keys := strings.Split(key, ".")
	current := root

	// Navigate to the parent of the target key
	for i := range len(keys) - 1 {
		if current.TypeSafe() != ast.V_OBJECT {
			return false
		}

		next := current.Get(keys[i])
		if !next.Valid() {
			// Create new object if it doesn't exist
			newObj := ast.NewObject([]ast.Pair{})
			if _, err := current.Set(keys[i], newObj); err != nil {
				return false
			}

			next = current.Get(keys[i])
		}

		current = next
	}

	if current.TypeSafe() != ast.V_OBJECT {
		return false
	}

	finalKey := keys[len(keys)-1]
	oldValue := current.Get(finalKey)

	// Capture the old value BEFORE we modify the node
	var (
		oldVal      any
		hasOldValue bool
	)

	if oldValue.Valid() {
		oldVal, _ = oldValue.Interface()
		hasOldValue = true
	} else {
		hasOldValue = false
	}

	// Create AST node from value
	var newNode ast.Node
	if value == nil {
		newNode = ast.NewNull()
	} else {
		switch v := value.(type) {
		case string:
			newNode = ast.NewString(v)
		case int:
			newNode = ast.NewNumber(strconv.Itoa(v))
		case int64:
			newNode = ast.NewNumber(strconv.FormatInt(v, 10))
		case float64:
			newNode = ast.NewNumber(strconv.FormatFloat(v, 'f', -1, 64))
		case bool:
			newNode = ast.NewBool(v)
		default:
			// Try to marshal and parse
			if bytes, err := sonic.Marshal(v); err == nil {
				if node, err := sonic.Get(bytes); err == nil {
					newNode = node
				} else {
					return false
				}
			} else {
				return false
			}
		}
	}

	if _, err := current.Set(finalKey, newNode); err != nil {
		return false
	}

	// Check if value actually changed
	if hasOldValue {
		newVal, _ := newNode.Interface()
		changed := fmt.Sprintf("%v", oldVal) != fmt.Sprintf("%v", newVal)
		return changed
	}

	return true
}

// deleteValueAST deletes a value from nested JSON structure using AST
func (p *Plugin) deleteValueAST(root *ast.Node, key string) bool {
	keys := strings.Split(key, ".")
	current := root

	// Navigate to the parent of the target key
	for i := range len(keys) - 1 {
		if current.TypeSafe() != ast.V_OBJECT {
			return false
		}

		next := current.Get(keys[i])
		if !next.Valid() {
			return false
		}

		current = next
	}

	if current.TypeSafe() != ast.V_OBJECT {
		return false
	}

	finalKey := keys[len(keys)-1]

	oldValue := current.Get(finalKey)
	if !oldValue.Valid() {
		return false
	}

	if _, err := current.Unset(finalKey); err != nil {
		return false
	}

	return true
}

// limitValueAST limits a numeric value to a maximum using AST
func (p *Plugin) limitValueAST(root *ast.Node, key string, maxValue any) bool {
	currentValue := p.getNestedValueAST(root, key)
	if currentValue == nil {
		return false
	}

	// Convert values to float64 for comparison
	currentFloat, err := ToFloat64(currentValue)
	if err != nil {
		return false
	}

	maxFloat, err := ToFloat64(maxValue)
	if err != nil {
		return false
	}

	// If current value exceeds the limit, set it to the limit
	if currentFloat > maxFloat {
		result := p.setValueAST(root, key, maxValue)
		return result
	}

	return false
}

// incrementValueAST increments a numeric value using AST
func (p *Plugin) incrementValueAST(root *ast.Node, key string, incrementValue any) bool {
	currentValue := p.getNestedValueAST(root, key)
	if currentValue == nil {
		return false
	}

	currentFloat, err := ToFloat64(currentValue)
	if err != nil {
		return false
	}

	incrementFloat, err := ToFloat64(incrementValue)
	if err != nil {
		return false
	}

	newValue := currentFloat + incrementFloat

	return p.setValueAST(root, key, newValue)
}

// decrementValueAST decrements a numeric value using AST
func (p *Plugin) decrementValueAST(root *ast.Node, key string, decrementValue any) bool {
	currentValue := p.getNestedValueAST(root, key)
	if currentValue == nil {
		return false
	}

	currentFloat, err := ToFloat64(currentValue)
	if err != nil {
		return false
	}

	decrementFloat, err := ToFloat64(decrementValue)
	if err != nil {
		return false
	}

	newValue := currentFloat - decrementFloat

	return p.setValueAST(root, key, newValue)
}

// multiplyValueAST multiplies a numeric value using AST
func (p *Plugin) multiplyValueAST(root *ast.Node, key string, multiplierValue any) bool {
	currentValue := p.getNestedValueAST(root, key)
	if currentValue == nil {
		return false
	}

	currentFloat, err := ToFloat64(currentValue)
	if err != nil {
		return false
	}

	multiplierFloat, err := ToFloat64(multiplierValue)
	if err != nil {
		return false
	}

	newValue := currentFloat * multiplierFloat

	return p.setValueAST(root, key, newValue)
}

// divideValueAST divides a numeric value using AST
func (p *Plugin) divideValueAST(root *ast.Node, key string, divisorValue any) bool {
	currentValue := p.getNestedValueAST(root, key)
	if currentValue == nil {
		return false
	}

	currentFloat, err := ToFloat64(currentValue)
	if err != nil {
		return false
	}

	divisorFloat, err := ToFloat64(divisorValue)
	if err != nil || divisorFloat == 0 {
		return false
	}

	newValue := currentFloat / divisorFloat

	return p.setValueAST(root, key, newValue)
}

// appendValueAST appends a value to an array using AST
func (p *Plugin) appendValueAST(root *ast.Node, key string, value any) bool {
	currentNode, exists := p.getNodeByKey(root, key)
	if !exists {
		// Create new array with the value
		valueNode := p.createASTNode(value)
		if !valueNode.Valid() {
			return false
		}

		newArray := ast.NewArray([]ast.Node{valueNode})

		return p.setValueAST(root, key, newArray)
	}

	if currentNode.TypeSafe() != ast.V_ARRAY {
		return false
	}

	valueNode := p.createASTNode(value)
	if !valueNode.Valid() {
		return false
	}

	if err := currentNode.Add(valueNode); err != nil {
		return false
	}

	return true
}

// prependValueAST prepends a value to an array using AST
func (p *Plugin) prependValueAST(root *ast.Node, key string, value any) bool {
	currentNode, exists := p.getNodeByKey(root, key)
	if !exists {
		// Create new array with the value
		valueNode := p.createASTNode(value)
		if !valueNode.Valid() {
			return false
		}

		newArray := ast.NewArray([]ast.Node{valueNode})

		return p.setValueAST(root, key, newArray)
	}

	if currentNode.TypeSafe() != ast.V_ARRAY {
		return false
	}

	valueNode := p.createASTNode(value)
	if !valueNode.Valid() {
		return false
	}

	// Get all existing elements
	length, err := currentNode.Len()
	if err != nil {
		return false
	}

	elements := make([]ast.Node, length+1)
	elements[0] = valueNode

	for i := range length {
		elem := currentNode.Index(i)
		if elem == nil {
			return false
		}

		elements[i+1] = *elem
	}

	// Rebuild array
	newArray := ast.NewArray(elements)

	return p.setValueAST(root, key, newArray)
}

// getNodeByKey gets an AST node by key path
func (p *Plugin) getNodeByKey(root *ast.Node, key string) (ast.Node, bool) {
	keys := strings.Split(key, ".")
	current := root

	for _, k := range keys {
		if current.TypeSafe() != ast.V_OBJECT {
			return ast.Node{}, false
		}

		next := current.Get(k)
		if !next.Valid() {
			return ast.Node{}, false
		}

		current = next
	}

	return *current, true
}

// createASTNode creates an AST node from a value
func (p *Plugin) createASTNode(value any) ast.Node {
	if value == nil {
		return ast.NewNull()
	}

	switch v := value.(type) {
	case string:
		return ast.NewString(v)
	case int:
		return ast.NewNumber(strconv.Itoa(v))
	case int64:
		return ast.NewNumber(strconv.FormatInt(v, 10))
	case float64:
		return ast.NewNumber(strconv.FormatFloat(v, 'f', -1, 64))
	case bool:
		return ast.NewBool(v)
	default:
		// Try to marshal and parse
		if bytes, err := sonic.Marshal(v); err == nil {
			if node, err := sonic.Get(bytes); err == nil {
				return node
			}
		}

		return ast.Node{}
	}
}

func ToFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// compareNumeric compares two numeric values
func (p *Plugin) compareNumeric(actualValue any, expectedValue, operator string) bool {
	actualFloat, err := ToFloat64(actualValue)
	if err != nil {
		return false
	}

	expectedFloat, err := strconv.ParseFloat(expectedValue, 64)
	if err != nil {
		return false
	}

	switch operator {
	case ">":
		return actualFloat > expectedFloat
	case "<":
		return actualFloat < expectedFloat
	case ">=":
		return actualFloat >= expectedFloat
	case "<=":
		return actualFloat <= expectedFloat
	default:
		return false
	}
}

// stringInSlice checks if a string is in a slice
func (p *Plugin) stringInSlice(str string, slice []string) bool {
	return slices.Contains(slice, str)
}

// applyLazyPatches applies patches queued in meta from other plugins
func (p *Plugin) applyLazyPatches(root *ast.Node, meta *meta.Meta) bool {
	lazyPatches := GetLazyPatches(meta)
	if len(lazyPatches) == 0 {
		return false
	}

	modified := false
	for _, lazyPatch := range lazyPatches {
		if opModified, err := p.applyOperation(&lazyPatch, root); err == nil && opModified {
			modified = true
		}
	}

	return modified
}

// resolvePlaceholdersAST replaces placeholders in values with actual values from JSON data using AST
func (p *Plugin) resolvePlaceholdersAST(value any, root *ast.Node) any {
	if strValue, ok := value.(string); ok {
		// Check if it's a placeholder pattern {{key}}
		if strings.HasPrefix(strValue, "{{") && strings.HasSuffix(strValue, "}}") {
			placeholderKey := strValue[2 : len(strValue)-2]
			if actualValue := p.getNestedValueAST(root, placeholderKey); actualValue != nil {
				return actualValue
			}
		}
	}

	return value
}
