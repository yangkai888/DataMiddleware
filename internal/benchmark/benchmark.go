package benchmark

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// BenchmarkConfig 基准测试配置
type BenchmarkConfig struct {
	// 并发数
	Concurrency int
	// 测试持续时间
	Duration time.Duration
	// 请求间隔
	RequestInterval time.Duration
	// 预热时间
	WarmupDuration time.Duration
	// 是否启用详细输出
	Verbose bool
}

// DefaultBenchmarkConfig 默认基准测试配置
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		Concurrency:     100,
		Duration:        30 * time.Second,
		RequestInterval: 10 * time.Millisecond,
		WarmupDuration:  5 * time.Second,
		Verbose:         false,
	}
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	// 测试配置
	Config BenchmarkConfig

	// 时间统计
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	// 请求统计
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64

	// 性能指标
	QPS             float64 // 每秒请求数
	AvgResponseTime time.Duration
	MinResponseTime time.Duration
	MaxResponseTime time.Duration
	P50ResponseTime time.Duration
	P95ResponseTime time.Duration
	P99ResponseTime time.Duration

	// 资源使用
	MemoryStats MemoryStats
	CPUUsage    float64

	// 错误统计
	Errors   map[string]int64
	errorsMu sync.RWMutex
}

// MemoryStats 内存统计
type MemoryStats struct {
	Alloc         uint64
	TotalAlloc    uint64
	Sys           uint64
	NumGC         uint32
	GCCPUFraction float64
}

// BenchmarkRunner 基准测试运行器
type BenchmarkRunner struct {
	config BenchmarkConfig
	logger logger.Logger
}

// NewBenchmarkRunner 创建基准测试运行器
func NewBenchmarkRunner(config BenchmarkConfig, logger logger.Logger) *BenchmarkRunner {
	return &BenchmarkRunner{
		config: config,
		logger: logger,
	}
}

// RunCacheBenchmark 运行缓存基准测试
func (br *BenchmarkRunner) RunCacheBenchmark(cache types.Cache) (*BenchmarkResult, error) {
	br.logger.Info("开始缓存基准测试",
		"concurrency", br.config.Concurrency,
		"duration", br.config.Duration)

	result := &BenchmarkResult{
		Config:    br.config,
		StartTime: time.Now(),
		Errors:    make(map[string]int64),
	}

	// 预热阶段
	if br.config.WarmupDuration > 0 {
		br.logger.Info("缓存预热开始", "duration", br.config.WarmupDuration)
		br.warmupCache(cache, br.config.WarmupDuration)
		br.logger.Info("缓存预热完成")
	}

	// 运行测试
	ctx, cancel := context.WithTimeout(context.Background(), br.config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	responseTimes := make([]time.Duration, 0, 100000)

	// 启动工作协程
	for i := 0; i < br.config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			br.cacheWorker(ctx, cache, result, &responseTimes, workerID)
		}(i)
	}

	// 等待测试完成
	wg.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 计算统计信息
	br.calculateStats(result, responseTimes)

	br.logger.Info("缓存基准测试完成",
		"total_requests", result.TotalRequests,
		"qps", result.QPS,
		"avg_response_time", result.AvgResponseTime,
		"p95_response_time", result.P95ResponseTime)

	return result, nil
}

// RunHTTPBenchmark 运行HTTP基准测试
func (br *BenchmarkRunner) RunHTTPBenchmark(url string) (*BenchmarkResult, error) {
	br.logger.Info("开始HTTP基准测试",
		"url", url,
		"concurrency", br.config.Concurrency,
		"duration", br.config.Duration)

	result := &BenchmarkResult{
		Config:    br.config,
		StartTime: time.Now(),
		Errors:    make(map[string]int64),
	}

	// 预热阶段
	if br.config.WarmupDuration > 0 {
		br.logger.Info("HTTP预热开始", "duration", br.config.WarmupDuration)
		br.warmupHTTP(url, br.config.WarmupDuration)
		br.logger.Info("HTTP预热完成")
	}

	// 运行测试
	ctx, cancel := context.WithTimeout(context.Background(), br.config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	responseTimes := make([]time.Duration, 0, 100000)

	// 启动工作协程
	for i := 0; i < br.config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			br.httpWorker(ctx, url, result, &responseTimes, workerID)
		}(i)
	}

	// 等待测试完成
	wg.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 计算统计信息
	br.calculateStats(result, responseTimes)

	br.logger.Info("HTTP基准测试完成",
		"total_requests", result.TotalRequests,
		"qps", result.QPS,
		"avg_response_time", result.AvgResponseTime,
		"p95_response_time", result.P95ResponseTime)

	return result, nil
}

