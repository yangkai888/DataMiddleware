package utils

import (
	"reflect"
	"sync"
	"unsafe"

	"datamiddleware/internal/infrastructure/logging"
)

// BufferPool 字节缓冲区对象池
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool 创建字节缓冲区池
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// 默认创建4KB缓冲区
				buf := make([]byte, 4096)
				return &buf
			},
		},
	}
}

// Get 获取缓冲区
func (p *BufferPool) Get() []byte {
	buf := p.pool.Get().(*[]byte)
	return (*buf)[:0] // 重置长度为0，但保持容量
}

// Put 归还缓冲区
func (p *BufferPool) Put(buf []byte) {
	if cap(buf) >= 4096 { // 只回收足够大的缓冲区
		p.pool.Put(&buf)
	}
}

// MessagePool 消息对象池
type MessagePool struct {
	pool sync.Pool
}

// Message 消息结构
type Message struct {
	ID       string
	Type     string
	Payload  []byte
	Metadata map[string]interface{}
}

// NewMessagePool 创建消息池
func NewMessagePool() *MessagePool {
	return &MessagePool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Message{
					Metadata: make(map[string]interface{}, 8),
				}
			},
		},
	}
}

// Get 获取消息对象
func (p *MessagePool) Get() *Message {
	msg := p.pool.Get().(*Message)
	// 重置消息状态
	msg.ID = ""
	msg.Type = ""
	msg.Payload = msg.Payload[:0]
	// 清空metadata但保持容量
	for k := range msg.Metadata {
		delete(msg.Metadata, k)
	}
	return msg
}

// Put 归还消息对象
func (p *MessagePool) Put(msg *Message) {
	p.pool.Put(msg)
}

// RingBuffer 环形缓冲区 - 零拷贝实现
type RingBuffer struct {
	buffer []byte
	size   int
	read   int
	write  int
	mu     sync.RWMutex
}

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]byte, size),
		size:   size,
	}
}

// Write 写入数据
func (rb *RingBuffer) Write(data []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	n := len(data)
	available := rb.availableSpace()

	if n > available {
		n = available
	}

	if n == 0 {
		return 0, nil
	}

	// 计算写入位置
	end := rb.write + n
	if end <= rb.size {
		copy(rb.buffer[rb.write:end], data[:n])
	} else {
		// 分两段写入
		first := rb.size - rb.write
		copy(rb.buffer[rb.write:], data[:first])
		copy(rb.buffer[:end-rb.size], data[first:n])
	}

	rb.write = (rb.write + n) % rb.size
	return n, nil
}

// Read 读取数据到指定缓冲区
func (rb *RingBuffer) Read(buf []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	n := len(buf)
	available := rb.availableData()

	if n > available {
		n = available
	}

	if n == 0 {
		return 0, nil
	}

	// 计算读取位置
	end := rb.read + n
	if end <= rb.size {
		copy(buf[:n], rb.buffer[rb.read:end])
	} else {
		// 分两段读取
		first := rb.size - rb.read
		copy(buf[:first], rb.buffer[rb.read:])
		copy(buf[first:n], rb.buffer[:end-rb.size])
	}

	rb.read = (rb.read + n) % rb.size
	return n, nil
}

// availableData 可读数据量
func (rb *RingBuffer) availableData() int {
	if rb.write >= rb.read {
		return rb.write - rb.read
	}
	return rb.size - rb.read + rb.write
}

// availableSpace 可写空间
func (rb *RingBuffer) availableSpace() int {
	return rb.size - rb.availableData() - 1
}

// Size 返回缓冲区大小
func (rb *RingBuffer) Size() int {
	return rb.size
}

// MemoryManager 内存管理器
type MemoryManager struct {
	bufferPool  *BufferPool
	messagePool *MessagePool
	logger      logger.Logger

	// 统计信息
	allocatedBuffers  int64
	allocatedMessages int64
	mu                sync.RWMutex
}

// NewMemoryManager 创建内存管理器
func NewMemoryManager(logger logger.Logger) *MemoryManager {
	return &MemoryManager{
		bufferPool:  NewBufferPool(),
		messagePool: NewMessagePool(),
		logger:      logger,
	}
}

