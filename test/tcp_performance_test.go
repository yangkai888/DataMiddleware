package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/benchmark"
	"datamiddleware/internal/config"
	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

func main() {
	fmt.Println("🔌 TCP协议性能测试")
	fmt.Println("====================")

	// 初始化配置
	cfg, err := config.Init()
	if err != nil {
		fmt.Printf("❌ 配置初始化失败: %v\n", err)
		return
	}

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

	// 测试TCP性能
	runTCPPerformanceTest(log, cfg)
}

// runTCPPerformanceTest TCP协议性能测试
func runTCPPerformanceTest(log logger.Logger, cfg *types.Config) {
	tcpAddr := fmt.Sprintf("%s:%d", cfg.Server.TCP.Host, cfg.Server.TCP.Port)

	// TCP消息格式 (二进制协议)
	messages := []struct {
		name        string
		messageType uint16
		body        []byte
		gameID      string
		userID      string
		description string
	}{
		{
			name:        "player_login",
			messageType: 1001,
			body:        []byte(`{"player_id":1001,"action":"login","game_id":"game1"}`),
			gameID:      "game1",
			userID:      "user1001",
			description: "TCP玩家登录",
		},
		{
			name:        "player_sync",
			messageType: 1002,
			body:        []byte(`{"player_id":1001,"action":"sync","position":{"x":100,"y":200}}`),
			gameID:      "game1",
			userID:      "user1001",
			description: "TCP玩家数据同步",
		},
		{
			name:        "item_use",
			messageType: 2001,
			body:        []byte(`{"player_id":1001,"item_id":2001,"action":"use","quantity":1}`),
			gameID:      "game1",
			userID:      "user1001",
			description: "TCP道具使用",
		},
	}

	// 并发测试配置
	concurrencyLevels := []int{10, 50, 100, 200}

	for _, msg := range messages {
		fmt.Printf("\n🔌 TCP测试场景: %s (%s)\n", msg.name, msg.description)

		// 构建TCP消息
		message := buildTCPMessage(msg.messageType, msg.body, msg.gameID, msg.userID)

		for _, concurrency := range concurrencyLevels {
			fmt.Printf("  🔄 TCP并发数: %d\n", concurrency)

			result := runTCPBenchmark(tcpAddr, message, concurrency, 10*time.Second, log)

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
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))

			// 发送消息
			_, err = conn.Write(message)
			if err != nil {
				conn.Close()
				atomic.AddInt64(&result.FailedRequests, 1)
				continue
			}

			// 读取响应
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

// buildTCPMessage 构建TCP消息 (二进制协议格式，与BinaryCodec.Encode完全一致)
func buildTCPMessage(msgType uint16, body []byte, gameID, userID string) []byte {
	gameIDBytes := []byte(gameID)
	userIDBytes := []byte(userID)

	// 计算消息总长度
	gameIDLen := uint16(len(gameIDBytes))
	userIDLen := uint16(len(userIDBytes))
	bodyLen := uint32(len(body))

	// 固定头部长度: 版本(1) + 类型(2) + 标志(1) + 序列号(4) + 时间戳(8) + 体长度(4) + 校验和(4) + 游戏ID长度(2) + 用户ID长度(2)
	fixedHeaderLen := 1 + 2 + 1 + 4 + 8 + 4 + 4 + 2 + 2
	totalLen := fixedHeaderLen + int(gameIDLen) + int(userIDLen) + int(bodyLen)

	buffer := make([]byte, totalLen)
	offset := 0

	// 版本 (1字节)
	buffer[offset] = 1
	offset++

	// 类型 (2字节)
	binary.BigEndian.PutUint16(buffer[offset:offset+2], msgType)
	offset += 2

	// 标志 (1字节)
	buffer[offset] = 0
	offset++

	// 序列号 (4字节)
	binary.BigEndian.PutUint32(buffer[offset:offset+4], 1)
	offset += 4

	// 时间戳 (8字节)
	binary.BigEndian.PutUint64(buffer[offset:offset+8], uint64(time.Now().Unix()))
	offset += 8

	// 消息体长度 (4字节)
	binary.BigEndian.PutUint32(buffer[offset:offset+4], bodyLen)
	offset += 4

	// 校验和 (4字节) - 跳过，稍后填充
	checksumOffset := offset
	offset += 4

	// 游戏ID长度 (2字节)
	binary.BigEndian.PutUint16(buffer[offset:offset+2], gameIDLen)
	offset += 2

	// 用户ID长度 (2字节)
	binary.BigEndian.PutUint16(buffer[offset:offset+2], userIDLen)
	offset += 2

	// 游戏ID
	copy(buffer[offset:offset+int(gameIDLen)], gameIDBytes)
	offset += int(gameIDLen)

	// 用户ID
	copy(buffer[offset:offset+int(userIDLen)], userIDBytes)
	offset += int(userIDLen)

	// 消息体
	copy(buffer[offset:], body)

	// 计算校验和（与BinaryCodec.Encode完全一致）
	checksumData := make([]byte, 0, len(buffer)-4)
	checksumData = append(checksumData, buffer[:checksumOffset]...)   // 校验和字段之前的所有数据
	checksumData = append(checksumData, buffer[checksumOffset+4:]...) // 校验和字段之后的所有数据
	checksum := crc32.ChecksumIEEE(checksumData)

	// 写入校验和
	binary.BigEndian.PutUint32(buffer[checksumOffset:checksumOffset+4], checksum)

	return buffer
}

// calculateStats 计算统计信息
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
	if result.P95ResponseTime > 0 {
		fmt.Printf("%s📊 P95响应: %v\n", prefix, result.P95ResponseTime)
	}
	if result.P99ResponseTime > 0 {
		fmt.Printf("%s📊 P99响应: %v\n", prefix, result.P99ResponseTime)
	}
	fmt.Printf("%s💾 内存使用: %d MB\n", prefix, result.MemoryStats.Alloc/1024/1024)
}
