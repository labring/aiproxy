package anthropic

// adjustThinkingBudgetTokens adjusts thinking.budget_tokens to ensure it's less than max_tokens
// according to the following rules:
// 1. If budget_tokens is 0 or >= max_tokens, set it to max_tokens / 2
// 2. If budget_tokens < 1024, set it to 1024
// 3. If max_tokens is still <= budget_tokens, set max_tokens to budget_tokens * 2
func adjustThinkingBudgetTokens(maxTokens, budgetTokens *int) {
	if maxTokens == nil || budgetTokens == nil {
		return
	}

	// Step 1: If budget_tokens is 0 or max_tokens <= budget_tokens, adjust budget_tokens to half of max_tokens
	if *budgetTokens == 0 || *maxTokens <= *budgetTokens {
		*budgetTokens = *maxTokens / 2
	}

	// Step 2: If budget_tokens < 1024, set it to 1024
	if *budgetTokens < 1024 {
		*budgetTokens = 1024
	}

	// Step 3: If max_tokens is still <= budget_tokens, adjust max_tokens to double budget_tokens
	if *maxTokens <= *budgetTokens {
		*maxTokens = *budgetTokens * 2
	}
}
