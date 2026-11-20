//nolint:testpackage
package controller

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/model"
	"github.com/stretchr/testify/require"
)

func withTokenControllerDB(t *testing.T) {
	t.Helper()

	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "token_controller.db"))
	require.NoError(t, err)
	require.NoError(
		t,
		db.AutoMigrate(&model.Token{}, &model.Group{}, &model.GroupSummaryMinute{}),
	)

	previousDB := model.DB
	previousLogDB := model.LogDB
	model.DB = db
	model.LogDB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.LogDB = previousLogDB
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
}

func tokenJSONRequest(
	t *testing.T,
	method, target, group, id string,
	body any,
) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{}

	if group != "" {
		c.Params = append(c.Params, gin.Param{Key: "group", Value: group})
	}

	if id != "" {
		c.Params = append(c.Params, gin.Param{Key: "id", Value: id})
	}

	var reader *strings.Reader
	if body != nil {
		payload, err := sonic.Marshal(body)
		require.NoError(t, err)

		reader = strings.NewReader(string(payload))
	} else {
		reader = strings.NewReader("")
	}

	c.Request = httptest.NewRequestWithContext(t.Context(), method, target, reader)

	return recorder, c
}

func TestAddGroupTokenHashedKeyRevealedOnce(t *testing.T) {
	withTokenControllerDB(t)

	recorder, c := tokenJSONRequest(
		t,
		http.MethodPost,
		"/api/token/hash-group",
		"hash-group",
		"",
		AddTokenRequest{Name: "hashed-token", HashedKey: true},
	)
	AddGroupToken(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Data struct {
			ID  int    `json:"id"`
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(recorder.Body.Bytes(), &response))
	rawKey := response.Data.Key
	require.Len(t, rawKey, 48)

	token, err := model.GetTokenByID(response.Data.ID)
	require.NoError(t, err)
	require.True(t, model.IsHashedTokenKey(token.Key))
	require.NotContains(t, token.Key, rawKey)
	require.Equal(t, model.TokenKeyPreview(rawKey), token.KeyPreview)

	_, err = model.GetAndValidateToken(rawKey)
	require.NoError(t, err)

	// list and detail responses only ever show the preview
	listRecorder, listC := tokenJSONRequest(t, http.MethodGet, "/api/tokens/", "", "", nil)
	GetTokens(listC)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	require.Contains(t, listRecorder.Body.String(), model.TokenKeyPreview(rawKey))
	require.NotContains(t, listRecorder.Body.String(), rawKey)
	require.NotContains(t, listRecorder.Body.String(), token.Key)
}

func TestTokenExpiredAtCreateAndRotate(t *testing.T) {
	withTokenControllerDB(t)

	expiredAt := time.Now().Add(48 * time.Hour)

	recorder, c := tokenJSONRequest(
		t,
		http.MethodPost,
		"/api/token/expire-group",
		"expire-group",
		"",
		AddTokenRequest{Name: "expire-token", ExpiredAt: expiredAt.UnixMilli()},
	)
	AddGroupToken(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Data struct {
			ID        int   `json:"id"`
			ExpiredAt int64 `json:"expired_at"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, expiredAt.UnixMilli(), response.Data.ExpiredAt)

	token, err := model.GetTokenByID(response.Data.ID)
	require.NoError(t, err)
	require.WithinDuration(t, expiredAt, token.ExpiredAt, time.Second)

	// rotate overrides the expiry in the same call
	recorder, c = tokenJSONRequest(
		t,
		http.MethodPost,
		"/api/tokens/"+strconv.Itoa(token.ID)+"/rotate",
		"",
		strconv.Itoa(token.ID),
		RotateTokenRequest{ExpiredAt: new(time.Now().Add(-time.Minute).UnixMilli())},
	)
	RotateToken(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	rotated, err := model.GetTokenByID(token.ID)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(-time.Minute), rotated.ExpiredAt, time.Second)

	_, err = model.GetAndValidateToken(extractTokenKey(t, recorder))
	require.ErrorContains(t, err, "is expired")
}

func TestAddGroupTokenHashedKeyIgnoreExistReturnsStoredToken(t *testing.T) {
	withTokenControllerDB(t)

	recorder, c := tokenJSONRequest(
		t,
		http.MethodPost,
		"/api/token/hash-group?ignore_exist=true",
		"hash-group",
		"",
		AddTokenRequest{Name: "dup-token"},
	)
	AddGroupToken(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	firstKey := extractTokenKey(t, recorder)

	recorder, c = tokenJSONRequest(
		t,
		http.MethodPost,
		"/api/token/hash-group?ignore_exist=true",
		"hash-group",
		"",
		AddTokenRequest{Name: "dup-token", HashedKey: true},
	)
	AddGroupToken(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	// the stored plain token wins; no unrelated key is revealed
	require.Equal(t, firstKey, extractTokenKey(t, recorder))
}

func extractTokenKey(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	var response struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(recorder.Body.Bytes(), &response))

	return response.Data.Key
}

func TestRotateTokenHandler(t *testing.T) {
	withTokenControllerDB(t)

	rawKey := model.GenerateTokenKey()
	token := &model.Token{
		Name:       "rotate-me",
		GroupID:    "rotate-group",
		Key:        model.HashTokenKey(rawKey),
		KeyPreview: model.TokenKeyPreview(rawKey),
	}
	require.NoError(t, model.InsertToken(token, true, false))

	_, err := model.GetAndValidateToken(rawKey)
	require.NoError(t, err)

	// invalid mode
	recorder, c := tokenJSONRequest(
		t,
		http.MethodPost,
		"/api/tokens/"+strconv.Itoa(token.ID)+"/rotate",
		"",
		strconv.Itoa(token.ID),
		RotateTokenRequest{Mode: "invalid"},
	)
	RotateToken(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	// switch to plain storage
	recorder, c = tokenJSONRequest(
		t,
		http.MethodPost,
		"/api/tokens/"+strconv.Itoa(token.ID)+"/rotate",
		"",
		strconv.Itoa(token.ID),
		RotateTokenRequest{Mode: model.TokenRotateModePlain},
	)
	RotateToken(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	newKey := extractTokenKey(t, recorder)
	require.Len(t, newKey, 48)

	_, err = model.GetAndValidateToken(rawKey)
	require.Error(t, err)
	_, err = model.GetAndValidateToken(newKey)
	require.NoError(t, err)

	stored, err := model.GetTokenByID(token.ID)
	require.NoError(t, err)
	require.Equal(t, newKey, stored.Key)
	require.Empty(t, stored.KeyPreview)

	// group variant rotates the same token
	recorder, c = tokenJSONRequest(
		t,
		http.MethodPost,
		"/api/token/rotate-group/"+strconv.Itoa(token.ID)+"/rotate",
		"rotate-group",
		strconv.Itoa(token.ID),
		RotateTokenRequest{Mode: model.TokenRotateModeHash},
	)
	RotateGroupToken(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	hashedKey := extractTokenKey(t, recorder)
	require.Len(t, hashedKey, 48)
	require.NotEqual(t, newKey, hashedKey)

	_, err = model.GetAndValidateToken(newKey)
	require.Error(t, err)
	_, err = model.GetAndValidateToken(hashedKey)
	require.NoError(t, err)
}

func TestChannelKeyPreview(t *testing.T) {
	require.Equal(t, "abcdef…wxyz", channelKeyPreview("abcdefghijklmnopqrstuvwxyz"))
	require.Equal(t, "ab…", channelKeyPreview("abcdef"))
	require.Equal(t, "…", channelKeyPreview("abc"))
}

func TestMaskChannelKeyConfig(t *testing.T) {
	previous := config.MaskChannelKeys
	t.Cleanup(func() { config.MaskChannelKeys = previous })

	longKey := "sk-channel-key-0123456789abcdef"

	config.MaskChannelKeys = false
	channel := maskChannel(&model.Channel{Key: longKey})
	groupChannel := maskGroupChannel(&model.GroupChannel{Key: longKey})
	require.Equal(t, longKey, channel.Key)
	require.Equal(t, longKey, groupChannel.Key)

	config.MaskChannelKeys = true
	channel = maskChannel(&model.Channel{Key: longKey})
	groupChannel = maskGroupChannel(&model.GroupChannel{Key: longKey})
	require.Equal(t, channelKeyPreview(longKey), channel.Key)
	require.Equal(t, channelKeyPreview(longKey), groupChannel.Key)
	require.NotContains(t, channel.Key, "0123456789abcdef")
}
