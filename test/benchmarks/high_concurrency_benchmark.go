package test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// HighConcurrencyBenchmark 单机高并发基准测试
type HighConcurrencyBenchmark struct {
	// 测试配置
	config BenchmarkConfig

	// 统计信息
	stats BenchmarkStats

	// 控制信号
	ctx    context.Context
	cancel context.CancelFunc

	// HTTP客户端池
	httpClients []*http.Client
}

// BenchmarkConfig 测试配置
type BenchmarkConfig struct {
	// TCP测试配置
	TCPConnections    int           // TCP并发连接数
	TCPDuration       time.Duration // TCP测试时长
	TCPMessageSize    int           // 每条消息大小
	TCPMessageRate    int           // 每秒消息发送率

	// HTTP测试配置
	HTTPConnections   int           // HTTP并发连接数
	HTTPDuration      time.Duration // HTTP测试时长
	HTTPRequestRate   int           // 每秒请求率
	HTTPURL           string        // 测试URL

	// 混合测试配置
	MixedConnections  int           // 混合负载连接数
	MixedDuration     time.Duration // 混合测试时长

	// 系统配置
	MaxWorkers        int           // 最大工作协程数
	ReportInterval    time.Duration // 报告间隔
}

// BenchmarkStats 测试统计
type BenchmarkStats struct {
	// TCP统计
	TCPConnectionsAttempted int64
	TCPConnectionsSuccess   int64
	TCPMessagesSent         int64
	TCPMessagesReceived     int64
	TCPBytesSent            int64
	TCPBytesReceived        int64
	TCPErrors               int64

	// HTTP统计
	HTTPRequestSent         int64
	HTTPResponsesReceived   int64
	HTTPBytesSent           int64
	HTTPBytesReceived       int64
	HTTPErrors              int64
	HTTPAvgResponseTime     int64 // 纳秒

	// 系统统计
	CPUUsage                float64
	MemoryUsage             uint64
	Goroutines              int64

	// 时间统计
	StartTime               time.Time
	EndTime                 time.Time
}

// NewHighConcurrencyBenchmark 创建高并发基准测试
func NewHighConcurrencyBenchmark(config BenchmarkConfig) *HighConcurrencyBenchmark {
	ctx, cancel := context.WithCancel(context.Background())

	return &HighConcurrencyBenchmark{
		config: config,
		ctx:    ctx,
		cancel: cancel,
		stats: BenchmarkStats{
			StartTime: time.Now(),
		},
	}
}

// RunTCPBenchmark 运行TCP基准测试
func (b *HighConcurrencyBenchmark) RunTCPBenchmark() error {
	log.Printf("🚀 开始TCP高并发测试: %d连接, 持续%s",
		b.config.TCPConnections, b.config.TCPDuration)

	// 创建工作池
	workerPool := make(chan struct{}, b.config.MaxWorkers)
	var wg sync.WaitGroup

	// 消息数据
	messageData := b.generateMessageData()

	// 启动监控协程
	go b.monitorSystemStats()

	// 启动TCP连接测试
	for i := 0; i < b.config.TCPConnections; i++ {
		wg.Add(1)
		go func(connID int) {
			defer wg.Done()

			workerPool <- struct{}{} // 获取工作槽
			defer func() { <-workerPool }()

			b.runTCPConnection(connID, messageData)
		}(i)

		// 控制连接创建速率，避免瞬间压力过大
		if i%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// 等待测试时长
	time.Sleep(b.config.TCPDuration)

	// 停止所有连接
	b.cancel()

	// 等待所有协程完成
	wg.Wait()

	b.stats.EndTime = time.Now()
	b.printTCPResults()

	return nil
}

// runTCPConnection 运行单个TCP连接测试
func (b *HighConcurrencyBenchmark) runTCPConnection(connID int, messageData []byte) {
	atomic.AddInt64(&b.stats.TCPConnectionsAttempted, 1)

	conn, err := net.DialTimeout("tcp", "localhost:9090", 5*time.Second)
	if err != nil {
		atomic.AddInt64(&b.stats.TCPErrors, 1)
		return
	}
	defer conn.Close()

	atomic.AddInt64(&b.stats.TCPConnectionsSuccess, 1)

	// 设置超时
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	messageCount := 0
	ticker := time.NewTicker(time.Second / time.Duration(b.config.TCPMessageRate))
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			// 发送消息
			_, err := conn.Write(messageData)
			if err != nil {
				atomic.AddInt64(&b.stats.TCPErrors, 1)
				return
			}

			atomic.AddInt64(&b.stats.TCPMessagesSent, 1)
			atomic.AddInt64(&b.stats.TCPBytesSent, int64(len(messageData)))

			// 读取响应
			response := make([]byte, 1024)
			n, err := conn.Read(response)
			if err != nil {
				atomic.AddInt64(&b.stats.TCPErrors, 1)
				return
			}

			atomic.AddInt64(&b.stats.TCPMessagesReceived, 1)
			atomic.AddInt64(&b.stats.TCPBytesReceived, int64(n))

			messageCount++
		}
	}
}

