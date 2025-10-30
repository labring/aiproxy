package howtocook

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"

	"github.com/bytedance/sonic"
	mcpservers "github.com/labring/aiproxy/mcp-servers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	RecipesURL = "https://weilei.site/all_recipes.json"
	Version    = "0.0.6"
)

// Server represents the HowToCook MCP server
type Server struct {
	*server.MCPServer
	recipes    []Recipe
	categories []string
}

// NewServer creates a new HowToCook MCP server
func NewServer(_, _ map[string]string) (mcpservers.Server, error) {
	// Create MCP server
	mcpServer := server.NewMCPServer("howtocook-mcp", Version)

	cookServer := &Server{
		MCPServer: mcpServer,
	}

	// Initialize recipes and categories
	if err := cookServer.initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize server: %w", err)
	}

	// Add tools
	cookServer.addTools()

	return cookServer, nil
}

func ListTools(ctx context.Context) ([]mcp.Tool, error) {
	cookServer := &Server{
		MCPServer: server.NewMCPServer("howtocook-mcp", Version),
	}
	cookServer.addTools()

	return mcpservers.ListServerTools(ctx, cookServer)
}

// initialize loads recipe data and categories
func (s *Server) initialize(ctx context.Context) error {
	recipes, err := s.fetchRecipes(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch recipes: %w", err)
	}

	if len(recipes) == 0 {
		return errors.New("no recipes found")
	}

	s.recipes = recipes
	s.categories = s.getAllCategories()

	return nil
}

// addTools adds all tools to the server
func (s *Server) addTools() {
	s.addGetAllRecipesTool()
	s.addGetRecipesByCategoryTool()
	s.addRecommendMealsTool()
	s.addWhatToEatTool()
}

// addGetAllRecipesTool adds the get all recipes tool
func (s *Server) addGetAllRecipesTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "mcp_howtocook_getAllRecipes",
			Description: "获取所有菜谱",
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Properties: map[string]any{},
				Required:   []string{},
			},
		},
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			simplifiedRecipes := make([]NameOnlyRecipe, 0, len(s.recipes))
			for _, recipe := range s.recipes {
				simplifiedRecipes = append(simplifiedRecipes, NameOnlyRecipe{
					Name:        recipe.Name,
					Description: recipe.Description,
				})
			}

			result, err := sonic.Marshal(simplifiedRecipes)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal recipes: %w", err)
			}

			return mcp.NewToolResultText(string(result)), nil
		},
	)
}

// addGetRecipesByCategoryTool adds the get recipes by category tool
func (s *Server) addGetRecipesByCategoryTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "mcp_howtocook_getRecipesByCategory",
			Description: "根据分类查询菜谱，可选分类有: " + strings.Join(s.categories, ", "),
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"category": map[string]any{
						"type":        "string",
						"description": "菜谱分类名称，如水产、早餐、荤菜、主食等",
						"enum":        s.categories,
					},
				},
				Required: []string{"category"},
			},
		},
		s.handleGetRecipesByCategory,
	)
}

// addRecommendMealsTool adds the recommend meals tool
func (s *Server) addRecommendMealsTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "mcp_howtocook_recommendMeals",
			Description: "根据用户的忌口、过敏原、人数智能推荐菜谱，创建一周的膳食计划以及大致的购物清单",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"allergies": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "过敏原列表，如[\"大蒜\", \"虾\"]",
					},
					"avoidItems": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "忌口食材列表，如[\"葱\", \"姜\"]",
					},
					"peopleCount": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     10,
						"description": "用餐人数，1-10之间的整数",
					},
				},
				Required: []string{"peopleCount"},
			},
		},
		s.handleRecommendMeals,
	)
}

// addWhatToEatTool adds the what to eat tool
func (s *Server) addWhatToEatTool() {
	s.AddTool(
		mcp.Tool{
			Name:        "mcp_howtocook_whatToEat",
			Description: "不知道吃什么？根据人数直接推荐适合的菜品组合",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"peopleCount": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     10,
						"description": "用餐人数，1-10之间的整数，会根据人数推荐合适数量的菜品",
					},
				},
				Required: []string{"peopleCount"},
			},
		},
		s.handleWhatToEat,
	)
}

