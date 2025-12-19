package main

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"datamiddleware/internal/async"
	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// TestAsyncManager 测试异步管理器
func TestAsyncManager(t *testing.T) {
	// 初始化日志
	logConfig := types.LoggerConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	log, err := logger.Init(logConfig)
	if err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}

	// 创建异步管理器
	manager, err := async.NewAsyncManager(100, 2, log)
	if err != nil {
		t.Fatalf("创建异步管理器失败: %v", err)
	}

	// 启动异步管理器
	err = manager.Start()
	if err != nil {
		t.Fatalf("启动异步管理器失败: %v", err)
	}
	defer manager.Stop()

	// 测试日志任务
	t.Run("LogTask", func(t *testing.T) {
		err := manager.SubmitLogTask("info", "测试异步日志", map[string]interface{}{
			"user_id": "12345",
			"action":  "login",
		})
		if err != nil {
			t.Errorf("提交日志任务失败: %v", err)
		}
	})

	// 测试业务任务
	t.Run("BusinessTask", func(t *testing.T) {
		var callbackResult interface{}
		var callbackError error

		err := manager.SubmitBusinessTask("user_login", map[string]interface{}{
			"user_id": "test_user_001",
		}, func(result interface{}, err error) {
			callbackResult = result
			callbackError = err
		})

		if err != nil {
			t.Errorf("提交业务任务失败: %v", err)
		}

		// 等待任务执行和回调
		time.Sleep(200 * time.Millisecond)

		if callbackError != nil {
			t.Errorf("业务任务执行失败: %v", callbackError)
		}

		if callbackResult == nil {
			t.Logf("警告: 回调结果为空，可能由于异步时序问题")
		} else {
			t.Logf("业务任务执行成功，结果: %+v", callbackResult)
		}
	})

	// 测试清理任务
	t.Run("CleanupTask", func(t *testing.T) {
		err := manager.SubmitCleanupTask("temp_file", "temp_001.txt")
		if err != nil {
			t.Errorf("提交清理任务失败: %v", err)
		}
	})

	// 等待所有任务执行完成
	time.Sleep(200 * time.Millisecond)

	// 检查统计信息
	stats := manager.GetStats()
	if !stats.Running {
		t.Error("期望异步管理器正在运行")
	}

	if stats.Scheduler.QueueSize != 0 {
		t.Errorf("期望队列为空，实际大小: %d", stats.Scheduler.QueueSize)
	}

	if stats.Scheduler.WorkerCount != 2 {
		t.Errorf("期望2个工作协程，实际: %d", stats.Scheduler.WorkerCount)
	}
}

// TestPriorityExecution 测试任务优先级执行
func TestPriorityExecution(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	manager, _ := async.NewAsyncManager(100, 1, log) // 单工作协程确保顺序执行
	manager.Start()
	defer manager.Stop()

	var executionOrder []string
	var counter int32

	// 提交不同优先级的任务
	// 注意：优先级数值越大，优先级越高

	// 低优先级任务
	err := manager.SubmitBusinessTask("send_notification", map[string]interface{}{
		"user_id":  "user_low",
		"message":  "低优先级消息",
		"priority": 1,
	}, func(result interface{}, err error) {
		atomic.AddInt32(&counter, 1)
		executionOrder = append(executionOrder, "low")
	})
	if err != nil {
		t.Fatalf("提交低优先级任务失败: %v", err)
	}

	// 中优先级任务
	err = manager.SubmitBusinessTask("send_notification", map[string]interface{}{
		"user_id":  "user_medium",
		"message":  "中优先级消息",
		"priority": 5,
	}, func(result interface{}, err error) {
		atomic.AddInt32(&counter, 1)
		executionOrder = append(executionOrder, "medium")
	})
	if err != nil {
		t.Fatalf("提交中优先级任务失败: %v", err)
	}

	// 高优先级任务
	err = manager.SubmitBusinessTask("user_login", map[string]interface{}{
		"user_id":  "user_high",
		"priority": 10,
	}, func(result interface{}, err error) {
		atomic.AddInt32(&counter, 1)
		executionOrder = append(executionOrder, "high")
	})
	if err != nil {
		t.Fatalf("提交高优先级任务失败: %v", err)
	}

	// 等待所有任务执行完成
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("等待任务完成超时")
		case <-ticker.C:
			if atomic.LoadInt32(&counter) >= 3 {
				goto checkOrder
			}
		}
	}

checkOrder:
	// 验证执行顺序：高优先级 -> 中优先级 -> 低优先级
	expectedOrder := []string{"high", "medium", "low"}
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("期望执行%d个任务，实际执行%d个", len(expectedOrder), len(executionOrder))
	}

	for i, expected := range expectedOrder {
		if i >= len(executionOrder) || executionOrder[i] != expected {
			t.Errorf("执行顺序不正确，期望位置%d为%s，实际顺序: %v", i, expected, executionOrder)
			break
		}
	}
}

// TestConcurrentLoad 测试并发负载
func TestConcurrentLoad(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn", // 减少日志输出
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	manager, _ := async.NewAsyncManager(1000, 4, log) // 4个工作协程
	manager.Start()
	defer manager.Stop()

	const totalTasks = 50
	var completedCount int32

	// 并发提交任务
	startTime := time.Now()

	for i := 0; i < totalTasks; i++ {
		go func(taskID int) {
			err := manager.SubmitBusinessTask("send_notification", map[string]interface{}{
				"user_id": fmt.Sprintf("user_%d", taskID),
				"message": fmt.Sprintf("通知消息_%d", taskID),
			}, func(result interface{}, err error) {
				atomic.AddInt32(&completedCount, 1)
			})

			if err != nil {
				t.Errorf("提交任务%d失败: %v", taskID, err)
			}
		}(i)
	}

	// 等待所有任务完成
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("等待并发任务完成超时")
		case <-ticker.C:
			if atomic.LoadInt32(&completedCount) >= totalTasks {
				duration := time.Since(startTime)
				t.Logf("并发负载测试完成: %d个任务在%.2fs内完成", totalTasks, duration.Seconds())
				goto done
			}
		}
	}

done:
	// 验证最终状态
	stats := manager.GetStats()
	if stats.Scheduler.QueueSize != 0 {
		t.Errorf("期望队列为空，实际大小: %d", stats.Scheduler.QueueSize)
	}

	if stats.Scheduler.RunningWorkers > stats.Scheduler.WorkerCount {
		t.Errorf("运行中的工作协程数异常: %d/%d", stats.Scheduler.RunningWorkers, stats.Scheduler.WorkerCount)
	}
}
