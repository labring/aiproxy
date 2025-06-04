package vertexai

import (
	"context"
	"fmt"
	"time"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/patrickmn/go-cache"
	"google.golang.org/api/option"
)

type ApplicationDefaultCredentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
	UniverseDomain          string `json:"universe_domain"`
}

var tokenCache = cache.New(30*time.Minute, time.Minute)

const defaultScope = "https://www.googleapis.com/auth/cloud-platform"

func getToken(ctx context.Context, adcJSON string) (string, error) {
	if tokenI, found := tokenCache.Get(adcJSON); found {
		token, ok := tokenI.(string)
		if !ok {
			panic(fmt.Sprintf("invalid cache value type: %T", tokenI))
		}
		return token, nil
	}
	adc := &ApplicationDefaultCredentials{}
	if err := sonic.UnmarshalString(adcJSON, adc); err != nil {
		return "", fmt.Errorf("failed to decode credentials file: %w", err)
	}

	c, err := credentials.NewIamCredentialsClient(
		ctx,
		option.WithCredentialsJSON(conv.StringToBytes(adcJSON)),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}
	defer c.Close()

	req := &credentialspb.GenerateAccessTokenRequest{
		// See
		// https://pkg.go.dev/cloud.google.com/go/iam/credentials/apiv1/credentialspb#GenerateAccessTokenRequest.
		Name:  "projects/-/serviceAccounts/" + adc.ClientEmail,
		Scope: []string{defaultScope},
	}
	resp, err := c.GenerateAccessToken(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}
	token := resp.GetAccessToken()
	if token == "" {
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}
	expireTime := resp.GetExpireTime()
	expireTimeTime := time.Minute * 30
	if expireTime != nil && expireTime.IsValid() {
		expireTimeTime = time.Until(expireTime.AsTime()) / 2
	}
	tokenCache.Set(adcJSON, token, expireTimeTime)
	return token, nil
}
