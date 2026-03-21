//go:build enterprise

package enterprise

import (
	"slices"
	"strings"
	"testing"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/mode"
)

// --- getSupportedEndpoints ---

func TestGetSupportedEndpoints_ChatFamily(t *testing.T) {
	chatFamilyModes := []mode.Mode{
		mode.ChatCompletions,
		mode.Completions,
		mode.Anthropic,
		mode.Gemini,
		mode.Responses,
	}
	want := endpointsChatFamily
	for _, m := range chatFamilyModes {
		got := getSupportedEndpoints(m)
		if len(got) != len(want) {
			t.Errorf("mode %s: got %d endpoints, want %d", m, len(got), len(want))
			continue
		}
		for i, ep := range got {
			if ep != want[i] {
				t.Errorf("mode %s [%d]: got %q, want %q", m, i, ep, want[i])
			}
		}
	}
}

func TestGetSupportedEndpoints_Embeddings(t *testing.T) {
	got := getSupportedEndpoints(mode.Embeddings)
	if len(got) != 1 || got[0] != "POST /v1/embeddings" {
		t.Errorf("Embeddings: got %v", got)
	}
}

func TestGetSupportedEndpoints_Moderations(t *testing.T) {
	got := getSupportedEndpoints(mode.Moderations)
	if len(got) != 1 || got[0] != "POST /v1/moderations" {
		t.Errorf("Moderations: got %v", got)
	}
}

func TestGetSupportedEndpoints_Images(t *testing.T) {
	for _, m := range []mode.Mode{mode.ImagesGenerations, mode.ImagesEdits} {
		got := getSupportedEndpoints(m)
		if len(got) != 2 {
			t.Errorf("mode %s: expected 2 image endpoints, got %v", m, got)
		}
	}
}

func TestGetSupportedEndpoints_AudioSpeech(t *testing.T) {
	got := getSupportedEndpoints(mode.AudioSpeech)
	if len(got) != 1 || got[0] != "POST /v1/audio/speech" {
		t.Errorf("AudioSpeech: got %v", got)
	}
}

func TestGetSupportedEndpoints_AudioTranscription(t *testing.T) {
	got := getSupportedEndpoints(mode.AudioTranscription)
	if len(got) != 1 || got[0] != "POST /v1/audio/transcriptions" {
		t.Errorf("AudioTranscription: got %v", got)
	}
}

func TestGetSupportedEndpoints_AudioTranslation(t *testing.T) {
	got := getSupportedEndpoints(mode.AudioTranslation)
	if len(got) != 1 || got[0] != "POST /v1/audio/translations" {
		t.Errorf("AudioTranslation: got %v", got)
	}
}

func TestGetSupportedEndpoints_Rerank(t *testing.T) {
	got := getSupportedEndpoints(mode.Rerank)
	if len(got) != 1 || got[0] != "POST /v1/rerank" {
		t.Errorf("Rerank: got %v", got)
	}
}

func TestGetSupportedEndpoints_ParsePdf(t *testing.T) {
	got := getSupportedEndpoints(mode.ParsePdf)
	if len(got) != 1 || got[0] != "POST /v1/parse/pdf" {
		t.Errorf("ParsePdf: got %v", got)
	}
}

func TestGetSupportedEndpoints_Video(t *testing.T) {
	videoModes := []mode.Mode{
		mode.VideoGenerationsJobs,
		mode.VideoGenerationsGetJobs,
		mode.VideoGenerationsContent,
	}
	for _, m := range videoModes {
		got := getSupportedEndpoints(m)
		if len(got) != 2 {
			t.Errorf("mode %s: expected 2 video endpoints, got %v", m, got)
		}
	}
}

func TestGetSupportedEndpoints_Unknown(t *testing.T) {
	got := getSupportedEndpoints(mode.Unknown)
	if got != nil {
		t.Errorf("Unknown mode: expected nil, got %v", got)
	}
}

