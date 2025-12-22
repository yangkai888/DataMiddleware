// Package benchmarks TCP QPS极限基准测试
// 测试DataMiddleware单机TCP QPS性能极限
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/common/types"
)

// TCP测试配置
type TCPBenchmarkConfig struct {
	TargetAddr       string
	Duration         time.Duration
	Concurrency      int
	MessageType      types.MessageType
	GameID           string
	UserID           string
	MessageBody      []byte
	ConnectTimeout   time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
}

// TCP测试结果
type TCPBenchmarkResult struct {
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
	TotalConnections   int64
	Errors             []string
	StartTime          time.Time
	EndTime            time.Time
	mu                 sync.Mutex
}

// 响应时间统计
type TCPResponseTimeStats struct {
	mu         sync.Mutex
	times      []time.Duration
	totalTime  time.Duration
	minTime    time.Duration
	maxTime    time.Duration
	count      int64
}

func (s *TCPResponseTimeStats) Add(duration time.Duration) {
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

func (s *TCPResponseTimeStats) CalculatePercentiles() (p50, p95, p99 time.Duration) {
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

// 创建TCP消息 (使用二进制协议，与服务器BinaryCodec兼容)
func createTCPMessage(config TCPBenchmarkConfig, sequenceID uint32) ([]byte, error) {
	header := types.MessageHeader{
		Version:    types.ProtocolVersion,
		Type:       config.MessageType,
		Flags:      types.FlagNeedResponse,
		SequenceID: sequenceID,
		GameID:     config.GameID,
		UserID:     config.UserID,
		Timestamp:  time.Now().Unix(),
		BodyLength: uint32(len(config.MessageBody)),
	}

	// 准备字符串数据
	gameIDBytes := []byte(header.GameID)
	userIDBytes := []byte(header.UserID)

	// 计算消息总长度
	gameIDLen := uint16(len(gameIDBytes))
	userIDLen := uint16(len(userIDBytes))
	bodyLen := uint32(len(config.MessageBody))

	// 固定头部长度: 版本(1) + 类型(2) + 标志(1) + 序列号(4) + 时间戳(8) + 体长度(4) + 校验和(4) + 游戏ID长度(2) + 用户ID长度(2)
	fixedHeaderLen := 1 + 2 + 1 + 4 + 8 + 4 + 4 + 2 + 2
	totalLen := fixedHeaderLen + int(gameIDLen) + int(userIDLen) + int(bodyLen)

	buffer := make([]byte, totalLen)
	offset := 0

	// 版本
	buffer[offset] = header.Version
	offset++

	// 类型
	binary.BigEndian.PutUint16(buffer[offset:offset+2], uint16(header.Type))
	offset += 2

	// 标志
	buffer[offset] = byte(header.Flags)
	offset++

	// 序列号
	binary.BigEndian.PutUint32(buffer[offset:offset+4], header.SequenceID)
	offset += 4

	// 时间戳
	binary.BigEndian.PutUint64(buffer[offset:offset+8], uint64(header.Timestamp))
	offset += 8

	// 消息体长度
	binary.BigEndian.PutUint32(buffer[offset:offset+4], bodyLen)
	offset += 4

	// 计算校验和 (暂时设为0，稍后计算)
	checksumOffset := offset
	binary.BigEndian.PutUint32(buffer[offset:offset+4], 0)
	offset += 4

	// 游戏ID长度
	binary.BigEndian.PutUint16(buffer[offset:offset+2], gameIDLen)
	offset += 2

	// 用户ID长度
	binary.BigEndian.PutUint16(buffer[offset:offset+2], userIDLen)
	offset += 2

	// 游戏ID
	copy(buffer[offset:offset+int(gameIDLen)], gameIDBytes)
	offset += int(gameIDLen)

	// 用户ID
	copy(buffer[offset:offset+int(userIDLen)], userIDBytes)
	offset += int(userIDLen)

	// 消息体
	copy(buffer[offset:], config.MessageBody)

	// 计算校验和 (对整个消息进行CRC32)
	checksum := crc32.ChecksumIEEE(buffer[:totalLen])
	binary.BigEndian.PutUint32(buffer[checksumOffset:checksumOffset+4], checksum)

	return buffer, nil
}

// TCP客户端worker
func tcpWorker(id int, config TCPBenchmarkConfig, stats *TCPResponseTimeStats, results *TCPBenchmarkResult, semaphore chan struct{}) {
	defer atomic.AddInt64(&results.TotalRequests, 1)

	// 获取信号量，控制并发
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	start := time.Now()

	// 建立TCP连接
	conn, err := net.DialTimeout("tcp", config.TargetAddr, config.ConnectTimeout)
	if err != nil {
		atomic.AddInt64(&results.FailedRequests, 1)

		results.mu.Lock()
		if len(results.Errors) < 5 {
			results.Errors = append(results.Errors, fmt.Sprintf("Worker %d: 连接失败: %v", id, err))
		}
		results.mu.Unlock()
		return
	}

	atomic.AddInt64(&results.TotalConnections, 1)
	defer conn.Close()

	// 设置连接超时
	conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))

	// 发送消息
	message, err := createTCPMessage(config, uint32(id))
	if err != nil {
		atomic.AddInt64(&results.FailedRequests, 1)
		return
	}

	_, err = conn.Write(message)
	if err != nil {
		atomic.AddInt64(&results.FailedRequests, 1)
		return
	}

	// 读取响应
	responseBuffer := make([]byte, 8192)
	_, err = conn.Read(responseBuffer)
	responseTime := time.Since(start)

	if err != nil {
		atomic.AddInt64(&results.FailedRequests, 1)
	} else {
		atomic.AddInt64(&results.SuccessfulRequests, 1)
		stats.Add(responseTime)
	}
}

