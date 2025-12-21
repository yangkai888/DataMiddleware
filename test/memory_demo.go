package main

import (
	"fmt"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/internal/utils"
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

	fmt.Println("开始内存优化功能测试...")

	// 创建内存管理器
	memoryManager := utils.NewMemoryManager(log)

	// 测试1: 缓冲区池
	fmt.Println("\n=== 测试1: 缓冲区池 ===")

	// 获取缓冲区并使用
	buf1 := memoryManager.GetBuffer()
	buf1 = append(buf1, []byte("Hello World!")...)
	fmt.Printf("缓冲区内容: %s, 长度: %d, 容量: %d\n", string(buf1), len(buf1), cap(buf1))

	buf2 := memoryManager.GetBuffer()
	buf2 = append(buf2, []byte("Go is awesome!")...)
	fmt.Printf("第二个缓冲区内容: %s, 长度: %d, 容量: %d\n", string(buf2), len(buf2), cap(buf2))

	// 归还缓冲区
	memoryManager.PutBuffer(buf1)
	memoryManager.PutBuffer(buf2)

	// 再次获取，应该复用
	buf3 := memoryManager.GetBuffer()
	fmt.Printf("复用的缓冲区容量: %d (应该大于0)\n", cap(buf3))
	memoryManager.PutBuffer(buf3)

	// 测试2: 消息对象池
	fmt.Println("\n=== 测试2: 消息对象池 ===")

	msg1 := memoryManager.GetMessage()
	msg1.ID = "msg001"
	msg1.Type = "test"
	msg1.Payload = []byte("测试消息内容")
	msg1.Metadata["priority"] = "high"
	msg1.Metadata["timestamp"] = time.Now().Unix()

	fmt.Printf("消息1: ID=%s, Type=%s, Payload=%s, Metadata=%v\n",
		msg1.ID, msg1.Type, string(msg1.Payload), msg1.Metadata)

	msg2 := memoryManager.GetMessage()
	msg2.ID = "msg002"
	msg2.Type = "response"
	msg2.Payload = []byte("响应消息")

	fmt.Printf("消息2: ID=%s, Type=%s, Payload=%s\n",
		msg2.ID, msg2.Type, string(msg2.Payload))

	// 归还消息
	memoryManager.PutMessage(msg1)
	memoryManager.PutMessage(msg2)

	// 复用测试
	msg3 := memoryManager.GetMessage()
	fmt.Printf("复用的消息对象，ID应该为空: '%s'\n", msg3.ID)
	memoryManager.PutMessage(msg3)

	// 测试3: 环形缓冲区
	fmt.Println("\n=== 测试3: 环形缓冲区 ===")

	ringBuf := utils.NewRingBuffer(16)

	// 写入数据
	data1 := []byte("Hello")
	n1, _ := ringBuf.Write(data1)
	fmt.Printf("写入 '%s', 写入字节数: %d\n", string(data1), n1)

	data2 := []byte(" World!")
	n2, _ := ringBuf.Write(data2)
	fmt.Printf("写入 '%s', 写入字节数: %d\n", string(data2), n2)

	// 读取数据
	readBuf := make([]byte, 12)
	n3, _ := ringBuf.Read(readBuf)
	fmt.Printf("读取结果: '%s', 读取字节数: %d\n", string(readBuf[:n3]), n3)

	// 测试4: 零拷贝操作
	fmt.Println("\n=== 测试4: 零拷贝操作 ===")

	original := []byte("Hello Zero Copy World")
	fmt.Printf("原始数据: %s\n", string(original))

	// 零拷贝切片
	slice1 := utils.ZeroCopySlice(original, 6, 9) // "Zero Copy"
	fmt.Printf("零拷贝切片1: %s\n", string(slice1))

	slice2 := utils.ZeroCopySlice(original, 0, 5) // "Hello"
	fmt.Printf("零拷贝切片2: %s\n", string(slice2))

	// 测试5: 批量分配器
	fmt.Println("\n=== 测试5: 批量分配器 ===")

	allocator := utils.NewBulkAllocator(1024, 5)

	// 分配缓冲区
	buffers := make([][]byte, 3)
	for i := range buffers {
		buffers[i] = allocator.Allocate()
		buffers[i] = append(buffers[i], []byte(fmt.Sprintf("Buffer %d content", i))...)
		fmt.Printf("分配的缓冲区 %d: 长度=%d, 容量=%d\n", i, len(buffers[i]), cap(buffers[i]))
	}

	// 查看统计信息
	stats := allocator.Stats()
	fmt.Printf("批量分配器统计: 总缓冲区=%d, 已用=%d, 缓冲区大小=%d\n",
		stats.TotalBuffers, stats.UsedBuffers, stats.BufferSize)

	// 测试6: 内存统计
	fmt.Println("\n=== 测试6: 内存统计 ===")

	memStats := memoryManager.GetStats()
	fmt.Printf("内存管理器统计: 分配的缓冲区=%d, 分配的消息=%d\n",
		memStats.AllocatedBuffers, memStats.AllocatedMessages)

	// 测试7: 性能对比
	fmt.Println("\n=== 测试7: 性能对比 ===")

	// 传统方式 vs 对象池方式
	testCount := 10000

	// 预热
	for i := 0; i < 100; i++ {
		buf := memoryManager.GetBuffer()
		memoryManager.PutBuffer(buf)
	}

	// 对象池方式测试（模拟真实使用场景）
	start := time.Now()
	for i := 0; i < testCount; i++ {
		buf := memoryManager.GetBuffer()
		// 模拟使用缓冲区
		buf = append(buf, []byte(fmt.Sprintf("test data %d", i))...)
		_ = len(buf) // 模拟读取操作
		memoryManager.PutBuffer(buf)
	}
	poolTime := time.Since(start)

	// 传统方式测试
	start = time.Now()
	for i := 0; i < testCount; i++ {
		buf := make([]byte, 0, 4096) // 模拟相同容量
		// 模拟使用缓冲区
		buf = append(buf, []byte(fmt.Sprintf("test data %d", i))...)
		_ = len(buf) // 模拟读取操作
	}
	traditionalTime := time.Since(start)

	fmt.Printf("性能对比 (10000次操作，包含实际使用):\n")
	fmt.Printf("  传统方式: %v\n", traditionalTime)
	fmt.Printf("  对象池方式: %v\n", poolTime)
	if poolTime > 0 {
		fmt.Printf("  性能提升: %.2fx\n", float64(traditionalTime)/float64(poolTime))
	}

	// 显示最终统计
	finalStats := memoryManager.GetStats()
	fmt.Printf("最终内存统计: 分配的缓冲区=%d, 分配的消息=%d\n",
		finalStats.AllocatedBuffers, finalStats.AllocatedMessages)

	fmt.Println("\n🎉 内存优化功能测试全部完成！")
}
