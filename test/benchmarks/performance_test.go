package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/benchmark"
	"datamiddleware/internal/config"
	"datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/common/types"
)

// 性能测试程序 - 测试数据中间件的并发量和QPS
func main() {
	fmt.Println("🚀 数据中间件性能测试程序")
	fmt.Println("=====================================")

	// 初始化日志
	log, err := logger.Init(types.LoggerConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})
	if err != nil {
		fmt.Printf("❌ 日志初始化失败: %v\n", err)
		return
	}

	// 初始化配置
	cfg, err := config.Init()
	if err != nil {
		fmt.Printf("❌ 配置初始化失败: %v\n", err)
		return
	}

	fmt.Printf("📋 测试目标服务器: HTTP %s:%d, TCP %s:%d\n",
		cfg.Server.HTTP.Host, cfg.Server.HTTP.Port,
		cfg.Server.TCP.Host, cfg.Server.TCP.Port)

	// 执行完整的性能测试套件
	runFullPerformanceTest(log, cfg)
}

// runFullPerformanceTest 运行完整的性能测试套件
func runFullPerformanceTest(log logger.Logger, cfg *config.Config) {
	fmt.Println("\n📊 开始完整性能测试套件...")

	// 测试阶段1: HTTP API性能测试
	fmt.Println("\n🏥 阶段1: HTTP API性能测试")
	runHTTPPerformanceTest(log, cfg)

	// 测试阶段2: TCP协议性能测试
	fmt.Println("\n🔌 阶段2: TCP协议性能测试")
	runTCPPerformanceTest(log, cfg)

	// 测试阶段3: 混合负载测试
	fmt.Println("\n🎭 阶段3: HTTP+TCP混合负载测试")
	runMixedLoadTest(log, cfg)

	// 测试阶段4: 极限压力测试
	fmt.Println("\n💥 阶段4: 极限压力测试")
	runStressTest(log, cfg)

	fmt.Println("\n🎉 性能测试套件执行完成！")
}

// HTTPPlayerRequest 玩家请求结构体
type HTTPPlayerRequest struct {
	PlayerID   int64  `json:"player_id"`
	Action     string `json:"action"`
	GameID     string `json:"game_id,omitempty"`
	AuthToken  string `json:"auth_token,omitempty"`
}

// HTTPItemRequest 道具请求结构体
type HTTPItemRequest struct {
	PlayerID int64  `json:"player_id"`
	ItemID   int64  `json:"item_id,omitempty"`
	Action   string `json:"action"`
	Quantity int32  `json:"quantity,omitempty"`
}

// HTTPOrderRequest 订单请求结构体
type HTTPOrderRequest struct {
	PlayerID int64   `json:"player_id"`
	Amount   float64 `json:"amount"`
	ItemID   int64   `json:"item_id"`
}

