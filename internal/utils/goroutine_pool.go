package utils

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/logger"

	"github.com/panjf2000/ants/v2"
)

// GoroutinePool 协程池管理器
type GoroutinePool struct {
	pool   *ants.Pool
	logger logger.Logger

	// 统计信息
	submittedTasks int64
	completedTasks int64
	failedTasks    int64
	activeWorkers  int64
	totalWorkers   int64
	mu             sync.RWMutex

	// 监控
	monitorEnabled bool
	monitorTicker  *time.Ticker
	stopMonitor    chan struct{}
	stopped        bool
}

// GoroutinePoolConfig 协程池配置
type GoroutinePoolConfig struct {
	// 协程池大小
	Size int
	// 非阻塞模式
	Nonblocking bool
	// 预分配协程
	PreAlloc bool
	// 最大阻塞任务数
	MaxBlockingTasks int
	// 监控间隔
	MonitorInterval time.Duration
	// 过期时间
	ExpiryDuration time.Duration
	// 禁用Purge
	DisablePurge bool
}

// DefaultGoroutinePoolConfig 默认协程池配置
func DefaultGoroutinePoolConfig() GoroutinePoolConfig {
	return GoroutinePoolConfig{
		Size:             100,
		Nonblocking:      false,
		PreAlloc:         true,
		MaxBlockingTasks: 0,
		MonitorInterval:  30 * time.Second,
		ExpiryDuration:   1 * time.Minute,
		DisablePurge:     false,
	}
}

// NewGoroutinePool 创建协程池管理器
func NewGoroutinePool(config GoroutinePoolConfig, logger logger.Logger) (*GoroutinePool, error) {
	// 创建ants协程池
	options := ants.Options{
		ExpiryDuration:   config.ExpiryDuration,
		Nonblocking:      config.Nonblocking,
		PreAlloc:         config.PreAlloc,
		MaxBlockingTasks: config.MaxBlockingTasks,
		DisablePurge:     config.DisablePurge,
	}

	pool, err := ants.NewPool(config.Size, ants.WithOptions(options))
	if err != nil {
		return nil, fmt.Errorf("创建协程池失败: %w", err)
	}

	gp := &GoroutinePool{
		pool:           pool,
		logger:         logger,
		monitorEnabled: config.MonitorInterval > 0,
		stopMonitor:    make(chan struct{}),
		totalWorkers:   int64(config.Size),
	}

	// 启动监控
	if gp.monitorEnabled {
		go gp.monitor(config.MonitorInterval)
	}

	gp.logger.Info("协程池创建成功",
		"size", config.Size,
		"pre_alloc", config.PreAlloc,
		"nonblocking", config.Nonblocking)

	return gp, nil
}

// Submit 提交任务到协程池
func (gp *GoroutinePool) Submit(task func()) error {
	atomic.AddInt64(&gp.submittedTasks, 1)

	err := gp.pool.Submit(func() {
		defer func() {
			atomic.AddInt64(&gp.completedTasks, 1)
			if r := recover(); r != nil {
				atomic.AddInt64(&gp.failedTasks, 1)
				gp.logger.Error("协程池任务执行失败", "panic", r)
			}
		}()

		task()
	})

	if err != nil {
		atomic.AddInt64(&gp.submittedTasks, -1) // 提交失败，减回去
		return fmt.Errorf("提交任务失败: %w", err)
	}

	return nil
}

// SubmitWithContext 提交带上下文的任务
func (gp *GoroutinePool) SubmitWithContext(ctx context.Context, task func(ctx context.Context)) error {
	atomic.AddInt64(&gp.submittedTasks, 1)

	err := gp.pool.Submit(func() {
		defer func() {
			atomic.AddInt64(&gp.completedTasks, 1)
			if r := recover(); r != nil {
				atomic.AddInt64(&gp.failedTasks, 1)
				gp.logger.Error("协程池任务执行失败", "panic", r)
			}
		}()

		task(ctx)
	})

	if err != nil {
		atomic.AddInt64(&gp.submittedTasks, -1)
		return fmt.Errorf("提交任务失败: %w", err)
	}

	return nil
}

// GetStats 获取协程池统计信息
func (gp *GoroutinePool) GetStats() GoroutinePoolStats {
	running := gp.pool.Running()
	free := gp.pool.Free()

	return GoroutinePoolStats{
		SubmittedTasks: atomic.LoadInt64(&gp.submittedTasks),
		CompletedTasks: atomic.LoadInt64(&gp.completedTasks),
		FailedTasks:    atomic.LoadInt64(&gp.failedTasks),
		RunningWorkers: int64(running),
		FreeWorkers:    int64(free),
		TotalWorkers:   gp.totalWorkers,
		Capacity:       gp.pool.Cap(),
	}
}

