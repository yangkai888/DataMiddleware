package test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/common/utils"
	"datamiddleware/internal/common/types"
)

// TestGoroutinePoolBasic 测试协程池基本功能
func TestGoroutinePoolBasic(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	config := utils.DefaultGoroutinePoolConfig()
	config.Size = 10 // 小池子便于测试
	config.MonitorInterval = 1 * time.Second // 更频繁的监控

	pool, err := utils.NewGoroutinePool(config, log)
	if err != nil {
		t.Fatalf("创建协程池失败: %v", err)
	}
	defer pool.Close()

	// 测试基本任务提交
	t.Run("BasicTask", func(t *testing.T) {
		var executed bool
		var mu sync.Mutex

		err := pool.Submit(func() {
			mu.Lock()
			executed = true
			mu.Unlock()
			time.Sleep(10 * time.Millisecond) // 模拟工作
		})

		if err != nil {
			t.Fatalf("提交任务失败: %v", err)
		}

		// 等待任务完成
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		if !executed {
			t.Error("任务没有执行")
		}
		mu.Unlock()
	})

	// 测试并发任务
	t.Run("ConcurrentTasks", func(t *testing.T) {
		const numTasks = 50
		var completed int64

		// 提交多个并发任务
		for i := 0; i < numTasks; i++ {
			taskID := i
			err := pool.Submit(func() {
				// 模拟一些工作
				time.Sleep(time.Duration(taskID%10+1) * time.Millisecond)
				atomic.AddInt64(&completed, 1)
			})

			if err != nil {
				t.Errorf("提交任务%d失败: %v", i, err)
			}
		}

		// 等待所有任务完成
		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				t.Fatal("等待任务完成超时")
			case <-ticker.C:
				if atomic.LoadInt64(&completed) >= numTasks {
					goto done
				}
			}
		}

	done:
		t.Logf("并发任务测试完成: %d个任务全部执行", numTasks)
	})

	// 测试协程池状态
	t.Run("PoolStats", func(t *testing.T) {
		// 等待监控更新
		time.Sleep(2 * time.Second)

		stats := pool.GetStats()
		t.Logf("协程池统计: %+v", stats)

		if stats.Capacity != 10 {
			t.Errorf("期望容量为10，实际为%d", stats.Capacity)
		}

		if stats.SubmittedTasks == 0 {
			t.Error("期望有提交的任务")
		}
	})
}

// TestGoroutinePoolPanic 测试协程池panic处理
func TestGoroutinePoolPanic(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	config := utils.DefaultGoroutinePoolConfig()
	config.Size = 5

	pool, err := utils.NewGoroutinePool(config, log)
	if err != nil {
		t.Fatalf("创建协程池失败: %v", err)
	}
	defer pool.Close()

	var normalTasks int64
	var panicTasks int64

	// 提交正常任务
	for i := 0; i < 10; i++ {
		err := pool.Submit(func() {
			atomic.AddInt64(&normalTasks, 1)
		})
		if err != nil {
			t.Errorf("提交正常任务失败: %v", err)
		}
	}

	// 提交会panic的任务
	for i := 0; i < 5; i++ {
		err := pool.Submit(func() {
			atomic.AddInt64(&panicTasks, 1)
			panic("测试panic")
		})
		if err != nil {
			t.Errorf("提交panic任务失败: %v", err)
		}
	}

	// 等待所有任务完成
	time.Sleep(1 * time.Second)

	if atomic.LoadInt64(&normalTasks) != 10 {
		t.Errorf("期望10个正常任务完成，实际%d个", atomic.LoadInt64(&normalTasks))
	}

	if atomic.LoadInt64(&panicTasks) != 5 {
		t.Errorf("期望5个panic任务执行，实际%d个", atomic.LoadInt64(&panicTasks))
	}

	stats := pool.GetStats()
		t.Logf("Panic测试后统计: %+v", stats)
}

// TestGoroutinePoolScaling 测试协程池扩缩容
func TestGoroutinePoolScaling(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	config := utils.DefaultGoroutinePoolConfig()
	config.Size = 20
	config.MonitorInterval = 500 * time.Millisecond // 更频繁监控

	pool, err := utils.NewGoroutinePool(config, log)
	if err != nil {
		t.Fatalf("创建协程池失败: %v", err)
	}
	defer pool.Close()

	// 模拟高负载场景
	const burstTasks = 100
	var completed int64

	start := time.Now()

	// 突发提交大量任务
	for i := 0; i < burstTasks; i++ {
		err := pool.Submit(func() {
			// 模拟中等负载工作
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt64(&completed, 1)
		})

		if err != nil {
			t.Errorf("提交突发任务失败: %v", err)
		}
	}

	// 等待任务完成
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("等待突发任务完成超时")
		case <-ticker.C:
			if atomic.LoadInt64(&completed) >= burstTasks {
				duration := time.Since(start)
				t.Logf("突发负载测试完成: %d个任务在%.2fs内完成", burstTasks, duration.Seconds())
				goto done
			}
		}
	}

done:
	// 检查最终状态
	stats := pool.GetStats()
		t.Logf("扩缩容测试最终统计: %+v", stats)

	// 验证没有任务丢失
	if stats.SubmittedTasks != burstTasks {
		t.Errorf("期望提交%d个任务，实际%d个", burstTasks, stats.SubmittedTasks)
	}
}

// TestGoroutinePoolMonitor 测试协程池监控
func TestGoroutinePoolMonitor(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "info", // 启用info级别以查看监控日志
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	config := utils.DefaultGoroutinePoolConfig()
	config.Size = 15
	config.MonitorInterval = 1 * time.Second // 1秒监控间隔

	pool, err := utils.NewGoroutinePool(config, log)
	if err != nil {
		t.Fatalf("创建协程池失败: %v", err)
	}
	defer pool.Close()

	// 运行一段时间让监控工作
	time.Sleep(3 * time.Second)

	// 执行一些任务
	for i := 0; i < 20; i++ {
		pool.Submit(func() {
			time.Sleep(10 * time.Millisecond)
		})
	}

	// 等待任务完成和监控周期
	time.Sleep(2 * time.Second)

	stats := pool.GetStats()
		t.Logf("监控测试统计: %+v", stats)

	// 验证监控数据合理性
	if stats.Capacity != 15 {
		t.Errorf("期望容量15，实际%d", stats.Capacity)
	}

	if stats.SubmittedTasks < 20 {
		t.Errorf("期望至少提交20个任务，实际%d", stats.SubmittedTasks)
	}
}

// TestGoroutinePoolWithContext 测试带上下文的协程池
func TestGoroutinePoolWithContext(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	config := utils.DefaultGoroutinePoolConfig()
	config.Size = 5

	pool, err := utils.NewGoroutinePool(config, log)
	if err != nil {
		t.Fatalf("创建协程池失败: %v", err)
	}
	defer pool.Close()

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		var executed bool
		var mu sync.Mutex

		err := pool.SubmitWithContext(ctx, func(ctx context.Context) {
			// 模拟长时间任务
			select {
			case <-time.After(100 * time.Millisecond):
				mu.Lock()
				executed = true
				mu.Unlock()
			case <-ctx.Done():
				// 上下文被取消
				return
			}
		})

		if err != nil {
			t.Fatalf("提交带上下文任务失败: %v", err)
		}

		// 等待任务可能完成或被取消
		time.Sleep(150 * time.Millisecond)

		mu.Lock()
		// 由于上下文超时，任务应该没有完全执行
		if executed {
			t.Log("任务在上下文超时前完成")
		} else {
			t.Log("任务因上下文超时而被取消")
		}
		mu.Unlock()
	})
}
