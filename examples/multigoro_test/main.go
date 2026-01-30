package main

import (
	"FastGo"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func main() {
	app := FastGo.NewFastGo()

	// 测试多goroutine处理
	app.Router().GET("/test/concurrent", func(c *FastGo.Context) {
		// 模拟耗时操作，验证多goroutine处理能力
		var wg sync.WaitGroup
		startTime := time.Now()

		// 启动多个并发任务
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// 模拟不同耗时的操作
				sleepTime := time.Duration(rand.Intn(200)+50) * time.Millisecond
				time.Sleep(sleepTime)

				// 并发安全写入响应
				message := fmt.Sprintf("Task %d completed in %v\n", id, sleepTime)
				c.ConcurrentWrite([]byte(message))
			}(i)
		}

		// 等待所有任务完成
		wg.Wait()

		// 添加总耗时信息
		totalTime := time.Since(startTime)
		summary := fmt.Sprintf("\nTotal processing time: %v\n", totalTime)
		c.ConcurrentWrite([]byte(summary))
	})

	// 测试传统的单goroutine处理进行对比
	app.Router().GET("/test/sequential", func(c *FastGo.Context) {
		startTime := time.Now()

		// 串行执行任务
		for i := 0; i < 10; i++ {
			sleepTime := time.Duration(rand.Intn(200)+50) * time.Millisecond
			time.Sleep(sleepTime)

			message := fmt.Sprintf("Task %d completed in %v\n", i, sleepTime)
			c.SendString(200, message)
		}

		totalTime := time.Since(startTime)
		summary := fmt.Sprintf("\nTotal processing time: %v\n", totalTime)
		c.SendString(200, summary)
	})

	fmt.Println("Server starting on :8080")
	fmt.Println("Test endpoints:")
	fmt.Println("  GET /test/concurrent  - Multi-goroutine processing")
	fmt.Println("  GET /test/sequential  - Single-goroutine processing (for comparison)")

	app.Run(":8080")
}
