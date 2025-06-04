package baidu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/utils"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
)

type AccessToken struct {
	AccessToken      string `json:"access_token"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	ExpiresIn        int64  `json:"expires_in,omitempty"`
}

var tokenCache = cache.New(time.Hour*23, time.Minute)

func GetAccessToken(ctx context.Context, apiKey string) (string, error) {
	if val, ok := tokenCache.Get(apiKey); ok {
		accessToken, ok := val.(string)
		if !ok {
			panic(fmt.Sprintf("invalid cache value type: %T", val))
		}
		return accessToken, nil
	}
	accessToken, err := getBaiduAccessTokenHelper(ctx, apiKey)
	if err != nil {
		log.Errorf("get baidu access token failed: %v", err)
		return "", errors.New("get baidu access token failed")
	}
	tokenCache.Set(
		apiKey,
		accessToken.AccessToken,
		time.Duration(accessToken.ExpiresIn)*time.Second-time.Minute*10,
	)
	return accessToken.AccessToken, nil
}

func getBaiduAccessTokenHelper(ctx context.Context, apiKey string) (*AccessToken, error) {
	clientID, clientSecret, err := getClientIDAndSecret(apiKey)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf(
			"https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s",
			clientID,
			clientSecret,
		),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	res, err := utils.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var accessToken AccessToken
	err = sonic.ConfigDefault.NewDecoder(res.Body).Decode(&accessToken)
	if err != nil {
		return nil, err
	}
	if accessToken.Error != "" {
		return nil, fmt.Errorf(
			"get baidu access token failed: %s: %s",
			accessToken.Error,
			accessToken.ErrorDescription,
		)
	}
	if accessToken.AccessToken == "" {
		return nil, errors.New("get baidu access token return empty access token")
	}
	return &accessToken, nil
}
