// Package benchmarks 极限QPS基准测试
// 测试DataMiddleware单机HTTP QPS极限性能
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// 测试配置
type QPSBenchmarkConfig struct {
	TargetURL       string
	Duration        time.Duration
	Concurrency     int
	Connections     int
	DisableKeepAlive bool
	Timeout         time.Duration
}

// 测试结果
type QPSBenchmarkResult struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	QPS                float64
	AvgResponseTime    time.Duration
	MinResponseTime    time.Duration
	MaxResponseTime    time.Duration
	P50ResponseTime    time.Duration
	P95ResponseTime    time.Duration
	P99ResponseTime    time.Duration
	Errors             []string
	StartTime          time.Time
	EndTime            time.Time
}

// HTTP客户端工厂
func createHTTPClient(config QPSBenchmarkConfig) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        config.Connections,
		MaxIdleConnsPerHost: config.Connections,
		DisableKeepAlives:   config.DisableKeepAlive,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}
}

// 响应时间统计
type ResponseTimeStats struct {
	mu         sync.Mutex
	times      []time.Duration
	totalTime  time.Duration
	minTime    time.Duration
	maxTime    time.Duration
	count      int64
}

func (s *ResponseTimeStats) Add(duration time.Duration) {
	atomic.AddInt64(&s.count, 1)
	atomic.AddInt64((*int64)(&s.totalTime), int64(duration))

	s.mu.Lock()
	defer s.mu.Unlock()

	s.times = append(s.times, duration)
	if s.minTime == 0 || duration < s.minTime {
		s.minTime = duration
	}
	if duration > s.maxTime {
		s.maxTime = duration
	}
}

func (s *ResponseTimeStats) CalculatePercentiles() (p50, p95, p99 time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.times) == 0 {
		return 0, 0, 0
	}

	// 简单的百分位数计算（排序后取值）
	sorted := make([]time.Duration, len(s.times))
	copy(sorted, s.times)

	// 简单排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	p50 = sorted[len(sorted)*50/100]
	p95 = sorted[len(sorted)*95/100]
	p99 = sorted[len(sorted)*99/100]

	return p50, p95, p99
}

// 单个worker执行HTTP请求
func httpWorker(ctx context.Context, client *http.Client, targetURL string, stats *ResponseTimeStats, results *QPSBenchmarkResult) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			start := time.Now()

			req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
			if err != nil {
				atomic.AddInt64(&results.FailedRequests, 1)
				continue
			}

			resp, err := client.Do(req)
			duration := time.Since(start)

			atomic.AddInt64(&results.TotalRequests, 1)
			stats.Add(duration)

			if err != nil {
				atomic.AddInt64(&results.FailedRequests, 1)
				continue
			}

			// 读取响应体
			_, err = io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				atomic.AddInt64(&results.FailedRequests, 1)
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				atomic.AddInt64(&results.SuccessfulRequests, 1)
			} else {
				atomic.AddInt64(&results.FailedRequests, 1)
			}
		}
	}
}

// 运行QPS基准测试
func RunQPSBenchmark(config QPSBenchmarkConfig) (*QPSBenchmarkResult, error) {
	log.Printf("开始QPS基准测试: 并发=%d, 时长=%v, 目标=%s",
		config.Concurrency, config.Duration, config.TargetURL)

	result := &QPSBenchmarkResult{
		StartTime: time.Now(),
	}

	stats := &ResponseTimeStats{}
	client := createHTTPClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	// 启动workers
	var wg sync.WaitGroup
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			httpWorker(ctx, client, config.TargetURL, stats, result)
		}()
	}

	// 等待测试完成
	wg.Wait()
	result.EndTime = time.Now()

	// 计算结果
	actualDuration := result.EndTime.Sub(result.StartTime)
	result.QPS = float64(result.TotalRequests) / actualDuration.Seconds()

	// 计算响应时间统计
	p50, p95, p99 := stats.CalculatePercentiles()
	result.AvgResponseTime = time.Duration(atomic.LoadInt64((*int64)(&stats.totalTime)) / atomic.LoadInt64(&stats.count))
	result.MinResponseTime = stats.minTime
	result.MaxResponseTime = stats.maxTime
	result.P50ResponseTime = p50
	result.P95ResponseTime = p95
	result.P99ResponseTime = p99

	return result, nil
}

