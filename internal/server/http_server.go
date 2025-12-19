package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"datamiddleware/internal/database"
	"datamiddleware/internal/errors"
	"datamiddleware/internal/logger"
	"datamiddleware/internal/monitor"
	"datamiddleware/pkg/types"

	"github.com/gin-gonic/gin"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	config       types.ServerConfig   `json:"config"` // 服务器配置
	engine       *gin.Engine          `json:"-"`      // Gin引擎
	logger       logger.Logger        `json:"-"`      // 日志器
	errorHandler *errors.ErrorHandler `json:"-"`      // 错误处理器
	dao          database.DAO         `json:"-"`      // 数据访问对象
	server       *http.Server         `json:"-"`      // HTTP服务器
	monitor      *monitor.Monitor     `json:"-"`      // 监控器
}

// NewHTTPServer 创建HTTP服务器
func NewHTTPServer(config types.ServerConfig, log logger.Logger, errorHandler *errors.ErrorHandler, dao database.DAO) *HTTPServer {
	// 根据环境设置Gin模式
	switch config.Env {
	case "prod":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()

	// 初始化监控器
	monitor := monitor.NewMonitor(log)

	server := &HTTPServer{
		config:       config,
		engine:       engine,
		logger:       log,
		errorHandler: errorHandler,
		dao:          dao,
		monitor:      monitor,
	}

	// 设置中间件
	server.setupMiddlewares()

	// 设置路由
	server.setupRoutes()

	// 注册监控路由
	server.setupMonitorRoutes()

	return server
}

// Start 启动HTTP服务器
func (s *HTTPServer) Start() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.HTTP.Host, s.config.HTTP.Port),
		Handler:      s.engine,
		ReadTimeout:  s.config.HTTP.ReadTimeout,
		WriteTimeout: s.config.HTTP.WriteTimeout,
	}

	s.logger.Info("HTTP服务器启动", "address", s.server.Addr)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP服务器启动失败", "error", err)
		}
	}()

	return nil
}

// Stop 停止HTTP服务器
func (s *HTTPServer) Stop() error {
	s.logger.Info("HTTP服务器停止中...")

	if s.server != nil {
		// 设置关闭超时
		timeout := 30 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error("HTTP服务器关闭失败", "error", err)
			return err
		}
	}

	s.logger.Info("HTTP服务器已停止")
	return nil
}

// GetMonitor 获取监控器
func (s *HTTPServer) GetMonitor() *monitor.Monitor {
	return s.monitor
}

// setupMiddlewares 设置中间件
func (s *HTTPServer) setupMiddlewares() {
	// 恢复中间件 - 捕获panic
	s.engine.Use(gin.Recovery())

	// 监控中间件
	s.engine.Use(s.monitoringMiddleware())

	// 日志中间件
	s.engine.Use(s.loggingMiddleware())

	// CORS中间件
	s.engine.Use(s.corsMiddleware())

	// 认证中间件
	s.engine.Use(s.authMiddleware())

	// 错误处理中间件
	s.engine.Use(s.errorMiddleware())
}

// setupMonitorRoutes 设置监控路由
func (s *HTTPServer) setupMonitorRoutes() {
	// 基础健康检查
	s.engine.GET("/health", s.healthCheck)

	// 详细健康检查
	s.engine.GET("/health/detailed", s.detailedHealthCheck)

	// 组件健康状态
	s.engine.GET("/health/components", s.componentHealth)
}

// setupRoutes 设置路由
func (s *HTTPServer) setupRoutes() {
	// API版本分组
	v1 := s.engine.Group("/api/v1")
	{
		// 健康检查
		v1.GET("/health", s.healthCheck)

		// 玩家相关接口
		players := v1.Group("/players")
		{
			players.POST("/login", s.playerLogin)
			players.POST("/logout", s.playerLogout)
			players.GET("/:id", s.getPlayer)
			players.PUT("/:id", s.updatePlayer)
		}

		// 道具相关接口
		items := v1.Group("/items")
		{
			items.GET("", s.getItems)
			items.POST("", s.createItem)
			items.GET("/:id", s.getItem)
			items.PUT("/:id", s.updateItem)
			items.DELETE("/:id", s.deleteItem)
		}

		// 订单相关接口
		orders := v1.Group("/orders")
		{
			orders.GET("", s.getOrders)
			orders.POST("", s.createOrder)
			orders.GET("/:id", s.getOrder)
			orders.PUT("/:id/status", s.updateOrderStatus)
		}

		// 游戏相关接口
		games := v1.Group("/games")
		{
			games.GET("", s.getGames)
			games.GET("/:id/stats", s.getGameStats)
		}
	}

	// 监控接口
	s.engine.GET("/metrics", s.metrics)

	// WebSocket接口（预留）
	s.engine.GET("/ws", s.websocketHandler)
}

