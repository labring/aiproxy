package utils

import (
	"strconv"
	"strings"

	"github.com/bytedance/sonic/ast"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func ParseOpenAIReasoning(req *relaymodel.GeneralOpenAIRequest) relaymodel.NormalizedReasoning {
	if req == nil {
		return relaymodel.NormalizedReasoning{}
	}

	if req.ReasoningEffort != nil {
		effort := relaymodel.NormalizeReasoningEffort(*req.ReasoningEffort)
		if effort != "" {
			return reasoningFromEffort(effort)
		}
	}

	return relaymodel.NormalizedReasoning{}
}

func ParseClaudeOpenAIReasoning(
	req *relaymodel.ClaudeOpenAIRequest,
) relaymodel.NormalizedReasoning {
	if req == nil || req.ReasoningEffort == nil {
		return relaymodel.NormalizedReasoning{}
	}

	effort := relaymodel.NormalizeReasoningEffort(*req.ReasoningEffort)
	if effort == "" {
		return relaymodel.NormalizedReasoning{}
	}

	return reasoningFromEffort(effort)
}

func ParseResponsesReasoning(
	req *relaymodel.CreateResponseRequest,
) relaymodel.NormalizedReasoning {
	if req == nil || req.Reasoning == nil || req.Reasoning.Effort == nil {
		return relaymodel.NormalizedReasoning{}
	}

	effort := relaymodel.NormalizeReasoningEffort(*req.Reasoning.Effort)
	if effort == "" {
		return relaymodel.NormalizedReasoning{}
	}

	return reasoningFromEffort(effort)
}

func ParseGeminiReasoning(
	config *relaymodel.GeminiThinkingConfig,
) relaymodel.NormalizedReasoning {
	if config == nil {
		return relaymodel.NormalizedReasoning{}
	}

	if config.ThinkingLevel != "" {
		effort := relaymodel.NormalizeReasoningEffort(config.ThinkingLevel)
		if effort != "" {
			return reasoningFromEffort(effort)
		}
	}

	if config.ThinkingBudget != nil {
		if *config.ThinkingBudget <= 0 {
			return relaymodel.NormalizedReasoning{
				Specified: true,
				Disabled:  true,
				Effort:    relaymodel.ReasoningEffortNone,
			}
		}

		budget := *config.ThinkingBudget

		return relaymodel.NormalizedReasoning{
			Specified:    true,
			Effort:       BudgetToEffort(budget),
			BudgetTokens: &budget,
		}
	}

	if config.IncludeThoughts {
		return relaymodel.NormalizedReasoning{
			Specified: true,
			Effort:    relaymodel.ReasoningEffortMedium,
		}
	}

	return relaymodel.NormalizedReasoning{
		Specified: true,
		Disabled:  true,
		Effort:    relaymodel.ReasoningEffortNone,
	}
}

func ParseClaudeReasoning(
	thinking *relaymodel.ClaudeThinking,
	outputConfig *relaymodel.ClaudeOutputConfig,
) relaymodel.NormalizedReasoning {
	if thinking == nil {
		if outputConfig != nil && outputConfig.Effort != nil {
			effort := relaymodel.NormalizeReasoningEffort(*outputConfig.Effort)
			if effort != "" {
				return reasoningFromEffort(effort)
			}
		}

		return relaymodel.NormalizedReasoning{}
	}

	switch thinking.Type {
	case relaymodel.ClaudeThinkingTypeDisabled:
		return relaymodel.NormalizedReasoning{
			Specified: true,
			Disabled:  true,
			Effort:    relaymodel.ReasoningEffortNone,
		}
	case relaymodel.ClaudeThinkingTypeEnabled, relaymodel.ClaudeThinkingTypeAdaptive:
		reasoning := relaymodel.NormalizedReasoning{
			Specified: true,
		}
		if outputConfig != nil && outputConfig.Effort != nil {
			effort := relaymodel.NormalizeReasoningEffort(*outputConfig.Effort)
			if effort != "" {
				reasoning.Effort = effort
			}
		}

		if thinking.BudgetTokens > 0 {
			budget := thinking.BudgetTokens

			reasoning.BudgetTokens = &budget
			if reasoning.Effort == "" {
				reasoning.Effort = BudgetToEffort(budget)
			}
		}

		if reasoning.Effort == "" {
			reasoning.Effort = relaymodel.ReasoningEffortMedium
		}

		return reasoning
	default:
		return relaymodel.NormalizedReasoning{}
	}
}

func ParseOpenAIReasoningFromNode(
	node *ast.Node,
) (relaymodel.NormalizedReasoning, error) {
	if node == nil {
		return relaymodel.NormalizedReasoning{}, nil
	}

	if reasoningEffortNode := node.Get("reasoning_effort"); reasoningEffortNode.Exists() {
		reasoningEffort, err := reasoningEffortNode.String()
		if err == nil {
			effort := relaymodel.NormalizeReasoningEffort(reasoningEffort)
			if effort != "" {
				return reasoningFromEffort(effort), nil
			}
		}
	}

	return relaymodel.NormalizedReasoning{}, nil
}

func ApplyReasoningToOpenAIRequest(
	req *relaymodel.GeneralOpenAIRequest,
	reasoning relaymodel.NormalizedReasoning,
) {
	if req == nil || !reasoning.Specified {
		return
	}

	effort := ReasoningToOpenAIEffort(reasoning)
	if effort == "" {
		return
	}

	effortString := effort
	req.ReasoningEffort = &effortString
	req.Thinking = nil
}

func ApplyReasoningToResponsesRequest(
	req *relaymodel.CreateResponseRequest,
	reasoning relaymodel.NormalizedReasoning,
) {
	if req == nil || !reasoning.Specified {
		return
	}

	effort := ReasoningToOpenAIEffort(reasoning)
	if effort == "" {
		return
	}

	effortString := effort
	req.Reasoning = &relaymodel.ResponseReasoning{
		Effort: &effortString,
	}
}

func ApplyReasoningToGeminiConfig(
	originModel string,
	actualModel string,
	config *relaymodel.GeminiChatGenerationConfig,
	reasoning relaymodel.NormalizedReasoning,
) {
	if config == nil || !reasoning.Specified {
		return
	}

	modelName := resolveGeminiModelName(originModel, actualModel)

	if GeminiUsesThinkingLevel(modelName) {
		if config.ThinkingConfig == nil {
			config.ThinkingConfig = &relaymodel.GeminiThinkingConfig{}
		}

		if reasoning.Disabled ||
			ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
			if GeminiSupportsDisableThinking(modelName) {
				zero := 0
				config.ThinkingConfig.ThinkingBudget = &zero
				config.ThinkingConfig.IncludeThoughts = false
				config.ThinkingConfig.ThinkingLevel = ""

				return
			}

			config.ThinkingConfig.ThinkingBudget = nil
			config.ThinkingConfig.IncludeThoughts = false
			config.ThinkingConfig.ThinkingLevel = GeminiMinimumThinkingLevel(modelName)

			return
		}

		config.ThinkingConfig.ThinkingBudget = nil
		config.ThinkingConfig.IncludeThoughts = false
		config.ThinkingConfig.ThinkingLevel = GeminiThinkingLevelForModel(modelName, reasoning)

		return
	}

	if config.ThinkingConfig == nil {
		config.ThinkingConfig = &relaymodel.GeminiThinkingConfig{}
	}

	minBudget, maxBudget, disableSupported, hasBudgetRange := GeminiThinkingBudgetRange(modelName)
	if reasoning.Disabled || ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
		config.ThinkingConfig.IncludeThoughts = false

		budget := 0
		if hasBudgetRange && !disableSupported {
			budget = minBudget
		}

		config.ThinkingConfig.ThinkingBudget = &budget
		config.ThinkingConfig.ThinkingLevel = ""

		return
	}

	budget := ReasoningToBudget(reasoning)
	if hasBudgetRange {
		if budget < minBudget {
			budget = minBudget
		}

		if maxBudget > 0 && budget > maxBudget {
			budget = maxBudget
		}
	}

	config.ThinkingConfig.IncludeThoughts = true
	if budget <= 0 {
		zero := 0
		config.ThinkingConfig.IncludeThoughts = false
		config.ThinkingConfig.ThinkingBudget = &zero
		config.ThinkingConfig.ThinkingLevel = ""

		return
	}

	config.ThinkingConfig.ThinkingBudget = &budget
	config.ThinkingConfig.ThinkingLevel = ""
}

