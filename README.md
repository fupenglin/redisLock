redisLock
=========

基于Redis的分布式锁

### Installation

    go get -u "github.com/fupenglin/redisLock"

### Example

```go
lock := redisLock.New(rdb, "key")
if ok, _ := lock.Lock(ctx); !ok {
	// lock fail
	return
} 
defer lock.Unlock()
// Do something with the user.
```
