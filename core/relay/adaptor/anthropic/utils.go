package anthropic

import (
	"regexp"
	"strconv"
	"strings"

	relaymodel "github.com/labring/aiproxy/core/relay/model"
	relayutils "github.com/labring/aiproxy/core/relay/utils"
)

var (
	claudeFamilyAfterVersionPattern = regexp.MustCompile(
		`(?i)^claude-(\d+)(?:-(\d+))?-(opus|sonnet|haiku)(?:-|$)`,
	)
	claudeFamilyBeforeVersionPattern = regexp.MustCompile(
		`(?i)^claude-(opus|sonnet|haiku)-(\d+)(?:-(\d+))?(?:-|$)`,
	)
)

func shouldAutoUseAdaptiveThinking(model string) bool {
	return isClaudeMythosPreview(model) || isClaudeAdaptiveThinkingModel(model)
}

func ResolveModelName(originModel, actualModel string) string {
	if modelName := relayutils.FirstMatchingModelName(
		originModel,
		actualModel,
		func(modelName string) bool {
			modelName = strings.ToLower(modelName)
			return strings.Contains(modelName, "claude") || strings.Contains(modelName, "mythos")
		},
	); modelName != "" {
		return modelName
	}

	return relayutils.PreferredModelName(originModel, actualModel)
}

func isClaudeAdaptiveThinkingModel(model string) bool {
	if isClaudeMythosPreview(model) {
		return true
	}

	family, major, minor, ok := parseClaudeFamilyVersion(model)
	if !ok {
		return true
	}

	switch {
	case major < 4:
		return false
	case major > 4:
		return true
	}

	switch family {
	case "opus", "sonnet", "haiku":
	default:
		return true
	}

	return minor >= 6
}

func isClaudeAdaptiveOnlyModel(model string) bool {
	family, major, minor, ok := parseClaudeFamilyVersion(model)
	if !ok {
		return false
	}

	return family == "opus" && major == 4 && minor >= 7
}

func isClaudeMythosPreview(model string) bool {
	model = strings.ToLower(model)

	return strings.Contains(model, "claude-mythos-preview") ||
		strings.Contains(model, "mythos-preview")
}

func parseClaudeFamilyVersion(model string) (family string, major, minor int, ok bool) {
	model = strings.ToLower(model)

	if matches := claudeFamilyAfterVersionPattern.FindStringSubmatch(model); len(matches) == 4 {
		major, err := strconv.Atoi(matches[1])
		if err != nil {
			return "", 0, 0, false
		}

		minor := 0
		if matches[2] != "" {
			parsedMinor, convErr := strconv.Atoi(matches[2])
			if convErr != nil {
				return "", 0, 0, false
			}

			minor = parsedMinor
		}

		return matches[3], major, minor, true
	}

	if matches := claudeFamilyBeforeVersionPattern.FindStringSubmatch(model); len(matches) == 4 {
		major, err := strconv.Atoi(matches[2])
		if err != nil {
			return "", 0, 0, false
		}

		minor := 0
		if matches[3] != "" {
			parsedMinor, convErr := strconv.Atoi(matches[3])
			if convErr != nil {
				return "", 0, 0, false
			}

			minor = parsedMinor
		}

		return matches[1], major, minor, true
	}

	return "", 0, 0, false
}

func normalizeClaudeThinking(
	model string,
	maxTokens *int,
	thinking **relaymodel.ClaudeThinking,
	outputConfig **relaymodel.ClaudeOutputConfig,
) {
	if thinking == nil || *thinking == nil {
		return
	}

	currentThinking := *thinking
	if currentThinking == nil {
		return
	}

	currentOutputConfig := func() *relaymodel.ClaudeOutputConfig {
		if outputConfig == nil {
			return nil
		}
		return *outputConfig
	}

	switch currentThinking.Type {
	case relaymodel.ClaudeThinkingTypeDisabled:
		currentThinking.BudgetTokens = 0

		if outputConfig != nil {
			*outputConfig = nil
		}

		if isClaudeMythosPreview(model) || isClaudeAdaptiveOnlyModel(model) {
			*thinking = nil
		}

		return
	case relaymodel.ClaudeThinkingTypeAdaptive:
		if !isClaudeAdaptiveThinkingModel(model) && !isClaudeMythosPreview(model) {
			currentThinking.Type = relaymodel.ClaudeThinkingTypeEnabled
			if outputConfig != nil && *outputConfig != nil && (*outputConfig).Effort != nil {
				currentThinking.BudgetTokens = relayutils.ReasoningToBudget(
					relaymodel.NormalizedReasoning{
						Specified: true,
						Effort:    relaymodel.NormalizeReasoningEffort(*(*outputConfig).Effort),
					},
				)
			}

			adjustThinkingBudgetTokens(maxTokens, &currentThinking.BudgetTokens)

			if outputConfig != nil {
				*outputConfig = nil
			}

			return
		}

		if outputConfig != nil {
			if *outputConfig == nil || (*outputConfig).Effort == nil {
				effort := relayutils.ClaudeOutputEffort(
					relayutils.ParseClaudeReasoning(currentThinking, currentOutputConfig()),
				)
				*outputConfig = &relaymodel.ClaudeOutputConfig{Effort: &effort}
			}
		}

		currentThinking.BudgetTokens = 0

		return
	case "", relaymodel.ClaudeThinkingTypeEnabled:
		if shouldAutoUseAdaptiveThinking(model) {
			effort := relayutils.ClaudeOutputEffort(
				relayutils.ParseClaudeReasoning(currentThinking, currentOutputConfig()),
			)
			currentThinking.Type = relaymodel.ClaudeThinkingTypeAdaptive
			currentThinking.BudgetTokens = 0

			if outputConfig != nil {
				*outputConfig = &relaymodel.ClaudeOutputConfig{Effort: &effort}
			}

			return
		}

		currentThinking.Type = relaymodel.ClaudeThinkingTypeEnabled
		adjustThinkingBudgetTokens(maxTokens, &currentThinking.BudgetTokens)

		if outputConfig != nil {
			*outputConfig = nil
		}

		return
	default:
		if shouldAutoUseAdaptiveThinking(model) {
			effort := relayutils.ClaudeOutputEffort(
				relayutils.ParseClaudeReasoning(currentThinking, currentOutputConfig()),
			)
			currentThinking.Type = relaymodel.ClaudeThinkingTypeAdaptive
			currentThinking.BudgetTokens = 0

			if outputConfig != nil {
				*outputConfig = &relaymodel.ClaudeOutputConfig{Effort: &effort}
			}

			return
		}

		currentThinking.Type = relaymodel.ClaudeThinkingTypeEnabled
		adjustThinkingBudgetTokens(maxTokens, &currentThinking.BudgetTokens)

		if outputConfig != nil {
			*outputConfig = nil
		}
	}
}

// adjustThinkingBudgetTokens adjusts thinking.budget_tokens to ensure it's less than max_tokens
// according to the following rules:
// 1. If budget_tokens is 0 or >= max_tokens, set it to max_tokens / 2
// 2. If budget_tokens < 1024, set it to 1024
// 3. If max_tokens is still <= budget_tokens, set max_tokens to budget_tokens * 2
func adjustThinkingBudgetTokens(maxTokens, budgetTokens *int) {
	if budgetTokens == nil {
		return
	}

	if *budgetTokens == 0 {
		*budgetTokens = 1024
	}

	if *budgetTokens < 1024 {
		*budgetTokens = 1024
	}

	if maxTokens == nil {
		return
	}

	if *maxTokens <= *budgetTokens {
		*maxTokens = max(*budgetTokens+1, 2048)
	}
}