// GoroutinePoolStats 协程池统计信息
type GoroutinePoolStats struct {
	SubmittedTasks int64 `json:"submitted_tasks"` // 已提交任务数
	CompletedTasks int64 `json:"completed_tasks"` // 已完成任务数
	FailedTasks    int64 `json:"failed_tasks"`    // 失败任务数
	RunningWorkers int64 `json:"running_workers"` // 运行中的工作协程数
	FreeWorkers    int64 `json:"free_workers"`    // 空闲工作协程数
	TotalWorkers   int64 `json:"total_workers"`   // 总工作协程数
	Capacity       int   `json:"capacity"`        // 协程池容量
}

// TuneCapacity 动态调整协程池容量
func (gp *GoroutinePool) TuneCapacity(size int) error {
	if size <= 0 {
		return fmt.Errorf("协程池容量必须大于0")
	}

	oldSize := gp.pool.Cap()
	gp.pool.Tune(size)
	atomic.StoreInt64(&gp.totalWorkers, int64(size))

	gp.logger.Info("协程池容量已调整",
		"old_size", oldSize,
		"new_size", size)

	return nil
}

// IsClosed 检查协程池是否已关闭
func (gp *GoroutinePool) IsClosed() bool {
	return gp.pool.IsClosed()
}

// Close 关闭协程池
func (gp *GoroutinePool) Close() error {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	if gp.stopped {
		return nil
	}

	gp.stopped = true

	if gp.monitorEnabled {
		select {
		case <-gp.stopMonitor:
			// already closed
		default:
			close(gp.stopMonitor)
		}
	}

	gp.pool.Release()
	gp.logger.Info("协程池已关闭")
	return nil
}

// monitor 监控协程
func (gp *GoroutinePool) monitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := gp.GetStats()

			gp.logger.Info("协程池监控",
				"submitted", stats.SubmittedTasks,
				"completed", stats.CompletedTasks,
				"failed", stats.FailedTasks,
				"running", stats.RunningWorkers,
				"free", stats.FreeWorkers,
				"capacity", stats.Capacity)

			// 自动调整容量（简单策略）
			gp.autoTune(stats)

		case <-gp.stopMonitor:
			return
		}
	}
}

// autoTune 自动调整协程池容量
func (gp *GoroutinePool) autoTune(stats GoroutinePoolStats) {
	currentCap := stats.Capacity
	runningRatio := float64(stats.RunningWorkers) / float64(currentCap)

	// 如果运行协程占比超过80%，考虑增加容量
	if runningRatio > 0.8 && currentCap < 1000 {
		newCap := int(float64(currentCap) * 1.2)
		if newCap > 1000 {
			newCap = 1000
		}
		if newCap != currentCap {
			gp.TuneCapacity(newCap)
		}
	}

	// 如果运行协程占比低于20%且空闲时间较长，考虑减少容量
	if runningRatio < 0.2 && currentCap > 10 {
		newCap := int(float64(currentCap) * 0.8)
		if newCap < 10 {
			newCap = 10
		}
		if newCap != currentCap {
			gp.TuneCapacity(newCap)
		}
	}
}

// AdaptiveGoroutinePool 自适应协程池
type AdaptiveGoroutinePool struct {
	pools   map[string]*GoroutinePool
	configs map[string]GoroutinePoolConfig
	logger  logger.Logger
	mu      sync.RWMutex
}

// NewAdaptiveGoroutinePool 创建自适应协程池
func NewAdaptiveGoroutinePool(logger logger.Logger) *AdaptiveGoroutinePool {
	return &AdaptiveGoroutinePool{
		pools:   make(map[string]*GoroutinePool),
		configs: make(map[string]GoroutinePoolConfig),
		logger:  logger,
	}
}

// RegisterPool 注册协程池
func (agp *AdaptiveGoroutinePool) RegisterPool(name string, config GoroutinePoolConfig) error {
	agp.mu.Lock()
	defer agp.mu.Unlock()

	if _, exists := agp.pools[name]; exists {
		return fmt.Errorf("协程池 %s 已存在", name)
	}

	pool, err := NewGoroutinePool(config, agp.logger)
	if err != nil {
		return fmt.Errorf("创建协程池 %s 失败: %w", name, err)
	}

	agp.pools[name] = pool
	agp.configs[name] = config

	return nil
}

// SubmitToPool 提交任务到指定协程池
func (agp *AdaptiveGoroutinePool) SubmitToPool(poolName string, task func()) error {
	agp.mu.RLock()
	pool, exists := agp.pools[poolName]
	agp.mu.RUnlock()

	if !exists {
		return fmt.Errorf("协程池 %s 不存在", poolName)
	}

	return pool.Submit(task)
}

