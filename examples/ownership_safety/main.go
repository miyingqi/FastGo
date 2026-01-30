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

	// 测试所有权检查功能
	app.Router().GET("/ownership/test", func(c *FastGo.Context) {
		// 这个会在正确的协程中执行
		c.SendString(200, "Correct owner - request processed successfully\n")
	})

	// 测试并发安全的错误处理
	app.Router().GET("/concurrent/error", func(c *FastGo.Context) {
		var wg sync.WaitGroup

		// 尝试在错误的协程中写入（应该被拦截）
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				// 这些写入应该被所有权检查拦截
				message := fmt.Sprintf("Unauthorized write attempt from goroutine %d\n", id)
				c.SendString(200, message) // 这会触发所有权警告
			}(i)
		}

		// 正确的响应
		c.SendString(200, "Main handler processed correctly\n")
		wg.Wait()
	})

	// 测试 panic 恢复
	app.Router().GET("/panic/recovery", func(c *FastGo.Context) {
		// 模拟处理器中的 panic
		panic("simulated panic in handler")
	})

	// 性能测试端点
	app.Router().GET("/performance/test", func(c *FastGo.Context) {
		startTime := time.Now()

		// 模拟一些处理工作
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

		processingTime := time.Since(startTime)
		response := fmt.Sprintf("Processed in %v by goroutine owner %d\n",
			processingTime, c.GetOwner()) // 假设我们添加了 GetOwner 方法

		c.SendString(200, response)
	})

	fmt.Println("Enhanced FastGo Server starting on :8080")
	fmt.Println("Test endpoints:")
	fmt.Println("  GET /ownership/test       - Ownership checking test")
	fmt.Println("  GET /concurrent/error     - Concurrent error handling")
	fmt.Println("  GET /panic/recovery       - Panic recovery test")
	fmt.Println("  GET /performance/test     - Performance test")
	fmt.Println("")
	fmt.Println("Monitoring logs will show Context usage statistics every 10 seconds")

	app.Run(":8080")
}
