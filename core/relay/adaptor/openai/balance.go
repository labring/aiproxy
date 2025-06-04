package openai

import (
	"context"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

var _ adaptor.Balancer = (*Adaptor)(nil)

func (a *Adaptor) GetBalance(channel *model.Channel) (float64, error) {
	return GetBalance(channel.BaseURL, channel.Key)
}

type SubscriptionResponse struct {
	Object             string  `json:"object"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	AccessUntil        int64   `json:"access_until"`
}

type UsageResponse struct {
	Object string `json:"object"`
	// DailyCosts []OpenAIUsageDailyCost `json:"daily_costs"`
	TotalUsage float64 `json:"total_usage"` // unit: 0.01 dollar
}

func GetBalance(baseURL, key string) (float64, error) {
	u := baseURL
	if u == "" {
		u = baseURL
	}
	url := u + "/v1/dashboard/billing/subscription"

	req1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req1.Header.Set("Authorization", "Bearer "+key)
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		return 0, err
	}
	defer res1.Body.Close()
	subscription := SubscriptionResponse{}
	err = sonic.ConfigDefault.NewDecoder(res1.Body).Decode(&subscription)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	startDate := now.Format("2006-01") + "-01"
	endDate := now.Format(time.DateOnly)
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format(time.DateOnly)
	}
	url = u + "/v1/dashboard/billing/usage?start_date=" + startDate + "&end_date=" + endDate
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req2.Header.Set("Authorization", "Bearer "+key)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return 0, err
	}
	defer res2.Body.Close()

	usage := UsageResponse{}
	err = sonic.ConfigDefault.NewDecoder(res2.Body).Decode(&usage)
	if err != nil {
		return 0, err
	}
	balance := subscription.HardLimitUSD - usage.TotalUsage/100
	return balance, nil
}
