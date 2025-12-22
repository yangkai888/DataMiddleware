package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/common/utils"
	"datamiddleware/internal/common/types"
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

	fmt.Println("开始协程池功能测试...")

	// 测试1: 基础协程池
	fmt.Println("\n=== 测试1: 基础协程池 ===")

	config := utils.DefaultGoroutinePoolConfig()
	config.Size = 10
	config.MonitorInterval = 5 * time.Second

	pool, err := utils.NewGoroutinePool(config, log)
	if err != nil {
		fmt.Printf("创建协程池失败: %v\n", err)
		return
	}
	defer pool.Close()

	// 提交一些任务
	fmt.Println("提交10个任务...")
	for i := 0; i < 10; i++ {
		taskID := i
		err := pool.Submit(func() {
			fmt.Printf("任务 %d 开始执行\n", taskID)
			time.Sleep(100 * time.Millisecond) // 模拟工作
			fmt.Printf("任务 %d 执行完成\n", taskID)
		})
		if err != nil {
			fmt.Printf("提交任务 %d 失败: %v\n", taskID, err)
		}
	}

	// 等待一下让任务执行
	time.Sleep(500 * time.Millisecond)

	// 查看统计信息
	stats := pool.GetStats()
	fmt.Printf("协程池统计: 提交=%d, 完成=%d, 失败=%d, 运行中=%d, 空闲=%d, 容量=%d\n",
		stats.SubmittedTasks, stats.CompletedTasks, stats.FailedTasks,
		stats.RunningWorkers, stats.FreeWorkers, stats.Capacity)

	// 测试2: 带上下文的任务
	fmt.Println("\n=== 测试2: 带上下文的任务 ===")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for i := 0; i < 5; i++ {
		taskID := i + 10
		err := pool.SubmitWithContext(ctx, func(ctx context.Context) {
			select {
			case <-time.After(200 * time.Millisecond):
				fmt.Printf("上下文任务 %d 正常完成\n", taskID)
			case <-ctx.Done():
				fmt.Printf("上下文任务 %d 被取消: %v\n", taskID, ctx.Err())
			}
		})
		if err != nil {
			fmt.Printf("提交上下文任务 %d 失败: %v\n", taskID, err)
		}
	}

	time.Sleep(1 * time.Second)

	// 测试3: 动态调整容量
	fmt.Println("\n=== 测试3: 动态调整容量 ===")

	fmt.Printf("当前容量: %d\n", pool.GetStats().Capacity)

	// 增加容量
	err = pool.TuneCapacity(20)
	if err != nil {
		fmt.Printf("调整容量失败: %v\n", err)
	} else {
		fmt.Printf("容量调整为: %d\n", pool.GetStats().Capacity)
	}

	// 提交更多任务测试新容量
	fmt.Println("提交20个任务到扩大后的协程池...")
	for i := 0; i < 20; i++ {
		taskID := i + 20
		err := pool.Submit(func() {
			time.Sleep(50 * time.Millisecond)
		})
		if err != nil {
			fmt.Printf("提交任务 %d 失败: %v\n", taskID, err)
		}
	}

	time.Sleep(1 * time.Second)
	fmt.Printf("扩大后统计: 运行中=%d, 空闲=%d\n",
		pool.GetStats().RunningWorkers, pool.GetStats().FreeWorkers)

	// 测试4: 自适应协程池
	fmt.Println("\n=== 测试4: 自适应协程池 ===")

	adaptivePool := utils.NewAdaptiveGoroutinePool(log)
	defer adaptivePool.Close()

	// 注册不同类型的协程池
	highPriorityConfig := utils.GoroutinePoolConfig{
		Size:            5,
		Nonblocking:     false,
		PreAlloc:        true,
		MonitorInterval: 10 * time.Second,
		ExpiryDuration:  30 * time.Second,
	}

	normalConfig := utils.GoroutinePoolConfig{
		Size:            15,
		Nonblocking:     false,
		PreAlloc:        true,
		MonitorInterval: 10 * time.Second,
		ExpiryDuration:  1 * time.Minute,
	}

	err = adaptivePool.RegisterPool("high_priority", highPriorityConfig)
	if err != nil {
		fmt.Printf("注册高优先级协程池失败: %v\n", err)
	}

	err = adaptivePool.RegisterPool("normal", normalConfig)
	if err != nil {
		fmt.Printf("注册普通协程池失败: %v\n", err)
	}

	// 提交任务到不同协程池
	fmt.Println("提交任务到不同协程池...")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)

		// 高优先级任务
		go func(id int) {
			defer wg.Done()
			err := adaptivePool.SubmitToPool("high_priority", func() {
				fmt.Printf("高优先级任务 %d 执行\n", id)
				time.Sleep(100 * time.Millisecond)
			})
			if err != nil {
				fmt.Printf("提交高优先级任务失败: %v\n", err)
			}
		}(i)

		// 普通任务
		go func(id int) {
			defer wg.Done()
			err := adaptivePool.SubmitToPool("normal", func() {
				fmt.Printf("普通任务 %d 执行\n", id)
				time.Sleep(150 * time.Millisecond)
			})
			if err != nil {
				fmt.Printf("提交普通任务失败: %v\n", err)
			}
		}(i)
	}

	wg.Wait()

	// 查看各协程池统计
	allStats := adaptivePool.GetAllPoolStats()
	fmt.Println("自适应协程池统计:")
	for name, stats := range allStats {
		fmt.Printf("  %s: 提交=%d, 完成=%d, 运行中=%d\n",
			name, stats.SubmittedTasks, stats.CompletedTasks, stats.RunningWorkers)
	}

	// 测试5: 协程监控器
	fmt.Println("\n=== 测试5: 协程监控器 ===")

	monitor := utils.NewGoroutineMonitor(log, 2*time.Second)
	monitor.Start()

	// 创建一些协程来测试监控
	fmt.Println("创建协程测试监控功能...")
	for i := 0; i < 50; i++ {
		go func(id int) {
			time.Sleep(3 * time.Second)
			fmt.Printf("协程 %d 结束\n", id)
		}(i)
	}

	time.Sleep(6 * time.Second)

	monitorStats := monitor.GetStats()
	fmt.Printf("协程监控统计: 当前=%d, 上次=%d, 增长率=%.2f%%\n",
		monitorStats.CurrentCount, monitorStats.LastCount, monitorStats.GrowthRate*100)

	monitor.Stop()

	fmt.Println("\n🎉 协程池功能测试全部完成！")

	// 关闭资源
	pool.Close()
	adaptivePool.Close()
}
