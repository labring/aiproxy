package websearch

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptors"
	"github.com/labring/aiproxy/core/relay/controller"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
	"github.com/labring/aiproxy/core/relay/plugin"
	"github.com/labring/aiproxy/core/relay/plugin/noop"
	"github.com/labring/aiproxy/core/relay/plugin/patch"
	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/labring/aiproxy/mcp-servers/hosted/web-search/engine"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var _ plugin.Plugin = (*WebSearch)(nil)

type GetChannel func(modelName string) (*model.Channel, error)

// WebSearch implements web search functionality
type WebSearch struct {
	noop.Noop
	GetChannel GetChannel
}

// NewWebSearchPlugin creates a new web search plugin
func NewWebSearchPlugin(getChannel GetChannel) plugin.Plugin {
	return &WebSearch{
		GetChannel: getChannel,
	}
}

//go:embed prompts/arxiv.md
var arxivSearchPrompts string

//go:embed prompts/internet.md
var internetSearchPrompts string

// Constants for metadata keys
const (
	searchCount  = "web-search-count"
	rewriteUsage = "web-search-rewrite-usage"
)

// Metadata helper functions
func setSearchCount(m *meta.Meta, count int) {
	m.Set(searchCount, count)
}

func getSearchCount(m *meta.Meta) int {
	return m.GetInt(searchCount)
}

func setRewriteUsage(m *meta.Meta, usage model.Usage) {
	m.Set(rewriteUsage, usage)
}

func getRewriteUsage(m *meta.Meta) *model.Usage {
	usage, ok := m.Get(rewriteUsage)
	if !ok {
		return nil
	}

	u, ok := usage.(model.Usage)
	if !ok {
		panic(fmt.Sprintf("rewrite usage type %T is not a model.Usage", usage))
	}

	return &u
}

func (p *WebSearch) getConfig(meta *meta.Meta) (Config, error) {
	pluginConfig := Config{}
	if err := meta.ModelConfig.LoadPluginConfig("web-search", &pluginConfig); err != nil {
		return Config{}, err
	}

	return pluginConfig, nil
}

func lazyRemoveSearchOption(meta *meta.Meta) {
	patch.AddLazyPatch(meta, patch.PatchOperation{
		Op: patch.OpFunction,
		Function: func(root *ast.Node) (bool, error) {
			ok, err := root.Unset("web_search_options")
			if err != nil {
				return false, err
			}

			return ok, nil
		},
	})
}

func fallback(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
	do adaptor.ConvertRequest,
) (adaptor.ConvertResult, error) {
	lazyRemoveSearchOption(meta)
	return do.ConvertRequest(meta, store, req)
}

