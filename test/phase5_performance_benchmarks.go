package test

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/internal/utils"
	"datamiddleware/pkg/types"
)

// TestPerformanceBenchmarks 性能基准测试
func TestPerformanceBenchmarks(t *testing.T) {
	t.Run("SingleThreadedPerformance", func(t *testing.T) {
		testSingleThreadedPerformance(t)
	})

	t.Run("ConcurrentPerformance", func(t *testing.T) {
		testConcurrentPerformance(t)
	})

	t.Run("MemoryPoolBenchmark", func(t *testing.T) {
		testMemoryPoolBenchmark(t)
	})

	t.Run("GoroutinePoolBenchmark", func(t *testing.T) {
		testGoroutinePoolBenchmark(t)
	})
}

// testSingleThreadedPerformance 单线程性能测试
func testSingleThreadedPerformance(t *testing.T) {

	// 测试数据
	testSizes := []int{64, 256, 1024, 4096, 16384}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("DataSize_%d", size), func(t *testing.T) {
			// 生成测试数据
			data := make([]byte, size)
			rand.Read(data)

			// 测试RingBuffer性能
			rb := utils.NewRingBuffer(size * 2)
			iterations := 10000

			start := time.Now()
			for i := 0; i < iterations; i++ {
				rb.Write(data)
				readBuf := make([]byte, len(data))
				rb.Read(readBuf)
			}
			duration := time.Since(start)

			throughput := float64(iterations*size) / duration.Seconds() / 1024 / 1024 // MB/s
			t.Logf("RingBuffer %d字节数据: %.2f MB/s", size, throughput)

			// 测试内存池性能
			pool := utils.NewBufferPool()
			start = time.Now()
			for i := 0; i < iterations; i++ {
				buf := pool.Get()
				copy(buf[:min(len(buf), len(data))], data)
				pool.Put(buf)
			}
			duration = time.Since(start)

			throughput = float64(iterations*size) / duration.Seconds() / 1024 / 1024 // MB/s
			t.Logf("BufferPool %d字节数据: %.2f MB/s", size, throughput)

			// 测试零拷贝性能
			start = time.Now()
			for i := 0; i < iterations; i++ {
				slice := utils.ZeroCopySlice(data, 0, min(size, len(data)))
				_ = len(slice) // 防止优化
			}
			duration = time.Since(start)

			throughput = float64(iterations*size) / duration.Seconds() / 1024 / 1024 // MB/s
			t.Logf("ZeroCopy %d字节数据: %.2f MB/s", size, throughput)
		})
	}
}

// testConcurrentPerformance 并发性能测试
func testConcurrentPerformance(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	// 测试不同的并发级别
	concurrencyLevels := []int{1, 2, 4, 8, 16}

	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			// 创建协程池
			config := utils.DefaultGoroutinePoolConfig()
			config.Size = concurrency * 10 // 池大小为并发数的10倍
			pool, err := utils.NewGoroutinePool(config, log)
			if err != nil {
				t.Fatalf("创建协程池失败: %v", err)
			}
			defer pool.Close()

			// 测试任务
			const tasksPerWorker = 1000
			totalTasks := concurrency * tasksPerWorker
			var completed int64

			start := time.Now()

			// 启动多个worker
			var wg sync.WaitGroup
			for worker := 0; worker < concurrency; worker++ {
				wg.Add(1)
				go func(w int) {
					defer wg.Done()
					for i := 0; i < tasksPerWorker; i++ {
						err := pool.Submit(func() {
							// 模拟工作负载
							time.Sleep(time.Microsecond * time.Duration(100+w*10))
							atomic.AddInt64(&completed, 1)
						})
						if err != nil {
							t.Errorf("提交任务失败: %v", err)
						}
					}
				}(worker)
			}

			wg.Wait()

			// 等待所有任务完成
			timeout := time.After(30 * time.Second)
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-timeout:
					t.Fatal("等待任务完成超时")
				case <-ticker.C:
					if atomic.LoadInt64(&completed) >= int64(totalTasks) {
						goto done
					}
				}
			}

		done:
			duration := time.Since(start)
			qps := float64(totalTasks) / duration.Seconds()

			t.Logf("并发级别 %d: %d任务，耗时%.2fs，QPS %.0f",
				concurrency, totalTasks, duration.Seconds(), qps)
		})
	}
}