// handleGetRecipesByCategory handles the get recipes by category request
func (s *Server) handleGetRecipesByCategory(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	category, ok := args["category"].(string)
	if !ok || category == "" {
		return nil, errors.New("category is required")
	}

	var filteredRecipes []SimpleRecipe
	for _, recipe := range s.recipes {
		if recipe.Category == category {
			filteredRecipes = append(filteredRecipes, s.simplifyRecipe(recipe))
		}
	}

	result, err := sonic.Marshal(filteredRecipes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal recipes: %w", err)
	}

	return mcp.NewToolResultText(string(result)), nil
}

// handleRecommendMeals handles the recommend meals request
func (s *Server) handleRecommendMeals(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	peopleCount, ok := args["peopleCount"].(float64)
	if !ok {
		return nil, errors.New("peopleCount is required")
	}

	var allergies, avoidItems []string
	if allergiesRaw, ok := args["allergies"].([]any); ok {
		for _, allergy := range allergiesRaw {
			if allergyStr, ok := allergy.(string); ok {
				allergies = append(allergies, allergyStr)
			}
		}
	}

	if avoidItemsRaw, ok := args["avoidItems"].([]any); ok {
		for _, item := range avoidItemsRaw {
			if itemStr, ok := item.(string); ok {
				avoidItems = append(avoidItems, itemStr)
			}
		}
	}

	mealPlan := s.generateMealPlan(int(peopleCount), allergies, avoidItems)

	result, err := sonic.Marshal(mealPlan)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal meal plan: %w", err)
	}

	return mcp.NewToolResultText(string(result)), nil
}

// handleWhatToEat handles the what to eat request
func (s *Server) handleWhatToEat(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	peopleCount, ok := args["peopleCount"].(float64)
	if !ok {
		return nil, errors.New("peopleCount is required")
	}

	recommendation := s.generateDishRecommendation(int(peopleCount))

	result, err := sonic.Marshal(recommendation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal recommendation: %w", err)
	}

	return mcp.NewToolResultText(string(result)), nil
}

// generateMealPlan generates a weekly meal plan
func (s *Server) generateMealPlan(peopleCount int, allergies, avoidItems []string) MealPlan {
	// Filter recipes based on allergies and avoid items
	filteredRecipes := s.filterRecipes(allergies, avoidItems)

	// Group recipes by category
	recipesByCategory := s.groupRecipesByCategory(filteredRecipes)

	mealPlan := MealPlan{
		Weekdays: make([]DayPlan, 5),
		Weekend:  make([]DayPlan, 2),
		GroceryList: GroceryList{
			Ingredients: []GroceryItem{},
			ShoppingPlan: ShoppingPlan{
				Fresh:  []string{},
				Pantry: []string{},
				Spices: []string{},
				Others: []string{},
			},
		},
	}

	var selectedRecipes []Recipe

	// Generate weekday plans
	weekdays := []string{"周一", "周二", "周三", "周四", "周五"}
	for i := range 5 {
		dayPlan := DayPlan{
			Day:       weekdays[i],
			Breakfast: []SimpleRecipe{},
			Lunch:     []SimpleRecipe{},
			Dinner:    []SimpleRecipe{},
		}

		// Breakfast
		breakfastCount := int(math.Max(1, math.Ceil(float64(peopleCount)/5)))
		dayPlan.Breakfast, selectedRecipes = s.selectMeals(
			recipesByCategory,
			"早餐",
			breakfastCount,
			selectedRecipes,
		)

		// Lunch and dinner
		mealCount := int(math.Max(2, math.Ceil(float64(peopleCount)/3)))
		dayPlan.Lunch, selectedRecipes = s.selectVariedMeals(
			recipesByCategory,
			mealCount,
			selectedRecipes,
		)
		dayPlan.Dinner, selectedRecipes = s.selectVariedMeals(
			recipesByCategory,
			mealCount,
			selectedRecipes,
		)

		mealPlan.Weekdays[i] = dayPlan
	}

	// Generate weekend plans
	weekendDays := []string{"周六", "周日"}
	for i := range 2 {
		dayPlan := DayPlan{
			Day:       weekendDays[i],
			Breakfast: []SimpleRecipe{},
			Lunch:     []SimpleRecipe{},
			Dinner:    []SimpleRecipe{},
		}

		// Weekend breakfast
		breakfastCount := int(math.Max(2, math.Ceil(float64(peopleCount)/3)))
		dayPlan.Breakfast, selectedRecipes = s.selectMeals(
			recipesByCategory,
			"早餐",
			breakfastCount,
			selectedRecipes,
		)

		// Weekend meals (more dishes)
		weekdayMealCount := int(math.Max(2, math.Ceil(float64(peopleCount)/3)))

		var weekendAddition int
		if peopleCount <= 4 {
			weekendAddition = 1
		} else {
			weekendAddition = 2
		}

		mealCount := weekdayMealCount + weekendAddition

		dayPlan.Lunch, selectedRecipes = s.selectWeekendMeals(
			recipesByCategory,
			mealCount,
			selectedRecipes,
		)
		dayPlan.Dinner, selectedRecipes = s.selectWeekendMeals(
			recipesByCategory,
			mealCount,
			selectedRecipes,
		)

		mealPlan.Weekend[i] = dayPlan
	}

	// Generate grocery list
	mealPlan.GroceryList = s.generateGroceryList(selectedRecipes)

	return mealPlan
}