// ConvertRequest intercepts and modifies requests to add web search capabilities
func (p *WebSearch) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
	do adaptor.ConvertRequest,
) (adaptor.ConvertResult, error) {
	// Skip if not chat completions mode
	if meta.Mode != mode.ChatCompletions {
		return do.ConvertRequest(meta, store, req)
	}

	// Load plugin configuration
	pluginConfig, err := p.getConfig(meta)
	if err != nil {
		return do.ConvertRequest(meta, store, req)
	}

	// Skip if plugin is disabled
	if !pluginConfig.Enable {
		return do.ConvertRequest(meta, store, req)
	}

	// Apply default configuration values if needed
	if err := p.validateAndApplyDefaults(&pluginConfig); err != nil {
		return fallback(meta, store, req, do)
	}

	// Initialize search engines
	engines, arxivExists, err := p.initializeSearchEngines(pluginConfig.SearchFrom)
	if err != nil || len(engines) == 0 {
		return fallback(meta, store, req, do)
	}

	// Read and parse request body
	body, err := common.GetRequestBodyReusable(req)
	if err != nil {
		return adaptor.ConvertResult{}, fmt.Errorf("failed to read request body: %w", err)
	}

	var chatRequest map[string]any
	if err := sonic.Unmarshal(body, &chatRequest); err != nil {
		return fallback(meta, store, req, do)
	}

	// Check if web search should be enabled for this request
	webSearchOptions, hasWebSearchOptions := chatRequest["web_search_options"].(map[string]any)
	if !pluginConfig.ForceSearch && !hasWebSearchOptions {
		return fallback(meta, store, req, do)
	}

	webSearchEnable, ok := webSearchOptions["enable"].(bool)
	if ok && !webSearchEnable {
		return fallback(meta, store, req, do)
	}

	// Extract user query from messages
	messages, ok := chatRequest["messages"].([]any)
	if !ok || len(messages) == 0 {
		return fallback(meta, store, req, do)
	}

	queryIndex, query := p.extractUserQuery(messages)
	if query == "" {
		return fallback(meta, store, req, do)
	}

	// Prepare search rewrite prompt if configured
	searchRewritePrompt := p.prepareSearchRewritePrompt(
		pluginConfig.SearchRewrite,
		arxivExists,
		webSearchOptions,
	)

	// Generate search contexts
	searchContexts, err := p.generateSearchContexts(
		meta,
		store,
		pluginConfig,
		query,
		searchRewritePrompt,
	)
	if err != nil {
		return fallback(meta, store, req, do)
	}

	if len(searchContexts) == 0 {
		return fallback(meta, store, req, do)
	}

	// Execute searches
	searchResult := p.executeSearches(context.Background(), engines, searchContexts)
	if searchResult.Count == 0 || len(searchResult.Results) == 0 {
		return fallback(meta, store, req, do)
	}

	setSearchCount(meta, searchResult.Count)

	// Format search results and modify request
	p.formatSearchResults(messages, queryIndex, query, searchResult.Results, pluginConfig)

	delete(chatRequest, "web_search_options")

	// Create new request body
	modifiedBody, err := sonic.Marshal(chatRequest)
	if err != nil {
		return fallback(meta, store, req, do)
	}

	// Update the request
	common.SetRequestBody(req, modifiedBody)
	defer common.SetRequestBody(req, body)

	// Store references in context if needed
	if pluginConfig.NeedReference {
		meta.Set("references", searchResult.Results)
	}

	return do.ConvertRequest(meta, store, req)
}

// validateAndApplyDefaults validates configuration and applies default values
func (p *WebSearch) validateAndApplyDefaults(config *Config) error {
	// Set default max results
	if config.MaxResults == 0 {
		config.MaxResults = 10
	}

	// Configure reference settings
	if config.NeedReference {
		if config.ReferenceFormat != "" && !strings.Contains(config.ReferenceFormat, "%s") {
			return errors.New("invalid reference format")
		}
	}

	// Set default prompt template if not provided
	if config.PromptTemplate == "" {
		config.PromptTemplate = p.getDefaultPromptTemplate(config.NeedReference)
	}

	// Validate prompt template
	if !strings.Contains(config.PromptTemplate, "{search_results}") ||
		!strings.Contains(config.PromptTemplate, "{question}") {
		return errors.New("invalid prompt template")
	}

	return nil
}

