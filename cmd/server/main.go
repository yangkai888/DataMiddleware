package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"datamiddleware/internal/config"
	"datamiddleware/internal/logger"
	"datamiddleware/internal/errors"
)

func main() {
	// 初始化配置
	cfg, err := config.Init()
	if err != nil {
		fmt.Printf("配置初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := logger.Init(cfg.Logger)
	if err != nil {
		fmt.Printf("日志初始化失败: %v\n", err)
		os.Exit(1)
	}

	log.Info("数据中间件服务启动中...",
		"version", "1.0.0",
		"env", cfg.Server.Env,
	)

	// 初始化错误处理
	errorHandler := errors.Init(log)
	_ = errorHandler // TODO: 在后续阶段使用错误处理器

	// 这里将添加服务器初始化代码
	// TODO: 初始化TCP服务器
	// TODO: 初始化HTTP服务器
	// TODO: 初始化数据库连接
	// TODO: 初始化缓存

	log.Info("数据中间件服务启动完成")

	// 等待中断信号优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("数据中间件服务正在关闭...")

	// TODO: 优雅关闭服务器
	// TODO: 关闭数据库连接
	// TODO: 关闭缓存连接

	log.Info("数据中间件服务已关闭")
}
