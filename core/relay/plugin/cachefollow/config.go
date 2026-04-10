package cachefollow

import "time"

const PluginName = "cachefollow"

type Config struct {
	Enable bool `json:"enable"`

	FollowedChannelTTLSeconds          int64 `json:"followed_channel_ttl_seconds,omitempty"`
	RecentChannelUpdateDebounceSeconds int64 `json:"recent_channel_update_debounce_seconds,omitempty"`
}

func (c Config) GetFollowedChannelTTL() time.Duration {
	if c.FollowedChannelTTLSeconds > 0 {
		return time.Duration(c.FollowedChannelTTLSeconds) * time.Second
	}

	return defaultFollowedChannelTTL
}

func (c Config) GetRecentChannelUpdateDebounce() time.Duration {
	if c.RecentChannelUpdateDebounceSeconds > 0 {
		return time.Duration(c.RecentChannelUpdateDebounceSeconds) * time.Second
	}

	return defaultRecentChannelUpdateDebounce
}