func ApplyReasoningToClaudeRequest(
	modelName string,
	maxTokens *int,
	thinking **relaymodel.ClaudeThinking,
	outputConfig **relaymodel.ClaudeOutputConfig,
	reasoning relaymodel.NormalizedReasoning,
) {
	if thinking == nil || outputConfig == nil || !reasoning.Specified {
		return
	}

	if reasoning.Disabled || ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
		*thinking = &relaymodel.ClaudeThinking{
			Type: relaymodel.ClaudeThinkingTypeDisabled,
		}
		*outputConfig = nil

		return
	}

	if ClaudeUsesAdaptiveThinking(modelName) {
		effortString := ClaudeOutputEffort(reasoning)
		*thinking = &relaymodel.ClaudeThinking{
			Type: relaymodel.ClaudeThinkingTypeAdaptive,
		}
		*outputConfig = &relaymodel.ClaudeOutputConfig{
			Effort: &effortString,
		}

		return
	}

	budget := ReasoningToBudget(reasoning)
	budget = ClampClaudeThinkingBudget(maxTokens, budget)
	*thinking = &relaymodel.ClaudeThinking{
		Type:         relaymodel.ClaudeThinkingTypeEnabled,
		BudgetTokens: budget,
	}
	*outputConfig = nil
}

func ApplyReasoningToAliNode(
	originModel string,
	actualModel string,
	node *ast.Node,
	reasoning relaymodel.NormalizedReasoning,
) error {
	if node == nil || !reasoning.Specified {
		return nil
	}

	modelName := resolveAliModelName(originModel, actualModel)

	_, _ = node.Unset("reasoning_effort")
	_, _ = node.Unset("thinking")

	if reasoning.Disabled || ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
		if _, err := node.Set("enable_thinking", ast.NewBool(false)); err != nil {
			return err
		}

		_, _ = node.Unset("thinking_budget")

		return nil
	}

	if _, err := node.Set("enable_thinking", ast.NewBool(true)); err != nil {
		return err
	}

	if !AliSupportsThinkingBudget(modelName) {
		_, _ = node.Unset("thinking_budget")
		return nil
	}

	budget := ReasoningToBudget(reasoning)
	if budget <= 0 {
		_, _ = node.Unset("thinking_budget")
		return nil
	}

	_, err := node.Set("thinking_budget", ast.NewNumber(strconv.Itoa(budget)))

	return err
}

