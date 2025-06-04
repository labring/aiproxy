package controller

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/redis/go-redis/v9"
)

// Interface for multi-producer, single-consumer message passing
type mpsc interface {
	recv(ctx context.Context, id string) ([]byte, error)
	send(ctx context.Context, id string, data []byte) error
}

// Global MPSC instances
var (
	memMCPMpsc       mpsc = newChannelMCPMpsc()
	redisMCPMpsc     mpsc
	redisMCPMpscOnce = &sync.Once{}
)

func getMCPMpsc() mpsc {
	if common.RedisEnabled {
		redisMCPMpscOnce.Do(func() {
			redisMCPMpsc = newRedisMCPMPSC(common.RDB)
		})
		return redisMCPMpsc
	}
	return memMCPMpsc
}

// In-memory channel-based MPSC implementation
type channelMCPMpsc struct {
	channels     map[string]chan []byte
	lastAccess   map[string]time.Time
	channelMutex sync.RWMutex
}

// newChannelMCPMpsc creates a new channel-based mpsc implementation
func newChannelMCPMpsc() *channelMCPMpsc {
	c := &channelMCPMpsc{
		channels:   make(map[string]chan []byte),
		lastAccess: make(map[string]time.Time),
	}

	// Start a goroutine to clean up expired channels
	go c.cleanupExpiredChannels()

	return c
}

// cleanupExpiredChannels periodically checks for and removes channels that haven't been accessed in
// 15 seconds
func (c *channelMCPMpsc) cleanupExpiredChannels() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.channelMutex.Lock()
		now := time.Now()
		for id, lastAccess := range c.lastAccess {
			if now.Sub(lastAccess) > 15*time.Second {
				// Close and delete the channel
				if ch, exists := c.channels[id]; exists {
					close(ch)
					delete(c.channels, id)
				}
				delete(c.lastAccess, id)
			}
		}
		c.channelMutex.Unlock()
	}
}

// getOrCreateChannel gets an existing channel or creates a new one for the session
func (c *channelMCPMpsc) getOrCreateChannel(id string) chan []byte {
	c.channelMutex.RLock()
	ch, exists := c.channels[id]
	c.channelMutex.RUnlock()

	if !exists {
		c.channelMutex.Lock()
		if ch, exists = c.channels[id]; !exists {
			ch = make(chan []byte, 10)
			c.channels[id] = ch
		}
		c.lastAccess[id] = time.Now()
		c.channelMutex.Unlock()
	} else {
		c.channelMutex.Lock()
		c.lastAccess[id] = time.Now()
		c.channelMutex.Unlock()
	}

	return ch
}

// recv receives data for the specified session
func (c *channelMCPMpsc) recv(ctx context.Context, id string) ([]byte, error) {
	ch := c.getOrCreateChannel(id)

	select {
	case data, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("channel closed for session %s", id)
		}
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// send sends data to the specified session
func (c *channelMCPMpsc) send(ctx context.Context, id string, data []byte) error {
	ch := c.getOrCreateChannel(id)

	select {
	case ch <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("channel buffer full for session %s", id)
	}
}

// Redis-based MPSC implementation
type redisMCPMPSC struct {
	rdb *redis.Client
}

// newRedisMCPMPSC creates a new Redis MPSC instance
func newRedisMCPMPSC(rdb *redis.Client) *redisMCPMPSC {
	return &redisMCPMPSC{rdb: rdb}
}

func (r *redisMCPMPSC) send(ctx context.Context, id string, data []byte) error {
	// Set expiration to 15 seconds when sending data
	id = "mcp:mpsc:" + id
	pipe := r.rdb.Pipeline()
	pipe.LPush(ctx, id, data)
	pipe.Expire(ctx, id, 15*time.Second)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisMCPMPSC) recv(ctx context.Context, id string) ([]byte, error) {
	id = "mcp:mpsc:" + id
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			result, err := r.rdb.BRPop(ctx, time.Second, id).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					runtime.Gosched()
					continue
				}
				return nil, err
			}
			if len(result) != 2 {
				return nil, errors.New("invalid BRPop result")
			}
			return []byte(result[1]), nil
		}
	}
}