// GetBuffer 获取缓冲区
func (mm *MemoryManager) GetBuffer() []byte {
	mm.mu.Lock()
	mm.allocatedBuffers++
	mm.mu.Unlock()

	return mm.bufferPool.Get()
}

// PutBuffer 归还缓冲区
func (mm *MemoryManager) PutBuffer(buf []byte) {
	mm.mu.Lock()
	if mm.allocatedBuffers > 0 {
		mm.allocatedBuffers--
	}
	mm.mu.Unlock()

	mm.bufferPool.Put(buf)
}

// GetMessage 获取消息对象
func (mm *MemoryManager) GetMessage() *Message {
	mm.mu.Lock()
	mm.allocatedMessages++
	mm.mu.Unlock()

	return mm.messagePool.Get()
}

// PutMessage 归还消息对象
func (mm *MemoryManager) PutMessage(msg *Message) {
	mm.mu.Lock()
	if mm.allocatedMessages > 0 {
		mm.allocatedMessages--
	}
	mm.mu.Unlock()

	mm.messagePool.Put(msg)
}

// GetStats 获取内存统计信息
func (mm *MemoryManager) GetStats() MemoryStats {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	return MemoryStats{
		AllocatedBuffers:  mm.allocatedBuffers,
		AllocatedMessages: mm.allocatedMessages,
	}
}

// MemoryStats 内存统计信息
type MemoryStats struct {
	AllocatedBuffers  int64 `json:"allocated_buffers"`  // 已分配的缓冲区数量
	AllocatedMessages int64 `json:"allocated_messages"` // 已分配的消息对象数量
}

// ZeroCopySlice 零拷贝切片操作
// 使用unsafe.Pointer避免不必要的内存拷贝
func ZeroCopySlice(data []byte, offset, length int) []byte {
	if offset < 0 || length < 0 || offset+length > len(data) {
		return nil
	}

	// 使用unsafe.Pointer进行零拷贝切片
	header := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	header.Data += uintptr(offset)
	header.Len = length
	header.Cap = cap(data) - offset // 修正容量计算

	// 返回新的切片引用
	return *(*[]byte)(unsafe.Pointer(header))
}

// CopyFreeConcat 零拷贝字符串连接
// 对于大量小字符串连接，使用[]byte和append避免多次内存分配
func CopyFreeConcat(strs ...string) string {
	if len(strs) == 0 {
		return ""
	}

	// 预计算总长度
	totalLen := 0
	for _, s := range strs {
		totalLen += len(s)
	}

	// 一次性分配足够的空间
	buf := make([]byte, totalLen)
	offset := 0

	// 依次拷贝
	for _, s := range strs {
		copy(buf[offset:], s)
		offset += len(s)
	}

	return string(buf)
}

// BulkAllocator 批量分配器 - 减少GC压力
type BulkAllocator struct {
	buffers [][]byte
	size    int
	count   int
	index   int
	mu      sync.Mutex
}

// NewBulkAllocator 创建批量分配器
func NewBulkAllocator(bufferSize, count int) *BulkAllocator {
	buffers := make([][]byte, count)
	for i := range buffers {
		buffers[i] = make([]byte, bufferSize)
	}

	return &BulkAllocator{
		buffers: buffers,
		size:    bufferSize,
		count:   count,
		index:   0,
	}
}

// Allocate 分配缓冲区
func (ba *BulkAllocator) Allocate() []byte {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if ba.index >= ba.count {
		// 扩展缓冲区
		newBuffers := make([][]byte, ba.count)
		for i := range newBuffers {
			newBuffers[i] = make([]byte, ba.size)
		}
		ba.buffers = append(ba.buffers, newBuffers...)
		ba.count += ba.count
	}

	buf := ba.buffers[ba.index]
	ba.index++

	// 返回重置长度的切片
	return buf[:0]
}

// Reset 重置分配器
func (ba *BulkAllocator) Reset() {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	ba.index = 0
}

// Stats 获取统计信息
func (ba *BulkAllocator) Stats() BulkStats {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	return BulkStats{
		TotalBuffers: ba.count,
		UsedBuffers:  ba.index,
		BufferSize:   ba.size,
	}
}

// BulkStats 批量分配统计
type BulkStats struct {
	TotalBuffers int `json:"total_buffers"`
	UsedBuffers  int `json:"used_buffers"`
	BufferSize   int `json:"buffer_size"`
}
