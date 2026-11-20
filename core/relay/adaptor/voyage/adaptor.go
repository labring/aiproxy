package voyage

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/openai"
	"github.com/labring/aiproxy/core/relay/adaptor/registry"
	"github.com/labring/aiproxy/core/relay/meta"
	"github.com/labring/aiproxy/core/relay/mode"
)

// Adaptor relays OpenAI-style embeddings and Cohere/Jina-style rerank requests to
// the Voyage AI API. Only embeddings and rerank are supported. Contextualized
// embeddings have no relay mode of their own: the voyage-context-* models are
// served through the embeddings mode, one string per document.
type Adaptor struct {
	openai.Adaptor
}

func init() {
	registry.Register(model.ChannelTypeVoyage, &Adaptor{})
}

const baseURL = "https://api.voyageai.com/v1"

func (a *Adaptor) DefaultBaseURL() string {
	return baseURL
}

// SupportMode limits the channel to the two modes Voyage serves; the embedded
// OpenAI adaptor would otherwise advertise chat, images, audio and more.
func (a *Adaptor) SupportMode(mt *meta.Meta) bool {
	m := adaptor.ModeFromMeta(mt)

	return m == mode.Embeddings || m == mode.Rerank
}

// GetRequestURL sends multimodal embedding models to /multimodalembeddings and
// contextualized models to /contextualizedembeddings; every other mode keeps the
// OpenAI-compatible path layout.
func (a *Adaptor) GetRequestURL(
	meta *meta.Meta,
	store adaptor.Store,
	c *gin.Context,
) (adaptor.RequestURL, error) {
	if meta.Mode == mode.Embeddings {
		path := ""

		switch {
		case isMultimodalModel(meta.ActualModel):
			path = "/multimodalembeddings"
		case isContextualModel(meta.ActualModel):
			path = "/contextualizedembeddings"
		}

		if path != "" {
			requestURL, err := url.JoinPath(meta.Channel.BaseURL, path)
			if err != nil {
				return adaptor.RequestURL{}, err
			}

			return adaptor.RequestURL{Method: http.MethodPost, URL: requestURL}, nil
		}
	}

	return a.Adaptor.GetRequestURL(meta, store, c)
}

func (a *Adaptor) ConvertRequest(
	meta *meta.Meta,
	store adaptor.Store,
	req *http.Request,
) (adaptor.ConvertResult, error) {
	switch meta.Mode {
	case mode.Embeddings:
		switch {
		case isMultimodalModel(meta.ActualModel):
			return ConvertMultimodalEmbeddingsRequest(meta, req)
		case isContextualModel(meta.ActualModel):
			return ConvertContextualizedEmbeddingsRequest(meta, req)
		}

		return ConvertEmbeddingsRequest(meta, req)
	case mode.Rerank:
		return ConvertRerankRequest(meta, req)
	default:
		return adaptor.ConvertResult{}, unsupportedModeError(meta.Mode)
	}
}

func (a *Adaptor) DoResponse(
	meta *meta.Meta,
	_ adaptor.Store,
	c *gin.Context,
	resp *http.Response,
) (adaptor.DoResponseResult, adaptor.Error) {
	switch meta.Mode {
	case mode.Embeddings:
		switch {
		case isMultimodalModel(meta.ActualModel):
			return MultimodalEmbeddingsHandler(meta, c, resp)
		case isContextualModel(meta.ActualModel):
			return ContextualizedEmbeddingsHandler(meta, c, resp)
		}

		return EmbeddingsHandler(meta, c, resp)
	case mode.Rerank:
		return RerankHandler(meta, c, resp)
	default:
		return adaptor.DoResponseResult{}, unsupportedModeError(meta.Mode)
	}
}

func (a *Adaptor) Metadata() adaptor.Metadata {
	return adaptor.Metadata{
		Readme: "https://www.voyageai.com\n" +
			"Supports text embeddings, contextualized embeddings (voyage-context-*, one " +
			"string per document, auto-chunked at 32000 tokens), multimodal embeddings " +
			"(text and images) and rerank.\n" +
			"The embeddings `dimensions` field is mapped to Voyage `output_dimension`; " +
			"`input_type` (`document` or `query`) and `truncation` are passed through.",
		KeyHelp:      "Voyage AI API key (https://dash.voyageai.com/api-keys)",
		ConfigSchema: openai.ConfigSchema(),
		Models:       ModelList,
	}
}