func TestGetSupportedEndpoints_UnknownHigh(t *testing.T) {
	// Arbitrary unrecognized mode value should return nil, not panic.
	got := getSupportedEndpoints(mode.Mode(999))
	if got != nil {
		t.Errorf("mode 999: expected nil, got %v", got)
	}
}

// --- getModelSupportedEndpoints ---

// makeModelConfig builds a minimal ModelConfig for testing.
func makeModelConfig(t mode.Mode, endpoints []string) model.ModelConfig {
	mc := model.ModelConfig{
		Model: "test-model",
		Type:  t,
	}
	if len(endpoints) > 0 {
		mc.Config = map[model.ModelConfigKey]any{
			"endpoints": endpoints,
		}
	}
	return mc
}

// makeModelConfigWithAnySlice simulates JSON-deserialized config where endpoints
// arrive as []any (strings unmarshalled via interface{}).
func makeModelConfigWithAnySlice(t mode.Mode, endpoints []string) model.ModelConfig {
	mc := model.ModelConfig{
		Model: "test-model",
		Type:  t,
	}
	raw := make([]any, len(endpoints))
	for i, ep := range endpoints {
		raw[i] = ep
	}
	mc.Config = map[model.ModelConfigKey]any{
		"endpoints": raw,
	}
	return mc
}

// TestGetModelSupportedEndpoints_PPIOChatOnly verifies that a PPIO model with only
// "chat/completions" slug returns the base chat family (3 endpoints, no /v1/responses)
// because PPIO doesn't declare the "responses" slug and the Responses API requires
// explicit upstream support.
func TestGetModelSupportedEndpoints_PPIOChatOnly(t *testing.T) {
	mc := makeModelConfig(mode.ChatCompletions, []string{"chat/completions"})
	got := getModelSupportedEndpoints(mc)
	if len(got) != len(endpointsChatFamilyBase) {
		t.Errorf("PPIO chat-only: expected base chat family (%d), got %v", len(endpointsChatFamilyBase), got)
	}
	if slices.Contains(got, "POST /v1/responses") {
		t.Errorf("PPIO chat-only: /v1/responses must not appear (PPIO upstream does not support it)")
	}
}

// TestGetModelSupportedEndpoints_PPIOAnthropicOnly verifies that a PPIO model with
// only "anthropic" slug (e.g., pa/claude-*) returns the base chat family — all 3
// protocol-conversion endpoints, but NOT /v1/responses which PPIO doesn't support.
func TestGetModelSupportedEndpoints_PPIOAnthropicOnly(t *testing.T) {
	mc := makeModelConfig(mode.ChatCompletions, []string{"anthropic"})
	got := getModelSupportedEndpoints(mc)
	if len(got) != len(endpointsChatFamilyBase) {
		t.Fatalf("PPIO anthropic-only: expected base chat family (%d), got %v", len(endpointsChatFamilyBase), got)
	}
	if !slices.Contains(got, "POST /v1/messages") {
		t.Errorf("PPIO anthropic-only: POST /v1/messages not in result %v", got)
	}
	if slices.Contains(got, "POST /v1/responses") {
		t.Errorf("PPIO anthropic-only: /v1/responses must not appear (PPIO upstream does not support it)")
	}
}

// TestGetModelSupportedEndpoints_PPIOMulti verifies a model with both
// "chat/completions" and "anthropic" slugs returns the base chat family (no /v1/responses).
func TestGetModelSupportedEndpoints_PPIOMulti(t *testing.T) {
	mc := makeModelConfig(mode.ChatCompletions, []string{"chat/completions", "anthropic"})
	got := getModelSupportedEndpoints(mc)
	if len(got) != len(endpointsChatFamilyBase) {
		t.Errorf("PPIO multi: expected base chat family (%d), got %v", len(endpointsChatFamilyBase), got)
	}
	if slices.Contains(got, "POST /v1/responses") {
		t.Errorf("PPIO multi: /v1/responses must not appear")
	}
}