// generateDishRecommendation generates dish recommendations based on people count
func (s *Server) generateDishRecommendation(peopleCount int) DishRecommendation {
	vegetableCount := (peopleCount + 1) / 2
	meatCount := int(math.Ceil(float64(peopleCount+1) / 2))

	var meatDishes []Recipe
	for _, recipe := range s.recipes {
		if recipe.Category == "荤菜" || recipe.Category == "水产" {
			meatDishes = append(meatDishes, recipe)
		}
	}

	var vegetableDishes []Recipe
	for _, recipe := range s.recipes {
		if recipe.Category != "荤菜" && recipe.Category != "水产" &&
			recipe.Category != "早餐" && recipe.Category != "主食" {
			vegetableDishes = append(vegetableDishes, recipe)
		}
	}

	var (
		recommendedDishes []Recipe
		fishDish          *Recipe
	)

	// Add fish dish for large groups

	if peopleCount > 8 {
		var fishDishes []Recipe
		for _, recipe := range s.recipes {
			if recipe.Category == "水产" {
				fishDishes = append(fishDishes, recipe)
			}
		}

		if len(fishDishes) > 0 {
			selected := fishDishes[rand.Intn(len(fishDishes))]
			fishDish = &selected
			recommendedDishes = append(recommendedDishes, selected)
		}
	}

	// Select meat dishes
	remainingMeatCount := meatCount
	if fishDish != nil {
		remainingMeatCount--
	}

	selectedMeatDishes := s.selectMeatDishes(meatDishes, remainingMeatCount)
	recommendedDishes = append(recommendedDishes, selectedMeatDishes...)

	// Select vegetable dishes
	selectedVegetableDishes := s.selectRandomDishes(vegetableDishes, vegetableCount)
	recommendedDishes = append(recommendedDishes, selectedVegetableDishes...)

	// Convert to simple recipes
	simpleDishes := make([]SimpleRecipe, len(recommendedDishes))
	for _, dish := range recommendedDishes {
		simpleDishes = append(simpleDishes, s.simplifyRecipe(dish))
	}

	fishCount := 0
	if fishDish != nil {
		fishCount = 1
	}

	return DishRecommendation{
		PeopleCount:        peopleCount,
		MeatDishCount:      len(selectedMeatDishes) + fishCount,
		VegetableDishCount: len(selectedVegetableDishes),
		Dishes:             simpleDishes,
		Message: fmt.Sprintf("为%d人推荐的菜品，包含%d个荤菜和%d个素菜。",
			peopleCount, len(selectedMeatDishes)+fishCount, len(selectedVegetableDishes)),
	}
}

// Helper methods would continue here...
// Due to length constraints, I'll provide the key helper methods:

// simplifyRecipe converts Recipe to SimpleRecipe
func (s *Server) simplifyRecipe(recipe Recipe) SimpleRecipe {
	ingredients := make([]SimpleIngredient, len(recipe.Ingredients))
	for _, ing := range recipe.Ingredients {
		ingredients = append(ingredients, SimpleIngredient{
			Name:         ing.Name,
			TextQuantity: ing.TextQuantity,
		})
	}

	return SimpleRecipe{
		ID:          recipe.ID,
		Name:        recipe.Name,
		Description: recipe.Description,
		Ingredients: ingredients,
	}
}