// getDefaultPromptTemplate returns the appropriate default prompt template based on configuration
func (p *WebSearch) getDefaultPromptTemplate(needReference bool) string {
	if needReference {
		return `# 以下内容是基于用户发送的消息的搜索结果:
{search_results}
在我给你的搜索结果中，每个结果都是[webpage X begin]...[webpage X end]格式的，X代表每篇文章的数字索引。请在适当的情况下在句子末尾引用上下文。请按照引用编号[X]的格式在答案中对应部分引用上下文。如果一句话源自多个上下文，请列出所有相关的引用编号，例如[3][5]，切记不要将引用集中在最后返回引用编号，而是在答案对应部分列出。
在回答时，请注意以下几点：
- 今天是北京时间：{cur_date}。
- 并非搜索结果的所有内容都与用户的问题密切相关，你需要结合问题，对搜索结果进行甄别、筛选。
- 对于列举类的问题（如列举所有航班信息），尽量将答案控制在10个要点以内，并告诉用户可以查看搜索来源、获得完整信息。优先提供信息完整、最相关的列举项；如非必要，不要主动告诉用户搜索结果未提供的内容。
- 对于创作类的问题（如写论文），请务必在正文的段落中引用对应的参考编号，例如[3][5]，不能只在文章末尾引用。你需要解读并概括用户的题目要求，选择合适的格式，充分利用搜索结果并抽取重要信息，生成符合用户要求、极具思想深度、富有创造力与专业性的答案。你的创作篇幅需要尽可能延长，对于每一个要点的论述要推测用户的意图，给出尽可能多角度的回答要点，且务必信息量大、论述详尽。
- 如果回答很长，请尽量结构化、分段落总结。如果需要分点作答，尽量控制在5个点以内，并合并相关的内容。
- 对于客观类的问答，如果问题的答案非常简短，可以适当补充一到两句相关信息，以丰富内容。
- 你需要根据用户要求和回答内容选择合适、美观的回答格式，确保可读性强。
- 你的回答应该综合多个相关网页来回答，不能重复引用一个网页。
- 除非用户要求，否则你回答的语言需要和用户提问的语言保持一致。

# 用户消息为：
{question}`
	}

	return `# 以下内容是基于用户发送的消息的搜索结果:
{search_results}
在我给你的搜索结果中，每个结果都是[webpage begin]...[webpage end]格式的。
在回答时，请注意以下几点：
- 今天是北京时间：{cur_date}。
- 并非搜索结果的所有内容都与用户的问题密切相关，你需要结合问题，对搜索结果进行甄别、筛选。
- 对于列举类的问题（如列举所有航班信息），尽量将答案控制在10个要点以内。如非必要，不要主动告诉用户搜索结果未提供的内容。
- 对于创作类的问题（如写论文），你需要解读并概括用户的题目要求，选择合适的格式，充分利用搜索结果并抽取重要信息，生成符合用户要求、极具思想深度、富有创造力与专业性的答案。你的创作篇幅需要尽可能延长，对于每一个要点的论述要推测用户的意图，给出尽可能多角度的回答要点，且务必信息量大、论述详尽。
- 如果回答很长，请尽量结构化、分段落总结。如果需要分点作答，尽量控制在5个点以内，并合并相关的内容。
- 对于客观类的问答，如果问题的答案非常简短，可以适当补充一到两句相关信息，以丰富内容。
- 你需要根据用户要求和回答内容选择合适、美观的回答格式，确保可读性强。
- 你的回答应该综合多个相关网页来回答，但回答中不要给出网页的引用来源。
- 除非用户要求，否则你回答的语言需要和用户提问的语言保持一致。

# 用户消息为：
{question}`
}

// initializeSearchEngines creates search engine instances based on configuration
func (p *WebSearch) initializeSearchEngines(configs []EngineConfig) ([]engine.Engine, bool, error) {
	var (
		engines     []engine.Engine
		arxivExists bool
	)

	for _, e := range configs {
		switch e.Type {
		case "bing":
			var spec BingSpec
			if err := e.LoadSpec(&spec); err != nil {
				return nil, false, err
			}

			engines = append(engines, engine.NewBingEngine(spec.APIKey))
		case "bingcn":
			engines = append(engines, engine.NewBingCNEngine())
		case "google":
			var spec GoogleSpec
			if err := e.LoadSpec(&spec); err != nil {
				return nil, false, err
			}

			engines = append(engines, engine.NewGoogleEngine(spec.APIKey, spec.CX))
		case "arxiv":
			engines = append(engines, engine.NewArxivEngine())
			arxivExists = true
		case "searchxng":
			var spec SearchXNGSpec
			if err := e.LoadSpec(&spec); err != nil {
				return nil, false, err
			}

			engines = append(engines, engine.NewSearchXNGEngine(spec.BaseURL))
		default:
			return nil, false, fmt.Errorf("unsupported engine type: %s", e.Type)
		}
	}

	return engines, arxivExists, nil
}

// extractUserQuery finds the last user message in the conversation
func (p *WebSearch) extractUserQuery(messages []any) (int, string) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]any)
		if !ok {
			continue
		}

		if role, ok := msg["role"].(string); ok && role == "user" {
			if content, ok := msg["content"].(string); ok {
				return i, content
			}
			return i, ""
		}
	}

	return -1, ""
}

// prepareSearchRewritePrompt prepares the prompt for search query rewriting
func (p *WebSearch) prepareSearchRewritePrompt(
	searchRewrite SearchRewrite,
	arxivExists bool,
	webSearchOptions map[string]any,
) string {
	if !searchRewrite.Enable {
		return ""
	}

	// Select appropriate prompt template
	var searchRewritePromptTemplate string
	if arxivExists {
		searchRewritePromptTemplate = arxivSearchPrompts
	} else {
		searchRewritePromptTemplate = internetSearchPrompts
	}

	// Adjust max count based on search context size if specified
	maxCount := searchRewrite.MaxCount
	if webSearchOptions != nil {
		if searchContextSize, ok := webSearchOptions["search_context_size"].(string); ok {
			switch searchContextSize {
			case "low":
				maxCount = 1
			case "medium":
				maxCount = 3
			case "high":
				maxCount = 5
			}
		}
	}

	// Replace placeholder with actual max count
	return strings.ReplaceAll(searchRewritePromptTemplate, "{max_count}", strconv.Itoa(maxCount))
}

