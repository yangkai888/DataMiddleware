package utils

import (
	"container/heap"
	"sync"
	"time"
)

// SafeMap 线程安全的Map
type SafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewSafeMap 创建线程安全的Map
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		m: make(map[K]V),
	}
}

// Set 设置值
func (sm *SafeMap[K, V]) Set(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

// Get 获取值
func (sm *SafeMap[K, V]) Get(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	value, exists := sm.m[key]
	return value, exists
}

// Delete 删除值
func (sm *SafeMap[K, V]) Delete(key K) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.m, key)
}

// Len 获取长度
func (sm *SafeMap[K, V]) Len() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.m)
}

// Keys 获取所有键
func (sm *SafeMap[K, V]) Keys() []K {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	keys := make([]K, 0, len(sm.m))
	for k := range sm.m {
		keys = append(keys, k)
	}
	return keys
}

// Values 获取所有值
func (sm *SafeMap[K, V]) Values() []V {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	values := make([]V, 0, len(sm.m))
	for _, v := range sm.m {
		values = append(values, v)
	}
	return values
}

// Clear 清空
func (sm *SafeMap[K, V]) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m = make(map[K]V)
}

// LRUCache LRU缓存
type LRUCache[K comparable, V any] struct {
	capacity int
	mu       sync.RWMutex
	cache    map[K]*lruItem[K, V]
	head     *lruItem[K, V] // 最近使用的
	tail     *lruItem[K, V] // 最少使用的
}

type lruItem[K comparable, V any] struct {
	key   K
	value V
	prev  *lruItem[K, V]
	next  *lruItem[K, V]
}

// NewLRUCache 创建LRU缓存
func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = 10
	}
	return &LRUCache[K, V]{
		capacity: capacity,
		cache:    make(map[K]*lruItem[K, V]),
	}
}

// Set 设置值
func (c *LRUCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.cache[key]; exists {
		// 更新值并移到头部
		item.value = value
		c.moveToHead(item)
		return
	}

	// 创建新项
	item := &lruItem[K, V]{
		key:   key,
		value: value,
	}
	c.cache[key] = item
	c.addToHead(item)

	// 检查容量
	if len(c.cache) > c.capacity {
		c.removeTail()
	}
}

// Get 获取值
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.cache[key]
	if !exists {
		var zero V
		return zero, false
	}

	// 移到头部
	c.moveToHead(item)
	return item.value, true
}

// Delete 删除值
func (c *LRUCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.cache[key]
	if !exists {
		return
	}

	c.removeItem(item)
	delete(c.cache, key)
}

// Len 获取长度
func (c *LRUCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Clear 清空缓存
func (c *LRUCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[K]*lruItem[K, V])
	c.head = nil
	c.tail = nil
}

func (c *LRUCache[K, V]) addToHead(item *lruItem[K, V]) {
	if c.head == nil {
		c.head = item
		c.tail = item
		return
	}

	item.next = c.head
	c.head.prev = item
	c.head = item
}

func (c *LRUCache[K, V]) moveToHead(item *lruItem[K, V]) {
	if item == c.head {
		return
	}

	c.removeItem(item)
	c.addToHead(item)
}

func (c *LRUCache[K, V]) removeItem(item *lruItem[K, V]) {
	if item.prev != nil {
		item.prev.next = item.next
	}
	if item.next != nil {
		item.next.prev = item.prev
	}
	if item == c.head {
		c.head = item.next
	}
	if item == c.tail {
		c.tail = item.prev
	}
	item.prev = nil
	item.next = nil
}

func (c *LRUCache[K, V]) removeTail() {
	if c.tail == nil {
		return
	}

	delete(c.cache, c.tail.key)
	c.removeItem(c.tail)
}

// PriorityQueueItem 优先级队列项
type PriorityQueueItem[T any] struct {
	Value    T
	Priority int
	Index    int // 在堆中的索引
}

// PriorityQueue 优先级队列
type PriorityQueue[T any] struct {
	items []*PriorityQueueItem[T]
	less  func(a, b *PriorityQueueItem[T]) bool // 比较函数，true表示a优先级高于b
}

// NewPriorityQueue 创建优先级队列
func NewPriorityQueue[T any](less func(a, b *PriorityQueueItem[T]) bool) *PriorityQueue[T] {
	return &PriorityQueue[T]{
		items: make([]*PriorityQueueItem[T], 0),
		less:  less,
	}
}