func ApplyReasoningToAliRequest(
	originModel string,
	actualModel string,
	req *relaymodel.GeneralOpenAIRequest,
	reasoning relaymodel.NormalizedReasoning,
) {
	if req == nil || !reasoning.Specified {
		return
	}

	modelName := resolveAliModelName(originModel, actualModel)

	req.ReasoningEffort = nil
	req.Thinking = nil
	req.ThinkingBudget = nil

	if reasoning.Disabled || ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
		enableThinking := false
		req.EnableThinking = &enableThinking
		return
	}

	enableThinking := true
	req.EnableThinking = &enableThinking

	if !AliSupportsThinkingBudget(modelName) {
		return
	}

	budget := ReasoningToBudget(reasoning)
	if budget <= 0 {
		return
	}

	req.ThinkingBudget = &budget
}

func ApplyReasoningToZhipuNode(
	node *ast.Node,
	reasoning relaymodel.NormalizedReasoning,
) error {
	return applyReasoningToThinkingNode(node, reasoning)
}

func ApplyReasoningToZhipuRequest(
	req *relaymodel.GeneralOpenAIRequest,
	reasoning relaymodel.NormalizedReasoning,
) {
	applyReasoningToThinkingRequest(req, reasoning)
}

func ApplyReasoningToDeepSeekNode(
	node *ast.Node,
	reasoning relaymodel.NormalizedReasoning,
) error {
	return applyReasoningToThinkingNode(node, reasoning)
}

func ApplyReasoningToDeepSeekRequest(
	req *relaymodel.GeneralOpenAIRequest,
	reasoning relaymodel.NormalizedReasoning,
) {
	applyReasoningToThinkingRequest(req, reasoning)
}

func ApplyReasoningToDoubaoNode(
	node *ast.Node,
	reasoning relaymodel.NormalizedReasoning,
) error {
	return applyReasoningToThinkingNode(node, reasoning)
}

func ApplyReasoningToDoubaoRequest(
	req *relaymodel.GeneralOpenAIRequest,
	reasoning relaymodel.NormalizedReasoning,
) {
	applyReasoningToThinkingRequest(req, reasoning)
}

