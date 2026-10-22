package modelcatalog

import "github.com/labring/aiproxy/core/model"

const UnsupportedMessage = "model catalog is unsupported for this channel type"

var unsupportedTypes = map[model.ChannelType]struct{}{
	model.ChannelTypeCoze:        {},
	model.ChannelTypeDoc2x:       {},
	model.ChannelTypeDoubaoAudio: {},
	model.ChannelTypeLingyiwanwu: {},
	model.ChannelTypeFake:        {},
	model.ChannelTypeFakeError:   {},
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
