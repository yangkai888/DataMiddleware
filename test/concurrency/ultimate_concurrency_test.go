package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SystemStats 系统统计信息
type SystemStats struct {
	CPUUsage    float64
	MemoryUsage float64
	Connections int64
	Goroutines  int64
}

// ConcurrencyTest 高并发测试
type ConcurrencyTest struct {
	// 测试配置
	tcpConnections  int
	httpConnections int
	testDuration    time.Duration
	messageSize     int

	// 统计信息
	tcpConnected    int64
	tcpMessages     int64
	httpRequests    int64
	httpResponses   int64
	tcpErrors       int64
	httpErrors      int64

	// 控制
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewConcurrencyTest 创建并发测试
func NewConcurrencyTest(tcpConn, httpConn int, duration time.Duration) *ConcurrencyTest {
	return &ConcurrencyTest{
		tcpConnections:  tcpConn,
		httpConnections: httpConn,
		testDuration:    duration,
		messageSize:     1024, // 1KB消息
		stopChan:        make(chan struct{}),
	}
}

// GetSystemStats 获取系统统计信息
func GetSystemStats() SystemStats {
	stats := SystemStats{
		Goroutines: int64(runtime.NumGoroutine()),
	}

	// 获取CPU使用率
	if cpu, err := getCPUUsage(); err == nil {
		stats.CPUUsage = cpu
	}

	// 获取内存使用率
	if mem, err := getMemoryUsage(); err == nil {
		stats.MemoryUsage = mem
	}

	// 获取连接数
	if conn, err := getConnectionCount(); err == nil {
		stats.Connections = conn
	}

	return stats
}

// getCPUUsage 获取CPU使用率
func getCPUUsage() (float64, error) {
	cmd := exec.Command("bash", "-c", "top -bn1 | grep 'Cpu(s)' | sed \"s/.*, *\\([0-9.]*\\)%* id.*/\\1/\" | awk '{print 100 - $1}'")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	cpuStr := strings.TrimSpace(string(output))
	return strconv.ParseFloat(cpuStr, 64)
}

// getMemoryUsage 获取内存使用率
func getMemoryUsage() (float64, error) {
	cmd := exec.Command("bash", "-c", "free | grep Mem | awk '{printf \"%.2f\", $3*100/$2}'")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	memStr := strings.TrimSpace(string(output))
	return strconv.ParseFloat(memStr, 64)
}

// getConnectionCount 获取TCP连接数
func getConnectionCount() (int64, error) {
	cmd := exec.Command("bash", "-c", "netstat -ant 2>/dev/null | grep ':8080\\|:9090' | grep ESTABLISHED | wc -l")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	connStr := strings.TrimSpace(string(output))
	return strconv.ParseInt(connStr, 10, 64)
}

// RunTCPTest 运行TCP并发测试
func (ct *ConcurrencyTest) RunTCPTest() {
	log.Printf("🔥 开始TCP并发测试: %d连接, 持续%s", ct.tcpConnections, ct.testDuration)

	// 启动监控协程
	go ct.monitorSystem()

	// 计算每批次启动的连接数，避免瞬间压力过大
	batchSize := 100
	batches := (ct.tcpConnections + batchSize - 1) / batchSize

	for batch := 0; batch < batches; batch++ {
		start := batch * batchSize
		end := start + batchSize
		if end > ct.tcpConnections {
			end = ct.tcpConnections
		}

		// 启动一批连接
		for i := start; i < end; i++ {
			ct.wg.Add(1)
			go ct.runTCPConnection(i)
		}

		// 批次间暂停，避免瞬间压力
		time.Sleep(50 * time.Millisecond)
	}

	// 等待测试时长
	time.Sleep(ct.testDuration)

	// 停止所有连接
	close(ct.stopChan)
	ct.wg.Wait()

	ct.printTCPResults()
}

// runTCPConnection 运行单个TCP连接
func (ct *ConcurrencyTest) runTCPConnection(id int) {
	defer ct.wg.Done()

	conn, err := net.DialTimeout("tcp", "localhost:9090", 5*time.Second)
	if err != nil {
		atomic.AddInt64(&ct.tcpErrors, 1)
		return
	}
	defer conn.Close()

	atomic.AddInt64(&ct.tcpConnected, 1)

	// 设置超时
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	// 创建消息
	message := make([]byte, ct.messageSize)
	for i := range message {
		message[i] = byte(id % 256) // 使用连接ID作为消息内容标识
	}

	// 发送消息循环
	ticker := time.NewTicker(100 * time.Millisecond) // 每100ms发送一次
	defer ticker.Stop()

	for {
		select {
		case <-ct.stopChan:
			return
		case <-ticker.C:
			// 发送消息
			_, err := conn.Write(message)
			if err != nil {
				atomic.AddInt64(&ct.tcpErrors, 1)
				return
			}
			atomic.AddInt64(&ct.tcpMessages, 1)

			// 读取响应
			response := make([]byte, 1024)
			_, err = conn.Read(response)
			if err != nil && err != io.EOF {
				atomic.AddInt64(&ct.tcpErrors, 1)
				return
			}
		}
	}
}

// RunHTTPTest 运行HTTP并发测试
func (ct *ConcurrencyTest) RunHTTPTest() {
	log.Printf("🌐 开始HTTP并发测试: %d连接, 持续%s", ct.httpConnections, ct.testDuration)

	// 初始化HTTP客户端池
	clients := make([]*http.Client, ct.httpConnections)
	for i := range clients {
		clients[i] = &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	// 启动监控
	go ct.monitorSystem()

	// 启动HTTP请求协程
	for i := 0; i < ct.httpConnections; i++ {
		ct.wg.Add(1)
		go ct.runHTTPWorker(i, clients[i])
	}

	// 等待测试时长
	time.Sleep(ct.testDuration)

	// 停止测试
	close(ct.stopChan)
	ct.wg.Wait()

	ct.printHTTPResults()
}

// runHTTPWorker 运行HTTP工作协程
func (ct *ConcurrencyTest) runHTTPWorker(id int, client *http.Client) {
	defer ct.wg.Done()

	ticker := time.NewTicker(200 * time.Millisecond) // 每200ms发送一次请求
	defer ticker.Stop()

	for {
		select {
		case <-ct.stopChan:
			return
		case <-ticker.C:
			atomic.AddInt64(&ct.httpRequests, 1)

			// 发送HTTP请求
			resp, err := client.Get("http://localhost:8080/health")
			if err != nil {
				atomic.AddInt64(&ct.httpErrors, 1)
				continue
			}

			// 读取响应体
			_, err = io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				atomic.AddInt64(&ct.httpErrors, 1)
				continue
			}

			atomic.AddInt64(&ct.httpResponses, 1)
		}
	}
}

// RunMixedTest 运行混合负载测试
func (ct *ConcurrencyTest) RunMixedTest() {
	log.Printf("🔄 开始混合负载测试: TCP %d + HTTP %d, 持续%s",
		ct.tcpConnections, ct.httpConnections, ct.testDuration)

	// 并行运行TCP和HTTP测试
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ct.RunTCPTest()
	}()

	go func() {
		defer wg.Done()
		ct.RunHTTPTest()
	}()

	wg.Wait()
	ct.printMixedResults()
}