// 打印测试结果
func printResults(result *QPSBenchmarkResult, config QPSBenchmarkConfig) {
	fmt.Println("
=== DataMiddleware QPS极限基准测试结果 ===")
	fmt.Printf("测试配置:\n")
	fmt.Printf("  目标URL: %s\n", config.TargetURL)
	fmt.Printf("  并发数: %d\n", config.Concurrency)
	fmt.Printf("  测试时长: %v\n", config.Duration)
	fmt.Printf("  连接数: %d\n", config.Connections)
	fmt.Printf("  Keep-Alive: %t\n", !config.DisableKeepAlive)

	fmt.Printf("\n性能指标:\n")
	fmt.Printf("  总请求数: %d\n", result.TotalRequests)
	fmt.Printf("  成功请求数: %d\n", result.SuccessfulRequests)
	fmt.Printf("  失败请求数: %d\n", result.FailedRequests)
	fmt.Printf("  成功率: %.2f%%\n", float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("  QPS: %.2f req/sec\n", result.QPS)
	fmt.Printf("  平均响应时间: %v\n", result.AvgResponseTime)
	fmt.Printf("  最小响应时间: %v\n", result.MinResponseTime)
	fmt.Printf("  最大响应时间: %v\n", result.MaxResponseTime)
	fmt.Printf("  P50响应时间: %v\n", result.P50ResponseTime)
	fmt.Printf("  P95响应时间: %v\n", result.P95ResponseTime)
	fmt.Printf("  P99响应时间: %v\n", result.P99ResponseTime)

	// 目标对比
	fmt.Printf("\n目标对比:\n")
	targetQPS := 80000.0 // 8万QPS目标
	achievement := (result.QPS / targetQPS) * 100
	fmt.Printf("  设计目标: %.0f QPS\n", targetQPS)
	fmt.Printf("  实际达成: %.2f QPS\n", result.QPS)
	fmt.Printf("  达成率: %.1f%%\n", achievement)

	if result.QPS >= targetQPS {
		fmt.Printf("  ✅ 达到设计目标!\n")
	} else if result.QPS >= targetQPS*0.5 {
		fmt.Printf("  ⚠️ 接近设计目标，继续优化\n")
	} else {
		fmt.Printf("  ❌ 距离目标差距较大，需要优化\n")
	}

	fmt.Printf("\n系统信息:\n")
	fmt.Printf("  Go版本: %s\n", runtime.Version())
	fmt.Printf("  CPU核心数: %d\n", runtime.NumCPU())
	fmt.Printf("  Goroutines: %d\n", runtime.NumGoroutine())
	fmt.Printf("  测试时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run qps_limit_benchmark.go <并发数> [URL]")
		fmt.Println("示例: go run qps_limit_benchmark.go 100 http://localhost:8080/health")
		os.Exit(1)
	}

	concurrency, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("无效的并发数: %v", err)
	}

	targetURL := "http://localhost:8080/health"
	if len(os.Args) > 2 {
		targetURL = os.Args[2]
	}

	config := QPSBenchmarkConfig{
		TargetURL:       targetURL,
		Duration:        60 * time.Second, // 1分钟测试
		Concurrency:     concurrency,
		Connections:     concurrency * 2, // 连接数是并发数的2倍
		DisableKeepAlive: false,
		Timeout:         30 * time.Second,
	}

	log.Printf("准备测试配置: 并发=%d, URL=%s", concurrency, targetURL)

	result, err := RunQPSBenchmark(config)
	if err != nil {
		log.Fatalf("基准测试失败: %v", err)
	}

	printResults(result, config)
}