// TestGetModelSupportedEndpoints_PPIOEmbeddings verifies embedding-only PPIO models.
func TestGetModelSupportedEndpoints_PPIOEmbeddings(t *testing.T) {
	// Embeddings models are Type=3 but PPIO stores "embeddings" slug
	mc := makeModelConfig(mode.Embeddings, []string{"embeddings"})
	got := getModelSupportedEndpoints(mc)
	if len(got) != 1 || got[0] != "POST /v1/embeddings" {
		t.Errorf("PPIO embeddings: got %v", got)
	}
}

// TestGetModelSupportedEndpoints_PPIOResponses verifies that a model with only the
// "responses" slug is promoted to the full chat family (responses is in chat-family).
func TestGetModelSupportedEndpoints_PPIOResponses(t *testing.T) {
	mc := makeModelConfig(mode.ChatCompletions, []string{"responses"})
	got := getModelSupportedEndpoints(mc)
	if len(got) != len(endpointsChatFamily) {
		t.Errorf("PPIO responses: expected full chat family (%d), got %v", len(endpointsChatFamily), got)
	}
}

// TestGetModelSupportedEndpoints_PPIOModerations verifies moderations models.
func TestGetModelSupportedEndpoints_PPIOModerations(t *testing.T) {
	mc := makeModelConfig(mode.Moderations, []string{"moderations"})
	got := getModelSupportedEndpoints(mc)
	if len(got) != 1 || got[0] != "POST /v1/moderations" {
		t.Errorf("PPIO moderations: got %v", got)
	}
}

// TestGetModelSupportedEndpoints_PPIORerank verifies rerank models.
func TestGetModelSupportedEndpoints_PPIORerank(t *testing.T) {
	mc := makeModelConfig(mode.Rerank, []string{"rerank"})
	got := getModelSupportedEndpoints(mc)
	if len(got) != 1 || got[0] != "POST /v1/rerank" {
		t.Errorf("PPIO rerank: got %v", got)
	}
}

// TestGetModelSupportedEndpoints_MixedChatAndNonChat verifies that a model with
// slugs spanning chat-family AND non-chat-family (e.g. embeddings) is NOT promoted
// to the full chat family — the exact slug paths are returned instead.
func TestGetModelSupportedEndpoints_MixedChatAndNonChat(t *testing.T) {
	mc := makeModelConfig(mode.ChatCompletions, []string{"chat/completions", "embeddings"})
	got := getModelSupportedEndpoints(mc)
	// Should NOT return the full chat family because "embeddings" is outside it.
	if len(got) == len(endpointsChatFamily) {
		t.Errorf("mixed chat+non-chat: should not return full chat family, got %v", got)
	}
	wantPaths := map[string]bool{
		"POST /v1/chat/completions": true,
		"POST /v1/embeddings":       true,
	}
	if len(got) != len(wantPaths) {
		t.Fatalf("mixed chat+non-chat: expected 2 paths, got %v", got)
	}
	for _, ep := range got {
		if !wantPaths[ep] {
			t.Errorf("mixed chat+non-chat: unexpected path %q", ep)
		}
	}
}

// TestGetModelSupportedEndpoints_UnknownSlugFallback verifies that an unrecognised
// PPIO slug falls back through model_type → mc.Type inference.
// makeModelConfig sets no Config["model_type"], so the final fallback is mc.Type.
func TestGetModelSupportedEndpoints_UnknownSlugFallback(t *testing.T) {
	// No model_type in Config → falls back to mc.Type = ChatCompletions → 4 paths
	mc := makeModelConfig(mode.ChatCompletions, []string{"unknown-future-slug"})
	got := getModelSupportedEndpoints(mc)
	want := endpointsChatFamily
	if len(got) != len(want) {
		t.Errorf("unknown slug fallback: got %v, want %v", got, want)
	}
}

// TestGetModelSupportedEndpoints_EmptyConfig falls back to type inference when
// Config is nil.
func TestGetModelSupportedEndpoints_EmptyConfig(t *testing.T) {
	mc := model.ModelConfig{
		Model: "gpt-4o",
		Type:  mode.ChatCompletions,
	}
	got := getModelSupportedEndpoints(mc)
	if len(got) != len(endpointsChatFamily) {
		t.Errorf("nil config: got %v", got)
	}
}