// monitoringMiddleware 监控中间件
func (s *HTTPServer) monitoringMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录请求统计
		duration := time.Since(start)
		statusCode := c.Writer.Status()
		success := statusCode >= 200 && statusCode < 400

		if s.monitor != nil {
			s.monitor.RecordRequest(duration, success)
		}
	}
}

// loggingMiddleware 日志中间件
func (s *HTTPServer) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 计算延迟
		latency := time.Since(start)

		// 获取客户端IP
		clientIP := c.ClientIP()

		// 获取请求方法和状态码
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		s.logger.Info("HTTP请求",
			"method", method,
			"path", path,
			"status", statusCode,
			"latency", latency,
			"client_ip", clientIP,
			"user_agent", c.Request.UserAgent(),
		)
	}
}

// corsMiddleware CORS中间件
func (s *HTTPServer) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// authMiddleware 认证中间件
func (s *HTTPServer) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过健康检查和监控接口
		if c.Request.URL.Path == "/api/v1/health" ||
			c.Request.URL.Path == "/health" ||
			c.Request.URL.Path == "/health/detailed" ||
			c.Request.URL.Path == "/health/components" ||
			c.Request.URL.Path == "/metrics" ||
			c.Request.URL.Path == "/api/v1/health/detailed" ||
			c.Request.URL.Path == "/api/v1/metrics" ||
			c.Request.URL.Path == "/api/v1/health/components" {
			c.Next()
			return
		}

		// TODO: 实现JWT认证逻辑
		// 这里暂时跳过认证
		c.Next()
	}
}

// errorMiddleware 错误处理中间件
func (s *HTTPServer) errorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			bizErr := s.errorHandler.Handle(err.Err, "HTTP请求处理失败")

			c.JSON(bizErr.HTTPStatus, gin.H{
				"code":    bizErr.Code,
				"message": bizErr.Message,
				"data":    nil,
			})
			c.Abort()
			return
		}
	}
}

// 路由处理器

// healthCheck 健康检查
func (s *HTTPServer) healthCheck(c *gin.Context) {
	metrics := s.monitor.GetSystemMetrics()

	// 检查是否有严重问题
	hasCriticalIssues := false
	for _, component := range metrics.Components {
		if component.Status == "unhealthy" {
			hasCriticalIssues = true
			break
		}
	}

	status := "ok"
	if hasCriticalIssues {
		status = "warning"
	}

	c.JSON(200, gin.H{
		"status":    status,
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"uptime":    metrics.Uptime,
	})
}

// detailedHealthCheck 详细健康检查
func (s *HTTPServer) detailedHealthCheck(c *gin.Context) {
	metrics := s.monitor.GetSystemMetrics()

	// 整体健康状态
	overallStatus := "healthy"
	unhealthyCount := 0

	for _, component := range metrics.Components {
		if component.Status == "unhealthy" {
			unhealthyCount++
			overallStatus = "unhealthy"
		} else if component.Status == "unknown" && overallStatus == "healthy" {
			overallStatus = "warning"
		}
	}

	response := gin.H{
		"status":    overallStatus,
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"uptime":    metrics.Uptime,
		"system_metrics": gin.H{
			"total_requests":    metrics.TotalRequests,
			"active_requests":   metrics.ActiveRequests,
			"failed_requests":   metrics.FailedRequests,
			"avg_response_time": metrics.AvgResponseTime.String(),
			"goroutines":        metrics.Goroutines,
		},
		"memory": gin.H{
			"alloc_mb":       float64(metrics.Memory.Alloc) / 1024 / 1024,
			"total_alloc_mb": float64(metrics.Memory.TotalAlloc) / 1024 / 1024,
			"sys_mb":         float64(metrics.Memory.Sys) / 1024 / 1024,
			"heap_alloc_mb":  float64(metrics.Memory.HeapAlloc) / 1024 / 1024,
			"heap_sys_mb":    float64(metrics.Memory.HeapSys) / 1024 / 1024,
			"heap_idle_mb":   float64(metrics.Memory.HeapIdle) / 1024 / 1024,
			"heap_inuse_mb":  float64(metrics.Memory.HeapInuse) / 1024 / 1024,
			"heap_objects":   metrics.Memory.HeapObjects,
			"num_gc":         metrics.Memory.NumGC,
		},
		"components":      metrics.Components,
		"unhealthy_count": unhealthyCount,
	}

	c.JSON(http.StatusOK, response)
}

