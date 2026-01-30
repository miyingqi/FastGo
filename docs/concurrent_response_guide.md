# FastGo 并发响应处理指南

## 问题背景

在 Go 的 HTTP 服务中，`http.ResponseWriter` 不是线程安全的。当多个 goroutine 同时尝试写入同一个 ResponseWriter 时，会发生竞态条件，导致不可预测的行为。

## 解决方案

FastGo 提供了三种并发安全的响应处理方式：

### 1. 互斥锁保护的写入 (`ConcurrentWrite`)

最基本的方式，使用互斥锁保护 ResponseWriter 的访问。

```go
app.Router().GET("/concurrent/basic", func(c *FastGo.Context) {
    var wg sync.WaitGroup
    
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            message := fmt.Sprintf("Goroutine %d response\n", id)
            c.ConcurrentWrite([]byte(message))
        }(i)
    }
    
    wg.Wait()
})
```

### 2. 并发响应处理器 (`WithConcurrentResponse`)

更高级的抽象，自动管理并发写入和错误处理。

```go
app.Router().GET("/concurrent/advanced", func(c *FastGo.Context) {
    c.WithConcurrentResponse(func(cr *FastGo.ConcurrentResponse) {
        var wg sync.WaitGroup
        
        // 并行任务1
        wg.Add(1)
        go func() {
            defer wg.Done()
            // 模拟数据库查询
            time.Sleep(50 * time.Millisecond)
            cr.Write([]byte("Database result loaded\n"))
        }()
        
        // 并行任务2
        wg.Add(1)
        go func() {
            defer wg.Done()
            // 模拟API调用
            time.Sleep(80 * time.Millisecond)
            cr.Write([]byte("API service responded\n"))
        }()
        
        wg.Wait()
    })
})
```

### 3. 并行执行器 (`ParallelExecute`)

最简化的使用方式，直接传入处理函数。

```go
app.Router().GET("/parallel/execute", func(c *FastGo.Context) {
    handlers := []func() ([]byte, error){
        func() ([]byte, error) {
            time.Sleep(100 * time.Millisecond)
            return []byte("Task 1 completed\n"), nil
        },
        func() ([]byte, error) {
            time.Sleep(150 * time.Millisecond)
            return []byte("Task 2 completed\n"), nil
        },
    }
    
    c.ParallelExecute(handlers...)
})
```

## 核心特性

### 线程安全保障
- 所有方法都使用互斥锁保护 ResponseWriter 访问
- 双重检查机制防止竞态条件
- 自动处理并发写入冲突

### 错误处理
- 支持并发错误收集和报告
- 自动恢复 panic 避免服务崩溃
- 优雅的错误传播机制

### 性能优化
- 使用缓冲通道提高并发性能
- 对象池复用减少内存分配
- 非阻塞写入提升响应速度

## 使用建议

### 适用场景
1. **微服务聚合** - 并行调用多个下游服务
2. **数据聚合** - 同时从多个数据源获取数据
3. **实时流处理** - 流式传输实时生成的内容
4. **批量处理** - 并行处理批量请求

### 最佳实践
1. **优先使用高级API** - `WithConcurrentResponse` 和 `ParallelExecute`
2. **合理控制并发度** - 避免创建过多goroutine
3. **及时关闭资源** - 使用 defer 确保资源正确释放
4. **监控性能指标** - 关注并发处理的性能表现

### 注意事项
1. **响应顺序不确定** - 并发写入不保证响应顺序
2. **内存使用** - 大量并发可能增加内存消耗
3. **超时控制** - 建议设置合理的超时时间
4. **错误处理** - 及时处理和记录并发错误

## 性能对比

| 方法 | 吞吐量 | 延迟 | 内存使用 | 复杂度 |
|------|--------|------|----------|--------|
| 串行处理 | 低 | 高 | 低 | 简单 |
| ConcurrentWrite | 中 | 中 | 中 | 中等 |
| WithConcurrentResponse | 高 | 低 | 中 | 中等 |
| ParallelExecute | 最高 | 最低 | 高 | 简单 |

## 示例运行

```bash
cd examples/concurrent_response
go run main.go
```

访问以下端点测试不同功能：
- `http://localhost:8080/concurrent/basic` - 基本并发写入
- `http://localhost:8080/concurrent/advanced` - 高级并发处理
- `http://localhost:8080/parallel/execute` - 并行执行
- `http://localhost:8080/concurrent/error` - 错误处理
- `http://localhost:8080/stream/concurrent` - 流式响应

## 故障排除

### 常见问题

1. **响应不完整**
   - 检查是否所有goroutine都已完成
   - 确保正确使用WaitGroup等待

2. **竞态条件**
   - 使用 `-race` 标志检测竞态
   - 确保使用提供的并发安全方法

3. **性能问题**
   - 监控goroutine数量
   - 调整缓冲区大小
   - 优化处理逻辑

### 调试技巧

```go
// 启用竞态检测
go run -race main.go

// 添加详细日志
c.WithConcurrentResponse(func(cr *FastGo.ConcurrentResponse) {
    log.Printf("Starting concurrent response processing")
    // ... 处理逻辑
    log.Printf("Completed concurrent response processing")
})
```

通过这些并发安全的响应处理机制，FastGo 可以安全高效地处理多goroutine环境下的HTTP响应写入。