// TestGetModelSupportedEndpoints_EmptyEndpointsSlice falls back to type inference
// when Config["endpoints"] is an empty slice.
func TestGetModelSupportedEndpoints_EmptyEndpointsSlice(t *testing.T) {
	mc := makeModelConfig(mode.ChatCompletions, []string{})
	got := getModelSupportedEndpoints(mc)
	// Empty slice → falls back to type-based
	if len(got) != len(endpointsChatFamily) {
		t.Errorf("empty slice fallback: got %v", got)
	}
}

// TestGetModelSupportedEndpoints_AnySliceDeserialization verifies that endpoints
// stored as []any (from JSON decode via interface{}) are handled correctly.
// Without "responses" slug, /v1/responses must not appear.
func TestGetModelSupportedEndpoints_AnySliceDeserialization(t *testing.T) {
	mc := makeModelConfigWithAnySlice(mode.ChatCompletions, []string{"chat/completions", "anthropic"})
	got := getModelSupportedEndpoints(mc)
	if len(got) != len(endpointsChatFamilyBase) {
		t.Fatalf("[]any deserialization: expected base chat family (%d), got %v", len(endpointsChatFamilyBase), got)
	}
	if slices.Contains(got, "POST /v1/responses") {
		t.Errorf("[]any deserialization: /v1/responses must not appear")
	}
}

// TestGetModelSupportedEndpoints_FallbackEmbedding verifies type-based fallback
// for a non-PPIO embedding model (no Config).
func TestGetModelSupportedEndpoints_FallbackEmbedding(t *testing.T) {
	mc := model.ModelConfig{
		Model: "text-embedding-3-small",
		Type:  mode.Embeddings,
	}
	got := getModelSupportedEndpoints(mc)
	if len(got) != 1 || got[0] != "POST /v1/embeddings" {
		t.Errorf("fallback embedding: got %v", got)
	}
}

// TestGetModelSupportedEndpoints_FallbackImage verifies type-based fallback for
// image generation models.
func TestGetModelSupportedEndpoints_FallbackImage(t *testing.T) {
	mc := model.ModelConfig{
		Model: "dall-e-3",
		Type:  mode.ImagesGenerations,
	}
	got := getModelSupportedEndpoints(mc)
	if len(got) != 2 {
		t.Errorf("fallback image: got %v", got)
	}
}

// TestGetModelSupportedEndpoints_FallbackAudio verifies audio model fallback.
func TestGetModelSupportedEndpoints_FallbackAudio(t *testing.T) {
	cases := []struct {
		modelType mode.Mode
		wantPath  string
	}{
		{mode.AudioSpeech, "POST /v1/audio/speech"},
		{mode.AudioTranscription, "POST /v1/audio/transcriptions"},
		{mode.AudioTranslation, "POST /v1/audio/translations"},
	}
	for _, tc := range cases {
		mc := model.ModelConfig{Type: tc.modelType}
		got := getModelSupportedEndpoints(mc)
		if len(got) != 1 || got[0] != tc.wantPath {
			t.Errorf("audio %s: got %v, want [%s]", tc.modelType, got, tc.wantPath)
		}
	}
}

// TestGetModelSupportedEndpoints_VideoFallback verifies video endpoint inference.
func TestGetModelSupportedEndpoints_VideoFallback(t *testing.T) {
	for _, m := range []mode.Mode{mode.VideoGenerationsJobs, mode.VideoGenerationsGetJobs, mode.VideoGenerationsContent} {
		mc := model.ModelConfig{Type: m}
		got := getModelSupportedEndpoints(mc)
		if len(got) != 2 {
			t.Errorf("video mode %s: expected 2 endpoints, got %v", m, got)
		}
	}
}

