package common

import "sync"

type KeyedLocker struct {
	mu    sync.Mutex
	locks map[string]*keyedLock
}

type keyedLock struct {
	mu   sync.Mutex
	refs int
}

func NewKeyedLocker() *KeyedLocker {
	return &KeyedLocker{
		locks: make(map[string]*keyedLock),
	}
}

func (k *KeyedLocker) Lock(key string) func() {
	k.mu.Lock()

	lock, ok := k.locks[key]
	if !ok {
		lock = &keyedLock{}
		k.locks[key] = lock
	}

	lock.refs++
	k.mu.Unlock()

	lock.mu.Lock()

	return func() {
		lock.mu.Unlock()

		k.mu.Lock()

		lock.refs--
		if lock.refs == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}

func LoadWithKeyLock[T any](
	locker *KeyedLocker,
	key string,
	getCached func() (T, bool),
	load func() (T, error),
) (T, error) {
	unlock := locker.Lock(key)
	defer unlock()

	if value, ok := getCached(); ok {
		return value, nil
	}

	return load()
}

func WithKeyLock(locker *KeyedLocker, key string, fn func()) {
	unlock := locker.Lock(key)
	defer unlock()

	fn()
}