// generateSearchContexts creates search contexts based on the user query
func (p *WebSearch) generateSearchContexts(
	m *meta.Meta,
	store adaptor.Store,
	config Config,
	query, searchRewritePrompt string,
) ([]engine.SearchQuery, error) {
	if searchRewritePrompt == "" {
		return []engine.SearchQuery{{
			Queries:  []string{query},
			Language: config.DefaultLanguage,
		}}, nil
	}

	// Prepare request for query rewriting
	rewriteBody, err := sonic.Marshal(map[string]any{
		"stream":     false,
		"max_tokens": 4096,
		"model":      config.SearchRewrite.ModelName,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": strings.ReplaceAll(searchRewritePrompt, "{question}", query),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Set up test context for rewrite request
	w := httptest.NewRecorder()
	newc, _ := gin.CreateTestContext(w)
	newc.Request = &http.Request{
		URL:    &url.URL{},
		Body:   io.NopCloser(bytes.NewReader(rewriteBody)),
		Header: make(http.Header),
	}
	middleware.SetRequestID(newc, "web-search-rewrite")

	// Set up metadata for rewrite request
	modelName := config.SearchRewrite.ModelName
	if modelName == "" {
		modelName = m.OriginModel
	}

	newMeta := meta.NewMeta(
		nil,
		mode.ChatCompletions,
		modelName,
		model.ModelConfig{
			Model: modelName,
			Type:  mode.ChatCompletions,
		},
		meta.WithRequestID("web-search-rewrite"),
	)

	// Set appropriate channel
	if config.SearchRewrite.ModelName == "" {
		newMeta.CopyChannelFromMeta(m)
	} else {
		channel, err := p.GetChannel(config.SearchRewrite.ModelName)
		if err != nil {
			return nil, err
		}

		newMeta.SetChannel(channel)
	}

	// Get adaptor and handle request
	adaptor, ok := adaptors.GetAdaptor(newMeta.Channel.Type)
	if !ok {
		return nil, errors.New("adaptor not found")
	}

	result := controller.Handle(adaptor, newc, newMeta, store)
	if result.Error != nil {
		return nil, result.Error
	}

	setRewriteUsage(m, result.Usage)

	// Extract content from response
	contentNode, err := sonic.Get(w.Body.Bytes(), "choices", 0, "message", "content")
	if err != nil {
		return nil, err
	}

	content, err := contentNode.String()
	if err != nil || content == "" {
		return nil, err
	}

	if strings.Contains(content, "none") {
		return nil, nil
	}

	// Parse search queries from LLM response
	return p.parseSearchContexts(config.DefaultLanguage, content), nil
}

// parseSearchContexts extracts search queries from LLM response
func (p *WebSearch) parseSearchContexts(defaultLanguage, content string) []engine.SearchQuery {
	var searchContexts []engine.SearchQuery
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		engineType := strings.TrimSpace(parts[0])
		queryStr := strings.TrimSpace(parts[1])

		var ctx engine.SearchQuery

		ctx.Language = defaultLanguage

		switch engineType {
		case "internet":
			ctx.Queries = []string{queryStr}
		default:
			// Arxiv category
			ctx.ArxivCategory = engineType

			ctx.Queries = strings.Split(queryStr, ",")
			for i := range ctx.Queries {
				ctx.Queries[i] = strings.TrimSpace(ctx.Queries[i])
			}
		}

		if len(ctx.Queries) > 0 {
			searchContexts = append(searchContexts, ctx)
			if ctx.ArxivCategory != "" {
				// Conduct inquiries in all areas to increase recall.
				backupCtx := ctx
				backupCtx.ArxivCategory = ""
				searchContexts = append(searchContexts, backupCtx)
			}
		}
	}

	return searchContexts
}

// Search result structure
type searchResult struct {
	Results []engine.SearchResult
	Count   int
}

// executeSearches performs searches using all configured engines
func (p *WebSearch) executeSearches(
	ctx context.Context,
	engines []engine.Engine,
	searchContexts []engine.SearchQuery,
) *searchResult {
	var (
		allResults []engine.SearchResult
		mu         sync.Mutex
	)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	for _, eng := range engines {
		for _, searchCtx := range searchContexts {
			g.Go(func() error {
				results, err := eng.Search(ctx, engine.SearchQuery{
					Queries:       searchCtx.Queries,
					MaxResults:    10,
					Language:      searchCtx.Language,
					ArxivCategory: searchCtx.ArxivCategory,
				})
				if err != nil {
					logrus.Errorf("search error: %v", err)
					return err
				}

				mu.Lock()

				allResults = append(allResults, results...)

				mu.Unlock()

				return nil
			})
		}
	}

	_ = g.Wait()

	seen := make(map[string]bool)

	var uniqueResults []engine.SearchResult
	for _, result := range allResults {
		if !seen[result.Link] {
			seen[result.Link] = true
			uniqueResults = append(uniqueResults, result)
		}
	}

	return &searchResult{
		Results: uniqueResults,
		Count:   len(engines) * len(searchContexts),
	}
}

// formatSearchResults formats search results for the prompt
func (p *WebSearch) formatSearchResults(
	messages []any,
	queryIndex int,
	query string,
	searchResults []engine.SearchResult,
	config Config,
) {
	message, ok := messages[queryIndex].(map[string]any)
	if !ok {
		return
	}

	var formattedResults []string

	for i, result := range searchResults {
		if config.NeedReference {
			formattedResults = append(formattedResults,
				fmt.Sprintf("[webpage %d begin]\n%s\n[webpage %d end]", i+1, result.Content, i+1))
		} else {
			formattedResults = append(formattedResults,
				fmt.Sprintf("[webpage begin]\n%s\n[webpage end]", result.Content))
		}
	}

	// Fill template
	curDate := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006年1月2日")
	searchResultsStr := strings.Join(formattedResults, "\n")

	prompt := strings.Replace(config.PromptTemplate, "{search_results}", searchResultsStr, 1)
	prompt = strings.Replace(prompt, "{question}", query, 1)
	prompt = strings.Replace(prompt, "{cur_date}", curDate, 1)

	// Update message
	message["content"] = prompt
}

// Custom response writer to handle metadata and references
type responseWriter struct {
	gin.ResponseWriter
	refWritten          bool
	referenceFormat     string
	references          []engine.SearchResult
	referencesLocation  string
	webSearchCount      int
	rewriteUsage        *model.Usage
	rewriteUsageWritten bool
	rewriteUsageField   string
	isStream            bool
}

// Write overrides the standard Write method to inject metadata
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.isStream || utils.IsStreamResponseWithHeader(rw.Header()) {
		rw.isStream = true
	}

	node, err := sonic.Get(b)
	if err != nil || !node.Valid() {
		return rw.ResponseWriter.Write(b)
	}

	// Process the response node
	rw.processRewriteUsage(&node)
	rw.processWebSearchCount(&node)
	rw.processReferences(&node)

	// Marshal the modified node
	nb, err := sonic.Marshal(&node)
	if err != nil {
		return rw.ResponseWriter.Write(b)
	}

	if !rw.isStream {
		if rw.ResponseWriter.Header().Get("Content-Length") != "" {
			rw.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(nb)))
		}
	}

	return rw.ResponseWriter.Write(nb)
}

