package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/benchmark"
	"datamiddleware/internal/cache"
	"datamiddleware/internal/config"
	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

func main() {
	// 初始化日志
	log, err := logger.Init(types.LoggerConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})
	if err != nil {
		fmt.Printf("日志初始化失败: %v\n", err)
		return
	}

	fmt.Println("开始性能测试套件演示...")

	// 初始化配置
	cfg, err := config.Init()
	if err != nil {
		fmt.Printf("配置初始化失败: %v\n", err)
		return
	}

	// 初始化缓存管理器
	cacheManager, err := cache.NewManager(cfg.Cache, log)
	if err != nil {
		fmt.Printf("缓存管理器初始化失败: %v\n", err)
		return
	}
	defer cacheManager.Close()

	// 测试1: 缓存基准测试
	fmt.Println("\n=== 测试1: 缓存基准测试 ===")

	benchConfig := benchmark.BenchmarkConfig{
		Concurrency:     10,
		Duration:        3 * time.Second,
		RequestInterval: 100 * time.Millisecond,
		WarmupDuration:  500 * time.Millisecond,
		Verbose:         false,
	}

	runner := benchmark.NewBenchmarkRunner(benchConfig, log)

	fmt.Printf("开始缓存基准测试: 并发数=%d, 持续时间=%v\n",
		benchConfig.Concurrency, benchConfig.Duration)

	cacheResult, err := runner.RunCacheBenchmark(cacheManager)
	if err != nil {
		fmt.Printf("缓存基准测试失败: %v\n", err)
		return
	}

	fmt.Printf("缓存测试结果:\n")
	fmt.Printf("  总请求数: %d\n", cacheResult.TotalRequests)
	fmt.Printf("  成功请求: %d\n", cacheResult.SuccessRequests)
	fmt.Printf("  失败请求: %d\n", cacheResult.FailedRequests)
	fmt.Printf("  QPS: %.2f\n", cacheResult.QPS)
	fmt.Printf("  平均响应时间: %v\n", cacheResult.AvgResponseTime)
	fmt.Printf("  内存使用: %d MB\n", cacheResult.MemoryStats.Alloc/1024/1024)

	// 测试2: 简单压力测试演示
	fmt.Println("\n=== 测试2: 简单压力测试演示 ===")

	fmt.Println("演示基本的压力测试概念...")

	// 简单的并发测试
	concurrency := 20
	requestsPerWorker := 50

	fmt.Printf("并发数: %d, 每个工作协程请求数: %d\n", concurrency, requestsPerWorker)

	var totalRequests int64
	var totalTime time.Duration

	start := time.Now()

	// 启动多个协程进行测试
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				key := fmt.Sprintf("test_key_%d_%d", workerID, j)
				value := []byte(fmt.Sprintf("test_value_%d_%d", workerID, time.Now().UnixNano()))

				// 执行缓存操作
				cacheManager.Set(key, value)
				atomic.AddInt64(&totalRequests, 1)
			}
		}(i)
	}

	wg.Wait()
	totalTime = time.Since(start)

	qps := float64(totalRequests) / totalTime.Seconds()
	fmt.Printf("测试完成:\n")
	fmt.Printf("  总请求数: %d\n", totalRequests)
	fmt.Printf("  总时间: %v\n", totalTime)
	fmt.Printf("  QPS: %.2f\n", qps)

	// 测试3: 性能对比测试
	fmt.Println("\n=== 测试3: 性能对比测试 ===")

	fmt.Println("对比不同并发度下的缓存性能...")

	concurrencies := []int{5, 10, 20}

	for _, conc := range concurrencies {
		fmt.Printf("\n测试并发数: %d\n", conc)

		config := benchmark.BenchmarkConfig{
			Concurrency:     conc,
			Duration:        2 * time.Second,
			RequestInterval: time.Duration(1000/conc) * time.Millisecond, // 动态调整间隔
			WarmupDuration:  200 * time.Millisecond,
			Verbose:         false,
		}

		testRunner := benchmark.NewBenchmarkRunner(config, log)
		result, err := testRunner.RunCacheBenchmark(cacheManager)
		if err != nil {
			fmt.Printf("  测试失败: %v\n", err)
			continue
		}

		fmt.Printf("  QPS: %.2f\n", result.QPS)
		fmt.Printf("  平均响应时间: %v\n", result.AvgResponseTime)
	}

	fmt.Println("\n🎉 性能测试套件演示全部完成！")
}
