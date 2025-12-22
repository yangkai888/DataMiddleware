// Package concurrency HTTP并发极限测试
// 测试DataMiddleware单机HTTP并发连接极限
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// 测试配置
type HTTPConcurrencyConfig struct {
	TargetAddr     string
	MaxConnections int
	TestDuration   time.Duration
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
}

// 测试结果
type HTTPConcurrencyResult struct {
	TotalAttempts      int64
	SuccessfulRequests int64
	FailedRequests     int64
	QPS                float64
	AvgResponseTime    time.Duration
	MinResponseTime    time.Duration
	MaxResponseTime    time.Duration
	StartTime          time.Time
	EndTime            time.Duration
	Errors             []string
	mu                 sync.Mutex
}

// 连接统计
type ConnectionStats struct {
	mu            sync.Mutex
	responseTimes []time.Duration
	totalTime     time.Duration
	minTime       time.Duration
	maxTime       time.Duration
	count         int64
}

func (s *ConnectionStats) Add(duration time.Duration) {
	atomic.AddInt64(&s.count, 1)
	atomic.AddInt64((*int64)(&s.totalTime), int64(duration))

	s.mu.Lock()
	defer s.mu.Unlock()

	s.responseTimes = append(s.responseTimes, duration)
	if s.minTime == 0 || duration < s.minTime {
		s.minTime = duration
	}
	if duration > s.maxTime {
		s.maxTime = duration
	}
}

// HTTP请求worker
func httpRequestWorker(id int, config HTTPConcurrencyConfig, client *http.Client, stats *ConnectionStats, results *HTTPConcurrencyResult, semaphore chan struct{}) {
	defer atomic.AddInt64(&results.TotalAttempts, 1)

	// 获取信号量，控制并发
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	start := time.Now()

	// 创建请求
	req, err := http.NewRequest("GET", config.TargetAddr, nil)
	if err != nil {
		atomic.AddInt64(&results.FailedRequests, 1)
		return
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	// 发送请求
	resp, err := client.Do(req)
	responseTime := time.Since(start)

	if err != nil {
		atomic.AddInt64(&results.FailedRequests, 1)

		// 记录错误（限制错误数量）
		results.mu.Lock()
		if len(results.Errors) < 5 {
			results.Errors = append(results.Errors, fmt.Sprintf("Worker %d: %v", id, err))
		}
		results.mu.Unlock()
		return
	}

	atomic.AddInt64(&results.SuccessfulRequests, 1)
	stats.Add(responseTime)

	// 读取响应体
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)

	if err != nil {
		atomic.AddInt64(&results.FailedRequests, 1)
		atomic.AddInt64(&results.SuccessfulRequests, -1)
	}
}

// HTTP客户端工厂
func createHTTPClient(config HTTPConcurrencyConfig) *http.Client {
	return &http.Client{
		Timeout: config.ReadTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        0, // 不保持空闲连接
			MaxIdleConnsPerHost: 0,
			DisableKeepAlives:   true, // 每次请求都建立新连接
		},
	}
}

// 运行HTTP并发测试
func RunHTTPConcurrencyTest(config HTTPConcurrencyConfig) (*HTTPConcurrencyResult, error) {
	log.Printf("开始HTTP并发测试: 目标=%s, 最大连接=%d, 时长=%v",
		config.TargetAddr, config.MaxConnections, config.TestDuration)

	result := &HTTPConcurrencyResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	stats := &ConnectionStats{}

	// 创建HTTP客户端
	client := createHTTPClient(config)

	// 控制并发度的信号量 (最多500个并发请求)
	semaphore := make(chan struct{}, 500)

	var wg sync.WaitGroup

	// 逐步增加连接数，找到极限
	batchSize := 50
	currentConnections := 0

	for currentConnections < config.MaxConnections {
		batchEnd := currentConnections + batchSize
		if batchEnd > config.MaxConnections {
			batchEnd = config.MaxConnections
		}

		log.Printf("测试连接数范围: %d-%d", currentConnections+1, batchEnd)

		// 启动这一批的请求测试
		for i := currentConnections; i < batchEnd; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				httpRequestWorker(workerID, config, client, stats, result, semaphore)
			}(i)
		}

		wg.Wait()

		currentConnections = batchEnd

		// 检查成功率，如果太低则停止
		total := atomic.LoadInt64(&result.TotalAttempts)
		success := atomic.LoadInt64(&result.SuccessfulRequests)

		if total > 0 {
			successRate := float64(success) / float64(total) * 100
			log.Printf("当前成功率: %.2f%% (%d/%d)", successRate, success, total)

			if successRate < 70 {
				log.Printf("成功率过低 (%.2f%%)，可能已达到系统极限", successRate)
				break
			}
		}

		// 检查是否超时
		if time.Since(result.StartTime) > config.TestDuration {
			log.Printf("测试时间已到，停止测试")
			break
		}
	}

	result.EndTime = time.Since(result.StartTime)

	// 计算最终统计
	total := atomic.LoadInt64(&result.TotalAttempts)
	success := atomic.LoadInt64(&result.SuccessfulRequests)
	result.QPS = float64(success) / result.EndTime.Seconds()

	if stats.count > 0 {
		result.AvgResponseTime = time.Duration(atomic.LoadInt64((*int64)(&stats.totalTime)) / atomic.LoadInt64(&stats.count))
		result.MinResponseTime = stats.minTime
		result.MaxResponseTime = stats.maxTime
	}

	return result, nil
}

