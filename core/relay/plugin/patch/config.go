// Package patch provides configuration types and structures for the patch plugin.
package patch

import "github.com/bytedance/sonic/ast"

// Config holds the configuration for the patch plugin
type Config struct {
	// Force Enabled

	// UserPatches are user-defined custom patches
	UserPatches []PatchRule `json:"user_patches,omitempty"`
}

// PatchRule defines a complete patch rule with conditions and operations
type PatchRule struct {
	// Name is a descriptive name for this patch rule
	Name string `json:"name"`
	// Description explains what this patch does
	Description string `json:"description,omitempty"`
	// Conditions determine when this patch should be applied
	Conditions []PatchCondition `json:"conditions,omitempty"`
	// ConditionLogic defines how conditions are combined (default: "and")
	ConditionLogic LogicOperator `json:"condition_logic,omitempty"`
	// Operations define what modifications to make
	Operations []PatchOperation `json:"operations"`
}

// PatchCondition defines when a patch should be applied
type PatchCondition struct {
	// Key is the field to check (supports dot notation for nested fields)
	// Special keys: "model", "original_model"
	Key string `json:"key"`
	// Operator defines how to compare the value
	Operator ConditionOperator `json:"operator"`
	// Value is the value to compare against
	Value string `json:"value"`
	// Values is an array of values for 'in' and 'not_in' operators
	Values []string `json:"values,omitempty"`
	// Negate inverts the result of this condition (for "not" logic)
	Negate bool `json:"negate,omitempty"`
}

type PatchFunction func(root *ast.Node) (bool, error)

// PatchOperation defines a modification to make to the JSON
type PatchOperation struct {
	// Op is the operation type
	Op OperationType `json:"op"`
	// Key is the field to modify (supports dot notation for nested fields)
	Key string `json:"key"`
	// Value is the new value to set (not used for delete operations)
	Value any `json:"value,omitempty"`
	// Function is the inline Function code for OpFunction operations
	Function PatchFunction `json:"-"`
}

// ConditionOperator defines how to evaluate a condition
type ConditionOperator string

const (
	OperatorEquals      ConditionOperator = "equals"
	OperatorNotEquals   ConditionOperator = "not_equals"
	OperatorContains    ConditionOperator = "contains"
	OperatorNotContains ConditionOperator = "not_contains"
	OperatorRegex       ConditionOperator = "regex"
	OperatorExists      ConditionOperator = "exists"
	OperatorNotExists   ConditionOperator = "not_exists"
	OperatorHasPrefix   ConditionOperator = "has_prefix"
	OperatorHasSuffix   ConditionOperator = "has_suffix"
	OperatorGreaterThan ConditionOperator = "greater_than"
	OperatorLessThan    ConditionOperator = "less_than"
	OperatorGreaterEq   ConditionOperator = "greater_eq"
	OperatorLessEq      ConditionOperator = "less_eq"
	OperatorIn          ConditionOperator = "in"
	OperatorNotIn       ConditionOperator = "not_in"
)

// LogicOperator defines how multiple conditions are combined
type LogicOperator string

const (
	// LogicAnd requires all conditions to be true (default)
	LogicAnd LogicOperator = "and"
	// LogicOr requires at least one condition to be true
	LogicOr LogicOperator = "or"
)

// OperationType defines the type of operation to perform
type OperationType string

const (
	// OpSet sets a field to a specific value
	OpSet OperationType = "set"
	// OpDelete removes a field
	OpDelete OperationType = "delete"
	// OpAdd adds a field only if it doesn't exist
	OpAdd OperationType = "add"
	// OpLimit limits a numeric field to a maximum value
	OpLimit OperationType = "limit"
	// OpIncrement increments a numeric field by a value
	OpIncrement OperationType = "increment"
	// OpDecrement decrements a numeric field by a value
	OpDecrement OperationType = "decrement"
	// OpMultiply multiplies a numeric field by a value
	OpMultiply OperationType = "multiply"
	// OpDivide divides a numeric field by a value
	OpDivide OperationType = "divide"
	// OpAppend appends value to an array field
	OpAppend OperationType = "append"
	// OpPrepend prepends value to an array field
	OpPrepend OperationType = "prepend"
	// OpFunction executes an inline function on the field
	OpFunction OperationType = "function"
)

// DefaultPredefinedPatches are built-in patches that are always available
var DefaultPredefinedPatches = []PatchRule{
	{
		Name:           "deepseek_max_tokens_limit",
		Description:    "Limit max_tokens to 16000 for DeepSeek models",
		ConditionLogic: LogicOr,
		Conditions: []PatchCondition{
			{
				Key:      "model",
				Operator: OperatorContains,
				Value:    "deepseek-v3",
			},
		},
		Operations: []PatchOperation{
			{
				Op:    OpLimit,
				Key:   "max_tokens",
				Value: 16384,
			},
		},
	},
	{
		Name:           "deepseek_3.1_max_tokens_limit",
		Description:    "Limit max_tokens to 16000 for DeepSeek models",
		ConditionLogic: LogicOr,
		Conditions: []PatchCondition{
			{
				Key:      "model",
				Operator: OperatorContains,
				Value:    "deepseek-v3.1",
			},
			{
				Key:      "model",
				Operator: OperatorContains,
				Value:    "deepseek-chat",
			},
		},
		Operations: []PatchOperation{
			{
				Op:    OpLimit,
				Key:   "max_tokens",
				Value: 8192,
			},
		},
	},
	{
		Name:        "gpt5_max_tokens_to_max_completion_tokens",
		Description: "Convert max_tokens to max_completion_tokens for GPT-5 models",
		Conditions: []PatchCondition{
			{
				Key:      "model",
				Operator: OperatorContains,
				Value:    "gpt-5",
			},
			{
				Key:      "max_tokens",
				Operator: OperatorExists,
			},
		},
		Operations: []PatchOperation{
			{
				Op:    OpSet,
				Key:   "max_completion_tokens",
				Value: "{{max_tokens}}", // Special placeholder that will be replaced with actual max_tokens value
			},
			{
				Op:  OpDelete,
				Key: "max_tokens",
			},
		},
	},
	{
		Name:        "gpt5_remove_temperature",
		Description: "Remove temperature field for GPT-5 models",
		Conditions: []PatchCondition{
			{
				Key:      "model",
				Operator: OperatorContains,
				Value:    "gpt-5",
			},
			{
				Key:      "temperature",
				Operator: OperatorExists,
			},
		},
		Operations: []PatchOperation{
			{
				Op:  OpDelete,
				Key: "temperature",
			},
		},
	},
	{
		Name:        "gpt5_remove_top_p",
		Description: "Remove top_p field for GPT-5 models",
		Conditions: []PatchCondition{
			{
				Key:      "model",
				Operator: OperatorContains,
				Value:    "gpt-5",
			},
			{
				Key:      "top_p",
				Operator: OperatorExists,
			},
		},
		Operations: []PatchOperation{
			{
				Op:  OpDelete,
				Key: "top_p",
			},
		},
	},
	{
		Name:        "gemini_gpt5_remove_generation_config_top_p",
		Description: "Remove generationConfig.topP for GPT-5 models in Gemini format",
		Conditions: []PatchCondition{
			{
				Key:      "model",
				Operator: OperatorContains,
				Value:    "gpt-5",
			},
			{
				Key:      "generationConfig.topP",
				Operator: OperatorExists,
			},
		},
		Operations: []PatchOperation{
			{
				Op:  OpDelete,
				Key: "generationConfig.topP",
			},
		},
	},
}