// TestGetModelSupportedEndpoints_AllKnownSlugs verifies every slug in ppioSlugToPath
// maps to at least one valid path and never panics.
func TestGetModelSupportedEndpoints_AllKnownSlugs(t *testing.T) {
	for slug, path := range ppioSlugToPath {
		if path == "" {
			t.Errorf("slug %q maps to empty path", slug)
		}
		mc := makeModelConfig(mode.ChatCompletions, []string{slug})
		got := getModelSupportedEndpoints(mc)
		if len(got) == 0 {
			t.Errorf("slug %q: got empty result", slug)
			continue
		}
		// Chat-family slugs get promoted; non-chat slugs stay exact.
		// "responses" slug → full family (4); other chat slugs → base family (3, no /v1/responses).
		if slices.Contains(endpointsChatFamily, path) {
			if slug == "responses" {
				if len(got) != len(endpointsChatFamily) {
					t.Errorf("slug %q: expected full family (%d), got %v", slug, len(endpointsChatFamily), got)
				}
			} else {
				if len(got) != len(endpointsChatFamilyBase) {
					t.Errorf("slug %q (chat-base): expected base family (%d), got %v", slug, len(endpointsChatFamilyBase), got)
				}
				if slices.Contains(got, "POST /v1/responses") {
					t.Errorf("slug %q: /v1/responses must not appear when slug is not 'responses'", slug)
				}
			}
		} else {
			if len(got) != 1 || got[0] != path {
				t.Errorf("slug %q (non-chat): got %v, want [%s]", slug, got, path)
			}
		}
	}
}

// TestGetModelSupportedEndpoints_NoDuplicates verifies that no slug mapping
// results in duplicate endpoint paths.
func TestGetModelSupportedEndpoints_NoDuplicates(t *testing.T) {
	allSlugs := make([]string, 0, len(ppioSlugToPath))
	for slug := range ppioSlugToPath {
		allSlugs = append(allSlugs, slug)
	}
	mc := makeModelConfig(mode.ChatCompletions, allSlugs)
	got := getModelSupportedEndpoints(mc)
	seen := make(map[string]bool, len(got))
	for _, ep := range got {
		if seen[ep] {
			t.Errorf("duplicate endpoint %q in result %v", ep, got)
		}
		seen[ep] = true
	}
}

// TestGetModelSupportedEndpoints_ChatFamilyContents verifies the exact content and
// order of the chat-family endpoint list (no truncation, no empty strings).
func TestGetModelSupportedEndpoints_ChatFamilyContents(t *testing.T) {
	mc := model.ModelConfig{Type: mode.ChatCompletions}
	got := getModelSupportedEndpoints(mc)
	expected := []string{
		"POST /v1/chat/completions",
		"POST /v1/completions",
		"POST /v1/messages",
		"POST /v1/responses",
	}
	if len(got) != len(expected) {
		t.Fatalf("chat family: got %d endpoints, want %d: %v", len(got), len(expected), got)
	}
	for i, ep := range got {
		if ep == "" {
			t.Errorf("chat family [%d]: empty string in endpoint list", i)
		}
		if ep != expected[i] {
			t.Errorf("chat family [%d]: got %q, want %q", i, ep, expected[i])
		}
	}
}

// TestGetModelSupportedEndpoints_NoNilOrEmptyInAnyPath ensures none of the
// type-based endpoint slices contain empty strings.
func TestGetModelSupportedEndpoints_NoNilOrEmptyInAnyPath(t *testing.T) {
	allModes := []mode.Mode{
		mode.ChatCompletions, mode.Completions, mode.Embeddings, mode.Moderations,
		mode.ImagesGenerations, mode.ImagesEdits, mode.AudioSpeech,
		mode.AudioTranscription, mode.AudioTranslation, mode.Rerank, mode.ParsePdf,
		mode.Anthropic, mode.Gemini, mode.Responses,
		mode.VideoGenerationsJobs, mode.VideoGenerationsGetJobs, mode.VideoGenerationsContent,
	}
	for _, m := range allModes {
		endpoints := getSupportedEndpoints(m)
		if len(endpoints) == 0 {
			t.Errorf("mode %s: no endpoints returned (expected at least one)", m)
		}
		for i, ep := range endpoints {
			if ep == "" {
				t.Errorf("mode %s [%d]: empty endpoint string", m, i)
			}
		}
	}
}

