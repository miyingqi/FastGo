package FastGo

import (
	"sync/atomic"
	"unsafe"
)

// HighPerformanceFIFO 高性能FIFO队列 - 严格的先进先出保证
type HighPerformanceFIFO struct {
	buffer   []interface{}
	head     int64
	tail     int64
	capacity int64
	mask     int64
	closed   int32
}

// NewHighPerformanceFIFO 创建新的高性能FIFO队列
func NewHighPerformanceFIFO(capacity int) *HighPerformanceFIFO {
	if capacity <= 0 {
		panic("capacity must be positive")
	}

	// 找到大于等于capacity的最小2的幂次，优化位运算
	size := 1
	for size < capacity {
		size <<= 1
	}

	return &HighPerformanceFIFO{
		buffer:   make([]interface{}, size),
		capacity: int64(size),
		mask:     int64(size - 1),
	}
}

// Enqueue 入队操作 - 严格保证FIFO顺序
func (fifo *HighPerformanceFIFO) Enqueue(item interface{}) bool {
	// 原子检查队列是否关闭
	if atomic.LoadInt32(&fifo.closed) == 1 {
		return false
	}

	tail := atomic.LoadInt64(&fifo.tail)
	head := atomic.LoadInt64(&fifo.head)

	// 检查队列是否已满 (FIFO特性：头尾差等于容量)
	if tail-head >= fifo.capacity {
		return false
	}

	// CAS操作确保多goroutine环境下的线程安全
	if atomic.CompareAndSwapInt64(&fifo.tail, tail, tail+1) {
		// 使用位运算快速计算索引位置
		fifo.buffer[tail&fifo.mask] = item
		return true
	}

	return false
}

// Dequeue 出队操作 - 严格按照FIFO顺序返回最早入队的元素
func (fifo *HighPerformanceFIFO) Dequeue() (interface{}, bool) {
	// 原子检查队列状态
	if atomic.LoadInt32(&fifo.closed) == 1 {
		head := atomic.LoadInt64(&fifo.head)
		tail := atomic.LoadInt64(&fifo.tail)
		if head >= tail {
			return nil, false
		}
	}

	head := atomic.LoadInt64(&fifo.head)
	tail := atomic.LoadInt64(&fifo.tail)

	// 检查队列是否为空
	if head >= tail {
		return nil, false
	}

	// CAS操作确保线程安全的出队
	if atomic.CompareAndSwapInt64(&fifo.head, head, head+1) {
		index := head & fifo.mask
		item := fifo.buffer[index]
		fifo.buffer[index] = nil // 清理引用避免内存泄漏
		return item, true
	}

	return nil, false
}

// TryEnqueue 非阻塞入队尝试
func (fifo *HighPerformanceFIFO) TryEnqueue(item interface{}) bool {
	return fifo.Enqueue(item)
}

// TryDequeue 非阻塞出队尝试
func (fifo *HighPerformanceFIFO) TryDequeue() (interface{}, bool) {
	return fifo.Dequeue()
}

// Close 关闭队列，阻止后续入队操作
func (fifo *HighPerformanceFIFO) Close() {
	atomic.StoreInt32(&fifo.closed, 1)
}

// IsClosed 检查队列是否已关闭
func (fifo *HighPerformanceFIFO) IsClosed() bool {
	return atomic.LoadInt32(&fifo.closed) == 1
}

// IsEmpty 检查队列是否为空
func (fifo *HighPerformanceFIFO) IsEmpty() bool {
	return fifo.Size() <= 0
}

// IsFull 检查队列是否已满
func (fifo *HighPerformanceFIFO) IsFull() bool {
	return fifo.Size() >= fifo.capacity
}

// Size 获取当前队列中元素数量
func (fifo *HighPerformanceFIFO) Size() int64 {
	return atomic.LoadInt64(&fifo.tail) - atomic.LoadInt64(&fifo.head)
}

// Capacity 获取队列总容量
func (fifo *HighPerformanceFIFO) Capacity() int64 {
	return fifo.capacity
}

// Clear 清空队列中的所有元素
func (fifo *HighPerformanceFIFO) Clear() {
	atomic.StoreInt64(&fifo.head, 0)
	atomic.StoreInt64(&fifo.tail, 0)
	// 清理所有缓冲区引用防止内存泄漏
	for i := range fifo.buffer {
		fifo.buffer[i] = nil
	}
}

// WaitFreeQueue 完全无等待的FIFO队列实现
type WaitFreeQueue struct {
	head unsafe.Pointer
	tail unsafe.Pointer
}

type wfNode struct {
	value unsafe.Pointer
	next  unsafe.Pointer
}