// processRewriteUsage adds rewrite usage information to the response
func (rw *responseWriter) processRewriteUsage(node *ast.Node) {
	if rw.rewriteUsage != nil && !rw.rewriteUsageWritten {
		field := rw.rewriteUsageField
		if field == "" {
			field = "rewrite_usage"
		}

		_, _ = node.SetAny(field, rw.rewriteUsage)
		rw.rewriteUsageWritten = true
	}
}

// processWebSearchCount adds or increments web search count in usage statistics
func (rw *responseWriter) processWebSearchCount(node *ast.Node) {
	if rw.webSearchCount <= 0 {
		return
	}

	usageNode := node.Get("usage")
	if usageNode == nil || !usageNode.Valid() {
		return
	}

	// Check if web_search_count already exists
	existingCount := usageNode.Get("web_search_count")
	if existingCount != nil && existingCount.Valid() {
		// If exists, add to the existing value
		currentCount, _ := existingCount.Int64()
		_, _ = usageNode.Set(
			"web_search_count",
			ast.NewNumber(strconv.FormatInt(currentCount+int64(rw.webSearchCount), 10)),
		)
	} else {
		// If not exists, set the value
		_, _ = usageNode.Set("web_search_count", ast.NewNumber(strconv.FormatInt(int64(rw.webSearchCount), 10)))
	}
}

