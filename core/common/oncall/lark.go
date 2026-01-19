package oncall

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bytedance/sonic"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// TextContent is the content structure for text messages
type TextContent struct {
	Text string `json:"text"`
}

var (
	clientCache   = make(map[string]*lark.Client)
	clientCacheMu sync.RWMutex
)

// getOrCreateClient gets or creates a Lark client for the given app
func getOrCreateClient(appID, appSecret string) *lark.Client {
	key := appID + ":" + appSecret

	clientCacheMu.RLock()

	if client, ok := clientCache[key]; ok {
		clientCacheMu.RUnlock()
		return client
	}

	clientCacheMu.RUnlock()

	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()

	// Double check after acquiring write lock
	if client, ok := clientCache[key]; ok {
		return client
	}

	client := lark.NewClient(appID, appSecret)
	clientCache[key] = client

	return client
}

// SendMessage sends a text message to the specified user and returns the message ID
func SendMessage(
	ctx context.Context,
	appID, appSecret, openID, title, message string,
) (string, error) {
	client := getOrCreateClient(appID, appSecret)

	textContent := TextContent{
		Text: fmt.Sprintf("【告警】%s\n\n%s", title, message),
	}

	content, err := sonic.MarshalString(textContent)
	if err != nil {
		return "", fmt.Errorf("failed to marshal content: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeOpenId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(openID).
			MsgType(larkim.MsgTypeText).
			Content(content).
			Build()).
		Build()

	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("failed to send message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", errors.New("message ID not returned")
	}

	return *resp.Data.MessageId, nil
}

// SendUrgentPhone sends an urgent phone call to the specified user
func SendUrgentPhone(ctx context.Context, appID, appSecret, messageID, openID string) error {
	if messageID == "" {
		return errors.New("message ID is required")
	}

	client := getOrCreateClient(appID, appSecret)

	req := larkim.NewUrgentPhoneMessageReqBuilder().
		MessageId(messageID).
		UserIdType(larkim.UserIdTypeOpenId).
		UrgentReceivers(larkim.NewUrgentReceiversBuilder().
			UserIdList([]string{openID}).
			Build()).
		Build()

	resp, err := client.Im.Message.UrgentPhone(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send urgent phone: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to send urgent phone: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}