// testMemoryPoolBenchmark 内存池性能基准测试
func testMemoryPoolBenchmark(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	// 内存池基准测试
	t.Run("BufferPoolThroughput", func(t *testing.T) {
		pool := utils.NewBufferPool()
		const iterations = 100000

		start := time.Now()
		for i := 0; i < iterations; i++ {
			buf := pool.Get()
			// 模拟使用
			buf = append(buf, []byte("test data")...)
			pool.Put(buf)
		}
		duration := time.Since(start)

		opsPerSec := float64(iterations) / duration.Seconds()
		t.Logf("BufferPool吞吐量: %.0f ops/sec", opsPerSec)
	})

	t.Run("MessagePoolThroughput", func(t *testing.T) {
		pool := utils.NewMessagePool()
		const iterations = 50000

		start := time.Now()
		for i := 0; i < iterations; i++ {
			msg := pool.Get()
			// 模拟使用
			msg.ID = fmt.Sprintf("msg_%d", i)
			msg.Type = "test"
			pool.Put(msg)
		}
		duration := time.Since(start)

		opsPerSec := float64(iterations) / duration.Seconds()
		t.Logf("MessagePool吞吐量: %.0f ops/sec", opsPerSec)
	})

	t.Run("MemoryManagerEfficiency", func(t *testing.T) {
		manager := utils.NewMemoryManager(log)

		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		const iterations = 10000

		start := time.Now()
		for i := 0; i < iterations; i++ {
			buf := manager.GetBuffer()
			buf = append(buf, []byte("benchmark data")...)
			manager.PutBuffer(buf)

			msg := manager.GetMessage()
			msg.ID = fmt.Sprintf("bench_%d", i)
			manager.PutMessage(msg)
		}
		duration := time.Since(start)

		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		opsPerSec := float64(iterations) / duration.Seconds()
		gcCount := m2.NumGC - m1.NumGC

		t.Logf("MemoryManager性能: %.0f ops/sec, GC次数: %d", opsPerSec, gcCount)

		stats := manager.GetStats()
		t.Logf("最终统计: 缓冲区%d, 消息%d", stats.AllocatedBuffers, stats.AllocatedMessages)
	})
}

// testGoroutinePoolBenchmark 协程池性能基准测试
func testGoroutinePoolBenchmark(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	// 测试不同配置的协程池性能
	configs := []struct {
		name   string
		config utils.GoroutinePoolConfig
	}{
		{
			name:   "SmallPool",
			config: utils.GoroutinePoolConfig{Size: 10, MonitorInterval: time.Second},
		},
		{
			name:   "MediumPool",
			config: utils.GoroutinePoolConfig{Size: 50, MonitorInterval: time.Second},
		},
		{
			name:   "LargePool",
			config: utils.GoroutinePoolConfig{Size: 200, MonitorInterval: time.Second},
		},
	}

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			pool, err := utils.NewGoroutinePool(cfg.config, log)
			if err != nil {
				t.Fatalf("创建协程池失败: %v", err)
			}
			defer pool.Close()

			const numTasks = 10000
			var completed int64

			start := time.Now()

			// 提交任务
			for i := 0; i < numTasks; i++ {
				err := pool.Submit(func() {
					// 不同复杂度的任务
					workload := i % 3
					switch workload {
					case 0:
						time.Sleep(time.Microsecond)
					case 1:
						_ = fmt.Sprintf("task_%d", i)
					case 2:
						data := make([]byte, 100)
						rand.Read(data)
						_ = len(data)
					}
					atomic.AddInt64(&completed, 1)
				})
				if err != nil {
					t.Errorf("提交任务失败: %v", err)
				}
			}

			// 等待完成
			timeout := time.After(30 * time.Second)
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-timeout:
					t.Fatal("等待任务完成超时")
				case <-ticker.C:
					if atomic.LoadInt64(&completed) >= numTasks {
						duration := time.Since(start)
						tps := float64(numTasks) / duration.Seconds()

						stats := pool.GetStats()
						t.Logf("%s 池性能: %.0f tasks/sec", cfg.name, tps)
						t.Logf("  统计: 提交%d, 完成%d, 运行中%d",
							stats.SubmittedTasks, stats.CompletedTasks, stats.RunningWorkers)
						return
					}
				}
			}
		})
	}
}