// cacheWorker 缓存测试工作协程
func (br *BenchmarkRunner) cacheWorker(ctx context.Context, cache types.Cache, result *BenchmarkResult, responseTimes *[]time.Duration, workerID int) {
	ticker := time.NewTicker(br.config.RequestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()

			// 执行缓存操作（读写混合）
			key := fmt.Sprintf("bench_key_%d_%d", workerID, atomic.AddInt64(&result.TotalRequests, 1))
			value := []byte(fmt.Sprintf("bench_value_%d_%d", workerID, time.Now().UnixNano()))

			var err error
			if atomic.LoadInt64(&result.TotalRequests)%2 == 0 {
				// 写操作
				err = cache.Set(key, value)
			} else {
				// 读操作
				_, err = cache.Get(key)
			}

			responseTime := time.Since(start)
			*responseTimes = append(*responseTimes, responseTime)

			if err != nil {
				atomic.AddInt64(&result.FailedRequests, 1)
				result.errorsMu.Lock()
				result.Errors[err.Error()]++
				result.errorsMu.Unlock()
				if br.config.Verbose {
					br.logger.Warn("缓存操作失败", "key", key, "error", err)
				}
			} else {
				atomic.AddInt64(&result.SuccessRequests, 1)
			}
		}
	}
}

// httpWorker HTTP测试工作协程
func (br *BenchmarkRunner) httpWorker(ctx context.Context, url string, result *BenchmarkResult, responseTimes *[]time.Duration, workerID int) {
	ticker := time.NewTicker(br.config.RequestInterval)
	defer ticker.Stop()

	client := &http.Client{Timeout: 5 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				atomic.AddInt64(&result.FailedRequests, 1)
				result.errorsMu.Lock()
				result.Errors[err.Error()]++
				result.errorsMu.Unlock()
				continue
			}

			resp, err := client.Do(req)
			responseTime := time.Since(start)
			*responseTimes = append(*responseTimes, responseTime)

			if err != nil {
				atomic.AddInt64(&result.FailedRequests, 1)
				result.errorsMu.Lock()
				result.Errors[err.Error()]++
				result.errorsMu.Unlock()
				if br.config.Verbose {
					br.logger.Warn("HTTP请求失败", "url", url, "error", err)
				}
			} else {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					atomic.AddInt64(&result.SuccessRequests, 1)
				} else {
					atomic.AddInt64(&result.FailedRequests, 1)
					result.Errors[fmt.Sprintf("HTTP %d", resp.StatusCode)]++
				}
			}
		}
	}
}

// warmupCache 缓存预热
func (br *BenchmarkRunner) warmupCache(cache types.Cache, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < br.config.Concurrency/4; i++ { // 使用较少的协程进行预热
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ticker := time.NewTicker(br.config.RequestInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					key := fmt.Sprintf("warmup_key_%d_%d", workerID, time.Now().UnixNano())
					value := []byte(fmt.Sprintf("warmup_value_%d", time.Now().UnixNano()))
					cache.Set(key, value)
				}
			}
		}(i)
	}
	wg.Wait()
}

// warmupHTTP HTTP预热
func (br *BenchmarkRunner) warmupHTTP(url string, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(br.config.RequestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
			if resp, err := client.Do(req); err == nil {
				resp.Body.Close()
			}
		}
	}
}

// calculateStats 计算统计信息
func (br *BenchmarkRunner) calculateStats(result *BenchmarkResult, responseTimes []time.Duration) {
	if len(responseTimes) == 0 {
		return
	}

	// 基础指标
	result.TotalRequests = result.SuccessRequests + result.FailedRequests
	if result.Duration.Seconds() > 0 {
		result.QPS = float64(result.TotalRequests) / result.Duration.Seconds()
	}

	// 响应时间统计
	totalTime := time.Duration(0)
	result.MinResponseTime = time.Duration(math.MaxInt64)
	result.MaxResponseTime = 0

	for _, rt := range responseTimes {
		totalTime += rt
		if rt < result.MinResponseTime {
			result.MinResponseTime = rt
		}
		if rt > result.MaxResponseTime {
			result.MaxResponseTime = rt
		}
	}

	if len(responseTimes) > 0 {
		result.AvgResponseTime = totalTime / time.Duration(len(responseTimes))

		// 计算百分位数
		sortedTimes := make([]time.Duration, len(responseTimes))
		copy(sortedTimes, responseTimes)

		// 简单排序（实际应该用更高效的算法）
		for i := 0; i < len(sortedTimes)-1; i++ {
			for j := i + 1; j < len(sortedTimes); j++ {
				if sortedTimes[i] > sortedTimes[j] {
					sortedTimes[i], sortedTimes[j] = sortedTimes[j], sortedTimes[i]
				}
			}
		}

		p50Index := int(float64(len(sortedTimes)) * 0.5)
		p95Index := int(float64(len(sortedTimes)) * 0.95)
		p99Index := int(float64(len(sortedTimes)) * 0.99)

		if p50Index < len(sortedTimes) {
			result.P50ResponseTime = sortedTimes[p50Index]
		}
		if p95Index < len(sortedTimes) {
			result.P95ResponseTime = sortedTimes[p95Index]
		}
		if p99Index < len(sortedTimes) {
			result.P99ResponseTime = sortedTimes[p99Index]
		}
	}

	// 内存统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	result.MemoryStats = MemoryStats{
		Alloc:         memStats.Alloc,
		TotalAlloc:    memStats.TotalAlloc,
		Sys:           memStats.Sys,
		NumGC:         memStats.NumGC,
		GCCPUFraction: memStats.GCCPUFraction,
	}
}

