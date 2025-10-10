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

// region|adcJSON or region|apikey or region|project_id|apikey
func getConfigFromKey(key string) (Config, error) {
	region, gkey, ok := strings.Cut(key, "|")
	if !ok {
		return Config{}, errors.New("invalid key format")
	}

	if region == gkey {
		region = ""
	}

	if !strings.HasPrefix(gkey, "{") {
		projectid, ngkey, ok := strings.Cut(gkey, "|")
		if ok {
			// region|project_id|apikey
			if projectid == ngkey {
				projectid = ""
			}
			return Config{
				Region:    region,
				Key:       ngkey,
				ProjectID: projectid,
			}, nil
		}
		// region|apikey
		return Config{
			Region: region,
			Key:    gkey,
		}, nil
	}

	// region|adcJSON
	node, err := sonic.GetFromString(gkey, "project_id")
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
		ADCJSON:   gkey,
	}, nil
}