// systemMetrics 系统指标
func (s *HTTPServer) systemMetrics(c *gin.Context) {
	metrics := s.monitor.GetSystemMetrics()
	customMetrics := s.monitor.GetAllCustomMetrics()

	response := gin.H{
		"timestamp": time.Now().Unix(),
		"system": gin.H{
			"uptime_seconds": metrics.Uptime,
			"goroutines":     metrics.Goroutines,
			"memory": gin.H{
				"alloc_bytes":       metrics.Memory.Alloc,
				"total_alloc_bytes": metrics.Memory.TotalAlloc,
				"sys_bytes":         metrics.Memory.Sys,
				"heap_alloc_bytes":  metrics.Memory.HeapAlloc,
				"heap_sys_bytes":    metrics.Memory.HeapSys,
				"heap_idle_bytes":   metrics.Memory.HeapIdle,
				"heap_inuse_bytes":  metrics.Memory.HeapInuse,
				"heap_objects":      metrics.Memory.HeapObjects,
				"num_gc":            metrics.Memory.NumGC,
			},
		},
		"requests": gin.H{
			"total":                metrics.TotalRequests,
			"active":               metrics.ActiveRequests,
			"failed":               metrics.FailedRequests,
			"avg_response_time_ns": metrics.AvgResponseTime.Nanoseconds(),
			"avg_response_time_ms": float64(metrics.AvgResponseTime.Nanoseconds()) / 1000000,
		},
		"components": metrics.Components,
		"custom":     customMetrics,
	}

	c.JSON(http.StatusOK, response)
}

// componentHealth 组件健康状态
func (s *HTTPServer) componentHealth(c *gin.Context) {
	metrics := s.monitor.GetSystemMetrics()

	// 支持查询参数过滤
	component := c.Query("component")
	if component != "" {
		if status, exists := metrics.Components[component]; exists {
			c.JSON(http.StatusOK, gin.H{
				"component": component,
				"status":    status,
			})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "组件不存在",
			"component": component,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"timestamp":  time.Now().Unix(),
		"components": metrics.Components,
	})
}

// playerLogin 玩家登录
func (s *HTTPServer) playerLogin(c *gin.Context) {
	var req struct {
		GameID   string `json:"game_id" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// TODO: 实现玩家登录逻辑
	s.logger.Info("玩家登录请求", "game_id", req.GameID, "username", req.Username)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "登录成功",
		"data": gin.H{
			"user_id":    "user123",
			"token":      "jwt_token_here",
			"expires_at": time.Now().Add(24 * time.Hour).Unix(),
		},
	})
}

// playerLogout 玩家登出
func (s *HTTPServer) playerLogout(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// TODO: 实现玩家登出逻辑
	s.logger.Info("玩家登出请求", "user_id", req.UserID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "登出成功",
	})
}

// getPlayer 获取玩家信息
func (s *HTTPServer) getPlayer(c *gin.Context) {
	userID := c.Param("id")

	// TODO: 从数据库获取玩家信息
	s.logger.Info("获取玩家信息", "user_id", userID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data": gin.H{
			"user_id":    userID,
			"username":   "testuser",
			"level":      10,
			"coins":      1000,
			"created_at": time.Now().Add(-24 * time.Hour).Unix(),
		},
	})
}

// updatePlayer 更新玩家信息
func (s *HTTPServer) updatePlayer(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Level int `json:"level"`
		Coins int `json:"coins"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// TODO: 更新玩家信息
	s.logger.Info("更新玩家信息", "user_id", userID, "level", req.Level, "coins", req.Coins)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// getItems 获取道具列表
func (s *HTTPServer) getItems(c *gin.Context) {
	userID := c.Query("user_id")
	gameID := c.Query("game_id")

	// TODO: 从数据库获取道具列表
	s.logger.Info("获取道具列表", "user_id", userID, "game_id", gameID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data": gin.H{
			"items": []gin.H{
				{
					"item_id":  "item001",
					"name":     "金币",
					"quantity": 1000,
					"type":     "currency",
				},
				{
					"item_id":  "item002",
					"name":     "钻石",
					"quantity": 100,
					"type":     "currency",
				},
			},
		},
	})
}

// createItem 创建道具
func (s *HTTPServer) createItem(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		GameID   string `json:"game_id" binding:"required"`
		ItemID   string `json:"item_id" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Quantity int    `json:"quantity" binding:"required"`
		Type     string `json:"type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// TODO: 创建道具
	s.logger.Info("创建道具", "user_id", req.UserID, "item_id", req.ItemID, "name", req.Name)

	c.JSON(201, gin.H{
		"code":    0,
		"message": "创建成功",
		"data": gin.H{
			"item_id": req.ItemID,
		},
	})
}

