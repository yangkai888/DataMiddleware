package test

import (
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRuntimeTuning 测试Go运行时调优
func TestRuntimeTuning(t *testing.T) {
	t.Run("GOMAXPROCSConfiguration", func(t *testing.T) {
		// 测试GOMAXPROCS设置
		currentMaxProcs := runtime.GOMAXPROCS(0) // 获取当前值，不改变
		numCPU := runtime.NumCPU()

		t.Logf("当前GOMAXPROCS: %d", currentMaxProcs)
		t.Logf("系统CPU核心数: %d", numCPU)

		// 验证GOMAXPROCS设置合理性
		if currentMaxProcs <= 0 {
			t.Errorf("GOMAXPROCS设置无效: %d", currentMaxProcs)
		}

		if currentMaxProcs > numCPU*2 {
			t.Logf("警告: GOMAXPROCS(%d)超过CPU核心数(%d)的2倍，可能造成过度调度", currentMaxProcs, numCPU)
		} else {
			t.Logf("✓ GOMAXPROCS设置合理: %d (CPU核心数: %d)", currentMaxProcs, numCPU)
		}

		// 测试动态调整GOMAXPROCS
		originalValue := runtime.GOMAXPROCS(0)
		testValue := 2
		if testValue > numCPU {
			testValue = numCPU
		}

		runtime.GOMAXPROCS(testValue)
		newValue := runtime.GOMAXPROCS(0)

		if newValue != testValue {
			t.Errorf("GOMAXPROCS设置失败，期望%d，实际%d", testValue, newValue)
		}

		// 恢复原始值
		runtime.GOMAXPROCS(originalValue)

		t.Logf("✓ GOMAXPROCS动态调整测试通过: %d -> %d -> %d", originalValue, testValue, runtime.GOMAXPROCS(0))
	})

	t.Run("GCConfiguration", func(t *testing.T) {
		// 测试GC配置
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		t.Logf("当前GC统计:")
		t.Logf("  - GC次数: %d", memStats.NumGC)
		t.Logf("  - 下次GC目标: %d bytes (%.2f MB)", memStats.NextGC, float64(memStats.NextGC)/1024/1024)
		t.Logf("  - 堆大小: %d bytes (%.2f MB)", memStats.HeapAlloc, float64(memStats.HeapAlloc)/1024/1024)
		t.Logf("  - 堆系统内存: %d bytes (%.2f MB)", memStats.HeapSys, float64(memStats.HeapSys)/1024/1024)

		// 测试手动GC
		runtime.GC()
		runtime.GC() // 两次GC确保清理完成

		var memStatsAfter runtime.MemStats
		runtime.ReadMemStats(&memStatsAfter)

		gcReduction := memStats.HeapAlloc - memStatsAfter.HeapAlloc
		t.Logf("手动GC后堆内存减少: %d bytes (%.2f MB)", gcReduction, float64(gcReduction)/1024/1024)

		if gcReduction > 0 {
			t.Logf("✓ 手动GC生效，释放内存: %d bytes", gcReduction)
		} else {
			t.Logf("手动GC未释放额外内存 (可能已经很干净了)")
		}
	})

	t.Run("MemoryLimits", func(t *testing.T) {
		// 测试内存限制设置
		originalLimit := debug.SetMemoryLimit(-1) // 获取当前限制
		t.Logf("当前内存限制: %d bytes", originalLimit)

		if originalLimit == -1 {
			t.Logf("✓ 无内存限制设置 (默认行为)")
		} else {
			limitMB := float64(originalLimit) / 1024 / 1024
			t.Logf("✓ 内存限制设置为: %.2f MB", limitMB)
		}

		// 恢复原始限制
		debug.SetMemoryLimit(originalLimit)
	})

	t.Run("GCFractionTuning", func(t *testing.T) {
		// 测试GC CPU使用率调整
		originalFraction := debug.SetGCPercent(-1) // 获取当前值

		t.Logf("当前GC目标百分比: %d%%", originalFraction)

		if originalFraction == 100 {
			t.Logf("✓ 使用默认GC触发百分比 (100%%)")
		} else {
			t.Logf("✓ 自定义GC触发百分比: %d%%", originalFraction)
		}

		// 测试临时调整GC百分比
		testPercent := 200 // 更激进的GC
		debug.SetGCPercent(testPercent)
		currentPercent := debug.SetGCPercent(-1)

		if currentPercent != testPercent {
			t.Errorf("GC百分比设置失败，期望%d，实际%d", testPercent, currentPercent)
		}

		// 恢复原始值
		debug.SetGCPercent(originalFraction)

		t.Logf("✓ GC百分比动态调整测试通过: %d%% -> %d%% -> %d%%",
			originalFraction, testPercent, debug.SetGCPercent(-1))
	})
}

// TestRuntimePerformance 测试运行时性能表现
func TestRuntimePerformance(t *testing.T) {
	t.Run("GoroutinePerformance", func(t *testing.T) {
		// 测试协程创建和销毁性能
		const numGoroutines = 10000
		var completed int64

		start := time.Now()

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				// 模拟一些工作
				time.Sleep(time.Microsecond)
				atomic.AddInt64(&completed, 1)
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("协程性能测试:")
		t.Logf("  - 创建协程数: %d", numGoroutines)
		t.Logf("  - 完成协程数: %d", atomic.LoadInt64(&completed))
		t.Logf("  - 总耗时: %v", duration)
		t.Logf("  - 平均协程耗时: %v", duration/numGoroutines)
		t.Logf("  - QPS: %.0f", float64(numGoroutines)/duration.Seconds())

		if atomic.LoadInt64(&completed) != numGoroutines {
			t.Errorf("协程完成数量不匹配: 期望%d，实际%d", numGoroutines, atomic.LoadInt64(&completed))
		}
	})

	t.Run("AllocationPerformance", func(t *testing.T) {
		// 测试内存分配性能
		const iterations = 100000
		var allocated int64

		runtime.GC() // 清理之前的垃圾
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		start := time.Now()

		for i := 0; i < iterations; i++ {
			// 分配一个小对象
			_ = make([]byte, 100)
			atomic.AddInt64(&allocated, 1)
		}

		duration := time.Since(start)

		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		t.Logf("内存分配性能测试:")
		t.Logf("  - 分配次数: %d", iterations)
		t.Logf("  - 总耗时: %v", duration)
		t.Logf("  - 平均分配耗时: %v", duration/iterations)
		t.Logf("  - 分配速率: %.0f 次/秒", float64(iterations)/duration.Seconds())
		t.Logf("  - GC次数: %d -> %d", m1.NumGC, m2.NumGC)

		if atomic.LoadInt64(&allocated) != iterations {
			t.Errorf("分配次数不匹配: 期望%d，实际%d", iterations, atomic.LoadInt64(&allocated))
		}
	})

	t.Run("ChannelPerformance", func(t *testing.T) {
		// 测试channel性能
		const numMessages = 100000
		ch := make(chan int, 1000) // 有缓冲的channel

		var sent, received int64

		// 发送协程
		go func() {
			for i := 0; i < numMessages; i++ {
				ch <- i
				atomic.AddInt64(&sent, 1)
			}
			close(ch)
		}()

		// 接收协程
		start := time.Now()
		for range ch {
			atomic.AddInt64(&received, 1)
		}
		duration := time.Since(start)

		t.Logf("Channel性能测试:")
		t.Logf("  - 发送消息数: %d", atomic.LoadInt64(&sent))
		t.Logf("  - 接收消息数: %d", atomic.LoadInt64(&received))
		t.Logf("  - 总耗时: %v", duration)
		t.Logf("  - 吞吐量: %.0f 消息/秒", float64(numMessages)/duration.Seconds())

		if atomic.LoadInt64(&sent) != numMessages || atomic.LoadInt64(&received) != numMessages {
			t.Errorf("消息数量不匹配: 发送%d，接收%d，期望%d", sent, received, numMessages)
		}
	})
}

// TestRuntimeMonitoring 测试运行时监控
func TestRuntimeMonitoring(t *testing.T) {
	t.Run("ContinuousMonitoring", func(t *testing.T) {
		// 连续监控运行时状态
		const monitorDuration = 5 * time.Second
		const sampleInterval = 500 * time.Millisecond

		samples := 0
		var totalGoroutines int64
		var minGoroutines, maxGoroutines int

		start := time.Now()

		for time.Since(start) < monitorDuration {
			goroutines := runtime.NumGoroutine()
			atomic.AddInt64(&totalGoroutines, int64(goroutines))

			if samples == 0 || goroutines < minGoroutines {
				minGoroutines = goroutines
			}
			if goroutines > maxGoroutines {
				maxGoroutines = goroutines
			}

			samples++
			time.Sleep(sampleInterval)
		}

		avgGoroutines := float64(totalGoroutines) / float64(samples)

		t.Logf("运行时监控结果:")
		t.Logf("  - 监控时长: %v", monitorDuration)
		t.Logf("  - 采样次数: %d", samples)
		t.Logf("  - 协程数范围: %d - %d", minGoroutines, maxGoroutines)
		t.Logf("  - 平均协程数: %.1f", avgGoroutines)

		// 验证协程数量合理性
		if avgGoroutines < 1 {
			t.Error("协程数量异常: 平均协程数小于1")
		}

		if maxGoroutines-minGoroutines > 50 {
			t.Logf("警告: 协程数量波动较大 (%d), 可能存在协程泄漏", maxGoroutines-minGoroutines)
		} else {
			t.Logf("✓ 协程数量稳定: 波动范围 %d", maxGoroutines-minGoroutines)
		}
	})

	t.Run("MemoryPressureTest", func(t *testing.T) {
		// 测试内存压力下的表现
		const pressureDuration = 3 * time.Second

		runtime.GC() // 初始清理
		var initialMem runtime.MemStats
		runtime.ReadMemStats(&initialMem)

		start := time.Now()

		// 在压力期间不断分配内存
		allocations := 0
		for time.Since(start) < pressureDuration {
			// 分配一些内存
			data := make([]byte, 1024*10) // 10KB
			_ = len(data)                 // 防止优化
			allocations++

			// 每100次分配休息一下
			if allocations%100 == 0 {
				time.Sleep(time.Millisecond)
			}
		}

		runtime.GC() // 压力测试后的清理
		var finalMem runtime.MemStats
		runtime.ReadMemStats(&finalMem)

		t.Logf("内存压力测试结果:")
		t.Logf("  - 测试时长: %v", pressureDuration)
		t.Logf("  - 内存分配次数: %d", allocations)
		t.Logf("  - 初始堆大小: %d bytes (%.2f MB)", initialMem.HeapAlloc, float64(initialMem.HeapAlloc)/1024/1024)
		t.Logf("  - 最终堆大小: %d bytes (%.2f MB)", finalMem.HeapAlloc, float64(finalMem.HeapAlloc)/1024/1024)
		t.Logf("  - GC次数: %d -> %d", initialMem.NumGC, finalMem.NumGC)

		heapGrowth := finalMem.HeapAlloc - initialMem.HeapAlloc
		if heapGrowth > 50*1024*1024 { // 50MB
			t.Logf("警告: 内存增长较大 (%d bytes), 可能存在内存泄漏", heapGrowth)
		} else {
			t.Logf("✓ 内存使用正常: 增长 %d bytes", heapGrowth)
		}
	})
}

// BenchmarkRuntimeOperations 基准测试运行时操作
func BenchmarkRuntimeOperations(b *testing.B) {
	b.Run("GoroutineCreation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			done := make(chan bool)
			go func() {
				done <- true
			}()
			<-done
		}
	})

	b.Run("MemoryAllocation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = make([]byte, 100)
		}
	})

	b.Run("GC", func(b *testing.B) {
		// 预先分配一些垃圾
		for i := 0; i < 1000; i++ {
			_ = make([]byte, 1024)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runtime.GC()
		}
	})
}