func applyReasoningToThinkingNode(
	node *ast.Node,
	reasoning relaymodel.NormalizedReasoning,
) error {
	if node == nil || !reasoning.Specified {
		return nil
	}

	_, _ = node.Unset("reasoning_effort")

	thinkingType := relaymodel.ClaudeThinkingTypeEnabled
	if reasoning.Disabled || ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
		thinkingType = relaymodel.ClaudeThinkingTypeDisabled
	}

	_, err := node.SetAny("thinking", relaymodel.ClaudeThinking{Type: thinkingType})

	return err
}

func applyReasoningToThinkingRequest(
	req *relaymodel.GeneralOpenAIRequest,
	reasoning relaymodel.NormalizedReasoning,
) {
	if req == nil || !reasoning.Specified {
		return
	}

	req.ReasoningEffort = nil

	thinkingType := relaymodel.ClaudeThinkingTypeEnabled
	if reasoning.Disabled || ReasoningToOpenAIEffort(reasoning) == relaymodel.ReasoningEffortNone {
		thinkingType = relaymodel.ClaudeThinkingTypeDisabled
	}

	req.Thinking = &relaymodel.ClaudeThinking{Type: thinkingType}
}

func ReasoningToOpenAIEffort(
	reasoning relaymodel.NormalizedReasoning,
) relaymodel.ReasoningEffort {
	if reasoning.Disabled {
		return relaymodel.ReasoningEffortNone
	}

	if reasoning.Effort != "" {
		return reasoning.Effort
	}

	if reasoning.BudgetTokens != nil {
		return BudgetToEffort(*reasoning.BudgetTokens)
	}

	if reasoning.Specified {
		return relaymodel.ReasoningEffortMedium
	}

	return ""
}

func ClaudeOutputEffort(reasoning relaymodel.NormalizedReasoning) string {
	switch ReasoningToOpenAIEffort(reasoning) {
	case relaymodel.ReasoningEffortXHigh:
		return "max"
	case relaymodel.ReasoningEffortNone, relaymodel.ReasoningEffortMinimal:
		return "low"
	case relaymodel.ReasoningEffortLow:
		return "low"
	case relaymodel.ReasoningEffortHigh:
		return "high"
	default:
		return "medium"
	}
}

func BudgetToEffort(budget int) relaymodel.ReasoningEffort {
	switch {
	case budget <= 0:
		return relaymodel.ReasoningEffortNone
	case budget <= 1024:
		return relaymodel.ReasoningEffortMinimal
	case budget <= 4096:
		return relaymodel.ReasoningEffortLow
	case budget <= 12288:
		return relaymodel.ReasoningEffortMedium
	case budget <= 24576:
		return relaymodel.ReasoningEffortHigh
	default:
		return relaymodel.ReasoningEffortXHigh
	}
}

func ReasoningToBudget(reasoning relaymodel.NormalizedReasoning) int {
	if reasoning.BudgetTokens != nil && *reasoning.BudgetTokens > 0 {
		return *reasoning.BudgetTokens
	}

	switch ReasoningToOpenAIEffort(reasoning) {
	case relaymodel.ReasoningEffortNone:
		return 0
	case relaymodel.ReasoningEffortMinimal:
		return 1024
	case relaymodel.ReasoningEffortLow:
		return 2048
	case relaymodel.ReasoningEffortHigh:
		return 16384
	case relaymodel.ReasoningEffortXHigh:
		return 32768
	default:
		return 8192
	}
}

func GeminiUsesThinkingLevel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	return strings.Contains(modelName, "gemini-3") ||
		strings.Contains(modelName, "gemini-4") ||
		strings.Contains(modelName, "gemini-5")
}

func GeminiSupportsDisableThinking(modelName string) bool {
	modelName = strings.ToLower(modelName)
	return strings.Contains(modelName, "2.5-flash") ||
		strings.Contains(modelName, "2.5-flash-lite")
}

func GeminiThinkingBudgetRange(
	modelName string,
) (minBudget, maxBudget int, disableSupported, ok bool) {
	modelName = strings.ToLower(modelName)

	switch {
	case strings.Contains(modelName, "2.5-pro"):
		return 128, 32768, false, true
	case strings.Contains(modelName, "2.5-flash-lite"):
		return 512, 24576, true, true
	case strings.Contains(modelName, "2.5-flash"):
		return 1, 24576, true, true
	default:
		return 0, 0, false, false
	}
}

