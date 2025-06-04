package aws

import (
	"github.com/labring/aiproxy/core/relay/adaptor"
	"github.com/labring/aiproxy/core/relay/adaptor/aws/utils"
)

var _ adaptor.KeyValidator = (*Adaptor)(nil)

func (a *Adaptor) ValidateKey(key string) error {
	_, err := utils.GetAwsConfigFromKey(key)
	if err != nil {
		return err
	}
	return nil
}
