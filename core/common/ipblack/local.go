package ipblack

import (
	"time"

	gcache "github.com/patrickmn/go-cache"
)

const (
	ipBlackLocalTTL     = time.Second
	ipBlackLocalMissTTL = 500 * time.Millisecond
)

var ipBlackLocalCache = gcache.New(2*time.Second, 5*time.Second)

func cacheSetIPBlackLocal(ip string, blocked bool) {
	ttl := ipBlackLocalTTL
	if !blocked {
		ttl = ipBlackLocalMissTTL
	}

	ipBlackLocalCache.Set(ip, blocked, ttl)
}

func cacheGetIPBlackLocal(ip string) (bool, bool) {
	v, ok := ipBlackLocalCache.Get(ip)
	if !ok {
		return false, false
	}

	blocked, ok := v.(bool)
	if !ok {
		panic("ip black local cache type mismatch")
	}

	return blocked, true
}
