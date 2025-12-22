package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	apiHandlers "datamiddleware/internal/api/handlers"
	businessCommon "datamiddleware/internal/business/common"
	errorCommon "datamiddleware/internal/common/errors"
	"datamiddleware/internal/config"
	dataPkg "datamiddleware/internal/data/dao"
	asyncInfra "datamiddleware/internal/infrastructure/async"
	authInfra "datamiddleware/internal/infrastructure/auth"
	cacheInfra "datamiddleware/internal/infrastructure/cache"
	loggingInfra "datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/router"
)

func main() {
	// 初始化配置
	cfg, err := config.Init()
	if err != nil {
		fmt.Printf("配置初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := loggingInfra.Init(cfg.Logger)
	if err != nil {
		fmt.Printf("日志初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 确保程序退出时同步日志缓冲区
	defer func() {
		log.Sync()
	}()

	log.Info("数据中间件服务启动中...",
		"version", "1.0.0",
		"env", cfg.Server.Env,
	)

	// 测试日志写入
	log.Info("测试日志写入文件 - 这条日志应该出现在文件中")
	log.Debug("调试信息测试 - SQL查询等详细信息")

	// 强制同步日志缓冲区，确保日志写入文件
	log.Sync()

	// 初始化错误处理
	errorHandler := errorCommon.Init(log)
	_ = errorHandler // TODO: 在后续阶段使用错误处理器

	// 初始化TCP服务器
	tcpServer := apiHandlers.NewTCPServer(cfg.Server, log)
	if err := tcpServer.Start(); err != nil {
		log.Error("TCP服务器启动失败", "error", err)
		os.Exit(1)
	}

	// 初始化数据库
	db := dataPkg.NewDatabase(cfg.Database, log)
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
	dao := dataPkg.NewDAO(db, log)

	// 初始化JWT服务
	jwtService := authInfra.NewJWTService(cfg.JWT, log)
	_ = jwtService // TODO: 在后续功能中使用JWT服务

	// 初始化业务服务
	playerService := businessCommon.NewPlayerService(dao, log, jwtService)
	itemService := businessCommon.NewItemService(dao, log)
	orderService := businessCommon.NewOrderService(dao, log)

	// 初始化缓存管理器
	cacheManager, err := cacheInfra.NewManager(cfg.Cache, log)
	if err != nil {
		log.Error("缓存管理器初始化失败", "error", err)
		os.Exit(1)
	}

	// 初始化异步任务调度器
	queue := asyncInfra.NewPriorityQueue(1000, log)
	taskScheduler := asyncInfra.NewTaskScheduler(queue, 4, log)
	if err := taskScheduler.Start(); err != nil {
		log.Error("任务调度器启动失败", "error", err)
		os.Exit(1)
	}

	// 初始化消息路由器
	messageRouter := router.NewMessageRouter(log)

	// 注册游戏处理器
	game1Handler := businessCommon.NewGameHandler("game1", playerService, itemService, orderService, log)
	game2Handler := businessCommon.NewGameHandler("game2", playerService, itemService, orderService, log)

	if err := messageRouter.RegisterGameHandler("game1", game1Handler); err != nil {
		log.Error("注册游戏处理器失败", "game_id", "game1", "error", err)
		os.Exit(1)
	}
	if err := messageRouter.RegisterGameHandler("game2", game2Handler); err != nil {
		log.Error("注册游戏处理器失败", "game_id", "game2", "error", err)
		os.Exit(1)
	}

	// 初始化HTTP服务器
	httpServer := apiHandlers.NewHTTPServer(cfg.Server, log, errorHandler, dao, jwtService, playerService, itemService, orderService, cacheManager, taskScheduler)
	if err := httpServer.Start(); err != nil {
		log.Error("HTTP服务器启动失败", "error", err)
		os.Exit(1)
	}

	// TODO: 注册健康检查器
	// 暂时简化实现，后续完善

	// 缓存预热
	warmup := cacheInfra.NewDefaultWarmup(log)
	if err := cacheManager.WarmupCache(warmup); err != nil {
		log.Warn("缓存预热失败", "error", err)
		// 预热失败不影响服务启动
	}

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

	// 关闭任务调度器
	if err := taskScheduler.Stop(); err != nil {
		log.Error("任务调度器关闭失败", "error", err)
	}

	// 关闭缓存管理器
	if err := cacheManager.Close(); err != nil {
		log.Error("缓存管理器关闭失败", "error", err)
	}

	log.Info("数据中间件服务已关闭")
}
