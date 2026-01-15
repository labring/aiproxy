package trylock

import (
	"context"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common"
	log "github.com/sirupsen/logrus"
)

var memRecord = sync.Map{}

func init() {
	go cleanMemLock()
}

func cleanMemLock() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for now := range ticker.C {
		memRecord.Range(func(key, value any) bool {
			exp, ok := value.(time.Time)
			if !ok || now.After(exp) {
				memRecord.CompareAndDelete(key, value)
			}

			return true
		})
	}
}

func MemLock(key string, expiration time.Duration) bool {
	now := time.Now()
	newExpiration := now.Add(expiration)

	for {
		actual, loaded := memRecord.LoadOrStore(key, newExpiration)
		if !loaded {
			return true
		}

		oldExpiration, ok := actual.(time.Time)
		if !ok {
			memRecord.CompareAndDelete(key, actual)
			continue
		}

		if now.After(oldExpiration) {
			if memRecord.CompareAndSwap(key, actual, newExpiration) {
				return true
			}
			continue
		}

		return false
	}
}

func Lock(key string, expiration time.Duration) bool {
	if !common.RedisEnabled {
		return MemLock(key, expiration)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := common.RDB.SetNX(ctx, common.RedisKey(key), true, expiration).Result()
	if err != nil {
		if MemLock("lockerror", 5*time.Second) {
			log.Errorf("try notify error: %v", err)
		}
		return MemLock(key, expiration)
	}

	return result
}
