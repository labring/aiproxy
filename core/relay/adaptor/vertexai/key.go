package vertexai

import (
	"errors"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

var _ adaptor.KeyValidator = (*Adaptor)(nil)

func (a *Adaptor) ValidateKey(key string) error {
	_, err := getConfigFromKey(key)
	if err != nil {
		return err
	}
	return nil
}

// region|adcJSON
func getConfigFromKey(key string) (Config, error) {
	region, adcJSON, ok := strings.Cut(key, "|")
	if !ok {
		return Config{}, errors.New("invalid key format")
	}
	node, err := sonic.GetFromString(adcJSON, "project_id")
	if err != nil {
		return Config{}, err
	}
	projectID, err := node.String()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Region:    region,
		ProjectID: projectID,
		ADCJSON:   adcJSON,
	}, nil
}