// NewWaitFreeQueue 创建完全无等待的队列
func NewWaitFreeQueue() *WaitFreeQueue {
	node := &wfNode{}
	queue := &WaitFreeQueue{
		head: unsafe.Pointer(node),
		tail: unsafe.Pointer(node),
	}
	return queue
}

// Enqueue 无等待入队操作
func (q *WaitFreeQueue) Enqueue(item interface{}) {
	newNode := &wfNode{}
	atomic.StorePointer(&newNode.value, unsafe.Pointer(&item))

	for {
		tail := loadWFPointer(q.tail)
		next := loadWFPointer(atomic.LoadPointer(&tail.next))

		if tail == loadWFPointer(q.tail) {
			if next == nil {
				// 当前是真正的队尾，尝试插入新节点
				if atomic.CompareAndSwapPointer(&tail.next, unsafe.Pointer(next), unsafe.Pointer(newNode)) {
					// 插入成功，尝试更新tail指针（即使失败也不影响正确性）
					atomic.CompareAndSwapPointer(&q.tail, unsafe.Pointer(tail), unsafe.Pointer(newNode))
					return
				}
			} else {
				// tail不是真正的尾部，帮助推进tail指针
				atomic.CompareAndSwapPointer(&q.tail, unsafe.Pointer(tail), unsafe.Pointer(next))
			}
		}
	}
}

// Dequeue 无等待出队操作
func (q *WaitFreeQueue) Dequeue() (interface{}, bool) {
	for {
		head := loadWFPointer(q.head)
		tail := loadWFPointer(q.tail)
		next := loadWFPointer(atomic.LoadPointer(&head.next))

		if head == loadWFPointer(q.head) {
			if head == tail {
				if next == nil {
					// 队列为空
					return nil, false
				}
				// 队列处于不一致状态，推进tail指针
				atomic.CompareAndSwapPointer(&q.tail, unsafe.Pointer(tail), unsafe.Pointer(next))
			} else {
				// 读取值并尝试移除节点
				valuePtr := atomic.LoadPointer(&next.value)
				if valuePtr != nil {
					value := *(*interface{})(valuePtr)
					if atomic.CompareAndSwapPointer(&q.head, unsafe.Pointer(head), unsafe.Pointer(next)) {
						return value, true
					}
				}
			}
		}
	}
}

// IsEmpty 检查无等待队列是否为空
func (q *WaitFreeQueue) IsEmpty() bool {
	head := loadWFPointer(q.head)
	tail := loadWFPointer(q.tail)
	next := loadWFPointer(atomic.LoadPointer(&head.next))
	return head == tail && next == nil
}

// loadWFPointer 安全加载指针的辅助函数
func loadWFPointer(ptr unsafe.Pointer) *wfNode {
	return (*wfNode)(ptr)
}

// LockFreeSPSCQueue 单生产者单消费者无锁队列
type LockFreeSPSCQueue struct {
	buffer   []interface{}
	read     int64
	write    int64
	capacity int64
	mask     int64
}

// NewLockFreeSPSCQueue 创建SPSC无锁队列
func NewLockFreeSPSCQueue(capacity int) *LockFreeSPSCQueue {
	if capacity <= 0 {
		panic("capacity must be positive")
	}

	size := 1
	for size < capacity {
		size <<= 1
	}

	return &LockFreeSPSCQueue{
		buffer:   make([]interface{}, size),
		capacity: int64(size),
		mask:     int64(size - 1),
	}
}

// Push 生产者入队（仅由单个goroutine调用）
func (q *LockFreeSPSCQueue) Push(item interface{}) bool {
	write := atomic.LoadInt64(&q.write)
	read := atomic.LoadInt64(&q.read)

	// 检查队列是否已满
	if write-read >= q.capacity {
		return false
	}

	// 直接写入，因为只有一个生产者
	q.buffer[write&q.mask] = item
	atomic.AddInt64(&q.write, 1)
	return true
}

// Pop 消费者出队（仅由单个goroutine调用）
func (q *LockFreeSPSCQueue) Pop() (interface{}, bool) {
	read := atomic.LoadInt64(&q.read)
	write := atomic.LoadInt64(&q.write)

	// 检查队列是否为空
	if read >= write {
		return nil, false
	}

	// 直接读取，因为只有一个消费者
	item := q.buffer[read&q.mask]
	q.buffer[read&q.mask] = nil
	atomic.AddInt64(&q.read, 1)
	return item, true
}

// Size 获取队列大小
func (q *LockFreeSPSCQueue) Size() int64 {
	return atomic.LoadInt64(&q.write) - atomic.LoadInt64(&q.read)
}

// IsEmpty 检查队列是否为空
func (q *LockFreeSPSCQueue) IsEmpty() bool {
	return q.Size() <= 0
}
