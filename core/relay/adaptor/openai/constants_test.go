package openai_test

import (
	"net/http"
	"testing"

	"github.com/labring/aiproxy/core/common/consume"
	coremodel "github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/stretchr/testify/require"
)

func TestGPT56ModelConfigs(t *testing.T) {
	tests := []struct {
		model              string
		inputPrice         float64
		cachedPrice        float64
		cacheCreationPrice float64
		outputPrice        float64
	}{
		{"gpt-5.6", 0.005, 0.0005, 0.00625, 0.030},
		{"gpt-5.6-sol", 0.005, 0.0005, 0.00625, 0.030},
		{"gpt-5.6-terra", 0.0025, 0.00025, 0.003125, 0.015},
		{"gpt-5.6-luna", 0.001, 0.0001, 0.00125, 0.006},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			config := findOpenAIModelConfig(t, tt.model)
			require.Equal(t, mode.ChatCompletions, config.Type)
			require.False(t, openai.IsResponsesOnlyModel(&config, tt.model))
			require.NoError(t, config.Price.ValidateConditionalPrices())

			price := config.Price.SelectConditionalPriceWithOptions(
				coremodel.Usage{InputTokens: 1000},
				coremodel.UsageContext{},
				coremodel.PriceSelectionOptions{},
			)
			require.InDelta(t, tt.inputPrice, float64(price.InputPrice), 1e-12)
			require.InDelta(t, tt.cachedPrice, float64(price.CachedPrice), 1e-12)
			require.InDelta(
				t,
				tt.cacheCreationPrice,
				float64(price.CacheCreationPrice),
				1e-12,
			)
			require.InDelta(t, tt.outputPrice, float64(price.OutputPrice), 1e-12)
		})
	}
}

func TestGPT6AstraModelConfig(t *testing.T) {
	config := findOpenAIModelConfig(t, "gpt-6-astra")
	require.Equal(t, mode.ChatCompletions, config.Type)
	require.True(t, openai.IsResponsesOnlyModel(&config, config.Model))
	require.True(t, config.ShouldSummaryServiceTier())
	require.NoError(t, config.Price.ValidateConditionalPrices())

	maxContextTokens, ok := config.MaxContextTokens()
	require.True(t, ok)
	require.Equal(t, 1050000, maxContextTokens)

	maxInputTokens, ok := config.MaxInputTokens()
	require.True(t, ok)
	require.Equal(t, 922000, maxInputTokens)

	maxOutputTokens, ok := config.MaxOutputTokens()
	require.True(t, ok)
	require.Equal(t, 128000, maxOutputTokens)
}

func TestGPT6AstraConditionalPricing(t *testing.T) {
	config := findOpenAIModelConfig(t, "gpt-6-astra")
	tests := []struct {
		name               string
		inputTokens        int64
		serviceTier        string
		inputPrice         float64
		cachedPrice        float64
		cacheCreationPrice float64
		outputPrice        float64
	}{
		{"standard", 272000, "", 0.010, 0.001, 0.0125, 0.050},
		{"standard long context", 272001, "", 0.020, 0.002, 0.025, 0.075},
		{"flex", 272000, "flex", 0.005, 0.0005, 0.00625, 0.025},
		{"flex long context", 272001, "flex", 0.010, 0.001, 0.0125, 0.0375},
		{"priority", 272000, "priority", 0.020, 0.002, 0.025, 0.100},
		{"priority long context", 272001, "priority", 0.040, 0.004, 0.050, 0.150},
		{"fast", 272000, "fast", 0.020, 0.002, 0.025, 0.100},
		{"fast long context", 272001, "fast", 0.040, 0.004, 0.050, 0.150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price := config.Price.SelectConditionalPriceWithOptions(
				coremodel.Usage{InputTokens: coremodel.ZeroNullInt64(tt.inputTokens)},
				coremodel.UsageContext{ServiceTier: tt.serviceTier},
				coremodel.PriceSelectionOptions{},
			)
			require.InDelta(t, tt.inputPrice, float64(price.InputPrice), 1e-12)
			require.InDelta(t, tt.cachedPrice, float64(price.CachedPrice), 1e-12)
			require.InDelta(
				t,
				tt.cacheCreationPrice,
				float64(price.CacheCreationPrice),
				1e-12,
			)
			require.InDelta(t, tt.outputPrice, float64(price.OutputPrice), 1e-12)
		})
	}
}

func TestGPT56SolConditionalPricing(t *testing.T) {
	config := findOpenAIModelConfig(t, "gpt-5.6-sol")
	tests := []struct {
		name               string
		inputTokens        int64
		serviceTier        string
		inputPrice         float64
		cacheCreationPrice float64
		outputPrice        float64
	}{
		{"standard", 272000, "", 0.005, 0.00625, 0.030},
		{"standard long context", 272001, "", 0.010, 0.0125, 0.045},
		{"flex", 272000, "flex", 0.0025, 0.003125, 0.015},
		{"flex long context", 272001, "flex", 0.005, 0.00625, 0.0225},
		{"priority", 272000, "priority", 0.010, 0.0125, 0.060},
		{"priority long context", 272001, "priority", 0.020, 0.025, 0.090},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price := config.Price.SelectConditionalPriceWithOptions(
				coremodel.Usage{InputTokens: coremodel.ZeroNullInt64(tt.inputTokens)},
				coremodel.UsageContext{ServiceTier: tt.serviceTier},
				coremodel.PriceSelectionOptions{},
			)
			require.InDelta(t, tt.inputPrice, float64(price.InputPrice), 1e-12)
			require.InDelta(
				t,
				tt.cacheCreationPrice,
				float64(price.CacheCreationPrice),
				1e-12,
			)
			require.InDelta(t, tt.outputPrice, float64(price.OutputPrice), 1e-12)
		})
	}
}

func TestGPT56CacheWriteBilling(t *testing.T) {
	config := findOpenAIModelConfig(t, "gpt-5.6-sol")
	amount := consume.CalculateAmountDetail(
		http.StatusOK,
		coremodel.Usage{
			InputTokens:         1000,
			OutputTokens:        100,
			CachedTokens:        300,
			CacheCreationTokens: 200,
		},
		coremodel.UsageContext{},
		config.Price,
	)

	require.InDelta(t, 0.0025, amount.InputAmount, 1e-12)
	require.InDelta(t, 0.00015, amount.CachedAmount, 1e-12)
	require.InDelta(t, 0.00125, amount.CacheCreationAmount, 1e-12)
	require.InDelta(t, 0.003, amount.OutputAmount, 1e-12)
	require.InDelta(t, 0.0069, amount.UsedAmount, 1e-12)
}

func findOpenAIModelConfig(t *testing.T, modelName string) coremodel.ModelConfig {
	t.Helper()

	for _, config := range openai.ModelList {
		if config.Model == modelName {
			return config
		}
	}

	t.Fatalf("OpenAI model config %q not found", modelName)

	return coremodel.ModelConfig{}
}
