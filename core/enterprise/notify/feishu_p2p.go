//go:build enterprise

package notify

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	log "github.com/sirupsen/logrus"
)

// FeishuP2PClient sends point-to-point messages to individual Feishu users
// via the Feishu IM API using the larksuite SDK.
type FeishuP2PClient struct {
	client *lark.Client
}

// NewFeishuP2PClient creates a new Feishu P2P client with the given app credentials.
func NewFeishuP2PClient(appID, appSecret string) *FeishuP2PClient {
	client := lark.NewClient(appID, appSecret)

	return &FeishuP2PClient{
		client: client,
	}
}

// SendMessage sends a message to a Feishu user identified by openID.
// msgType is the Feishu message type (e.g., "text", "interactive").
// content is the JSON-encoded message content.
func (c *FeishuP2PClient) SendMessage(ctx context.Context, openID, msgType, content string) error {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeOpenId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(openID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		log.WithError(err).WithField("open_id", openID).Error("failed to send feishu p2p message")
		return fmt.Errorf("failed to send feishu p2p message: %w", err)
	}

	if !resp.Success() {
		log.WithFields(log.Fields{
			"open_id": openID,
			"code":    resp.Code,
			"msg":     resp.Msg,
		}).Error("feishu p2p message API returned error")

		return fmt.Errorf("feishu p2p message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// SendTextMessage sends a plain text message to a Feishu user.
func (c *FeishuP2PClient) SendTextMessage(ctx context.Context, openID, text string) error {
	content := fmt.Sprintf(`{"text":%q}`, text)
	return c.SendMessage(ctx, openID, larkim.MsgTypeText, content)
}

// SendCardMessage sends an interactive card message to a Feishu user.
// color should be one of: "green", "orange", "red", etc.
func (c *FeishuP2PClient) SendCardMessage(ctx context.Context, openID, title, content, color string) error {
	card := buildCardJSON(title, content, color)
	return c.SendMessage(ctx, openID, larkim.MsgTypeInteractive, card)
}

// SendChatCardMessage sends an interactive card message to a Feishu group chat.
func (c *FeishuP2PClient) SendChatCardMessage(ctx context.Context, chatID, title, content, color string) error {
	card := buildCardJSON(title, content, color)

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeInteractive).
			Content(card).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		log.WithError(err).WithField("chat_id", chatID).Error("failed to send feishu chat message")
		return fmt.Errorf("failed to send feishu chat message: %w", err)
	}

	if !resp.Success() {
		log.WithFields(log.Fields{
			"chat_id": chatID,
			"code":    resp.Code,
			"msg":     resp.Msg,
		}).Error("feishu chat message API returned error")
		return fmt.Errorf("feishu chat message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

func buildCardJSON(title, content, color string) string {
	return fmt.Sprintf(
		`{"config":{"wide_screen_mode":true},"header":{"title":{"tag":"plain_text","content":%q},"template":%q},"elements":[{"tag":"div","text":{"tag":"lark_md","content":%q}}]}`,
		title, color, content,
	)
}
