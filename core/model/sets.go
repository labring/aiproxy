package model

import (
	"slices"
	"strings"
)

func NormalizeAvailableSets(sets []string) []string {
	normalized := cleanAvailableSets(sets)
	if len(normalized) == 0 {
		return []string{ChannelDefaultSet}
	}

	return normalized
}

func cleanAvailableSets(sets []string) []string {
	normalized := make([]string, 0, len(sets))
	seen := make(map[string]struct{}, len(sets))

	for _, set := range sets {
		set = strings.TrimSpace(set)
		if set == "" {
			continue
		}

		if _, ok := seen[set]; ok {
			continue
		}

		seen[set] = struct{}{}
		normalized = append(normalized, set)
	}

	return normalized
}

func IntersectAvailableSets(groupSets, tokenSets []string) []string {
	groupSets = NormalizeAvailableSets(groupSets)
	tokenSets = cleanAvailableSets(tokenSets)

	if len(tokenSets) == 0 {
		return groupSets
	}

	tokenSetMap := make(map[string]struct{}, len(tokenSets))
	for _, set := range tokenSets {
		tokenSetMap[set] = struct{}{}
	}

	result := make([]string, 0, min(len(groupSets), len(tokenSets)))
	for _, set := range groupSets {
		if _, ok := tokenSetMap[set]; ok {
			result = append(result, set)
		}
	}

	return result
}

func ResolveTokenAvailableSets(groupSets, tokenSets []string) []string {
	return IntersectAvailableSets(groupSets, tokenSets)
}

func ResolveTokenGroupChannelAvailableSets(channelSets, tokenSets []string) []string {
	return IntersectAvailableSets(channelSets, tokenSets)
}

func SetsFromModelMap(modelsBySet map[string][]string) []string {
	sets := make([]string, 0, len(modelsBySet))
	for set := range modelsBySet {
		sets = append(sets, set)
	}

	slices.Sort(sets)

	return NormalizeAvailableSets(sets)
}

func FilterModelsBySet(
	modelsBySet map[string][]string,
	availableSets []string,
) map[string][]string {
	availableSets = cleanAvailableSets(availableSets)
	if len(modelsBySet) == 0 || len(availableSets) == 0 {
		return map[string][]string{}
	}

	result := make(map[string][]string, len(modelsBySet))
	for _, set := range availableSets {
		if models, ok := modelsBySet[set]; ok {
			result[set] = models
		}
	}

	return result
}

func FindTokenModel(
	token TokenCache,
	modelName string,
	availableSets []string,
	modelsBySet map[string][]string,
) string {
	return FindModelWithAllowList(token.Models, modelName, availableSets, modelsBySet)
}

func FindModelWithAllowList(
	allowedModels []string,
	modelName string,
	availableSets []string,
	modelsBySet map[string][]string,
) string {
	var findModel string
	if len(allowedModels) != 0 {
		if !slices.ContainsFunc(allowedModels, func(item string) bool {
			ok := strings.EqualFold(item, modelName)
			if ok {
				findModel = item
			}

			return ok
		}) {
			return findModel
		}
	}

	return findModelInSets(modelName, availableSets, modelsBySet)
}

func findModelInSets(
	modelName string,
	availableSets []string,
	modelsBySet map[string][]string,
) string {
	var findModel string
	for _, set := range availableSets {
		if slices.ContainsFunc(modelsBySet[set], func(item string) bool {
			ok := strings.EqualFold(item, modelName)
			if ok {
				findModel = item
			}

			return ok
		}) {
			return findModel
		}
	}

	return findModel
}

func RangeTokenModels(
	token TokenCache,
	availableSets []string,
	modelsBySet map[string][]string,
	fn func(model string) bool,
) {
	RangeModelsWithAllowList(token.Models, availableSets, modelsBySet, fn)
}

func RangeModelsWithAllowList(
	allowedModels []string,
	availableSets []string,
	modelsBySet map[string][]string,
	fn func(model string) bool,
) {
	ranged := make(map[string]struct{})
	if len(allowedModels) != 0 {
		for _, modelName := range allowedModels {
			if _, ok := ranged[modelName]; ok {
				continue
			}

			modelName = findModelInSets(modelName, availableSets, modelsBySet)
			if modelName == "" {
				continue
			}

			ranged[modelName] = struct{}{}
			if !fn(modelName) {
				return
			}
		}

		return
	}

	for _, set := range availableSets {
		for _, modelName := range modelsBySet[set] {
			if _, ok := ranged[modelName]; ok {
				continue
			}

			ranged[modelName] = struct{}{}
			if !fn(modelName) {
				return
			}
		}
	}
}
