package oncall

import (
	"context"
	"os"
	"testing"
	"time"
)

// Test credentials (from environment variables)
// Set these environment variables before running tests:
//
//	export TEST_LARK_APP_ID=cli_a82791f5be3d1013
//	export TEST_LARK_APP_SECRET=aUZIMmmHiDp9NzvOeEmxlcMAVbYATjdm
//	export TEST_LARK_OPEN_ID=ou_26b77434903693adc3d6c343df94ccb0
func getTestCredentials(t *testing.T) (appID, appSecret, openID string) {
	appID = os.Getenv("TEST_LARK_APP_ID")
	appSecret = os.Getenv("TEST_LARK_APP_SECRET")
	openID = os.Getenv("TEST_LARK_OPEN_ID")

	if appID == "" || appSecret == "" || openID == "" {
		t.Skip("TEST_LARK_APP_ID, TEST_LARK_APP_SECRET, TEST_LARK_OPEN_ID environment variables not set")
	}

	return appID, appSecret, openID
}

func TestSendMessage(t *testing.T) {
	appID, appSecret, openID := getTestCredentials(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messageID, err := SendMessage(ctx, appID, appSecret, openID, "Test Alert", "This is a test message from oncall unit test.\n\nTime: "+time.Now().Format(time.RFC3339))
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

	messageID, err := SendMessage(ctx, appID, appSecret, openID, title, message)
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
	messageID, err := SendMessage(ctx, appID, appSecret, openID, "Urgent Test", "This is an urgent test message.\n\nTime: "+time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	t.Logf("Message sent, messageID: %s", messageID)

	// Then send urgent phone call
	err = SendUrgentPhone(ctx, appID, appSecret, messageID, openID)
	if err != nil {
		t.Fatalf("SendUrgentPhone failed: %v", err)
	}

	t.Log("Urgent phone call sent successfully")
}

