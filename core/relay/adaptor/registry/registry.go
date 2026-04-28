package registry

import (
	"maps"
	"slices"
	"sync"

	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/relay/adaptor"
)

var (
	mu       sync.RWMutex
	adaptors = make(map[model.ChannelType]adaptor.Adaptor)
)

func Register(channelType model.ChannelType, a adaptor.Adaptor) {
	mu.Lock()
	defer mu.Unlock()

	adaptors[channelType] = a
}

func Get(channelType model.ChannelType) (adaptor.Adaptor, bool) {
	mu.RLock()
	defer mu.RUnlock()

	a, ok := adaptors[channelType]

	return a, ok
}

func Snapshot() map[model.ChannelType]adaptor.Adaptor {
	mu.RLock()
	defer mu.RUnlock()

	out := make(map[model.ChannelType]adaptor.Adaptor, len(adaptors))
	maps.Copy(out, adaptors)

	return out
}

func SortedTypes() []model.ChannelType {
	mu.RLock()
	defer mu.RUnlock()

	types := make([]model.ChannelType, 0, len(adaptors))
	for t := range adaptors {
		types = append(types, t)
	}

	slices.Sort(types)

	return types
}