// StressTest 压力测试
type StressTest struct {
	config    BenchmarkConfig
	logger    logger.Logger
	runners   []StressTestRunner
	stopChan  chan struct{}
	isRunning bool
	mu        sync.RWMutex
}

// StressTestRunner 压力测试运行器接口
type StressTestRunner interface {
	Run(ctx context.Context, result *BenchmarkResult) error
	Name() string
}

// NewStressTest 创建压力测试
func NewStressTest(config BenchmarkConfig, logger logger.Logger) *StressTest {
	return &StressTest{
		config:   config,
		logger:   logger,
		runners:  make([]StressTestRunner, 0),
		stopChan: make(chan struct{}),
	}
}

// AddRunner 添加测试运行器
func (st *StressTest) AddRunner(runner StressTestRunner) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.runners = append(st.runners, runner)
}

// Start 开始压力测试
func (st *StressTest) Start() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.isRunning {
		return fmt.Errorf("压力测试已在运行")
	}

	st.isRunning = true
	go st.run()

	st.logger.Info("压力测试已启动",
		"runners", len(st.runners),
		"concurrency", st.config.Concurrency,
		"duration", st.config.Duration)

	return nil
}

// Stop 停止压力测试
func (st *StressTest) Stop() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if !st.isRunning {
		return nil
	}

	st.isRunning = false
	close(st.stopChan)

	st.logger.Info("压力测试已停止")
	return nil
}

// run 运行压力测试
func (st *StressTest) run() {
	ctx, cancel := context.WithTimeout(context.Background(), st.config.Duration)
	defer cancel()

	results := make(map[string]*BenchmarkResult)

	var wg sync.WaitGroup
	for _, runner := range st.runners {
		wg.Add(1)
		go func(r StressTestRunner) {
			defer wg.Done()

			result := &BenchmarkResult{
				Config:    st.config,
				StartTime: time.Now(),
				Errors:    make(map[string]int64),
			}

			err := r.Run(ctx, result)
			if err != nil {
				st.logger.Error("压力测试运行器执行失败",
					"runner", r.Name(),
					"error", err)
				return
			}

			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)

			st.mu.Lock()
			results[r.Name()] = result
			st.mu.Unlock()

			st.logger.Info("压力测试运行器完成",
				"runner", r.Name(),
				"total_requests", result.TotalRequests,
				"qps", result.QPS,
				"avg_response_time", result.AvgResponseTime)
		}(runner)
	}

	wg.Wait()

	// 输出汇总报告
	st.generateReport(results)
}

// generateReport 生成测试报告
func (st *StressTest) generateReport(results map[string]*BenchmarkResult) {
	st.logger.Info("=== 压力测试报告 ===")

	totalRequests := int64(0)
	totalQPS := float64(0)
	runnerCount := 0

	for name, result := range results {
		st.logger.Info(fmt.Sprintf("运行器: %s", name))
		st.logger.Info(fmt.Sprintf("  总请求数: %d", result.TotalRequests))
		st.logger.Info(fmt.Sprintf("  成功请求: %d", result.SuccessRequests))
		st.logger.Info(fmt.Sprintf("  失败请求: %d", result.FailedRequests))
		st.logger.Info(fmt.Sprintf("  QPS: %.2f", result.QPS))
		st.logger.Info(fmt.Sprintf("  平均响应时间: %v", result.AvgResponseTime))
		st.logger.Info(fmt.Sprintf("  P95响应时间: %v", result.P95ResponseTime))
		st.logger.Info(fmt.Sprintf("  内存使用: %d MB", result.MemoryStats.Alloc/1024/1024))

		totalRequests += result.TotalRequests
		totalQPS += result.QPS
		runnerCount++
	}

	if runnerCount > 0 {
		st.logger.Info("=== 汇总统计 ===")
		st.logger.Info(fmt.Sprintf("总请求数: %d", totalRequests))
		st.logger.Info(fmt.Sprintf("平均QPS: %.2f", totalQPS/float64(runnerCount)))
	}
}
