package tiktoken

import (
	"errors"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/tiktoken-go/tokenizer"
)

// tokenEncoderMap won't grow after initialization
var (
	tokenEncoderMap     = map[string]tokenizer.Codec{}
	defaultTokenEncoder tokenizer.Codec
	tokenEncoderLock    sync.RWMutex
)

func init() {
	gpt4oTokenEncoder, err := tokenizer.ForModel(tokenizer.GPT4o)
	if err != nil {
		log.Fatal("failed to get gpt-4o token encoder: " + err.Error())
	}

	defaultTokenEncoder = gpt4oTokenEncoder
}

func GetTokenEncoder(model string) tokenizer.Codec {
	tokenEncoderLock.RLock()

	tokenEncoder, ok := tokenEncoderMap[model]

	tokenEncoderLock.RUnlock()

	if ok {
		return tokenEncoder
	}

	tokenEncoderLock.Lock()
	defer tokenEncoderLock.Unlock()

	if tokenEncoder, ok := tokenEncoderMap[model]; ok {
		return tokenEncoder
	}

	log.Info("loading encoding for model " + model)

	// ForModel has built-in prefix matching for model names
	tokenEncoder, err := tokenizer.ForModel(tokenizer.Model(model))
	if err != nil {
		if errors.Is(err, tokenizer.ErrModelNotSupported) {
			log.Warnf("model %s not supported, using default encoder (gpt-4o)", model)
			tokenEncoderMap[model] = defaultTokenEncoder
			return defaultTokenEncoder
		}

		log.Errorf(
			"failed to get token encoder for model %s: %v, using default encoder",
			model,
			err,
		)
		tokenEncoderMap[model] = defaultTokenEncoder

		return defaultTokenEncoder
	}

	log.Infof("loaded encoding for model %s: %s", model, tokenEncoder.GetName())

	tokenEncoderMap[model] = tokenEncoder

	return tokenEncoder
}
