package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"datamiddleware/test"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 定义不同的测试配置
	configs := []test.BenchmarkConfig{
		// TCP测试配置
		{
			TCPConnections:    1000,
			TCPDuration:       30 * time.Second,
			TCPMessageSize:    1024,
			TCPMessageRate:    10,
			MaxWorkers:        100,
			ReportInterval:    5 * time.Second,
		},
		// HTTP测试配置
		{
			HTTPConnections:   500,
			HTTPDuration:      30 * time.Second,
			HTTPRequestRate:   50,
			HTTPURL:           "http://localhost:8080/health",
			MaxWorkers:        50,
			ReportInterval:    5 * time.Second,
		},
		// 高负载TCP测试
		{
			TCPConnections:    2000,
			TCPDuration:       60 * time.Second,
			TCPMessageSize:    512,
			TCPMessageRate:    20,
			MaxWorkers:        200,
			ReportInterval:    10 * time.Second,
		},
		// 混合负载测试
		{
			MixedConnections:  1000,
			MixedDuration:     45 * time.Second,
			MaxWorkers:        150,
			ReportInterval:    5 * time.Second,
		},
	}

	log.Println("🎯 单机高并发极限测试")
	log.Println("测试场景：")
	log.Println("1. TCP 1000连接测试 (30秒)")
	log.Println("2. HTTP 500连接测试 (30秒)")
	log.Println("3. TCP 2000连接压力测试 (60秒)")
	log.Println("4. 混合负载测试 (45秒)")

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 运行所有测试
	for i, config := range configs {
		log.Printf("\n🔥 开始测试场景 %d/%d", i+1, len(configs))

		benchmark := test.NewHighConcurrencyBenchmark(config)

		// 根据配置类型运行相应测试
		if config.TCPConnections > 0 && config.HTTPConnections == 0 {
			// 纯TCP测试
			if err := benchmark.RunTCPBenchmark(); err != nil {
				log.Printf("❌ TCP测试失败: %v", err)
				continue
			}
		} else if config.HTTPConnections > 0 && config.TCPConnections == 0 {
			// 纯HTTP测试
			if err := benchmark.RunHTTPBenchmark(); err != nil {
				log.Printf("❌ HTTP测试失败: %v", err)
				continue
			}
		} else if config.MixedConnections > 0 {
			// 混合测试
			if err := benchmark.RunMixedBenchmark(); err != nil {
				log.Printf("❌ 混合测试失败: %v", err)
				continue
			}
		}

		log.Printf("✅ 测试场景 %d 完成", i+1)

		// 测试间隔
		if i < len(configs)-1 {
			log.Println("⏳ 准备下一个测试场景...")
			time.Sleep(5 * time.Second)
		}
	}

	log.Println("\n🎉 所有高并发测试完成！")
	log.Println("📊 查看上方详细的性能指标和统计信息")

	// 等待用户查看结果
	log.Println("按 Ctrl+C 退出...")
	<-sigChan
	log.Println("👋 测试程序退出")
}