// 打印HTTP并发测试结果
func printHTTPConcurrencyResults(result *HTTPConcurrencyResult, config HTTPConcurrencyConfig) {
	fmt.Println("")
	fmt.Println("=== DataMiddleware HTTP并发极限测试结果 ===")
	fmt.Printf("测试配置:\n")
	fmt.Printf("  目标地址: %s\n", config.TargetAddr)
	fmt.Printf("  最大连接数: %d\n", config.MaxConnections)
	fmt.Printf("  测试时长: %v\n", config.TestDuration)
	fmt.Printf("  连接超时: %v\n", config.ConnectTimeout)

	fmt.Printf("\n连接统计:\n")
	fmt.Printf("  总尝试数: %d\n", result.TotalAttempts)
	fmt.Printf("  成功请求数: %d\n", result.SuccessfulRequests)
	fmt.Printf("  失败请求数: %d\n", result.FailedRequests)
	fmt.Printf("  成功率: %.2f%%\n", float64(result.SuccessfulRequests)/float64(result.TotalAttempts)*100)
	fmt.Printf("  实际QPS: %.2f req/sec\n", result.QPS)

	if result.AvgResponseTime > 0 {
		fmt.Printf("\n响应时间统计:\n")
		fmt.Printf("  平均响应时间: %v\n", result.AvgResponseTime)
		fmt.Printf("  最快响应时间: %v\n", result.MinResponseTime)
		fmt.Printf("  最慢响应时间: %v\n", result.MaxResponseTime)
	}

	// 性能评估
	fmt.Printf("\n性能评估:\n")
	if result.QPS > 5000 {
		fmt.Printf("  性能等级: 优秀 (QPS > 5000)\n")
	} else if result.QPS > 2000 {
		fmt.Printf("  性能等级: 良好 (QPS > 2000)\n")
	} else if result.QPS > 1000 {
		fmt.Printf("  性能等级: 一般 (QPS > 1000)\n")
	} else {
		fmt.Printf("  性能等级: 待优化 (QPS < 1000)\n")
	}

	fmt.Printf("  测试并发上限: %d\n", result.TotalAttempts)
	fmt.Printf("  系统处理能力: %.0f req/sec\n", result.QPS)

	fmt.Printf("\n系统信息:\n")
	fmt.Printf("  测试时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	if len(result.Errors) > 0 {
		fmt.Printf("\n常见错误:\n")
		for i, err := range result.Errors {
			if i >= 3 { // 只显示前3个错误
				break
			}
			fmt.Printf("  - %s\n", err)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run http_concurrency_test.go <最大连接数> [地址]")
		fmt.Println("示例: go run http_concurrency_test.go 1000 http://localhost:8080/health")
		os.Exit(1)
	}

	maxConnections, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("无效的最大连接数: %v", err)
	}

	targetAddr := "http://localhost:8080/health"
	if len(os.Args) > 2 {
		targetAddr = os.Args[2]
	}

	config := HTTPConcurrencyConfig{
		TargetAddr:     targetAddr,
		MaxConnections: maxConnections,
		TestDuration:   3 * time.Minute, // 3分钟测试时间
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    30 * time.Second,
	}

	log.Printf("准备HTTP并发极限测试: 最大连接=%d, 目标=%s", maxConnections, targetAddr)

	result, err := RunHTTPConcurrencyTest(config)
	if err != nil {
		log.Fatalf("HTTP并发测试失败: %v", err)
	}

	printHTTPConcurrencyResults(result, config)
}