// getItem 获取道具详情
func (s *HTTPServer) getItem(c *gin.Context) {
	itemID := c.Param("id")

	// TODO: 获取道具详情
	s.logger.Info("获取道具详情", "item_id", itemID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data": gin.H{
			"item_id":     itemID,
			"name":        "测试道具",
			"quantity":    50,
			"type":        "consumable",
			"description": "这是一个测试道具",
		},
	})
}

// updateItem 更新道具
func (s *HTTPServer) updateItem(c *gin.Context) {
	itemID := c.Param("id")

	var req struct {
		Quantity int `json:"quantity" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// TODO: 更新道具
	s.logger.Info("更新道具", "item_id", itemID, "quantity", req.Quantity)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// deleteItem 删除道具
func (s *HTTPServer) deleteItem(c *gin.Context) {
	itemID := c.Param("id")

	// TODO: 删除道具
	s.logger.Info("删除道具", "item_id", itemID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// getOrders 获取订单列表
func (s *HTTPServer) getOrders(c *gin.Context) {
	userID := c.Query("user_id")
	status := c.Query("status")

	// TODO: 获取订单列表
	s.logger.Info("获取订单列表", "user_id", userID, "status", status)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data": gin.H{
			"orders": []gin.H{
				{
					"order_id":   "order001",
					"user_id":    userID,
					"amount":     100,
					"currency":   "CNY",
					"status":     "completed",
					"created_at": time.Now().Add(-1 * time.Hour).Unix(),
				},
			},
		},
	})
}

// createOrder 创建订单
func (s *HTTPServer) createOrder(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		GameID   string `json:"game_id" binding:"required"`
		Amount   int    `json:"amount" binding:"required"`
		Currency string `json:"currency" binding:"required"`
		ItemID   string `json:"item_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// TODO: 创建订单
	s.logger.Info("创建订单", "user_id", req.UserID, "amount", req.Amount, "currency", req.Currency)

	c.JSON(201, gin.H{
		"code":    0,
		"message": "订单创建成功",
		"data": gin.H{
			"order_id": "order_" + time.Now().Format("20060102150405"),
			"status":   "pending",
		},
	})
}

// getOrder 获取订单详情
func (s *HTTPServer) getOrder(c *gin.Context) {
	orderID := c.Param("id")

	// TODO: 获取订单详情
	s.logger.Info("获取订单详情", "order_id", orderID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data": gin.H{
			"order_id":   orderID,
			"user_id":    "user123",
			"amount":     100,
			"currency":   "CNY",
			"status":     "completed",
			"created_at": time.Now().Add(-1 * time.Hour).Unix(),
			"updated_at": time.Now().Unix(),
		},
	})
}

// updateOrderStatus 更新订单状态
func (s *HTTPServer) updateOrderStatus(c *gin.Context) {
	orderID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// TODO: 更新订单状态
	s.logger.Info("更新订单状态", "order_id", orderID, "status", req.Status)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// getGames 获取游戏列表
func (s *HTTPServer) getGames(c *gin.Context) {
	// TODO: 获取游戏列表
	s.logger.Info("获取游戏列表")

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data": gin.H{
			"games": []gin.H{
				{
					"game_id":    "game1",
					"name":       "游戏1",
					"status":     "active",
					"players":    1250,
					"created_at": time.Now().Add(-30 * 24 * time.Hour).Unix(),
				},
				{
					"game_id":    "game2",
					"name":       "游戏2",
					"status":     "active",
					"players":    890,
					"created_at": time.Now().Add(-15 * 24 * time.Hour).Unix(),
				},
			},
		},
	})
}

// getGameStats 获取游戏统计
func (s *HTTPServer) getGameStats(c *gin.Context) {
	gameID := c.Param("id")

	// TODO: 获取游戏统计
	s.logger.Info("获取游戏统计", "game_id", gameID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data": gin.H{
			"game_id":        gameID,
			"total_players":  1250,
			"active_players": 450,
			"total_orders":   5600,
			"total_revenue":  125000,
			"currency":       "CNY",
			"last_updated":   time.Now().Unix(),
		},
	})
}

// metrics 监控指标
func (s *HTTPServer) metrics(c *gin.Context) {
	// TODO: 实现Prometheus监控指标
	c.JSON(200, gin.H{
		"status":    "metrics endpoint - TODO",
		"timestamp": time.Now().Unix(),
	})
}

// websocketHandler WebSocket处理器
func (s *HTTPServer) websocketHandler(c *gin.Context) {
	// TODO: 实现WebSocket连接处理
	c.JSON(501, gin.H{
		"code":    501,
		"message": "WebSocket功能尚未实现",
	})
}
