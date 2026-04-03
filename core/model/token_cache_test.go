package model

import (
	"slices"
	"testing"
)

func TestTokenCacheRangeFiltersStaleModels(t *testing.T) {
	token := TokenCache{
		Models: redisStringSlice{
			"deepseek/deepseek-v3.2",
			"gpt-4o",
			"moonshotai/kimi-k2.5",
		},
		availableSets: []string{ChannelDefaultSet},
		modelsBySet: map[string][]string{
			ChannelDefaultSet: {
				"gpt-4o",
				"deepseek/deepseek-v3",
			},
		},
	}

	got := make([]string, 0)
	token.Range(func(model string) bool {
		got = append(got, model)
		return true
	})

	want := []string{"gpt-4o"}
	if !slices.Equal(got, want) {
		t.Fatalf("Range() = %v, want %v", got, want)
	}
}

func TestTokenCacheFindModelCaseInsensitive(t *testing.T) {
	token := TokenCache{
		Models:        redisStringSlice{"gpt-4o"},
		availableSets: []string{ChannelDefaultSet},
		modelsBySet: map[string][]string{
			ChannelDefaultSet: {"gpt-4o"},
		},
	}

	got := token.FindModel("GPT-4O")
	if got != "gpt-4o" {
		t.Fatalf("FindModel() = %q, want %q", got, "gpt-4o")
	}
}

func TestIsModelAllowedByToken(t *testing.T) {
	cases := []struct {
		name   string
		models redisStringSlice
		model  string
		want   bool
	}{
		{name: "empty whitelist allows all", models: nil, model: "gpt-4o", want: true},
		{name: "model in whitelist", models: redisStringSlice{"gpt-4o"}, model: "gpt-4o", want: true},
		{name: "case-insensitive match", models: redisStringSlice{"GPT-4O"}, model: "gpt-4o", want: true},
		{name: "model not in whitelist", models: redisStringSlice{"gpt-4o"}, model: "gpt-5", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok := TokenCache{Models: tc.models}
			if got := tok.IsModelAllowedByToken(tc.model); got != tc.want {
				t.Fatalf("IsModelAllowedByToken(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}

func TestBuildPassthroughChannelsBySet(t *testing.T) {
	ch1 := &Channel{
		ID:   1,
		Sets: []string{ChannelDefaultSet},
		Configs: ChannelConfigs{
			ChannelConfigAllowPassthroughUnknown: true,
		},
	}
	ch2 := &Channel{
		ID:   2,
		Sets: []string{"enterprise"},
		Configs: ChannelConfigs{
			ChannelConfigAllowPassthroughUnknown: false,
		},
	}
	ch3 := &Channel{
		ID:      3,
		Sets:    []string{ChannelDefaultSet, "enterprise"},
		Configs: ChannelConfigs{},
	}

	result := buildPassthroughChannelsBySet([]*Channel{ch1, ch2, ch3})

	if len(result[ChannelDefaultSet]) != 1 || result[ChannelDefaultSet][0].ID != 1 {
		t.Fatalf("expected ch1 in default set, got %v", result[ChannelDefaultSet])
	}

	if len(result["enterprise"]) != 0 {
		t.Fatalf("ch2 has allow=false, should not appear; got %v", result["enterprise"])
	}
}

func TestModelCachesHasPassthroughChannels(t *testing.T) {
	mc := &ModelCaches{
		PassthroughChannelsBySet: map[string][]*Channel{
			ChannelDefaultSet: {{ID: 1}},
		},
	}

	if !mc.HasPassthroughChannels([]string{ChannelDefaultSet}) {
		t.Fatal("expected true for default set")
	}

	if mc.HasPassthroughChannels([]string{"other"}) {
		t.Fatal("expected false for unknown set")
	}

	if mc.HasPassthroughChannels(nil) {
		t.Fatal("expected false for nil sets")
	}
}

func TestTokenCacheRangeReturnsAllGroupModelsWhenTokenModelsEmpty(t *testing.T) {
	token := TokenCache{
		availableSets: []string{ChannelDefaultSet, "alt"},
		modelsBySet: map[string][]string{
			ChannelDefaultSet: {"gpt-4o", "gpt-4.1"},
			"alt":             {"claude-sonnet-4-5-20250929", "gpt-4o"},
		},
	}

	got := make([]string, 0)
	token.Range(func(model string) bool {
		got = append(got, model)
		return true
	})

	want := []string{
		"gpt-4o",
		"gpt-4.1",
		"claude-sonnet-4-5-20250929",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Range() = %v, want %v", got, want)
	}
}