func GeminiMinimumThinkingLevel(modelName string) string {
	modelName = strings.ToLower(modelName)
	if strings.Contains(modelName, "pro") {
		return "low"
	}

	return "minimal"
}

func GeminiThinkingLevelForModel(
	modelName string,
	reasoning relaymodel.NormalizedReasoning,
) string {
	effort := ReasoningToOpenAIEffort(reasoning)
	modelName = strings.ToLower(modelName)

	if strings.Contains(modelName, "pro") {
		switch effort {
		case relaymodel.ReasoningEffortHigh, relaymodel.ReasoningEffortXHigh:
			return "high"
		default:
			return "low"
		}
	}

	switch effort {
	case relaymodel.ReasoningEffortNone:
		return "minimal"
	case relaymodel.ReasoningEffortLow:
		return "low"
	case relaymodel.ReasoningEffortMedium:
		return "medium"
	case relaymodel.ReasoningEffortHigh, relaymodel.ReasoningEffortXHigh:
		return "high"
	default:
		return "minimal"
	}
}

func ClaudeUsesAdaptiveThinking(modelName string) bool {
	modelName = strings.ToLower(modelName)

	return strings.Contains(modelName, "mythos") ||
		strings.Contains(modelName, "opus-4-6") ||
		strings.Contains(modelName, "sonnet-4-6") ||
		strings.Contains(modelName, "opus-4-7")
}

func AliSupportsThinkingBudget(modelName string) bool {
	modelName = strings.ToLower(modelName)

	return strings.HasPrefix(modelName, "qwen3-") ||
		strings.HasPrefix(modelName, "qwq-") ||
		strings.Contains(modelName, "glm") ||
		strings.Contains(modelName, "kimi")
}

func reasoningFromEffort(
	effort relaymodel.ReasoningEffort,
) relaymodel.NormalizedReasoning {
	return relaymodel.NormalizedReasoning{
		Specified: true,
		Disabled:  effort == relaymodel.ReasoningEffortNone,
		Effort:    effort,
	}
}

func PreferredModelName(originModel, actualModel string) string {
	if originModel != "" {
		return originModel
	}
	return actualModel
}

func FirstMatchingModelName(
	originModel string,
	actualModel string,
	match func(string) bool,
) string {
	if match == nil {
		return PreferredModelName(originModel, actualModel)
	}

	if originModel != "" && match(originModel) {
		return originModel
	}

	if actualModel != "" && actualModel != originModel && match(actualModel) {
		return actualModel
	}

	return ""
}

func ClampClaudeThinkingBudget(maxTokens *int, budget int) int {
	if budget <= 0 {
		budget = 1024
	}

	if budget < 1024 {
		budget = 1024
	}

	if maxTokens == nil {
		return budget
	}

	requiredMaxTokens := max(budget+1, 2048)

	if *maxTokens < requiredMaxTokens {
		*maxTokens = requiredMaxTokens
	}

	if budget >= *maxTokens {
		return *maxTokens - 1
	}

	return budget
}

func ClampReasoningBudget(maxTokens *int, budget int) int {
	if maxTokens == nil || *maxTokens <= 0 || budget <= 0 {
		return budget
	}

	if budget < *maxTokens {
		return budget
	}

	if *maxTokens <= 1 {
		return 0
	}

	return *maxTokens - 1
}

func resolveGeminiModelName(originModel, actualModel string) string {
	if modelName := FirstMatchingModelName(originModel, actualModel, func(modelName string) bool {
		return strings.Contains(strings.ToLower(modelName), "gemini")
	}); modelName != "" {
		return modelName
	}

	return PreferredModelName(originModel, actualModel)
}

func resolveAliModelName(originModel, actualModel string) string {
	if modelName := FirstMatchingModelName(originModel, actualModel, func(modelName string) bool {
		modelName = strings.ToLower(modelName)

		return strings.HasPrefix(modelName, "qwen") ||
			strings.HasPrefix(modelName, "qwq-") ||
			strings.Contains(modelName, "glm") ||
			strings.Contains(modelName, "kimi")
	}); modelName != "" {
		return modelName
	}

	return PreferredModelName(originModel, actualModel)
}