// RunHTTPBenchmark 运行HTTP基准测试
func (b *HighConcurrencyBenchmark) RunHTTPBenchmark() error {
	log.Printf("🚀 开始HTTP高并发测试: %d连接, 持续%s",
		b.config.HTTPConnections, b.config.HTTPDuration)

	// 初始化HTTP客户端池
	b.initHTTPClients()

	var wg sync.WaitGroup
	workerPool := make(chan struct{}, b.config.MaxWorkers)

	// 启动监控
	go b.monitorSystemStats()

	// 启动HTTP请求测试
	for i := 0; i < b.config.HTTPConnections; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			workerPool <- struct{}{}
			defer func() { <-workerPool }()

			b.runHTTPWorker(workerID)
		}(i)
	}

	// 等待测试时长
	time.Sleep(b.config.HTTPDuration)
	b.cancel()

	// 等待所有协程完成
	wg.Wait()

	b.stats.EndTime = time.Now()
	b.printHTTPResults()

	return nil
}

// runHTTPWorker 运行HTTP工作协程
func (b *HighConcurrencyBenchmark) runHTTPWorker(workerID int) {
	client := b.httpClients[workerID % len(b.httpClients)]
	ticker := time.NewTicker(time.Second / time.Duration(b.config.HTTPRequestRate))
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()

			// 发送HTTP请求
			req, _ := http.NewRequest("GET", b.config.HTTPURL, nil)
			req.Header.Set("User-Agent", "BenchmarkClient/1.0")

			atomic.AddInt64(&b.stats.HTTPRequestSent, 1)

			resp, err := client.Do(req)
			if err != nil {
				atomic.AddInt64(&b.stats.HTTPErrors, 1)
				continue
			}

			// 读取响应体
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				atomic.AddInt64(&b.stats.HTTPErrors, 1)
				continue
			}

			atomic.AddInt64(&b.stats.HTTPResponsesReceived, 1)
			atomic.AddInt64(&b.stats.HTTPBytesReceived, int64(len(body)))

			// 计算响应时间
			responseTime := time.Since(start).Nanoseconds()
			atomic.AddInt64(&b.stats.HTTPAvgResponseTime, responseTime)
		}
	}
}

// RunMixedBenchmark 运行混合负载测试
func (b *HighConcurrencyBenchmark) RunMixedBenchmark() error {
	log.Printf("🚀 开始混合负载测试: %d连接, 持续%s",
		b.config.MixedConnections, b.config.MixedDuration)

	// 混合测试：50% TCP + 50% HTTP
	tcpConnections := b.config.MixedConnections / 2
	httpConnections := b.config.MixedConnections / 2

	// 更新配置
	b.config.TCPConnections = tcpConnections
	b.config.HTTPConnections = httpConnections
	b.config.TCPDuration = b.config.MixedDuration
	b.config.HTTPDuration = b.config.MixedDuration

	// 并行运行TCP和HTTP测试
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		b.RunTCPBenchmark()
	}()

	go func() {
		defer wg.Done()
		b.RunHTTPBenchmark()
	}()

	wg.Wait()

	b.printMixedResults()
	return nil
}

// initHTTPClients 初始化HTTP客户端池
func (b *HighConcurrencyBenchmark) initHTTPClients() {
	b.httpClients = make([]*http.Client, b.config.HTTPConnections)

	for i := range b.httpClients {
		b.httpClients[i] = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}
}

// generateMessageData 生成测试消息数据
func (b *HighConcurrencyBenchmark) generateMessageData() []byte {
	data := make([]byte, b.config.TCPMessageSize)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	return data
}

// monitorSystemStats 监控系统状态
func (b *HighConcurrencyBenchmark) monitorSystemStats() {
	ticker := time.NewTicker(b.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			// 获取系统统计
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			atomic.StoreInt64(&b.stats.Goroutines, int64(runtime.NumGoroutine()))
			atomic.StoreUint64(&b.stats.MemoryUsage, m.Alloc)

			// 简单的CPU使用率估算（这里是简化的实现）
			// 实际项目中应该使用更精确的CPU监控
			atomic.StoreFloat64(&b.stats.CPUUsage, 0.0) // TODO: 实现CPU监控

			b.printProgress()
		}
	}
}

// printProgress 打印进度信息
func (b *HighConcurrencyBenchmark) printProgress() {
	elapsed := time.Since(b.stats.StartTime)

	log.Printf("📊 测试进度 - 运行时间: %v, Goroutines: %d, 内存: %d MB",
		elapsed.Truncate(time.Second),
		atomic.LoadInt64(&b.stats.Goroutines),
		atomic.LoadUint64(&b.stats.MemoryUsage)/(1024*1024))
}

