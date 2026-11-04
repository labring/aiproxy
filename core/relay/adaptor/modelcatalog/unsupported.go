package modelcatalog

import "github.com/labring/aiproxy/core/model"

const UnsupportedMessage = "model catalog is unsupported for this channel type"

var unsupportedTypes = map[model.ChannelType]struct{}{
	model.ChannelTypeAntLing:     {}, // The embedded OpenAI adaptor promotes its ListModels method.
	model.ChannelTypeBaidu:       {},
	model.ChannelTypeCoze:        {},
	model.ChannelTypeDoc2x:       {},
	model.ChannelTypeDoubao:      {},
	model.ChannelTypeDoubaoAudio: {},
	model.ChannelTypeLingyiwanwu: {},
	model.ChannelTypeAWS:         {},
	model.ChannelTypeFake:        {},
	model.ChannelTypeFakeError:   {},
	model.ChannelTypeVertexAI:    {},
}

func UnsupportedTypes() []model.ChannelType {
	types := make([]model.ChannelType, 0, len(unsupportedTypes))
	for channelType := range unsupportedTypes {
		types = append(types, channelType)
	}

	return types
}

func IsUnsupportedType(channelType model.ChannelType) bool {
	_, ok := unsupportedTypes[channelType]
	return ok
}
