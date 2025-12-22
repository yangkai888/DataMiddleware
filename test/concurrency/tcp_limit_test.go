// Package concurrency TCP连接极限测试
// 测试DataMiddleware单机TCP并发连接极限
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// 测试配置
type TCPConcurrencyConfig struct {
	TargetAddr    string
	MaxConnections int
	TestDuration  time.Duration
	ConnectTimeout time.Duration
	ReadTimeout   time.Duration
}

// 测试结果
type TCPConcurrencyResult struct {
	TotalAttempts      int64
	SuccessfulConnections int64
	FailedConnections int64
	ActiveConnections int64
	SuccessRate       float64
	AverageConnectTime time.Duration
	MinConnectTime     time.Duration
	MaxConnectTime     time.Duration
	StartTime          time.Time
	EndTime            time.Duration
	Errors             []string
}

// 连接统计
type ConnectionStats struct {
	mu            sync.Mutex
	connectTimes  []time.Duration
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

	s.connectTimes = append(s.connectTimes, duration)
	if s.minTime == 0 || duration < s.minTime {
		s.minTime = duration
	}
	if duration > s.maxTime {
		s.maxTime = duration
	}
}

// 单个TCP连接worker
func tcpConnectionWorker(id int, config TCPConcurrencyConfig, stats *ConnectionStats, results *TCPConcurrencyResult, semaphore chan struct{}) {
	defer atomic.AddInt64(&results.TotalAttempts, 1)

	// 获取信号量，控制并发
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	start := time.Now()

	conn, err := net.DialTimeout("tcp", config.TargetAddr, config.ConnectTimeout)
	connectTime := time.Since(start)

	if err != nil {
		atomic.AddInt64(&results.FailedConnections, 1)

		// 记录错误（限制错误数量）
		results.mu.Lock()
		if len(results.Errors) < 10 {
			results.Errors = append(results.Errors, fmt.Sprintf("Worker %d: %v", id, err))
		}
		results.mu.Unlock()
		return
	}

	atomic.AddInt64(&results.SuccessfulConnections, 1)
	atomic.AddInt64(&results.ActiveConnections, 1)
	stats.Add(connectTime)

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))

	// 发送测试消息
	message := fmt.Sprintf("TEST%d\n", id)
	_, err = conn.Write([]byte(message))
	if err != nil {
		atomic.AddInt64(&results.FailedConnections, 1)
		atomic.AddInt64(&results.SuccessfulConnections, -1)
		conn.Close()
		atomic.AddInt64(&results.ActiveConnections, -1)
		return
	}

	// 读取响应
	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	conn.Close()
	atomic.AddInt64(&results.ActiveConnections, -1)

	if err != nil {
		atomic.AddInt64(&results.FailedConnections, 1)
		atomic.AddInt64(&results.SuccessfulConnections, -1)
	}
}

