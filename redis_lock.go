package redisLock

import (
	"context"
	"github.com/google/uuid"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
)

const defaultExpiration = 30 * time.Second
const defaultRetryInternal = 200 * time.Millisecond
const dynamicRetryInternal = 100 * time.Millisecond

type RLocker struct {
	rdb    *redis.Client
	key    string
	value  string
	expiry time.Duration
	stop   chan struct{}
	r      *rand.Rand
}

func New(rdb *redis.Client, key string) *RLocker {
	return &RLocker{
		rdb:    rdb,
		key:    key,
		value:  uuid.New().String(),
		stop:   make(chan struct{}, 1),
		expiry: defaultExpiration,
		r:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (rl *RLocker) SetExpiration(expiry time.Duration) {
	if expiry > 0 {
		rl.expiry = expiry
	}
}

func (rl *RLocker) Lock(ctx context.Context) (ok bool, err error) {
	for {
		if ok, err = rl.tryLock(ctx); ok {
			go rl.tryExpand(ctx)
			break
		} else if err != nil {
			break
		} else {
			time.Sleep(rl.randInternal())
		}
	}
	return
}

func (rl *RLocker) TryLock(ctx context.Context, retry int) (ok bool, err error) {
	for ; retry >= 0; retry-- {
		if ok, err = rl.tryLock(ctx); ok {
			go rl.tryExpand(ctx)
			break
		} else if err != nil {
			break
		} else {
			time.Sleep(rl.randInternal())
		}
	}
	return
}

func (rl *RLocker) Unlock(ctx context.Context) {
	script := `
				if (redis.call('exists', KEYS[1]) == 1)
				then
					local value = redis.call('get', KEYS[1])
					if (value == ARGV[1])
					then
						return redis.call('del', KEYS[1])
					end
				end
				return 0`
	rl.stop <- struct{}{}
	rl.rdb.Eval(ctx, script, []string{rl.key}, rl.value)
}

func (rl *RLocker) Expand(ctx context.Context) (bool, error) {
	script := `
				if (redis.call('exists', KEYS[1]) == 1)
				then
					if (redis.call('get', KEYS[1]) == ARGV[1])
					then
						redis.call('pexpire', KEYS[1], ARGV[2])
						return 1
					end
				end
				return 0`
	return rl.rdb.Eval(ctx, script, []string{rl.key}, rl.value, int64(rl.expiry)).Bool()
}

func (rl *RLocker) tryLock(ctx context.Context) (bool, error) {
	script := `
				if (redis.call('exists', KEYS[1]) == 1) then
					return 0
				end
				redis.call('set', KEYS[1], ARGV[1])
				redis.call('pexpire', KEYS[1], ARGV[2])
				return 1`

	return rl.rdb.Eval(ctx, script, []string{rl.key}, rl.value, int64(rl.expiry)).Bool()
}

func (rl *RLocker) tryExpand(ctx context.Context) {
	for {
		select {
		case <-rl.stop:
			return
		case <-time.After(rl.expiry / 3):
			ok, err := rl.Expand(ctx)
			if err != nil {
				break
			} else if !ok {
				break
			}
		}
	}
}

func (rl *RLocker) randInternal() time.Duration {
	num := rl.r.Int63() % int64(dynamicRetryInternal)
	return defaultRetryInternal + time.Duration(num)
}
