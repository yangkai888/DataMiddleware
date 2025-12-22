package utils

import (
	"sync"
	"time"

	"datamiddleware/internal/infrastructure/logging"
)

// PoolConfig 对象池配置
type PoolConfig struct {
	// 池的最大容量，0表示无限制
	MaxSize int
	// 池的初始大小
	InitSize int
	// 对象最大空闲时间，超过此时间未使用将被清理
	MaxIdleTime time.Duration
	// 清理间隔
	CleanupInterval time.Duration
}

// DefaultPoolConfig 默认对象池配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxSize:         1000,
		InitSize:        10,
		MaxIdleTime:     5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
}

// ObjectFactory 对象工厂接口
type ObjectFactory[T any] interface {
	// Create 创建新对象
	Create() (T, error)
	// Destroy 销毁对象
	Destroy(T) error
	// Reset 重置对象状态
	Reset(T) error
}

// SimpleFactory 简单对象工厂
type SimpleFactory[T any] struct {
	createFunc  func() (T, error)
	destroyFunc func(T) error
	resetFunc   func(T) error
}

// NewSimpleFactory 创建简单工厂
func NewSimpleFactory[T any](
	createFunc func() (T, error),
	destroyFunc func(T) error,
	resetFunc func(T) error,
) *SimpleFactory[T] {
	return &SimpleFactory[T]{
		createFunc:  createFunc,
		destroyFunc: destroyFunc,
		resetFunc:   resetFunc,
	}
}

// Create 创建对象
func (f *SimpleFactory[T]) Create() (T, error) {
	if f.createFunc != nil {
		return f.createFunc()
	}
	var zero T
	return zero, nil
}

// Destroy 销毁对象
func (f *SimpleFactory[T]) Destroy(obj T) error {
	if f.destroyFunc != nil {
		return f.destroyFunc(obj)
	}
	return nil
}

// Reset 重置对象
func (f *SimpleFactory[T]) Reset(obj T) error {
	if f.resetFunc != nil {
		return f.resetFunc(obj)
	}
	return nil
}

// PoolItem 池中对象的包装
type PoolItem[T any] struct {
	Object    T
	CreatedAt time.Time
	LastUsed  time.Time
	InUse     bool
}

// ObjectPool 通用对象池
type ObjectPool[T any] struct {
	config     PoolConfig
	factory    ObjectFactory[T]
	mu         sync.RWMutex
	items      []*PoolItem[T]
	created    int // 已创建的对象数量
	inUse      int // 正在使用的对象数量
	logger     logger.Logger
	stopChan   chan struct{}
	stopped    bool
}

// NewObjectPool 创建对象池
func NewObjectPool[T any](config PoolConfig, factory ObjectFactory[T], log logger.Logger) *ObjectPool[T] {
	pool := &ObjectPool[T]{
		config:   config,
		factory:  factory,
		items:    make([]*PoolItem[T], 0, config.MaxSize),
		logger:   log,
		stopChan: make(chan struct{}),
	}

	// 初始化对象
	pool.initPool()

	// 启动清理协程
	if config.CleanupInterval > 0 {
		go pool.cleanupLoop()
	}

	return pool
}

// initPool 初始化对象池
func (p *ObjectPool[T]) initPool() {
	for i := 0; i < p.config.InitSize; i++ {
		if err := p.createItem(); err != nil {
			p.logger.Error("初始化对象池失败", "error", err)
			break
		}
	}
}

// createItem 创建新对象并添加到池中
func (p *ObjectPool[T]) createItem() error {
	obj, err := p.factory.Create()
	if err != nil {
		return err
	}

	item := &PoolItem[T]{
		Object:    obj,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		InUse:     false,
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.items = append(p.items, item)
	p.created++

	return nil
}

// Get 获取对象
func (p *ObjectPool[T]) Get() (T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 查找空闲对象
	for _, item := range p.items {
		if !item.InUse {
			item.InUse = true
			item.LastUsed = time.Now()
			p.inUse++
			return item.Object, nil
		}
	}

	// 如果池未满，创建新对象
	if p.config.MaxSize == 0 || p.created < p.config.MaxSize {
		if err := p.createItem(); err != nil {
			var zero T
			return zero, err
		}
		// 获取刚创建的对象
		lastItem := p.items[len(p.items)-1]
		lastItem.InUse = true
		lastItem.LastUsed = time.Now()
		p.inUse++
		return lastItem.Object, nil
	}

	// 池已满，返回零值
	var zero T
	return zero, ErrPoolExhausted
}

// Put 归还对象到池中
func (p *ObjectPool[T]) Put(obj T) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 查找对应的对象
	for _, item := range p.items {
		if item.InUse && p.objectsEqual(item.Object, obj) {
			item.InUse = false
			item.LastUsed = time.Now()
			p.inUse--

			// 重置对象状态
			return p.factory.Reset(item.Object)
		}
	}

	// 对象不属于此池，直接销毁
	return p.factory.Destroy(obj)
}

// objectsEqual 比较两个对象是否相等
// 注意：这是一个简化的实现，对于复杂对象可能需要自定义比较逻辑
func (p *ObjectPool[T]) objectsEqual(a, b T) bool {
	// 对于指针类型，比较指针值
	// 对于值类型，这个方法可能不够准确
	return any(a) == any(b)
}

// Stats 获取池的统计信息
func (p *ObjectPool[T]) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PoolStats{
		Created:     p.created,
		InUse:       p.inUse,
		Available:   len(p.items) - p.inUse,
		MaxSize:     p.config.MaxSize,
	}
}

// PoolStats 池统计信息
type PoolStats struct {
	Created   int // 已创建的对象总数
	InUse     int // 正在使用的对象数量
	Available int // 可用的对象数量
	MaxSize   int // 池的最大容量
}

// cleanupLoop 清理循环
func (p *ObjectPool[T]) cleanupLoop() {
	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanup()
		case <-p.stopChan:
			return
		}
	}
}

// cleanup 清理过期对象
func (p *ObjectPool[T]) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var activeItems []*PoolItem[T]

	for _, item := range p.items {
		// 只清理不在使用的对象
		if !item.InUse {
			// 检查是否过期
			if now.Sub(item.LastUsed) > p.config.MaxIdleTime {
				// 销毁过期对象
				if err := p.factory.Destroy(item.Object); err != nil {
					p.logger.Error("销毁过期对象失败", "error", err)
				}
				p.created--
				continue
			}
		}
		activeItems = append(activeItems, item)
	}

	p.items = activeItems
}

// Close 关闭对象池
func (p *ObjectPool[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return nil
	}

	p.stopped = true
	close(p.stopChan)

	// 销毁所有对象
	for _, item := range p.items {
		if err := p.factory.Destroy(item.Object); err != nil {
			p.logger.Error("关闭对象池时销毁对象失败", "error", err)
		}
	}

	p.items = nil
	p.created = 0
	p.inUse = 0

	return nil
}