// 运行TCP基准测试
func RunTCPBenchmark(config TCPBenchmarkConfig) (*TCPBenchmarkResult, error) {
	log.Printf("开始TCP基准测试: 并发=%d, 时长=%v, 目标=%s",
		config.Concurrency, config.Duration, config.TargetAddr)

	result := &TCPBenchmarkResult{
		StartTime: time.Now(),
		Errors:    make([]string, 0),
	}

	stats := &TCPResponseTimeStats{}

	// 控制并发度的信号量 (最多500个并发TCP连接)
	semaphore := make(chan struct{}, 500)

	var wg sync.WaitGroup

	// 启动workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			tcpWorker(workerID, config, stats, result, semaphore)
		}(i)
	}

	wg.Wait()
	result.EndTime = time.Now()

	// 计算结果
	actualDuration := result.EndTime.Sub(result.StartTime)
	result.QPS = float64(result.TotalRequests) / actualDuration.Seconds()

	// 计算响应时间统计
	p50, p95, p99 := stats.CalculatePercentiles()
	if stats.count > 0 {
		result.AvgResponseTime = time.Duration(atomic.LoadInt64((*int64)(&stats.totalTime)) / atomic.LoadInt64(&stats.count))
	}
	result.MinResponseTime = stats.minTime
	result.MaxResponseTime = stats.maxTime
	result.P50ResponseTime = p50
	result.P95ResponseTime = p95
	result.P99ResponseTime = p99

	return result, nil
}

// 打印TCP测试结果
func printTCPResults(result *TCPBenchmarkResult, config TCPBenchmarkConfig) {
	fmt.Println("")
	fmt.Println("=== DataMiddleware TCP QPS极限基准测试结果 ===")
	fmt.Printf("测试配置:\n")
	fmt.Printf("  目标地址: %s\n", config.TargetAddr)
	fmt.Printf("  消息类型: %d\n", config.MessageType)
	fmt.Printf("  并发数: %d\n", config.Concurrency)
	fmt.Printf("  测试时长: %v\n", config.Duration)
	fmt.Printf("  游戏ID: %s\n", config.GameID)
	fmt.Printf("  用户ID: %s\n", config.UserID)

	fmt.Printf("\n连接统计:\n")
	fmt.Printf("  总请求数: %d\n", result.TotalRequests)
	fmt.Printf("  成功请求数: %d\n", result.SuccessfulRequests)
	fmt.Printf("  失败请求数: %d\n", result.FailedRequests)
	fmt.Printf("  成功率: %.2f%%\n", float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("  总连接数: %d\n", result.TotalConnections)
	fmt.Printf("  QPS: %.2f req/sec\n", result.QPS)

	if result.AvgResponseTime > 0 {
		fmt.Printf("\n响应时间统计:\n")
		fmt.Printf("  平均响应时间: %v\n", result.AvgResponseTime)
		fmt.Printf("  最快响应时间: %v\n", result.MinResponseTime)
		fmt.Printf("  最慢响应时间: %v\n", result.MaxResponseTime)
		fmt.Printf("  P50响应时间: %v\n", result.P50ResponseTime)
		fmt.Printf("  P95响应时间: %v\n", result.P95ResponseTime)
		fmt.Printf("  P99响应时间: %v\n", result.P99ResponseTime)
	}

	// 目标对比
	fmt.Printf("\n目标对比:\n")
	targetQPS := 100000.0 // 10万QPS目标
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
	fmt.Printf("  测试时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	if len(result.Errors) > 0 {
		fmt.Printf("\n错误信息:\n")
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
		fmt.Println("用法: go run tcp_qps_benchmark.go <并发数> [地址] [时长秒] [消息类型]")
		fmt.Println("示例: go run tcp_qps_benchmark.go 100 localhost:9090 60 4097")
		fmt.Println("消息类型: 4097=心跳, 4098=握手, 4353=玩家登录, 4354=玩家数据")
		os.Exit(1)
	}

	concurrency, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("无效的并发数: %v", err)
	}

	targetAddr := "localhost:9090"
	if len(os.Args) > 2 {
		targetAddr = os.Args[2]
	}

	duration := 30 * time.Second
	if len(os.Args) > 3 {
		if d, err := strconv.Atoi(os.Args[3]); err == nil {
			duration = time.Duration(d) * time.Second
		}
	}

	messageType := types.MessageTypeHeartbeat // 默认心跳消息
	if len(os.Args) > 4 {
		if mt, err := strconv.Atoi(os.Args[4]); err == nil {
			messageType = types.MessageType(mt)
		}
	}

	// 创建测试消息体
	var messageBody []byte
	switch messageType {
	case types.MessageTypeHeartbeat:
		messageBody = []byte(`{"type":"ping"}`)
	case types.MessageTypePlayerLogin:
		messageBody = []byte(`{"username":"testuser","password":"testpass"}`)
	case types.MessageTypePlayerData:
		messageBody = []byte(`{"action":"get_player_info","player_id":"12345"}`)
	default:
		messageBody = []byte(`{"type":"test"}`)
	}

	config := TCPBenchmarkConfig{
		TargetAddr:     targetAddr,
		Duration:       duration,
		Concurrency:    concurrency,
		MessageType:    messageType,
		GameID:         "game1",
		UserID:         fmt.Sprintf("user_%d", concurrency),
		MessageBody:    messageBody,
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   5 * time.Second,
	}

	log.Printf("准备TCP QPS测试: 并发=%d, 地址=%s, 时长=%v, 消息类型=%d",
		concurrency, targetAddr, duration, messageType)

	result, err := RunTCPBenchmark(config)
	if err != nil {
		log.Fatalf("TCP基准测试失败: %v", err)
	}

	printTCPResults(result, config)
}