// Len 实现heap.Interface
func (pq *PriorityQueue[T]) Len() int { return len(pq.items) }

// Less 实现heap.Interface
func (pq *PriorityQueue[T]) Less(i, j int) bool {
	return pq.less(pq.items[i], pq.items[j])
}

// Swap 实现heap.Interface
func (pq *PriorityQueue[T]) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].Index = i
	pq.items[j].Index = j
}

// Push 实现heap.Interface
func (pq *PriorityQueue[T]) Push(x interface{}) {
	item := x.(*PriorityQueueItem[T])
	item.Index = len(pq.items)
	pq.items = append(pq.items, item)
}

// Pop 实现heap.Interface
func (pq *PriorityQueue[T]) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.Index = -1
	pq.items = old[0 : n-1]
	return item
}

// PushItem 添加项
func (pq *PriorityQueue[T]) PushItem(value T, priority int) *PriorityQueueItem[T] {
	item := &PriorityQueueItem[T]{
		Value:    value,
		Priority: priority,
	}
	heap.Push(pq, item)
	return item
}

// PopItem 弹出最高优先级项
func (pq *PriorityQueue[T]) PopItem() (T, bool) {
	if pq.Len() == 0 {
		var zero T
		return zero, false
	}
	item := heap.Pop(pq).(*PriorityQueueItem[T])
	return item.Value, true
}

// Peek 查看最高优先级项
func (pq *PriorityQueue[T]) Peek() (T, bool) {
	if pq.Len() == 0 {
		var zero T
		return zero, false
	}
	return pq.items[0].Value, true
}

// Update 更新项的优先级
func (pq *PriorityQueue[T]) Update(item *PriorityQueueItem[T], value T, priority int) {
	item.Value = value
	item.Priority = priority
	heap.Fix(pq, item.Index)
}

// Remove 移除项
func (pq *PriorityQueue[T]) Remove(item *PriorityQueueItem[T]) {
	heap.Remove(pq, item.Index)
}

// TimeWheel 时间轮（用于定时任务）
type TimeWheel struct {
	tickMs    int64         // 每个刻度的毫秒数
	wheelSize int           // 时间轮大小
	current   int           // 当前位置
	wheel     [][]func()    // 时间轮槽位，每个槽位包含多个任务
	ticker    *time.Ticker
	stopChan  chan struct{}
}

// NewTimeWheel 创建时间轮
func NewTimeWheel(tickMs int64, wheelSize int) *TimeWheel {
	if tickMs <= 0 {
		tickMs = 1000 // 默认1秒
	}
	if wheelSize <= 0 {
		wheelSize = 60 // 默认60个槽位
	}

	tw := &TimeWheel{
		tickMs:   tickMs,
		wheelSize: wheelSize,
		wheel:    make([][]func(), wheelSize),
		stopChan: make(chan struct{}),
	}

	return tw
}

// Start 启动时间轮
func (tw *TimeWheel) Start() {
	tw.ticker = time.NewTicker(time.Duration(tw.tickMs) * time.Millisecond)
	go tw.run()
}

// Stop 停止时间轮
func (tw *TimeWheel) Stop() {
	if tw.ticker != nil {
		tw.ticker.Stop()
	}
	close(tw.stopChan)
}

// AddTask 添加任务
func (tw *TimeWheel) AddTask(delayMs int64, task func()) {
	if delayMs <= 0 {
		// 立即执行
		go task()
		return
	}

	ticks := delayMs / tw.tickMs
	if ticks <= 0 {
		ticks = 1
	}

	// 计算槽位位置
	pos := (tw.current + int(ticks)) % tw.wheelSize

	// 添加到对应槽位
	tw.wheel[pos] = append(tw.wheel[pos], task)
}

// run 运行时间轮
func (tw *TimeWheel) run() {
	for {
		select {
		case <-tw.ticker.C:
			tw.tick()
		case <-tw.stopChan:
			return
		}
	}
}

// tick 执行一个tick
func (tw *TimeWheel) tick() {
	// 获取当前槽位的任务
	tasks := tw.wheel[tw.current]
	tw.wheel[tw.current] = nil // 清空槽位

	// 执行任务
	for _, task := range tasks {
		go task()
	}

	// 移动到下一个槽位
	tw.current = (tw.current + 1) % tw.wheelSize
}