// runHTTPPerformanceTest HTTP API性能测试
func runHTTPPerformanceTest(log logger.Logger, cfg *config.Config) {
	baseURL := fmt.Sprintf("http://%s:%d", cfg.Server.HTTP.Host, cfg.Server.HTTP.Port)

	// 测试场景配置
	scenarios := []struct {
		name        string
		url         string
		method      string
		body        interface{}
		description string
	}{
		{
			name:        "player_login",
			url:         baseURL + "/api/game1/player/login",
			method:      "POST",
			body:        HTTPPlayerRequest{PlayerID: 1001, Action: "login", GameID: "game1"},
			description: "玩家登录",
		},
		{
			name:        "player_info",
			url:         baseURL + "/api/game1/player/1001",
			method:      "GET",
			body:        nil,
			description: "获取玩家信息",
		},
		{
			name:        "item_list",
			url:         baseURL + "/api/game1/player/1001/items",
			method:      "GET",
			body:        nil,
			description: "获取玩家道具列表",
		},
		{
			name:        "item_use",
			url:         baseURL + "/api/game1/player/1001/items/use",
			method:      "POST",
			body:        HTTPItemRequest{PlayerID: 1001, ItemID: 2001, Action: "use", Quantity: 1},
			description: "使用道具",
		},
		{
			name:        "order_create",
			url:         baseURL + "/api/game1/orders",
			method:      "POST",
			body:        HTTPOrderRequest{PlayerID: 1001, Amount: 99.99, ItemID: 2001},
			description: "创建订单",
		},
	}

	// 并发测试配置
	concurrencyLevels := []int{10, 50, 100, 200, 500, 1000}
	testDuration := 30 * time.Second

	for _, scenario := range scenarios {
		fmt.Printf("\n🎯 测试场景: %s (%s)\n", scenario.name, scenario.description)

		for _, concurrency := range concurrencyLevels {
			fmt.Printf("  🔄 并发数: %d\n", concurrency)

			config := benchmark.BenchmarkConfig{
				Concurrency:     concurrency,
				Duration:        testDuration,
				RequestInterval: time.Duration(1000000/concurrency) * time.Microsecond, // 动态调整间隔
				WarmupDuration:  5 * time.Second,
				Verbose:         false,
			}

			runner := benchmark.NewBenchmarkRunner(config, log)
			result, err := runner.RunHTTPBenchmark(scenario.url)
			if err != nil {
				fmt.Printf("    ❌ 测试失败: %v\n", err)
				continue
			}

			printBenchmarkResult(result, "    ")
		}
	}
}

// runTCPPerformanceTest TCP协议性能测试
func runTCPPerformanceTest(log logger.Logger, cfg *config.Config) {
	tcpAddr := fmt.Sprintf("%s:%d", cfg.Server.TCP.Host, cfg.Server.TCP.Port)

	// TCP消息格式 (自定义协议)
	// 消息头: [长度(4字节)] + [消息类型(2字节)] + [消息体]
	messages := []struct {
		name        string
		messageType uint16
		body        []byte
		description string
	}{
		{
			name:        "player_login",
			messageType: 1001,
			body:        []byte(`{"player_id":1001,"action":"login","game_id":"game1"}`),
			description: "TCP玩家登录",
		},
		{
			name:        "player_sync",
			messageType: 1002,
			body:        []byte(`{"player_id":1001,"action":"sync","position":{"x":100,"y":200}}`),
			description: "TCP玩家数据同步",
		},
		{
			name:        "item_use",
			messageType: 2001,
			body:        []byte(`{"player_id":1001,"item_id":2001,"action":"use","quantity":1}`),
			description: "TCP道具使用",
		},
	}

	// 并发测试配置
	concurrencyLevels := []int{50, 100, 200, 500, 1000}
	testDuration := 30 * time.Second

	for _, msg := range messages {
		fmt.Printf("\n🔌 TCP测试场景: %s (%s)\n", msg.name, msg.description)

		// 构建TCP消息
		bodyLen := len(msg.body)
		message := make([]byte, 6+bodyLen)
		// 长度 (4字节，大端序)
		message[0] = byte(bodyLen >> 24)
		message[1] = byte(bodyLen >> 16)
		message[2] = byte(bodyLen >> 8)
		message[3] = byte(bodyLen)
		// 消息类型 (2字节，大端序)
		message[4] = byte(msg.messageType >> 8)
		message[5] = byte(msg.messageType)
		// 消息体
		copy(message[6:], msg.body)

		for _, concurrency := range concurrencyLevels {
			fmt.Printf("  🔄 TCP并发数: %d\n", concurrency)

			result := runTCPBenchmark(tcpAddr, message, concurrency, testDuration, log)

			printBenchmarkResult(result, "    ")
		}
	}
}