// filterRecipes filters recipes based on allergies and avoid items
func (s *Server) filterRecipes(allergies, avoidItems []string) []Recipe {
	var filtered []Recipe
	for _, recipe := range s.recipes {
		hasAllergyOrAvoid := false
		for _, ingredient := range recipe.Ingredients {
			name := strings.ToLower(ingredient.Name)
			for _, allergy := range allergies {
				if strings.Contains(name, strings.ToLower(allergy)) {
					hasAllergyOrAvoid = true
					break
				}
			}

			if hasAllergyOrAvoid {
				break
			}

			for _, avoid := range avoidItems {
				if strings.Contains(name, strings.ToLower(avoid)) {
					hasAllergyOrAvoid = true
					break
				}
			}

			if hasAllergyOrAvoid {
				break
			}
		}

		if !hasAllergyOrAvoid {
			filtered = append(filtered, recipe)
		}
	}

	return filtered
}

func (s *Server) groupRecipesByCategory(recipes []Recipe) map[string][]Recipe {
	recipesByCategory := make(map[string][]Recipe)
	targetCategories := []string{"水产", "早餐", "荤菜", "主食", "素菜", "甜品", "汤羹"}

	for _, recipe := range recipes {
		for _, category := range targetCategories {
			if recipe.Category == category {
				if recipesByCategory[category] == nil {
					recipesByCategory[category] = []Recipe{}
				}

				recipesByCategory[category] = append(recipesByCategory[category], recipe)

				break
			}
		}
	}

	return recipesByCategory
}

// selectMeals selects meals from a specific category
func (s *Server) selectMeals(
	recipesByCategory map[string][]Recipe,
	category string,
	count int,
	selectedRecipes []Recipe,
) ([]SimpleRecipe, []Recipe) {
	var meals []SimpleRecipe

	if recipes, exists := recipesByCategory[category]; exists && len(recipes) > 0 {
		for i := 0; i < count && len(recipes) > 0; i++ {
			index := rand.Intn(len(recipes))
			selectedRecipe := recipes[index]
			selectedRecipes = append(selectedRecipes, selectedRecipe)
			meals = append(meals, s.simplifyRecipe(selectedRecipe))

			// Remove selected recipe to avoid duplication
			recipes = append(recipes[:index], recipes[index+1:]...)
			recipesByCategory[category] = recipes
		}
	}

	return meals, selectedRecipes
}

// selectVariedMeals selects meals from various categories for lunch/dinner
func (s *Server) selectVariedMeals(
	recipesByCategory map[string][]Recipe,
	count int,
	selectedRecipes []Recipe,
) ([]SimpleRecipe, []Recipe) {
	var meals []SimpleRecipe

	categories := []string{"主食", "水产", "荤菜", "素菜", "甜品"}

	for range count {
		selectedCategory := categories[rand.Intn(len(categories))]

		// Try to find a category with available recipes
		attempts := 0
		for attempts < len(categories) {
			if recipes, exists := recipesByCategory[selectedCategory]; exists && len(recipes) > 0 {
				index := rand.Intn(len(recipes))
				selectedRecipe := recipes[index]
				selectedRecipes = append(selectedRecipes, selectedRecipe)
				meals = append(meals, s.simplifyRecipe(selectedRecipe))

				// Remove selected recipe
				recipes = append(recipes[:index], recipes[index+1:]...)
				recipesByCategory[selectedCategory] = recipes

				break
			}

			// Try next category
			attempts++
			selectedCategory = categories[(rand.Intn(len(categories)))]
		}
	}

	return meals, selectedRecipes
}

