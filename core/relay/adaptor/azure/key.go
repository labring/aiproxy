package azure

import (
	"errors"
	"strings"

	"github.com/labring/aiproxy/core/relay/adaptor"
)

var _ adaptor.KeyValidator = (*Adaptor)(nil)

func (a *Adaptor) ValidateKey(key string) error {
	_, _, err := GetTokenAndAPIVersion(key)
	if err != nil {
		return err
	}
	return nil
}

const DefaultAPIVersion = "2025-04-01-preview"

func GetTokenAndAPIVersion(key string) (string, string, error) {
	split := strings.Split(key, "|")
	if len(split) == 1 {
		return key, DefaultAPIVersion, nil
	}
	if len(split) != 2 {
		return "", "", errors.New("invalid key format")
	}
	return split[0], split[1], nil
}
