package model

import "fmt"

type ChannelType int

func (c ChannelType) String() string {
	if name, ok := channelTypeNames[c]; ok {
		return name
	}
	return fmt.Sprintf("unknow(%d)", c)
}

const (
	ChannelTypeOpenAI                  ChannelType = 1
	ChannelTypeAzure                   ChannelType = 3
	ChannelTypeAzure2                  ChannelType = 4
	ChannelTypeGoogleGeminiOpenAI      ChannelType = 12
	ChannelTypeBaiduV2                 ChannelType = 13
	ChannelTypeAnthropic               ChannelType = 14
	ChannelTypeBaidu                   ChannelType = 15
	ChannelTypeZhipu                   ChannelType = 16
	ChannelTypeAli                     ChannelType = 17
	ChannelTypeXunfei                  ChannelType = 18
	ChannelTypeAI360                   ChannelType = 19
	ChannelTypeOpenRouter              ChannelType = 20
	ChannelTypeTencent                 ChannelType = 23
	ChannelTypeGoogleGemini            ChannelType = 24
	ChannelTypeMoonshot                ChannelType = 25
	ChannelTypeBaichuan                ChannelType = 26
	ChannelTypeMinimax                 ChannelType = 27
	ChannelTypeMistral                 ChannelType = 28
	ChannelTypeGroq                    ChannelType = 29
	ChannelTypeOllama                  ChannelType = 30
	ChannelTypeLingyiwanwu             ChannelType = 31
	ChannelTypeStepfun                 ChannelType = 32
	ChannelTypeAWS                     ChannelType = 33
	ChannelTypeCoze                    ChannelType = 34
	ChannelTypeCohere                  ChannelType = 35
	ChannelTypeDeepseek                ChannelType = 36
	ChannelTypeCloudflare              ChannelType = 37
	ChannelTypeDoubao                  ChannelType = 40
	ChannelTypeNovita                  ChannelType = 41
	ChannelTypeVertexAI                ChannelType = 42
	ChannelTypeSiliconflow             ChannelType = 43
	ChannelTypeDoubaoAudio             ChannelType = 44
	ChannelTypeXAI                     ChannelType = 45
	ChannelTypeDoc2x                   ChannelType = 46
	ChannelTypeJina                    ChannelType = 47
	ChannelTypeTextEmbeddingsInference ChannelType = 48
	ChannelTypeQianfan                 ChannelType = 49
	ChannelTypeSangforAICP             ChannelType = 50
	ChannelTypeStreamlake              ChannelType = 51
	ChannelTypeZhipuCoding             ChannelType = 52
)

var channelTypeNames = map[ChannelType]string{
	ChannelTypeOpenAI:                  "openai",
	ChannelTypeAzure:                   "azure",
	ChannelTypeAzure2:                  "azure (model name support contain '.')",
	ChannelTypeGoogleGeminiOpenAI:      "google gemini (openai)",
	ChannelTypeBaiduV2:                 "baidu v2",
	ChannelTypeAnthropic:               "anthropic",
	ChannelTypeBaidu:                   "baidu",
	ChannelTypeZhipu:                   "zhipu",
	ChannelTypeAli:                     "ali",
	ChannelTypeXunfei:                  "xunfei",
	ChannelTypeAI360:                   "ai360",
	ChannelTypeOpenRouter:              "openrouter",
	ChannelTypeTencent:                 "tencent",
	ChannelTypeGoogleGemini:            "google gemini",
	ChannelTypeMoonshot:                "moonshot",
	ChannelTypeBaichuan:                "baichuan",
	ChannelTypeMinimax:                 "minimax",
	ChannelTypeMistral:                 "mistral",
	ChannelTypeGroq:                    "groq",
	ChannelTypeOllama:                  "ollama",
	ChannelTypeLingyiwanwu:             "lingyiwanwu",
	ChannelTypeStepfun:                 "stepfun",
	ChannelTypeAWS:                     "aws",
	ChannelTypeCoze:                    "coze",
	ChannelTypeCohere:                  "Cohere",
	ChannelTypeDeepseek:                "deepseek",
	ChannelTypeCloudflare:              "cloudflare",
	ChannelTypeDoubao:                  "doubao",
	ChannelTypeNovita:                  "novita",
	ChannelTypeVertexAI:                "vertexai",
	ChannelTypeSiliconflow:             "siliconflow",
	ChannelTypeDoubaoAudio:             "doubao audio",
	ChannelTypeXAI:                     "xai",
	ChannelTypeDoc2x:                   "doc2x",
	ChannelTypeJina:                    "jina",
	ChannelTypeTextEmbeddingsInference: "huggingface text-embeddings-inference",
	ChannelTypeQianfan:                 "qianfan",
	ChannelTypeSangforAICP:             "Sangfor AICP",
	ChannelTypeStreamlake:              "Streamlake",
	ChannelTypeZhipuCoding:             "zhipu coding",
}

func AllChannelTypes() []ChannelType {
	types := make([]ChannelType, 0, len(channelTypeNames))
	for t := range channelTypeNames {
		types = append(types, t)
	}

	return types
}