// monitorSystem 监控系统状态
func (ct *ConcurrencyTest) monitorSystem() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	log.Println("📊 开始系统监控...")

	for {
		select {
		case <-ct.stopChan:
			log.Println("📊 系统监控已停止")
			return
		case <-ticker.C:
			stats := GetSystemStats()

			log.Printf("📊 系统状态 - CPU: %.1f%%, 内存: %.1f%%, 连接: %d, Goroutines: %d",
				stats.CPUUsage, stats.MemoryUsage, stats.Connections, stats.Goroutines)
		}
	}
}

// printTCPResults 打印TCP测试结果
func (ct *ConcurrencyTest) printTCPResults() {
	duration := ct.testDuration.Seconds()
	connected := atomic.LoadInt64(&ct.tcpConnected)
	messages := atomic.LoadInt64(&ct.tcpMessages)
	errors := atomic.LoadInt64(&ct.tcpErrors)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🎯 TCP高并发测试结果")
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("测试时长: %.1f秒\n", duration)
	fmt.Printf("目标连接数: %d\n", ct.tcpConnections)
	fmt.Printf("成功连接数: %d\n", connected)
	fmt.Printf("连接成功率: %.2f%%\n", float64(connected)/float64(ct.tcpConnections)*100)

	fmt.Printf("发送消息数: %d\n", messages)
	fmt.Printf("消息发送速率: %.0f msg/sec\n", float64(messages)/duration)

	totalBytes := messages * int64(ct.messageSize)
	fmt.Printf("总传输字节: %d MB\n", totalBytes/(1024*1024))
	fmt.Printf("网络吞吐量: %.2f MB/sec\n", float64(totalBytes)/(duration*1024*1024))

	fmt.Printf("错误数量: %d\n", errors)
	if messages > 0 {
		fmt.Printf("错误率: %.4f%%\n", float64(errors)/float64(messages)*100)
	}
}