// TestGetModelSupportedEndpoints_ModelTypeFallback verifies that Config["model_type"]
// is used as a reliable fallback when Config["endpoints"] is absent or has unknown slugs.
// This matters for PPIO models synced before inferModeFromPPIO was introduced, where
// mc.Type may still be stale (= 1 = ChatCompletions for all types).
func TestGetModelSupportedEndpoints_ModelTypeFallback(t *testing.T) {
	cases := []struct {
		name       string
		modelType  string // Config["model_type"]
		mcType     mode.Mode
		wantLen    int
		wantFirst  string
	}{
		{
			name:      "embedding model with stale mc.Type=1",
			modelType: "embedding",
			mcType:    mode.ChatCompletions, // stale
			wantLen:   1,
			wantFirst: "POST /v1/embeddings",
		},
		{
			name:      "rerank model with stale mc.Type=1",
			modelType: "rerank",
			mcType:    mode.ChatCompletions, // stale
			wantLen:   1,
			wantFirst: "POST /v1/rerank",
		},
		{
			name:      "moderation model with stale mc.Type",
			modelType: "moderation",
			mcType:    mode.ChatCompletions,
			wantLen:   1,
			wantFirst: "POST /v1/moderations",
		},
		{
			name:      "chat model — model_type agrees with mc.Type",
			modelType: "chat",
			mcType:    mode.ChatCompletions,
			wantLen:   len(endpointsChatFamily),
		},
	}
	for _, tc := range cases {
		mc := model.ModelConfig{
			Model: "test-" + tc.name,
			Type:  tc.mcType,
			Config: map[model.ModelConfigKey]any{
				"model_type": tc.modelType,
				// no "endpoints" key — forces fallback path
			},
		}
		got := getModelSupportedEndpoints(mc)
		if len(got) != tc.wantLen {
			t.Errorf("%s: got %d endpoints %v, want %d", tc.name, len(got), got, tc.wantLen)
			continue
		}
		if tc.wantFirst != "" && (len(got) == 0 || got[0] != tc.wantFirst) {
			t.Errorf("%s: got[0]=%q, want %q", tc.name, got[0], tc.wantFirst)
		}
	}
}

// TestGetModelSupportedEndpoints_ModelTypeFallbackUnknownSlug verifies that when
// Config["endpoints"] has only unknown slugs AND Config["model_type"] is present,
// the model_type takes precedence over mc.Type for the fallback.
func TestGetModelSupportedEndpoints_ModelTypeFallbackUnknownSlug(t *testing.T) {
	// Embedding model with unknown slug + correct model_type + stale mc.Type
	mc := model.ModelConfig{
		Model: "bge-m3",
		Type:  mode.ChatCompletions, // stale pre-fix value
		Config: map[model.ModelConfigKey]any{
			"endpoints":  []string{"future-unknown-slug"},
			"model_type": "embedding",
		},
	}
	got := getModelSupportedEndpoints(mc)
	if len(got) != 1 || got[0] != "POST /v1/embeddings" {
		t.Errorf("unknown slug + model_type fallback: got %v, want [POST /v1/embeddings]", got)
	}
}

// TestPpioSlugToPath_PathFormat verifies all mapped paths follow "METHOD /path" format.
func TestPpioSlugToPath_PathFormat(t *testing.T) {
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
	for slug, path := range ppioSlugToPath {
		method, apiPath, ok := strings.Cut(path, " ")
		if !ok {
			t.Errorf("slug %q: path %q has no space separating method from path", slug, path)
			continue
		}
		if !validMethods[method] {
			t.Errorf("slug %q: method %q is not a valid HTTP method", slug, method)
		}
		if !strings.HasPrefix(apiPath, "/") {
			t.Errorf("slug %q: API path %q must start with /", slug, apiPath)
		}
	}
}
