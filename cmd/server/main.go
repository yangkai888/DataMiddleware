package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"datamiddleware/internal/config"
	"datamiddleware/internal/database"
	"datamiddleware/internal/logger"
	"datamiddleware/internal/errors"
	"datamiddleware/internal/server"
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

	// 初始化TCP服务器
	tcpServer := server.NewTCPServer(cfg.Server.TCP, log)
	if err := tcpServer.Start(); err != nil {
		log.Error("TCP服务器启动失败", "error", err)
		os.Exit(1)
	}

	// 初始化数据库
	db := database.NewDatabase(cfg.Database, log)
	if err := db.Connect(); err != nil {
		log.Error("数据库连接失败", "error", err)
		os.Exit(1)
	}

	// 自动迁移数据库表结构
	if err := db.AutoMigrate(); err != nil {
		log.Error("数据库表结构迁移失败", "error", err)
		os.Exit(1)
	}

	// 初始化DAO层
	dao := database.NewDAO(db, log)

	// 初始化HTTP服务器
	httpServer := server.NewHTTPServer(cfg.Server, log, errorHandler, dao)
	if err := httpServer.Start(); err != nil {
		log.Error("HTTP服务器启动失败", "error", err)
		os.Exit(1)
	}

	// 这里将添加其他服务器初始化代码
	// TODO: 初始化缓存

	log.Info("数据中间件服务启动完成")

	// 等待中断信号优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("数据中间件服务正在关闭...")

	// 优雅关闭HTTP服务器
	if err := httpServer.Stop(); err != nil {
		log.Error("HTTP服务器停止失败", "error", err)
	}

	// 优雅关闭TCP服务器
	if err := tcpServer.Stop(); err != nil {
		log.Error("TCP服务器停止失败", "error", err)
	}

	// 关闭数据库连接
	if err := db.Close(); err != nil {
		log.Error("数据库关闭失败", "error", err)
	}

	// TODO: 关闭缓存连接

	log.Info("数据中间件服务已关闭")
}