// TestHTTPPerformance HTTP性能测试
func TestHTTPPerformance(t *testing.T) {
	t.Run("HTTPThroughput", func(t *testing.T) {
		// 测试HTTP服务器的吞吐量
		const numRequests = 1000
		var successfulRequests int64

		start := time.Now()

		// 并发发送HTTP请求
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ { // 10个并发worker
			wg.Add(1)
			go func() {
				defer wg.Done()
				client := &http.Client{Timeout: 10 * time.Second}

				for j := 0; j < numRequests/10; j++ {
					resp, err := client.Get("http://localhost:8080/health")
					if err == nil {
						resp.Body.Close()
						atomic.AddInt64(&successfulRequests, 1)
					}
				}
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		rps := float64(atomic.LoadInt64(&successfulRequests)) / duration.Seconds()
		t.Logf("HTTP吞吐量: %.0f requests/sec", rps)
	})

	t.Run("HTTPPayloadPerformance", func(t *testing.T) {
		// 测试不同payload大小的HTTP性能
		payloadSizes := []int{1, 10, 100, 1000} // KB

		for _, sizeKB := range payloadSizes {
			t.Run(fmt.Sprintf("Payload_%dKB", sizeKB), func(t *testing.T) {
				// 生成测试数据
				data := make([]byte, sizeKB*1024)
				rand.Read(data)

				const numRequests = 100
				var totalBytes int64

				start := time.Now()

				client := &http.Client{Timeout: 30 * time.Second}

				for i := 0; i < numRequests; i++ {
					// 这里我们只能测试GET请求，因为没有专门的POST端点
					// 实际项目中应该有专门的性能测试端点
					resp, err := client.Get("http://localhost:8080/health")
					if err == nil {
						// 读取响应体
						body, err := io.ReadAll(resp.Body)
						resp.Body.Close()
						if err == nil {
							atomic.AddInt64(&totalBytes, int64(len(body)))
						}
					}
				}

				duration := time.Since(start)
				throughput := float64(atomic.LoadInt64(&totalBytes)) / duration.Seconds() / 1024 / 1024 // MB/s

				t.Logf("%dKB payload: %.2f MB/s", sizeKB, throughput)
			})
		}
	})
}

// TestSystemResourceUsage 系统资源使用测试
func TestSystemResourceUsage(t *testing.T) {
	t.Run("ResourceMonitoring", func(t *testing.T) {
		// 监控系统资源使用情况
		const monitorDuration = 10 * time.Second
		const sampleInterval = time.Second

		var samples []ResourceSample

		for i := 0; i < int(monitorDuration/sampleInterval); i++ {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			sample := ResourceSample{
				Timestamp: time.Now(),
				Goroutines: runtime.NumGoroutine(),
				HeapAlloc:  m.HeapAlloc,
				HeapSys:    m.HeapSys,
				GCCount:    m.NumGC,
			}
			samples = append(samples, sample)

			time.Sleep(sampleInterval)
		}

		// 分析结果
		if len(samples) > 0 {
			first := samples[0]
			last := samples[len(samples)-1]

			heapGrowth := last.HeapAlloc - first.HeapAlloc
			gcCount := last.GCCount - first.GCCount

			t.Logf("资源监控结果 (%d秒):", int(monitorDuration.Seconds()))
			t.Logf("  协程数: %d -> %d", first.Goroutines, last.Goroutines)
			t.Logf("  堆内存: %d -> %d bytes (增长: %d bytes)",
				first.HeapAlloc, last.HeapAlloc, heapGrowth)
			t.Logf("  GC次数: %d", gcCount)
			if gcCount > 0 {
				t.Logf("  平均GC间隔: %.1f秒", monitorDuration.Seconds()/float64(gcCount))
			} else {
				t.Logf("  GC次数: 0")
			}
		}
	})
}

// ResourceSample 资源样本
type ResourceSample struct {
	Timestamp  time.Time
	Goroutines int
	HeapAlloc  uint64
	HeapSys    uint64
	GCCount    uint32
}

// BenchmarkMemoryOperations 内存操作基准测试
func BenchmarkMemoryOperations(b *testing.B) {

	b.Run("BufferPool_GetPut", func(b *testing.B) {
		pool := utils.NewBufferPool()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buf := pool.Get()
			pool.Put(buf)
		}
	})

	b.Run("MessagePool_GetPut", func(b *testing.B) {
		pool := utils.NewMessagePool()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			msg := pool.Get()
			pool.Put(msg)
		}
	})

	b.Run("RingBuffer_WriteRead", func(b *testing.B) {
		rb := utils.NewRingBuffer(4096)
		data := []byte("benchmark data")

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			rb.Write(data)
			buf := make([]byte, len(data))
			rb.Read(buf)
		}
	})

	b.Run("ZeroCopySlice", func(b *testing.B) {
		data := make([]byte, 1024)
		rand.Read(data)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			slice := utils.ZeroCopySlice(data, 100, 200)
			_ = len(slice)
		}
	})
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
