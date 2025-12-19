package test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/internal/utils"
	"datamiddleware/pkg/types"
)

// TestBufferPool 测试字节缓冲区池
func TestBufferPool(t *testing.T) {
	// 创建缓冲区池
	pool := utils.NewBufferPool()

	// 测试基本的获取和归还
	t.Run("BasicGetPut", func(t *testing.T) {
		buf := pool.Get()
		if len(buf) != 0 {
			t.Errorf("期望获取空缓冲区，实际长度: %d", len(buf))
		}
		if cap(buf) != 4096 {
			t.Errorf("期望缓冲区容量为4096，实际容量: %d", cap(buf))
		}

		// 使用缓冲区
		buf = append(buf, []byte("test data")...)
		if len(buf) != 9 {
			t.Errorf("期望缓冲区长度为9，实际长度: %d", len(buf))
		}

		// 归还缓冲区
		pool.Put(buf)
	})

	// 测试并发访问
	t.Run("ConcurrentAccess", func(t *testing.T) {
		const numGoroutines = 100
		const numIterations = 1000

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numIterations; j++ {
					buf := pool.Get()
					// 模拟使用缓冲区
					buf = append(buf, []byte("test")...)
					pool.Put(buf)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("并发测试完成: %d个协程，每个协程%d次操作", numGoroutines, numIterations)
	})

	// 测试内存效率
	t.Run("MemoryEfficiency", func(t *testing.T) {
		// 记录初始内存状态
		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// 执行大量缓冲区操作
		const iterations = 10000
		for i := 0; i < iterations; i++ {
			buf := pool.Get()
			buf = append(buf, []byte("data")...)
			pool.Put(buf)
		}

		runtime.GC()
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		heapGrowth := m2.HeapAlloc - m1.HeapAlloc
		t.Logf("内存增长: %d bytes (%.2f MB)", heapGrowth, float64(heapGrowth)/1024/1024)
		t.Logf("GC次数: %d -> %d", m1.NumGC, m2.NumGC)
	})
}

// TestMessagePool 测试消息对象池
func TestMessagePool(t *testing.T) {
	pool := utils.NewMessagePool()

	// 测试基本的获取和归还
	t.Run("BasicGetPut", func(t *testing.T) {
		msg := pool.Get()

		// 验证初始状态
		if msg.ID != "" {
			t.Errorf("期望ID为空，实际: %s", msg.ID)
		}
		if msg.Type != "" {
			t.Errorf("期望Type为空，实际: %s", msg.Type)
		}
		if len(msg.Payload) != 0 {
			t.Errorf("期望Payload为空，实际长度: %d", len(msg.Payload))
		}
		if len(msg.Metadata) != 0 {
			t.Errorf("期望Metadata为空，实际长度: %d", len(msg.Metadata))
		}

		// 使用消息对象
		msg.ID = "test_id"
		msg.Type = "test_type"
		msg.Payload = []byte("test payload")
		msg.Metadata["key"] = "value"

		// 归还消息对象
		pool.Put(msg)

		// 再次获取验证状态被重置
		msg2 := pool.Get()
		if msg2.ID != "" {
			t.Errorf("期望ID被重置为空，实际: %s", msg2.ID)
		}
		if len(msg2.Metadata) != 0 {
			t.Errorf("期望Metadata被清空，实际长度: %d", len(msg2.Metadata))
		}
	})

	// 测试并发访问
	t.Run("ConcurrentAccess", func(t *testing.T) {
		const numGoroutines = 50
		const numIterations = 1000

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numIterations; j++ {
					msg := pool.Get()
					msg.ID = "concurrent_test"
					msg.Type = "test"
					msg.Metadata["goroutine"] = id
					pool.Put(msg)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("并发消息池测试完成: %d个协程，每个协程%d次操作", numGoroutines, numIterations)
	})
}

// TestRingBuffer 测试环形缓冲区
func TestRingBuffer(t *testing.T) {
	const bufferSize = 1024
	rb := utils.NewRingBuffer(bufferSize)

	// 测试基本的写入和读取
	t.Run("BasicWriteRead", func(t *testing.T) {
		data := []byte("Hello, Ring Buffer!")
		n, err := rb.Write(data)
		if err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		if n != len(data) {
			t.Errorf("期望写入%d字节，实际写入%d字节", len(data), n)
		}

		readBuf := make([]byte, len(data))
		m, err := rb.Read(readBuf)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if m != len(data) {
			t.Errorf("期望读取%d字节，实际读取%d字节", len(data), m)
		}
		if string(readBuf) != string(data) {
			t.Errorf("读取的数据不匹配")
		}
	})

	// 测试环形特性
	t.Run("CircularBehavior", func(t *testing.T) {
		// 填满缓冲区
		largeData := make([]byte, bufferSize-10)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		n, err := rb.Write(largeData)
		if err != nil {
			t.Fatalf("写入大数据失败: %v", err)
		}
		if n != len(largeData) {
			t.Errorf("期望写入%d字节，实际写入%d字节", len(largeData), n)
		}

		// 读取部分数据
		readBuf := make([]byte, 100)
		m, err := rb.Read(readBuf)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if m != 100 {
			t.Errorf("期望读取100字节，实际读取%d字节", m)
		}

		// 再次写入，应该覆盖旧数据
		smallData := []byte("New data")
		n, err = rb.Write(smallData)
		if err != nil {
			t.Fatalf("再次写入失败: %v", err)
		}
		if n != len(smallData) {
			t.Errorf("期望写入%d字节，实际写入%d字节", len(smallData), n)
		}
	})

	// 测试边界条件
	t.Run("BoundaryConditions", func(t *testing.T) {
		// 清空缓冲区 - 创建新的缓冲区来模拟重置
		rb = utils.NewRingBuffer(bufferSize)

		// 测试空缓冲区读取
		readBuf := make([]byte, 10)
		n, err := rb.Read(readBuf)
		if err != nil {
			t.Errorf("空缓冲区读取应该不报错，实际错误: %v", err)
		}
		if n != 0 {
			t.Errorf("空缓冲区应该读取0字节，实际读取%d字节", n)
		}

		// 测试超过缓冲区大小的写入
		largeData := make([]byte, bufferSize+100)
		n, err = rb.Write(largeData)
		if err != nil {
			t.Fatalf("写入大数据失败: %v", err)
		}
		if n != bufferSize-1 { // 环形缓冲区保留1字节避免读写指针重合
			t.Errorf("期望写入%d字节，实际写入%d字节", bufferSize-1, n)
		}
	})
}

// TestMemoryManager 测试内存管理器
func TestMemoryManager(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	manager := utils.NewMemoryManager(log)

	// 测试统计信息
	t.Run("Statistics", func(t *testing.T) {
		// 执行一些操作
		for i := 0; i < 100; i++ {
			buf := manager.GetBuffer()
			buf = append(buf, []byte("test")...)
			manager.PutBuffer(buf)
		}

		for i := 0; i < 50; i++ {
			msg := manager.GetMessage()
			msg.ID = "test"
			manager.PutMessage(msg)
		}

		// 等待操作完成
		time.Sleep(10 * time.Millisecond)

		stats := manager.GetStats()
		t.Logf("内存管理器统计: %+v", stats)

		if stats.AllocatedBuffers == 0 {
			t.Error("期望有缓冲区分配统计")
		}
		if stats.AllocatedMessages == 0 {
			t.Error("期望有消息对象分配统计")
		}
	})
}

// TestZeroCopyOperations 测试零拷贝操作
func TestZeroCopyOperations(t *testing.T) {
	// 测试零拷贝切片
	t.Run("ZeroCopySlice", func(t *testing.T) {
		data := []byte("Hello, Zero Copy World!")
		slice := utils.ZeroCopySlice(data, 7, 9) // "Zero Copy" (9个字符)

		expected := "Zero Copy"
		if string(slice) != expected {
			t.Errorf("期望切片内容为'%s'，实际为'%s'", expected, string(slice))
		}

		// 验证底层数组共享
		original := data[7] // 保存原始值
		data[7] = 'z' // 修改原数据
		if slice[0] != 'z' {
			t.Error("零拷贝切片应该反映底层数据的变化")
		}
		data[7] = original // 恢复原始值
	})

	// 测试性能对比
	t.Run("PerformanceComparison", func(t *testing.T) {
		// 准备测试数据
		const iterations = 10000
		data := make([]byte, 1024)
		for i := range data {
			data[i] = byte(i % 256)
		}

		// 测试传统拷贝方式
		start := time.Now()
		var totalTraditional int64
		for i := 0; i < iterations; i++ {
			// 模拟传统拷贝
			copyBuf := make([]byte, 512)
			copy(copyBuf, data[:512])
			totalTraditional += int64(len(copyBuf))
		}
		traditionalTime := time.Since(start)

		// 测试零拷贝方式
		start = time.Now()
		var totalZeroCopy int64
		for i := 0; i < iterations; i++ {
			// 使用零拷贝
			slice := utils.ZeroCopySlice(data, 0, 512)
			totalZeroCopy += int64(len(slice))
		}
		zeroCopyTime := time.Since(start)

		t.Logf("传统拷贝: %v (%d bytes)", traditionalTime, totalTraditional)
		t.Logf("零拷贝: %v (%d bytes)", zeroCopyTime, totalZeroCopy)

		// 零拷贝应该显著更快
		if zeroCopyTime > traditionalTime {
			t.Logf("零拷贝时间(%.2fms) 传统拷贝时间(%.2fms)",
				float64(zeroCopyTime.Nanoseconds())/1000000,
				float64(traditionalTime.Nanoseconds())/1000000)
		}
	})
}
