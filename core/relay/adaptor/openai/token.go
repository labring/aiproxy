package openai

import (
	"errors"
	"math"
	"strings"

	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/image"
	intertiktoken "github.com/labring/aiproxy/core/common/tiktoken"
	"github.com/labring/aiproxy/core/relay/model"
	log "github.com/sirupsen/logrus"
	"github.com/tiktoken-go/tokenizer"
)

func getTokenNum(tokenEncoder tokenizer.Codec, text string) int64 {
	// Check fuzzy token threshold
	threshold := config.GetFuzzyTokenThreshold()
	textLen := len(text)

	// If threshold is set and text length exceeds it, use fuzzy calculation
	if threshold > 0 && int64(textLen) >= threshold {
		return int64(textLen / 4)
	}

	// Otherwise, use precise token counting
	count, err := tokenEncoder.Count(text)
	if err != nil {
		log.Warnf("failed to count tokens: %v, fallback to length/4", err)
		// Fallback to rough estimation if counting fails
		return int64(textLen / 4)
	}

	return int64(count)
}

func CountTokenMessages(messages []model.Message, model string) int64 {
	tokenEncoder := intertiktoken.GetTokenEncoder(model)
	// Reference:
	// https://github.com/openai/openai-cookbook/blob/main/examples/How_to_count_tokens_with_tiktoken.ipynb
	//
	// Every message follows <|start|>{role/name}\n{content}<|end|>\n
	var (
		tokensPerMessage int64
		tokensPerName    int64
	)

	if model == "gpt-3.5-turbo-0301" {
		tokensPerMessage = 4
		tokensPerName = -1 // If there's a name, the role is omitted
	} else {
		tokensPerMessage = 3
		tokensPerName = 1
	}

	var (
		textBuilder     strings.Builder
		tokenNum        int64
		imageTokenCount int64
	)

	for _, message := range messages {
		tokenNum += tokensPerMessage

		textBuilder.WriteString(message.Role)

		switch v := message.Content.(type) {
		case string:
			textBuilder.WriteString(v)

		case []any:
			for _, it := range v {
				m, ok := it.(map[string]any)
				if !ok {
					continue
				}

				switch m["type"] {
				case "text":
					if textValue, ok := m["text"]; ok {
						if textString, ok := textValue.(string); ok {
							textBuilder.WriteString(textString)
						}
					}
				case "image_url":
					imageURL, ok := m["image_url"].(map[string]any)
					if ok {
						url, ok := imageURL["url"].(string)
						if !ok {
							continue
						}

						detail := ""
						if imageURL["detail"] != nil {
							detail, _ = imageURL["detail"].(string)
						}

						imageTokens, err := countImageTokens(url, detail, model)
						if err != nil {
							log.Error("error counting image tokens: " + err.Error())
						} else {
							imageTokenCount += imageTokens
						}
					}
				}
			}
		}

		if message.Name != nil {
			tokenNum += tokensPerName

			textBuilder.WriteString(*message.Name)
		}
	}

	if textBuilder.Len() > 0 {
		tokenNum += getTokenNum(tokenEncoder, textBuilder.String())
	}

	tokenNum += imageTokenCount
	tokenNum += 3 // Every reply is primed with <|start|>assistant<|message|>

	return tokenNum
}

const (
	lowDetailCost         = 85
	highDetailCostPerTile = 170
	additionalCost        = 85
	// gpt-4o-mini cost higher than other model
	gpt4oMiniLowDetailCost  = 2833
	gpt4oMiniHighDetailCost = 5667
	gpt4oMiniAdditionalCost = 2833
)

// https://platform.openai.com/docs/guides/vision/calculating-costs
// https://github.com/openai/openai-cookbook/blob/05e3f9be4c7a2ae7ecf029a7c32065b024730ebe/examples/How_to_count_tokens_with_tiktoken.ipynb
func countImageTokens(url, detail, model string) (_ int64, err error) {
	fetchSize := true

	var width, height int
	// Reference:
	// https://platform.openai.com/docs/guides/vision/low-or-high-fidelity-image-understanding
	// detail == "auto" is undocumented on how it works, it just said the model will use the auto
	// setting which will look at the image input size and decide if it should use the low or high
	// setting.
	// According to the official guide, "low" disable the high-res model,
	// and only receive low-res 512px x 512px version of the image, indicating
	// that image is treated as low-res when size is smaller than 512px x 512px,
	// then we can assume that image size larger than 512px x 512px is treated
	// as high-res. Then we have the following logic:
	// if detail == "" || detail == "auto" {
	// 	width, height, err = image.GetImageSize(url)
	// 	if err != nil {
	// 		return 0, err
	// 	}
	// 	fetchSize = false
	// 	// not sure if this is correct
	// 	if width > 512 || height > 512 {
	// 		detail = "high"
	// 	} else {
	// 		detail = "low"
	// 	}
	// }

	// However, in my test, it seems to be always the same as "high".
	// The following image, which is 125x50, is still treated as high-res, taken
	// 255 tokens in the response of non-stream chat completion api.
	// https://upload.wikimedia.org/wikipedia/commons/1/10/18_Infantry_Division_Messina.jpg
	if detail == "" || detail == "auto" {
		// assume by test, not sure if this is correct
		detail = "high"
	}

	switch detail {
	case "low":
		if strings.HasPrefix(model, "gpt-4o-mini") {
			return gpt4oMiniLowDetailCost, nil
		}
		return lowDetailCost, nil
	case "high":
		if fetchSize {
			width, height, err = image.GetImageSize(url)
			if err != nil {
				return 0, err
			}
		}

		if width > 2048 || height > 2048 { // max(width, height) > 2048
			ratio := float64(2048) / math.Max(float64(width), float64(height))
			width = int(float64(width) * ratio)
			height = int(float64(height) * ratio)
		}

		if width > 768 && height > 768 { // min(width, height) > 768
			ratio := float64(768) / math.Min(float64(width), float64(height))
			width = int(float64(width) * ratio)
			height = int(float64(height) * ratio)
		}

		numSquares := int64(math.Ceil(float64(width)/512) * math.Ceil(float64(height)/512))
		if strings.HasPrefix(model, "gpt-4o-mini") {
			return numSquares*gpt4oMiniHighDetailCost + gpt4oMiniAdditionalCost, nil
		}

		result := numSquares*highDetailCostPerTile + additionalCost

		return result, nil
	default:
		return 0, errors.New("invalid detail option")
	}
}

func CountTokenInput(input any, model string) int64 {
	switch v := input.(type) {
	case string:
		return CountTokenText(v, model)
	case []any:
		var num int64
		for _, s := range v {
			num += CountTokenInput(s, model)
		}

		return num
	case []string:
		builder := strings.Builder{}
		for _, s := range v {
			builder.WriteString(s)
		}

		return CountTokenText(builder.String(), model)
	}

	return 0
}

func CountTokenText(text, model string) int64 {
	return getTokenNum(intertiktoken.GetTokenEncoder(model), text)
}
