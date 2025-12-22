package main

import (
	"fmt"
	"time"

	"datamiddleware/internal/infrastructure/async"
	"datamiddleware/internal/infrastructure/logging"
)

func main() {
	// 初始化日志
	log, err := logger.Init(logger.Config{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})
	if err != nil {
		fmt.Printf("日志初始化失败: %v\n", err)
		return
	}

	fmt.Println("开始异步队列功能测试...")

	// 创建异步管理器
	manager, err := async.NewAsyncManager(1000, 3, log)
	if err != nil {
		fmt.Printf("异步管理器创建失败: %v\n", err)
		return
	}

	// 启动异步管理器
	if err := manager.Start(); err != nil {
		fmt.Printf("异步管理器启动失败: %v\n", err)
		return
	}
	defer manager.Stop()

	fmt.Println("✅ 异步管理器启动成功")

	// 测试1: 提交日志任务
	fmt.Println("\n=== 测试1: 异步日志任务 ===")
	logFields := map[string]interface{}{
		"user_id":    "12345",
		"action":     "login",
		"ip":         "192.168.1.1",
		"user_agent": "Mozilla/5.0",
	}

	if err := manager.SubmitLogTask("INFO", "用户登录成功", logFields); err != nil {
		fmt.Printf("提交日志任务失败: %v\n", err)
		return
	}
	fmt.Println("✅ 日志任务提交成功")

	// 测试2: 提交业务任务
	fmt.Println("\n=== 测试2: 异步业务任务 ===")

	// 用户登录任务
	loginCallback := func(result interface{}, err error) {
		if err != nil {
			fmt.Printf("登录任务失败: %v\n", err)
			return
		}
		fmt.Printf("登录任务完成: %+v\n", result)
	}

	loginParams := map[string]interface{}{
		"user_id": "user123",
	}
	if err := manager.SubmitBusinessTask("user_login", loginParams, loginCallback); err != nil {
		fmt.Printf("提交登录任务失败: %v\n", err)
		return
	}
	fmt.Println("✅ 登录业务任务提交成功")

	// 发送通知任务
	notifyCallback := func(result interface{}, err error) {
		if err != nil {
			fmt.Printf("通知任务失败: %v\n", err)
			return
		}
		fmt.Printf("通知任务完成: %+v\n", result)
	}

	notifyParams := map[string]interface{}{
		"user_id": "user123",
		"message": "欢迎登录系统！",
	}
	if err := manager.SubmitBusinessTask("send_notification", notifyParams, notifyCallback); err != nil {
		fmt.Printf("提交通知任务失败: %v\n", err)
		return
	}
	fmt.Println("✅ 通知业务任务提交成功")

	// 数据同步任务
	syncCallback := func(result interface{}, err error) {
		if err != nil {
			fmt.Printf("同步任务失败: %v\n", err)
			return
		}
		fmt.Printf("同步任务完成: %+v\n", result)
	}

	syncParams := map[string]interface{}{
		"table": "user_sessions",
	}
	if err := manager.SubmitBusinessTask("data_sync", syncParams, syncCallback); err != nil {
		fmt.Printf("提交同步任务失败: %v\n", err)
		return
	}
	fmt.Println("✅ 数据同步任务提交成功")

	// 测试3: 提交清理任务
	fmt.Println("\n=== 测试3: 异步清理任务 ===")

	if err := manager.SubmitCleanupTask("temp_file", "/tmp/temp_001.txt"); err != nil {
		fmt.Printf("提交清理任务失败: %v\n", err)
		return
	}
	fmt.Println("✅ 清理任务提交成功")

	// 测试4: 等待任务执行完成
	fmt.Println("\n=== 测试4: 等待任务执行 ===")
	time.Sleep(2 * time.Second) // 等待异步任务完成

	// 测试5: 查看统计信息
	fmt.Println("\n=== 测试5: 查看统计信息 ===")
	stats := manager.GetStats()
	fmt.Printf("异步管理器状态:\n")
	fmt.Printf("  运行中: %v\n", stats.Running)
	fmt.Printf("  工作协程数: %d\n", stats.Scheduler.WorkerCount)
	fmt.Printf("  运行中的工作协程: %d\n", stats.Scheduler.RunningWorkers)
	fmt.Printf("  队列大小: %d\n", stats.Scheduler.QueueSize)

	fmt.Println("\n🎉 异步队列功能测试全部完成！")
}
