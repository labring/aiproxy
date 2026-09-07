package cachefollow

import "time"

const PluginName = "cachefollow"

type Config struct {
	Enable bool `json:"enable"`

	EnableGenericFollow                bool  `json:"enable_generic_follow,omitempty"`
	FollowedChannelTTLSeconds          int64 `json:"followed_channel_ttl_seconds,omitempty"`
	RecentChannelUpdateDebounceSeconds int64 `json:"recent_channel_update_debounce_seconds,omitempty"`
}

func (c Config) GetFollowedChannelTTL() time.Duration {
	if c.FollowedChannelTTLSeconds > 0 {
		seconds := min(c.FollowedChannelTTLSeconds, int64(maxFollowedChannelTTL/time.Second))
		return time.Duration(seconds) * time.Second
	}

	return defaultFollowedChannelTTL
}

func (c Config) GetRecentChannelUpdateDebounce() time.Duration {
	if c.RecentChannelUpdateDebounceSeconds > 0 {
		return time.Duration(c.RecentChannelUpdateDebounceSeconds) * time.Second
	}

	return defaultRecentChannelUpdateDebounce
}
