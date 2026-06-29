package siliconflow

import (
	"strings"

	"github.com/bytedance/sonic/ast"
	relaymodel "github.com/labring/aiproxy/core/relay/model"
)

func patchChatMultimodalContent(node *ast.Node) error {
	messagesNode := node.Get("messages")
	if !messagesNode.Exists() || messagesNode.TypeSafe() != ast.V_ARRAY {
		return nil
	}

	var patchErr error

	err := messagesNode.ForEach(func(_ ast.Sequence, messageNode *ast.Node) bool {
		contentNode := messageNode.Get("content")
		if !contentNode.Exists() || contentNode.TypeSafe() != ast.V_ARRAY {
			return true
		}

		patchErr = patchChatContentItems(contentNode)

		return patchErr == nil
	})
	if err != nil {
		return err
	}

	return patchErr
}

func patchSiliconFlowMultimodalContent(openAIReq *relaymodel.GeneralOpenAIRequest) error {
	for i := range openAIReq.Messages {
		patchSiliconFlowMessageContent(&openAIReq.Messages[i])
	}

	return nil
}

func patchSiliconFlowMessageContent(message *relaymodel.Message) {
	contentParts := message.ParseContent()
	if len(contentParts) == 0 {
		return
	}

	patchedParts := make([]map[string]any, 0, len(contentParts))
	for _, part := range contentParts {
		switch part.Type {
		case relaymodel.ContentTypeText:
			patchedParts = append(patchedParts, map[string]any{
				"type": "text",
				"text": part.Text,
			})
		case relaymodel.ContentTypeImageURL:
			if part.ImageURL == nil {
				continue
			}

			patchedParts = append(patchedParts, map[string]any{
				"type":      "image_url",
				"image_url": part.ImageURL,
			})
		case relaymodel.ContentTypeInputAudio:
			if part.InputAudio == nil {
				continue
			}

			patchedParts = append(patchedParts, map[string]any{
				"type": "audio_url",
				"audio_url": map[string]string{
					"url": openAIInputAudioDataURL(part.InputAudio),
				},
			})
		case relaymodel.ContentTypeVideoURL:
			if part.VideoURL == nil {
				continue
			}

			patchedParts = append(patchedParts, map[string]any{
				"type":      "video_url",
				"video_url": part.VideoURL,
			})
		}
	}

	if len(patchedParts) == 1 && patchedParts[0]["type"] == relaymodel.ContentTypeText {
		message.Content = patchedParts[0]["text"]
		return
	}

	message.Content = patchedParts
}

func patchChatContentItems(contentNode *ast.Node) error {
	return contentNode.ForEach(func(_ ast.Sequence, item *ast.Node) bool {
		if item == nil || item.TypeSafe() != ast.V_OBJECT {
			return true
		}

		contentType, ok, err := chatContentType(item)
		if err != nil {
			return false
		}

		if !ok || contentType != "input_audio" {
			return true
		}

		audioURL, ok, err := openAIInputAudioURL(item)
		if err != nil {
			return false
		}

		if ok {
			*item = newSiliconFlowAudioURLContent(audioURL)
		}

		return true
	})
}

func chatContentType(item *ast.Node) (string, bool, error) {
	typeNode := item.Get("type")
	if !typeNode.Exists() {
		return "", false, nil
	}

	contentType, err := typeNode.String()
	if err != nil {
		return "", false, err
	}

	return contentType, true, nil
}

func openAIInputAudioURL(item *ast.Node) (string, bool, error) {
	audioNode := item.Get("input_audio")
	if !audioNode.Exists() || audioNode.TypeSafe() != ast.V_OBJECT {
		return "", false, nil
	}

	urlNode := audioNode.Get("url")
	if urlNode.Exists() && urlNode.TypeSafe() == ast.V_STRING {
		audioURL, err := urlNode.String()
		if err != nil {
			return "", false, err
		}

		if audioURL != "" {
			return audioURL, true, nil
		}
	}

	dataNode := audioNode.Get("data")
	if !dataNode.Exists() || dataNode.TypeSafe() != ast.V_STRING {
		return "", false, nil
	}

	data, err := dataNode.String()
	if err != nil {
		return "", false, err
	}

	if data == "" {
		return "", false, nil
	}

	if isOpenAIInputAudioURL(data) || strings.HasPrefix(data, "data:audio/") {
		return data, true, nil
	}

	format := "wav"

	formatNode := audioNode.Get("format")
	if formatNode.Exists() && formatNode.TypeSafe() == ast.V_STRING {
		format, err = formatNode.String()
		if err != nil {
			return "", false, err
		}
	}

	format = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(format)), ".")
	if format == "" {
		format = "wav"
	}

	return "data:audio/" + format + ";base64," + data, true, nil
}

func openAIInputAudioDataURL(inputAudio *relaymodel.InputAudio) string {
	if inputAudio == nil {
		return ""
	}

	if inputAudio.URL != "" {
		return inputAudio.URL
	}

	data := strings.TrimSpace(inputAudio.Data)
	if data == "" {
		return ""
	}

	if strings.HasPrefix(data, "data:audio/") {
		return data
	}

	if isOpenAIInputAudioURL(data) {
		return data
	}

	format := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(inputAudio.Format)), ".")
	if format == "" {
		format = "wav"
	}

	return "data:audio/" + format + ";base64," + data
}

func isOpenAIInputAudioURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func newSiliconFlowAudioURLContent(audioURL string) ast.Node {
	return ast.NewObject([]ast.Pair{
		ast.NewPair("type", ast.NewString("audio_url")),
		ast.NewPair("audio_url", ast.NewObject([]ast.Pair{
			ast.NewPair("url", ast.NewString(audioURL)),
		})),
	})
}