// GetPoolStats 获取指定协程池统计信息
func (agp *AdaptiveGoroutinePool) GetPoolStats(poolName string) (GoroutinePoolStats, error) {
	agp.mu.RLock()
	pool, exists := agp.pools[poolName]
	agp.mu.RUnlock()

	if !exists {
		return GoroutinePoolStats{}, fmt.Errorf("协程池 %s 不存在", poolName)
	}

	return pool.GetStats(), nil
}

// GetAllPoolStats 获取所有协程池统计信息
func (agp *AdaptiveGoroutinePool) GetAllPoolStats() map[string]GoroutinePoolStats {
	agp.mu.RLock()
	defer agp.mu.RUnlock()

	stats := make(map[string]GoroutinePoolStats)
	for name, pool := range agp.pools {
		stats[name] = pool.GetStats()
	}

	return stats
}

// TunePoolCapacity 调整指定协程池容量
func (agp *AdaptiveGoroutinePool) TunePoolCapacity(poolName string, size int) error {
	agp.mu.RLock()
	pool, exists := agp.pools[poolName]
	agp.mu.RUnlock()

	if !exists {
		return fmt.Errorf("协程池 %s 不存在", poolName)
	}

	return pool.TuneCapacity(size)
}

// ClosePool 关闭指定协程池
func (agp *AdaptiveGoroutinePool) ClosePool(poolName string) error {
	agp.mu.Lock()
	defer agp.mu.Unlock()

	pool, exists := agp.pools[poolName]
	if !exists {
		return fmt.Errorf("协程池 %s 不存在", poolName)
	}

	err := pool.Close()
	delete(agp.pools, poolName)
	delete(agp.configs, poolName)

	return err
}

// Close 关闭所有协程池
func (agp *AdaptiveGoroutinePool) Close() error {
	agp.mu.Lock()
	defer agp.mu.Unlock()

	var errs []error
	for name, pool := range agp.pools {
		if err := pool.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭协程池 %s 失败: %w", name, err))
		}
	}

	agp.pools = make(map[string]*GoroutinePool)
	agp.configs = make(map[string]GoroutinePoolConfig)

	if len(errs) > 0 {
		return fmt.Errorf("关闭协程池时发生错误: %v", errs)
	}

	return nil
}

// GoroutineMonitor 协程监控器
type GoroutineMonitor struct {
	logger     logger.Logger
	interval   time.Duration
	stopChan   chan struct{}
	lastCount  int
	growthRate float64
	mu         sync.RWMutex
}

// NewGoroutineMonitor 创建协程监控器
func NewGoroutineMonitor(logger logger.Logger, interval time.Duration) *GoroutineMonitor {
	return &GoroutineMonitor{
		logger:   logger,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start 启动监控
func (gm *GoroutineMonitor) Start() {
	go gm.monitor()
	gm.logger.Info("协程监控器已启动", "interval", gm.interval)
}

// Stop 停止监控
func (gm *GoroutineMonitor) Stop() {
	close(gm.stopChan)
	gm.logger.Info("协程监控器已停止")
}

func (gm *GoroutineMonitor) monitor() {
	ticker := time.NewTicker(gm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentCount := runtime.NumGoroutine()

			gm.mu.Lock()
			if gm.lastCount > 0 {
				gm.growthRate = float64(currentCount-gm.lastCount) / float64(gm.lastCount)
			}
			gm.lastCount = currentCount
			growthRate := gm.growthRate
			gm.mu.Unlock()

			gm.logger.Info("协程数量监控",
				"current", currentCount,
				"growth_rate", fmt.Sprintf("%.2f%%", growthRate*100))

			// 预警检查
			if currentCount > 10000 {
				gm.logger.Warn("协程数量异常", "count", currentCount)
			}

			if growthRate > 0.5 { // 增长率超过50%
				gm.logger.Warn("协程增长过快", "growth_rate", fmt.Sprintf("%.2f%%", growthRate*100))
			}

		case <-gm.stopChan:
			return
		}
	}
}

// GetStats 获取监控统计
func (gm *GoroutineMonitor) GetStats() GoroutineMonitorStats {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	return GoroutineMonitorStats{
		CurrentCount: runtime.NumGoroutine(),
		LastCount:    gm.lastCount,
		GrowthRate:   gm.growthRate,
	}
}

// GoroutineMonitorStats 协程监控统计
type GoroutineMonitorStats struct {
	CurrentCount int     `json:"current_count"` // 当前协程数量
	LastCount    int     `json:"last_count"`    // 上次记录的协程数量
	GrowthRate   float64 `json:"growth_rate"`   // 增长率
}