// runTCPBenchmark 运行TCP基准测试
func runTCPBenchmark(addr string, message []byte, concurrency int, duration time.Duration, log logger.Logger) *benchmark.BenchmarkResult {
	result := &benchmark.BenchmarkResult{
		Config: benchmark.BenchmarkConfig{
			Concurrency: concurrency,
			Duration:    duration,
		},
		StartTime: time.Now(),
		Errors:    make(map[string]int64),
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	responseTimes := make([]time.Duration, 0, 100000)

	// 启动TCP工作协程
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			tcpWorker(ctx, addr, message, result, &responseTimes, workerID)
		}(i)
	}

	wg.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// 计算统计信息
	calculateStats(result, responseTimes)

	return result
}

// tcpWorker TCP测试工作协程
func tcpWorker(ctx context.Context, addr string, message []byte, result *benchmark.BenchmarkResult, responseTimes *[]time.Duration, workerID int) {
	ticker := time.NewTicker(10 * time.Millisecond) // 每10ms发送一个请求
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()

			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err != nil {
				atomic.AddInt64(&result.FailedRequests, 1)
				continue
			}

			// 设置读写超时
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

			// 发送消息
			_, err = conn.Write(message)
			if err != nil {
				conn.Close()
				atomic.AddInt64(&result.FailedRequests, 1)
				continue
			}

			// 读取响应 (简单读取，实际应该解析协议)
			buffer := make([]byte, 1024)
			_, err = conn.Read(buffer)
			conn.Close()

			responseTime := time.Since(start)
			*responseTimes = append(*responseTimes, responseTime)

			if err != nil {
				atomic.AddInt64(&result.FailedRequests, 1)
			} else {
				atomic.AddInt64(&result.SuccessRequests, 1)
			}
		}
	}
}

// runMixedLoadTest 混合负载测试
func runMixedLoadTest(log logger.Logger, cfg *config.Config) {
	fmt.Println("混合负载测试: HTTP和TCP同时运行...")

	// 同时运行HTTP和TCP测试
	httpURL := fmt.Sprintf("http://%s:%d/api/game1/player/1001", cfg.Server.HTTP.Host, cfg.Server.HTTP.Port)
	tcpAddr := fmt.Sprintf("%s:%d", cfg.Server.TCP.Host, cfg.Server.TCP.Port)

	// 混合测试配置
	mixedConfigs := []struct {
		httpConcurrency int
		tcpConcurrency  int
		duration        time.Duration
		description     string
	}{
		{50, 50, 30 * time.Second, "HTTP 50并发 + TCP 50并发"},
		{100, 100, 30 * time.Second, "HTTP 100并发 + TCP 100并发"},
		{200, 200, 30 * time.Second, "HTTP 200并发 + TCP 200并发"},
	}

	for _, config := range mixedConfigs {
		fmt.Printf("\n🎭 %s\n", config.description)

		var wg sync.WaitGroup
		var httpResult, tcpResult *benchmark.BenchmarkResult

		// HTTP测试
		wg.Add(1)
		go func() {
			defer wg.Done()
			httpConfig := benchmark.BenchmarkConfig{
				Concurrency:     config.httpConcurrency,
				Duration:        config.duration,
				RequestInterval: time.Duration(1000000/config.httpConcurrency) * time.Microsecond,
				WarmupDuration:  3 * time.Second,
				Verbose:         false,
			}
			runner := benchmark.NewBenchmarkRunner(httpConfig, log)
			result, err := runner.RunHTTPBenchmark(httpURL)
			if err == nil {
				httpResult = result
			}
		}()

		// TCP测试
		wg.Add(1)
		go func() {
			defer wg.Done()
			tcpMessage := []byte{0, 0, 0, 10, 0, 1, '{', '"', 't', 'e', 's', 't', '"', ':', '1', '}'}
			tcpResult = runTCPBenchmark(tcpAddr, tcpMessage, config.tcpConcurrency, config.duration, log)
		}()

		wg.Wait()

		// 输出结果
		if httpResult != nil {
			fmt.Printf("  🌐 HTTP结果: QPS=%.2f, 平均响应=%v\n",
				httpResult.QPS, httpResult.AvgResponseTime)
		}
		if tcpResult != nil {
			fmt.Printf("  🔌 TCP结果: QPS=%.2f, 平均响应=%v\n",
				tcpResult.QPS, tcpResult.AvgResponseTime)
		}
	}
}