// printHTTPResults 打印HTTP测试结果
func (ct *ConcurrencyTest) printHTTPResults() {
	duration := ct.testDuration.Seconds()
	requests := atomic.LoadInt64(&ct.httpRequests)
	responses := atomic.LoadInt64(&ct.httpResponses)
	errors := atomic.LoadInt64(&ct.httpErrors)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🌐 HTTP高并发测试结果")
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("测试时长: %.1f秒\n", duration)
	fmt.Printf("并发连接数: %d\n", ct.httpConnections)

	fmt.Printf("发送请求数: %d\n", requests)
	fmt.Printf("接收响应数: %d\n", responses)
	fmt.Printf("QPS (实际): %.0f req/sec\n", float64(requests)/duration)
	fmt.Printf("QPS (成功): %.0f req/sec\n", float64(responses)/duration)

	fmt.Printf("错误数量: %d\n", errors)
	if requests > 0 {
		fmt.Printf("错误率: %.2f%%\n", float64(errors)/float64(requests)*100)
	}
}

// printMixedResults 打印混合测试结果
func (ct *ConcurrencyTest) printMixedResults() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🔄 混合负载测试汇总")
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("总并发连接: %d (TCP %d + HTTP %d)\n",
		ct.tcpConnections+ct.httpConnections, ct.tcpConnections, ct.httpConnections)

	duration := ct.testDuration.Seconds()
	tcpMessages := atomic.LoadInt64(&ct.tcpMessages)
	httpRequests := atomic.LoadInt64(&ct.httpRequests)

	fmt.Printf("TCP消息速率: %.0f msg/sec\n", float64(tcpMessages)/duration)
	fmt.Printf("HTTP请求速率: %.0f req/sec\n", float64(httpRequests)/duration)
	fmt.Printf("总操作速率: %.0f op/sec\n", float64(tcpMessages+httpRequests)/duration)

	// 系统资源评估
	stats := GetSystemStats()
	fmt.Printf("\n系统资源使用:\n")
	fmt.Printf("  CPU使用率: %.1f%%\n", stats.CPUUsage)
	fmt.Printf("  内存使用率: %.1f%%\n", stats.MemoryUsage)
	fmt.Printf("  活跃连接数: %d\n", stats.Connections)
	fmt.Printf("  Goroutines数: %d\n", stats.Goroutines)
}

// RunUltimateTest 运行终极并发测试
func RunUltimateTest() {
	log.Println("🚀 开始单机高并发极限测试")
	log.Println("测试场景：")
	log.Println("1. TCP 1000连接测试")
	log.Println("2. TCP 5000连接测试")
	log.Println("3. HTTP 1000并发测试")
	log.Println("4. 混合负载测试")

	// 测试场景
	tests := []struct {
		name string
		tcp  int
		http int
		dur  time.Duration
	}{
		{"TCP 1000连接", 1000, 0, 30 * time.Second},
		{"TCP 5000连接", 5000, 0, 45 * time.Second},
		{"HTTP 1000并发", 0, 1000, 30 * time.Second},
		{"混合负载测试", 2000, 500, 45 * time.Second},
	}

	for i, test := range tests {
		log.Printf("\n🎯 测试场景 %d/%d: %s", i+1, len(tests), test.name)

		concurrencyTest := NewConcurrencyTest(test.tcp, test.http, test.dur)

		if test.tcp > 0 && test.http == 0 {
			concurrencyTest.RunTCPTest()
		} else if test.http > 0 && test.tcp == 0 {
			concurrencyTest.RunHTTPTest()
		} else {
			concurrencyTest.RunMixedTest()
		}

		// 测试间暂停
		if i < len(tests)-1 {
			log.Println("⏳ 准备下一个测试场景...")
			time.Sleep(10 * time.Second)
		}
	}

	log.Println("\n🎉 所有高并发测试完成！")
}

func main() {
	// 检查命令行参数
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "tcp":
			if len(os.Args) >= 3 {
				if conn, err := strconv.Atoi(os.Args[2]); err == nil {
					test := NewConcurrencyTest(conn, 0, 30*time.Second)
					test.RunTCPTest()
					return
				}
			}
		case "http":
			if len(os.Args) >= 3 {
				if conn, err := strconv.Atoi(os.Args[2]); err == nil {
					test := NewConcurrencyTest(0, conn, 30*time.Second)
					test.RunHTTPTest()
					return
				}
			}
		case "mixed":
			if len(os.Args) >= 4 {
				if tcpConn, err := strconv.Atoi(os.Args[2]); err == nil {
					if httpConn, err := strconv.Atoi(os.Args[3]); err == nil {
						test := NewConcurrencyTest(tcpConn, httpConn, 30*time.Second)
						test.RunMixedTest()
						return
					}
				}
			}
		}

		fmt.Println("用法:")
		fmt.Println("  ./ultimate_concurrency_test          # 运行完整测试套件")
		fmt.Println("  ./ultimate_concurrency_test tcp N    # TCP N连接测试")
		fmt.Println("  ./ultimate_concurrency_test http N   # HTTP N并发测试")
		fmt.Println("  ./ultimate_concurrency_test mixed T H # TCP T连接 + HTTP H并发测试")
		return
	}

	// 运行完整测试套件
	RunUltimateTest()
}
