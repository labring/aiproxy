package baiduv2

import (
	"errors"
	"strings"

	"github.com/labring/aiproxy/core/relay/adaptor"
)

var _ adaptor.KeyValidator = (*Adaptor)(nil)

func (a *Adaptor) ValidateKey(key string) error {
	_, _, err := getAKAndSK(key)
	return err
}

// key格式: ak|sk
func getAKAndSK(key string) (string, string, error) {
	parts := strings.Split(key, "|")
	if len(parts) != 2 {
		return "", "", errors.New("invalid key format")
	}
	return parts[0], parts[1], nil
}
