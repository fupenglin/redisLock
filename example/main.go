package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/fupenglin/redisLock"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

var ctx = context.Background()

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
	})
	rdb.Set(ctx, "stock", 10000, 0)
	start := time.Now().UnixNano()
	wg := sync.WaitGroup{}
	count := 100
	wg.Add(count)
	for i := 0; i < count; i++ {
		go submit(&wg, rdb)
	}
	wg.Wait()
	end := time.Now().UnixNano()
	fmt.Println("time: ", (end-start)/1000000)
}

func submit(wg *sync.WaitGroup, rdb *redis.Client) {
	uuid := uuid.NewString()
	rlocker := redisLock.New(rdb, "stock_key", uuid)
	if ok, _ := rlocker.Lock(ctx); ok {
		defer rlocker.Unlock(ctx)
		key := "stock"
		stock, _ := strconv.Atoi(rdb.Get(ctx, key).Val())
		if stock > 0 {
			stock = stock - 1
			rdb.Set(ctx, key, stock, 0)
			fmt.Printf("扣减库存, stock: %d\n", stock)
		} else {
			fmt.Printf("库存不足\n")
		}
	} else {
		fmt.Println("加锁失败")
	}
	wg.Done()
}
