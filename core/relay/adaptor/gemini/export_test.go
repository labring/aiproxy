package gemini

import "context"

import relaymodel "github.com/labring/aiproxy/core/relay/model"

func BuildMessagePartForTest(
	message relaymodel.MessageContent,
) *relaymodel.GeminiPart {
	return buildMessageParts(message)
}

func ProcessImageTasksForTest(ctx context.Context, imageTasks []*relaymodel.GeminiPart) error {
	return processImageTasks(ctx, imageTasks)
}