// selectWeekendMeals selects meals for weekend with preference for meat and seafood
func (s *Server) selectWeekendMeals(
	recipesByCategory map[string][]Recipe,
	count int,
	selectedRecipes []Recipe,
) ([]SimpleRecipe, []Recipe) {
	var meals []SimpleRecipe

	categories := []string{"荤菜", "水产"}

	for i := range count {
		category := categories[i%len(categories)]

		if recipes, exists := recipesByCategory[category]; exists && len(recipes) > 0 {
			index := rand.Intn(len(recipes))
			selectedRecipe := recipes[index]
			selectedRecipes = append(selectedRecipes, selectedRecipe)
			meals = append(meals, s.simplifyRecipe(selectedRecipe))

			// Remove selected recipe
			recipes = append(recipes[:index], recipes[index+1:]...)
			recipesByCategory[category] = recipes
		} else if recipes, exists := recipesByCategory["主食"]; exists && len(recipes) > 0 {
			// Fallback to 主食 if no meat/seafood available
			index := rand.Intn(len(recipes))
			selectedRecipe := recipes[index]
			selectedRecipes = append(selectedRecipes, selectedRecipe)
			meals = append(meals, s.simplifyRecipe(selectedRecipe))

			// Remove selected recipe
			recipes = append(recipes[:index], recipes[index+1:]...)
			recipesByCategory["主食"] = recipes
		}
	}

	return meals, selectedRecipes
}

// selectMeatDishes selects meat dishes with preference for different meat types
func (s *Server) selectMeatDishes(meatDishes []Recipe, count int) []Recipe {
	//nolint:prealloc
	var selectedMeatDishes []Recipe

	meatTypes := []string{"猪肉", "鸡肉", "牛肉", "羊肉", "鸭肉", "鱼肉"}
	availableDishes := make([]Recipe, len(meatDishes))
	copy(availableDishes, meatDishes)

	// Try to select different meat types
	for _, meatType := range meatTypes {
		if len(selectedMeatDishes) >= count {
			break
		}

		var (
			meatTypeOptions []Recipe
			meatTypeIndices []int
		)

		for i, dish := range availableDishes {
			for _, ingredient := range dish.Ingredients {
				if strings.Contains(strings.ToLower(ingredient.Name), strings.ToLower(meatType)) {
					meatTypeOptions = append(meatTypeOptions, dish)
					meatTypeIndices = append(meatTypeIndices, i)
					break
				}
			}
		}

		if len(meatTypeOptions) > 0 {
			selectedIndex := rand.Intn(len(meatTypeOptions))
			selectedMeatDishes = append(selectedMeatDishes, meatTypeOptions[selectedIndex])

			// Remove selected dish from available dishes
			originalIndex := meatTypeIndices[selectedIndex]
			availableDishes = append(
				availableDishes[:originalIndex],
				availableDishes[originalIndex+1:]...)

			// Adjust indices for remaining items
			for j := range meatTypeIndices {
				if meatTypeIndices[j] > originalIndex {
					meatTypeIndices[j]--
				}
			}
		}
	}

	// Fill remaining slots with random meat dishes
	for len(selectedMeatDishes) < count && len(availableDishes) > 0 {
		index := rand.Intn(len(availableDishes))
		selectedMeatDishes = append(selectedMeatDishes, availableDishes[index])
		availableDishes = append(availableDishes[:index], availableDishes[index+1:]...)
	}

	return selectedMeatDishes
}

// selectRandomDishes selects random dishes from a list
func (s *Server) selectRandomDishes(dishes []Recipe, count int) []Recipe {
	//nolint:prealloc
	var selectedDishes []Recipe

	availableDishes := make([]Recipe, len(dishes))
	copy(availableDishes, dishes)

	for len(selectedDishes) < count && len(availableDishes) > 0 {
		index := rand.Intn(len(availableDishes))
		selectedDishes = append(selectedDishes, availableDishes[index])
		availableDishes = append(availableDishes[:index], availableDishes[index+1:]...)
	}

	return selectedDishes
}

