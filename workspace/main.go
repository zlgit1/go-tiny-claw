package main

import (
	"fmt"
	"sync"
)

func main() {
	// 全局计数器
	var count int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 启动 1000 个 Goroutine 去并发累加
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			count++
			mu.Unlock()
		}()
	}

	wg.Wait()
	fmt.Printf("最终的 Count 是: %d\n", count)
}