// 运行TCP并发测试
func RunTCPConcurrencyTest(config TCPConcurrencyConfig) (*TCPConcurrencyResult, error) {
	log.Printf("开始TCP并发测试: 目标=%s, 最大连接=%d, 时长=%v",
		config.TargetAddr, config.MaxConnections, config.TestDuration)

	result := &TCPConcurrencyResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	stats := &ConnectionStats{}

	// 控制并发度的信号量 (最多1000个并发连接尝试)
	semaphore := make(chan struct{}, 1000)

	var wg sync.WaitGroup

	// 逐步增加连接数，找到极限
	batchSize := 1000
	currentConnections := 0

	for currentConnections < config.MaxConnections {
		batchEnd := currentConnections + batchSize
		if batchEnd > config.MaxConnections {
			batchEnd = config.MaxConnections
		}

		log.Printf("测试连接数范围: %d-%d", currentConnections+1, batchEnd)

		// 启动这一批的连接测试
		for i := currentConnections; i < batchEnd; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				tcpConnectionWorker(workerID, config, stats, result, semaphore)
			}(i)
		}

		wg.Wait()

		currentConnections = batchEnd

		// 检查成功率，如果太低则停止
		total := atomic.LoadInt64(&result.TotalAttempts)
		success := atomic.LoadInt64(&result.SuccessfulConnections)

		if total > 0 {
			successRate := float64(success) / float64(total) * 100
			log.Printf("当前成功率: %.2f%% (%d/%d)", successRate, success, total)

			if successRate < 50 {
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
	success := atomic.LoadInt64(&result.SuccessfulConnections)
	result.SuccessRate = float64(success) / float64(total) * 100

	if stats.count > 0 {
		result.AverageConnectTime = time.Duration(atomic.LoadInt64((*int64)(&stats.totalTime)) / atomic.LoadInt64(&stats.count))
		result.MinConnectTime = stats.minTime
		result.MaxConnectTime = stats.maxTime
	}

	return result, nil
}

// 打印TCP测试结果
func printTCPResults(result *TCPConcurrencyResult, config TCPConcurrencyConfig) {
	fmt.Println("
=== DataMiddleware TCP连接极限测试结果 ===")
	fmt.Printf("测试配置:\n")
	fmt.Printf("  目标地址: %s\n", config.TargetAddr)
	fmt.Printf("  最大连接数: %d\n", config.MaxConnections)
	fmt.Printf("  测试时长: %v\n", config.TestDuration)
	fmt.Printf("  连接超时: %v\n", config.ConnectTimeout)

	fmt.Printf("\n连接统计:\n")
	fmt.Printf("  总尝试数: %d\n", result.TotalAttempts)
	fmt.Printf("  成功连接数: %d\n", result.SuccessfulConnections)
	fmt.Printf("  失败连接数: %d\n", result.FailedConnections)
	fmt.Printf("  当前活跃连接: %d\n", result.ActiveConnections)
	fmt.Printf("  成功率: %.2f%%\n", result.SuccessRate)

	if result.AverageConnectTime > 0 {
		fmt.Printf("\n连接时间统计:\n")
		fmt.Printf("  平均连接时间: %v\n", result.AverageConnectTime)
		fmt.Printf("  最快连接时间: %v\n", result.MinConnectTime)
		fmt.Printf("  最慢连接时间: %v\n", result.MaxConnectTime)
	}

	fmt.Printf("\n目标对比:\n")
	targetConnections := 200000.0 // 20万连接目标
	achievement := (float64(result.SuccessfulConnections) / targetConnections) * 100
	fmt.Printf("  设计目标: %.0f 并发连接\n", targetConnections)
	fmt.Printf("  实际达成: %d 并发连接\n", result.SuccessfulConnections)
	fmt.Printf("  达成率: %.1f%%\n", achievement)

	if float64(result.SuccessfulConnections) >= targetConnections {
		fmt.Printf("  ✅ 达到设计目标!\n")
	} else if float64(result.SuccessfulConnections) >= targetConnections*0.5 {
		fmt.Printf("  ⚠️ 接近设计目标，继续优化\n")
	} else {
		fmt.Printf("  ❌ 距离目标差距较大，需要优化\n")
	}

	fmt.Printf("\n系统限制分析:\n")
	fmt.Printf("  文件描述符限制: ulimit -n $(ulimit -n)\n")
	fmt.Printf("  测试时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	if len(result.Errors) > 0 {
		fmt.Printf("\n常见错误:\n")
		for i, err := range result.Errors {
			if i >= 5 { // 只显示前5个错误
				break
			}
			fmt.Printf("  - %s\n", err)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run tcp_limit_test.go <最大连接数> [地址]")
		fmt.Println("示例: go run tcp_limit_test.go 10000 localhost:9090")
		os.Exit(1)
	}

	maxConnections, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("无效的最大连接数: %v", err)
	}

	targetAddr := "localhost:9090"
	if len(os.Args) > 2 {
		targetAddr = os.Args[2]
	}

	config := TCPConcurrencyConfig{
		TargetAddr:     targetAddr,
		MaxConnections: maxConnections,
		TestDuration:   5 * time.Minute, // 5分钟测试时间
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:   3 * time.Second,
	}

	log.Printf("准备TCP连接极限测试: 最大连接=%d, 目标=%s", maxConnections, targetAddr)

	result, err := RunTCPConcurrencyTest(config)
	if err != nil {
		log.Fatalf("TCP并发测试失败: %v", err)
	}

	printTCPResults(result, config)
}