// generateGroceryList generates a grocery list from selected recipes
func (s *Server) generateGroceryList(selectedRecipes []Recipe) GroceryList {
	ingredientMap := make(map[string]*GroceryItem)

	// Process all recipes
	for _, recipe := range selectedRecipes {
		s.processRecipeIngredients(recipe, ingredientMap)
	}

	// Convert map to slice
	ingredients := make([]GroceryItem, len(ingredientMap))
	for _, item := range ingredientMap {
		ingredients = append(ingredients, *item)
	}

	// Sort by usage frequency
	for i := range len(ingredients) - 1 {
		for j := range len(ingredients) - 1 - i {
			if ingredients[j].RecipeCount < ingredients[j+1].RecipeCount {
				ingredients[j], ingredients[j+1] = ingredients[j+1], ingredients[j]
			}
		}
	}

	// Generate shopping plan
	shoppingPlan := ShoppingPlan{
		Fresh:  []string{},
		Pantry: []string{},
		Spices: []string{},
		Others: []string{},
	}

	s.categorizeIngredients(ingredients, &shoppingPlan)

	return GroceryList{
		Ingredients:  ingredients,
		ShoppingPlan: shoppingPlan,
	}
}

// processRecipeIngredients processes ingredients from a recipe
func (s *Server) processRecipeIngredients(recipe Recipe, ingredientMap map[string]*GroceryItem) {
	for _, ingredient := range recipe.Ingredients {
		key := strings.ToLower(ingredient.Name)

		if existingItem, exists := ingredientMap[key]; exists {
			// Update existing ingredient
			if existingItem.Unit != nil && ingredient.Unit != nil &&
				*existingItem.Unit == *ingredient.Unit &&
				existingItem.TotalQuantity != nil && ingredient.Quantity != nil {
				*existingItem.TotalQuantity += *ingredient.Quantity
			} else {
				// Set to nil if units don't match or quantities are uncertain
				existingItem.TotalQuantity = nil
				existingItem.Unit = nil
			}

			existingItem.RecipeCount++

			// Add recipe name if not already present
			found := slices.Contains(existingItem.Recipes, recipe.Name)

			if !found {
				existingItem.Recipes = append(existingItem.Recipes, recipe.Name)
			}
		} else {
			// Create new ingredient entry
			newItem := &GroceryItem{
				Name:          ingredient.Name,
				TotalQuantity: ingredient.Quantity,
				Unit:          ingredient.Unit,
				RecipeCount:   1,
				Recipes:       []string{recipe.Name},
			}
			ingredientMap[key] = newItem
		}
	}
}

// categorizeIngredients categorizes ingredients into shopping plan categories
func (s *Server) categorizeIngredients(ingredients []GroceryItem, shoppingPlan *ShoppingPlan) {
	spiceKeywords := []string{
		"盐",
		"糖",
		"酱油",
		"醋",
		"料酒",
		"香料",
		"胡椒",
		"孜然",
		"辣椒",
		"花椒",
		"姜",
		"蒜",
		"葱",
		"调味",
	}
	freshKeywords := []string{
		"肉",
		"鱼",
		"虾",
		"蛋",
		"奶",
		"菜",
		"菠菜",
		"白菜",
		"青菜",
		"豆腐",
		"生菜",
		"水产",
		"豆芽",
		"西红柿",
		"番茄",
		"水果",
		"香菇",
		"木耳",
		"蘑菇",
	}
	pantryKeywords := []string{
		"米",
		"面",
		"粉",
		"油",
		"酒",
		"醋",
		"糖",
		"盐",
		"酱",
		"豆",
		"干",
		"罐头",
		"方便面",
		"面条",
		"米饭",
		"意大利面",
		"燕麦",
	}

	for _, ingredient := range ingredients {
		name := strings.ToLower(ingredient.Name)

		categorized := false

		// Check spices
		for _, keyword := range spiceKeywords {
			if strings.Contains(name, keyword) {
				shoppingPlan.Spices = append(shoppingPlan.Spices, ingredient.Name)
				categorized = true
				break
			}
		}

		if !categorized {
			// Check fresh items
			for _, keyword := range freshKeywords {
				if strings.Contains(name, keyword) {
					shoppingPlan.Fresh = append(shoppingPlan.Fresh, ingredient.Name)
					categorized = true
					break
				}
			}
		}

		if !categorized {
			// Check pantry items
			for _, keyword := range pantryKeywords {
				if strings.Contains(name, keyword) {
					shoppingPlan.Pantry = append(shoppingPlan.Pantry, ingredient.Name)
					categorized = true
					break
				}
			}
		}

		if !categorized {
			// Default to others
			shoppingPlan.Others = append(shoppingPlan.Others, ingredient.Name)
		}
	}
}