// printTCPResults 打印TCP测试结果
func (b *HighConcurrencyBenchmark) printTCPResults() {
	duration := b.stats.EndTime.Sub(b.stats.StartTime)

	fmt.Println("\n" + "="*80)
	fmt.Println("🎯 TCP高并发测试结果")
	fmt.Println("="*80)

	fmt.Printf("测试时长: %.2f秒\n", duration.Seconds())
	fmt.Printf("尝试连接: %d\n", atomic.LoadInt64(&b.stats.TCPConnectionsAttempted))
	fmt.Printf("成功连接: %d\n", atomic.LoadInt64(&b.stats.TCPConnectionsSuccess))
	fmt.Printf("连接成功率: %.2f%%\n",
		float64(atomic.LoadInt64(&b.stats.TCPConnectionsSuccess))/
		float64(atomic.LoadInt64(&b.stats.TCPConnectionsAttempted))*100)

	fmt.Printf("发送消息: %d\n", atomic.LoadInt64(&b.stats.TCPMessagesSent))
	fmt.Printf("接收消息: %d\n", atomic.LoadInt64(&b.stats.TCPMessagesReceived))
	fmt.Printf("发送字节: %d MB\n", atomic.LoadInt64(&b.stats.TCPBytesSent)/(1024*1024))
	fmt.Printf("接收字节: %d MB\n", atomic.LoadInt64(&b.stats.TCPBytesReceived)/(1024*1024))

	messagesPerSec := float64(atomic.LoadInt64(&b.stats.TCPMessagesSent)) / duration.Seconds()
	fmt.Printf("消息吞吐量: %.0f msg/sec\n", messagesPerSec)

	bytesPerSec := float64(atomic.LoadInt64(&b.stats.TCPBytesSent)) / duration.Seconds()
	fmt.Printf("网络吞吐量: %.2f MB/sec\n", bytesPerSec/(1024*1024))

	fmt.Printf("错误数量: %d\n", atomic.LoadInt64(&b.stats.TCPErrors))
	fmt.Printf("错误率: %.2f%%\n",
		float64(atomic.LoadInt64(&b.stats.TCPErrors))/
		float64(atomic.LoadInt64(&b.stats.TCPMessagesSent))*100)
}

// printHTTPResults 打印HTTP测试结果
func (b *HighConcurrencyBenchmark) printHTTPResults() {
	duration := b.stats.EndTime.Sub(b.stats.StartTime)

	fmt.Println("\n" + "="*80)
	fmt.Println("🌐 HTTP高并发测试结果")
	fmt.Println("="*80)

	fmt.Printf("测试时长: %.2f秒\n", duration.Seconds())
	fmt.Printf("发送请求: %d\n", atomic.LoadInt64(&b.stats.HTTPRequestSent))
	fmt.Printf("接收响应: %d\n", atomic.LoadInt64(&b.stats.HTTPResponsesReceived))

	requestsPerSec := float64(atomic.LoadInt64(&b.stats.HTTPRequestSent)) / duration.Seconds()
	fmt.Printf("QPS: %.0f req/sec\n", requestsPerSec)

	avgResponseTime := float64(atomic.LoadInt64(&b.stats.HTTPAvgResponseTime)) /
		float64(atomic.LoadInt64(&b.stats.HTTPResponsesReceived)) / 1000000 // 转换为毫秒

	fmt.Printf("平均响应时间: %.2f ms\n", avgResponseTime)

	fmt.Printf("发送字节: %d MB\n", atomic.LoadInt64(&b.stats.HTTPBytesSent)/(1024*1024))
	fmt.Printf("接收字节: %d MB\n", atomic.LoadInt64(&b.stats.HTTPBytesReceived)/(1024*1024))

	fmt.Printf("错误数量: %d\n", atomic.LoadInt64(&b.stats.HTTPErrors))
	fmt.Printf("错误率: %.2f%%\n",
		float64(atomic.LoadInt64(&b.stats.HTTPErrors))/
		float64(atomic.LoadInt64(&b.stats.HTTPRequestSent))*100)
}

// printMixedResults 打印混合测试结果
func (b *HighConcurrencyBenchmark) printMixedResults() {
	fmt.Println("\n" + "="*80)
	fmt.Println("🔄 混合负载测试结果")
	fmt.Println("="*80)

	// 合并显示TCP和HTTP结果
	b.printTCPResults()
	b.printHTTPResults()
}

// RunFullBenchmark 运行完整基准测试
func (b *HighConcurrencyBenchmark) RunFullBenchmark() error {
	log.Println("🚀 开始完整高并发基准测试")

	// 1. TCP测试
	log.Println("📡 第一阶段：TCP连接测试")
	if err := b.RunTCPBenchmark(); err != nil {
		return fmt.Errorf("TCP测试失败: %w", err)
	}

	// 重置统计
	b.stats = BenchmarkStats{StartTime: time.Now()}

	// 2. HTTP测试
	log.Println("🌐 第二阶段：HTTP请求测试")
	if err := b.RunHTTPBenchmark(); err != nil {
		return fmt.Errorf("HTTP测试失败: %w", err)
	}

	// 重置统计
	b.stats = BenchmarkStats{StartTime: time.Now()}

	// 3. 混合测试
	log.Println("🔄 第三阶段：混合负载测试")
	if err := b.RunMixedBenchmark(); err != nil {
		return fmt.Errorf("混合测试失败: %w", err)
	}

	log.Println("✅ 完整基准测试完成")
	return nil
}