// runStressTest 极限压力测试
func runStressTest(log logger.Logger, cfg *config.Config) {
	fmt.Println("极限压力测试: 逐步增加负载至系统极限...")

	baseURL := fmt.Sprintf("http://%s:%d/api/game1/player/1001", cfg.Server.HTTP.Host, cfg.Server.HTTP.Port)

	// 逐步增加并发数的压力测试
	maxConcurrency := 2000
	step := 200
	testDuration := 20 * time.Second

	fmt.Printf("逐步增加并发数: 200 → %d\n", maxConcurrency)

	for concurrency := 200; concurrency <= maxConcurrency; concurrency += step {
		fmt.Printf("\n💥 压力测试 - 并发数: %d\n", concurrency)

		config := benchmark.BenchmarkConfig{
			Concurrency:     concurrency,
			Duration:        testDuration,
			RequestInterval: time.Duration(1000000/concurrency) * time.Microsecond,
			WarmupDuration:  2 * time.Second,
			Verbose:         false,
		}

		runner := benchmark.NewBenchmarkRunner(config, log)
		result, err := runner.RunHTTPBenchmark(baseURL)
		if err != nil {
			fmt.Printf("  ❌ 测试失败: %v\n", err)
			break
		}

		printBenchmarkResult(result, "  ")

		// 如果失败率太高，停止测试
		if result.FailedRequests > result.SuccessRequests/10 { // 失败率超过10%
			fmt.Printf("  ⚠️  失败率过高 (%d/%d)，停止压力测试\n",
				result.FailedRequests, result.TotalRequests)
			break
		}

		// 如果平均响应时间超过1秒，停止测试
		if result.AvgResponseTime > time.Second {
			fmt.Printf("  ⚠️  响应时间过长 (%v)，停止压力测试\n", result.AvgResponseTime)
			break
		}
	}
}

// calculateStats 计算统计信息 (复制自benchmark包以避免依赖问题)
func calculateStats(result *benchmark.BenchmarkResult, responseTimes []time.Duration) {
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
	result.MinResponseTime = time.Duration(1<<63 - 1) // Max duration
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

		// 计算百分位数 (简化版本)
		sortedTimes := make([]time.Duration, len(responseTimes))
		copy(sortedTimes, responseTimes)

		// 简单排序
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
	result.MemoryStats = benchmark.MemoryStats{
		Alloc:         memStats.Alloc,
		TotalAlloc:    memStats.TotalAlloc,
		Sys:           memStats.Sys,
		NumGC:         memStats.NumGC,
		GCCPUFraction: memStats.GCCPUFraction,
	}
}

// printBenchmarkResult 打印基准测试结果
func printBenchmarkResult(result *benchmark.BenchmarkResult, prefix string) {
	fmt.Printf("%s✅ 总请求: %d\n", prefix, result.TotalRequests)
	fmt.Printf("%s✅ 成功请求: %d\n", prefix, result.SuccessRequests)
	fmt.Printf("%s❌ 失败请求: %d\n", prefix, result.FailedRequests)
	fmt.Printf("%s🚀 QPS: %.2f\n", prefix, result.QPS)
	fmt.Printf("%s⏱️  平均响应: %v\n", prefix, result.AvgResponseTime)
	fmt.Printf("%s📊 P95响应: %v\n", prefix, result.P95ResponseTime)
	fmt.Printf("%s📊 P99响应: %v\n", prefix, result.P99ResponseTime)
	fmt.Printf("%s💾 内存使用: %d MB\n", prefix, result.MemoryStats.Alloc/1024/1024)
}
