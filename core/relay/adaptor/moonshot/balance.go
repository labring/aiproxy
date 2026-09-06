package moonshot

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/utils"
)

var _ adaptor.Balancer = (*Adaptor)(nil)

func (a *Adaptor) GetBalance(channel *model.Channel) (float64, error) {
	u := channel.BaseURL
	if u == "" {
		u = baseURL
	}

	url := u + "/users/me/balance"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+channel.Key)

	client, err := utils.LoadHTTPClientWithTLSConfigE(0, channel.ProxyURL, channel.SkipTLSVerify)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var response BalanceResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, err
	}

	if response.Error != nil {
		return 0, fmt.Errorf("type: %s, message: %s", response.Error.Type, response.Error.Message)
	}

	return response.Data.AvailableBalance, nil
}

type BalanceResponse struct {
	Error *BalanceError `json:"error"`
	Data  BalanceData   `json:"data"`
}

type BalanceData struct {
	AvailableBalance float64 `json:"available_balance"`
}

type BalanceError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}
