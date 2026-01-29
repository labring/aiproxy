package oncall_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/labring/aiproxy/core/common/oncall"
)

func getTestCredentials(t *testing.T) (appID, appSecret, openID string) {
	t.Helper()

	appID = os.Getenv("TEST_LARK_APP_ID")
	appSecret = os.Getenv("TEST_LARK_APP_SECRET")
	openID = os.Getenv("TEST_LARK_OPEN_ID")

	if appID == "" || appSecret == "" || openID == "" {
		t.Skip(
			"TEST_LARK_APP_ID, TEST_LARK_APP_SECRET, TEST_LARK_OPEN_ID environment variables not set",
		)
	}

	return appID, appSecret, openID
}

func TestSendMessage(t *testing.T) {
	appID, appSecret, openID := getTestCredentials(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messageID, err := oncall.SendMessage(
		ctx,
		appID,
		appSecret,
		openID,
		"Test Alert",
		"This is a test message from oncall unit test.\n\nTime: "+time.Now().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if messageID == "" {
		t.Fatal("SendMessage returned empty message ID")
	}

	t.Logf("Message sent successfully, messageID: %s", messageID)
}

func TestSendMessageWithSpecialChars(t *testing.T) {
	appID, appSecret, openID := getTestCredentials(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with special characters that need JSON escaping
	title := `Test "Alert" with 'quotes'`
	message := "Line1\nLine2\tTabbed\nPath: C:\\Users\\test\n\"Quoted text\""

	messageID, err := oncall.SendMessage(ctx, appID, appSecret, openID, title, message)
	if err != nil {
		t.Fatalf("SendMessage with special chars failed: %v", err)
	}

	if messageID == "" {
		t.Fatal("SendMessage returned empty message ID")
	}

	t.Logf("Message with special chars sent successfully, messageID: %s", messageID)
}

func TestSendUrgentPhone(t *testing.T) {
	appID, appSecret, openID := getTestCredentials(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First send a message to get messageID
	messageID, err := oncall.SendMessage(
		ctx,
		appID,
		appSecret,
		openID,
		"Urgent Test",
		"This is an urgent test message.\n\nTime: "+time.Now().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	t.Logf("Message sent, messageID: %s", messageID)

	// Then send urgent phone call
	err = oncall.SendUrgentPhone(ctx, appID, appSecret, messageID, openID)
	if err != nil {
		t.Fatalf("SendUrgentPhone failed: %v", err)
	}

	t.Log("Urgent phone call sent successfully")
}
