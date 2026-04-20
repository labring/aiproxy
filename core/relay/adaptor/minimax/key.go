package minimax

import (
	"errors"
	"strings"

	"github.com/labring/aiproxy/core/relay/adaptor"
)

var _ adaptor.KeyValidator = (*Adaptor)(nil)

func (a *Adaptor) ValidateKey(key string) error {
	_, _, err := GetAPIKeyAndGroupID(key)
	if err != nil {
		return err
	}

	return nil
}

func GetAPIKeyAndGroupID(key string) (string, string, error) {
	if key == "" {
		return "", "", errors.New("invalid key format")
	}

	keys := strings.Split(key, "|")
	switch len(keys) {
	case 1:
		if keys[0] == "" {
			return "", "", errors.New("invalid key format")
		}

		return keys[0], "", nil
	case 2:
		if keys[0] == "" {
			return "", "", errors.New("invalid key format")
		}

		return keys[0], keys[1], nil
	default:
		return "", "", errors.New("invalid key format")
	}
}
