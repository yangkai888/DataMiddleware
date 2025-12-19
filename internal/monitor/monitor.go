package monitor

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/logger"
)

// Monitor 监控器
type Monitor struct {
	logger logger.Logger

	// 基础指标
	startTime      time.Time
	totalRequests  int64
	activeRequests int64
	failedRequests int64

	// 性能指标
	totalResponseTime time.Duration
	mutex             sync.RWMutex

	// 组件健康状态
	componentHealth map[string]HealthStatus
	healthMutex     sync.RWMutex

	// 自定义指标
	customMetrics map[string]interface{}
	metricsMutex  sync.RWMutex
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status    string    `json:"status"`    // "healthy", "unhealthy", "unknown"
	Message   string    `json:"message"`   // 状态消息
	Timestamp time.Time `json:"timestamp"` // 检查时间
	Response  int64     `json:"response"`  // 响应时间(ms)
}

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthStatus
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	Uptime          int64                   `json:"uptime"`            // 运行时间(秒)
	TotalRequests   int64                   `json:"total_requests"`    // 总请求数
	ActiveRequests  int64                   `json:"active_requests"`   // 活跃请求数
	FailedRequests  int64                   `json:"failed_requests"`   // 失败请求数
	AvgResponseTime time.Duration           `json:"avg_response_time"` // 平均响应时间
	Goroutines      int                     `json:"goroutines"`        // goroutine数量
	Memory          MemoryStats             `json:"memory"`            // 内存统计
	Components      map[string]HealthStatus `json:"components"`        // 组件健康状态
}

// MemoryStats 内存统计
type MemoryStats struct {
	Alloc        uint64 `json:"alloc"`         // 已分配内存
	TotalAlloc   uint64 `json:"total_alloc"`   // 总分配内存
	Sys          uint64 `json:"sys"`           // 系统内存
	Lookups      uint64 `json:"lookups"`       // 指针查找次数
	Mallocs      uint64 `json:"mallocs"`       // 分配次数
	Frees        uint64 `json:"frees"`         // 释放次数
	HeapAlloc    uint64 `json:"heap_alloc"`    // 堆内存分配
	HeapSys      uint64 `json:"heap_sys"`      // 堆系统内存
	HeapIdle     uint64 `json:"heap_idle"`     // 空闲堆内存
	HeapInuse    uint64 `json:"heap_inuse"`    // 使用中堆内存
	HeapReleased uint64 `json:"heap_released"` // 已释放堆内存
	HeapObjects  uint64 `json:"heap_objects"`  // 堆对象数量
	NumGC        uint32 `json:"num_gc"`        // GC次数
}

// NewMonitor 创建监控器
func NewMonitor(logger logger.Logger) *Monitor {
	return &Monitor{
		logger:          logger,
		startTime:       time.Now(),
		componentHealth: make(map[string]HealthStatus),
		customMetrics:   make(map[string]interface{}),
	}
}

// RecordRequest 记录请求
func (m *Monitor) RecordRequest(duration time.Duration, success bool) {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.activeRequests, 1)
	defer atomic.AddInt64(&m.activeRequests, -1)

	m.mutex.Lock()
	m.totalResponseTime += duration
	m.mutex.Unlock()

	if !success {
		atomic.AddInt64(&m.failedRequests, 1)
	}
}

// RegisterHealthChecker 注册健康检查器
func (m *Monitor) RegisterHealthChecker(checker HealthChecker) {
	// 定期检查健康状态
	go func() {
		ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				status := checker.Check(ctx)
				cancel()

				m.healthMutex.Lock()
				m.componentHealth[checker.Name()] = status
				m.healthMutex.Unlock()

				if status.Status != "healthy" {
					m.logger.Warn("组件健康检查失败",
						"component", checker.Name(),
						"status", status.Status,
						"message", status.Message)
				}
			}
		}
	}()
}

// GetSystemMetrics 获取系统指标
func (m *Monitor) GetSystemMetrics() SystemMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mutex.RLock()
	totalRequests := atomic.LoadInt64(&m.totalRequests)
	activeRequests := atomic.LoadInt64(&m.activeRequests)
	failedRequests := atomic.LoadInt64(&m.failedRequests)
	totalResponseTime := m.totalResponseTime
	m.mutex.RUnlock()

	var avgResponseTime time.Duration
	if totalRequests > 0 {
		avgResponseTime = totalResponseTime / time.Duration(totalRequests)
	}

	m.healthMutex.RLock()
	components := make(map[string]HealthStatus)
	for k, v := range m.componentHealth {
		components[k] = v
	}
	m.healthMutex.RUnlock()

	return SystemMetrics{
		Uptime:          int64(time.Since(m.startTime).Seconds()),
		TotalRequests:   totalRequests,
		ActiveRequests:  activeRequests,
		FailedRequests:  failedRequests,
		AvgResponseTime: avgResponseTime,
		Goroutines:      runtime.NumGoroutine(),
		Memory: MemoryStats{
			Alloc:        memStats.Alloc,
			TotalAlloc:   memStats.TotalAlloc,
			Sys:          memStats.Sys,
			Lookups:      memStats.Lookups,
			Mallocs:      memStats.Mallocs,
			Frees:        memStats.Frees,
			HeapAlloc:    memStats.HeapAlloc,
			HeapSys:      memStats.HeapSys,
			HeapIdle:     memStats.HeapIdle,
			HeapInuse:    memStats.HeapInuse,
			HeapReleased: memStats.HeapReleased,
			HeapObjects:  memStats.HeapObjects,
			NumGC:        memStats.NumGC,
		},
		Components: components,
	}
}

// SetCustomMetric 设置自定义指标
func (m *Monitor) SetCustomMetric(key string, value interface{}) {
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	m.customMetrics[key] = value
}

// GetCustomMetric 获取自定义指标
func (m *Monitor) GetCustomMetric(key string) interface{} {
	m.metricsMutex.RLock()
	defer m.metricsMutex.RUnlock()
	return m.customMetrics[key]
}

// GetAllCustomMetrics 获取所有自定义指标
func (m *Monitor) GetAllCustomMetrics() map[string]interface{} {
	m.metricsMutex.RLock()
	defer m.metricsMutex.RUnlock()

	result := make(map[string]interface{})
	for k, v := range m.customMetrics {
		result[k] = v
	}
	return result
}