func buildReferenceContent(searchResults []engine.SearchResult) string {
	formattedReferences := make([]string, len(searchResults))
	for i, result := range searchResults {
		formattedReferences[i] = fmt.Sprintf("[%d] [%s](%s)", i+1, result.Title, result.Link)
	}

	return strings.Join(formattedReferences, "\n\n")
}

// processReferences adds reference information to the content
func (rw *responseWriter) processReferences(node *ast.Node) {
	if rw.refWritten || len(rw.references) == 0 {
		return
	}

	rw.refWritten = true

	if rw.referencesLocation == "" || rw.referencesLocation == "content" {
		var contentNode *ast.Node
		if rw.isStream {
			contentNode = node.GetByPath("choices", 0, "delta", "content")
		} else {
			contentNode = node.GetByPath("choices", 0, "message", "content")
		}

		if contentNode != nil && contentNode.Valid() {
			content, err := contentNode.String()
			if err == nil {
				format := rw.referenceFormat
				if format == "" {
					format = "**References:**\n%s"
				}

				ref := fmt.Sprintf(format, buildReferenceContent(rw.references))
				refContent := fmt.Sprintf("%s\n\n%s", ref, content)
				*contentNode = ast.NewString(refContent)
			}
		}
	} else {
		var outterLocation *ast.Node
		if rw.isStream {
			outterLocation = node.GetByPath("choices", 0, "delta")
		} else {
			outterLocation = node.GetByPath("choices", 0, "message")
		}

		if outterLocation != nil && outterLocation.Valid() {
			_, _ = outterLocation.SetAny(rw.referencesLocation, rw.references)
		}
	}
}

// WriteString implements the WriteString method for the custom response writer
func (rw *responseWriter) WriteString(s string) (int, error) {
	return rw.Write(conv.StringToBytes(s))
}

// DoResponse handles response modification for references
func (p *WebSearch) DoResponse(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
) (model.Usage, adaptor.Error) {
	if meta.Mode != mode.ChatCompletions {
		return do.DoResponse(meta, store, c, resp)
	}

	var references []engine.SearchResult

	referencesI, ok := meta.Get("references")
	if ok {
		references, ok = referencesI.([]engine.SearchResult)
		if !ok {
			panic(fmt.Sprintf("references type %T is not a []engine.SearchResult", referencesI))
		}
	}

	count := getSearchCount(meta)

	var rewriteUsage *model.Usage

	rewriteUsageField := ""
	pluginConfig, _ := p.getConfig(meta)

	if pluginConfig.SearchRewrite.AddRewriteUsage {
		rewriteUsage = getRewriteUsage(meta)
		rewriteUsageField = pluginConfig.SearchRewrite.RewriteUsageField
	}

	// Check if we need to wrap the response writer
	if len(references) > 0 || rewriteUsage != nil || count > 0 {
		rw := &responseWriter{
			ResponseWriter:      c.Writer,
			webSearchCount:      count,
			rewriteUsageWritten: false,
			rewriteUsage:        rewriteUsage,
			rewriteUsageField:   rewriteUsageField,
			references:          references,
			referencesLocation:  pluginConfig.ReferenceLocation,
			referenceFormat:     pluginConfig.ReferenceFormat,
		}

		c.Writer = rw
		defer func() {
			c.Writer = rw.ResponseWriter
		}()
	}

	return p.doResponseWithCount(meta, store, c, resp, do, count)
}

// doResponseWithCount adds search count to usage statistics
func (p *WebSearch) doResponseWithCount(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
	resp *http.Response,
	do adaptor.DoResponse,
	count int,
) (model.Usage, adaptor.Error) {
	u, err := do.DoResponse(meta, store, c, resp)
	if err != nil {
		return model.Usage{}, err
	}

	u.WebSearchCount += model.ZeroNullInt64(int64(count))

	return u, nil
}
