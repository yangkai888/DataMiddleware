package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"datamiddleware/internal/async"
	"datamiddleware/internal/auth"
	"datamiddleware/internal/cache"
	"datamiddleware/internal/database"
	"datamiddleware/internal/errors"
	"datamiddleware/internal/logger"
	"datamiddleware/internal/monitor"
	"datamiddleware/internal/services"
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
	jwtService   *auth.JWTService     `json:"-"`      // JWT认证服务
	playerService *services.PlayerService `json:"-"`  // 玩家服务
	itemService   *services.ItemService   `json:"-"`  // 道具服务
	orderService  *services.OrderService  `json:"-"`  // 订单服务
	cacheManager *cache.Manager          `json:"-"`  // 缓存管理器
	taskScheduler *async.TaskScheduler   `json:"-"`  // 任务调度器
}

// NewHTTPServer 创建HTTP服务器
func NewHTTPServer(config types.ServerConfig, log logger.Logger, errorHandler *errors.ErrorHandler, dao database.DAO, jwtService *auth.JWTService, playerService *services.PlayerService, itemService *services.ItemService, orderService *services.OrderService, cacheManager *cache.Manager, taskScheduler *async.TaskScheduler) *HTTPServer {
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
		config:        config,
		engine:        engine,
		logger:        log,
		errorHandler:  errorHandler,
		dao:           dao,
		monitor:       monitor,
		jwtService:    jwtService,
		playerService: playerService,
		itemService:   itemService,
		orderService:  orderService,
		cacheManager:  cacheManager,
		taskScheduler: taskScheduler,
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
	s.logger.Info("开始设置路由...路由设置函数被调用")
	// API版本分组
	v1 := s.engine.Group("/api/v1")
	{
		// 健康检查
		v1.GET("/health", s.healthCheck)

		// 玩家相关接口
		players := v1.Group("/players")
		{
			players.POST("/register", s.playerRegister)
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

		// 缓存相关接口
		cache := v1.Group("/cache")
		{
			cache.POST("/set", s.setCache)
			cache.GET("/get", s.getCache)
			cache.POST("/set-json", s.setCacheJSON)
			cache.GET("/get-json", s.getCacheJSON)
			cache.DELETE("/delete", s.deleteCache)
			cache.GET("/exists", s.existsCache)
			cache.POST("/warmup", s.warmupCache)
			cache.GET("/protection/stats", s.getProtectionStats)
			cache.POST("/invalidate", s.invalidateCache)
		}

		// 异步任务接口
		async := v1.Group("/async")
		{
			async.POST("/task", s.submitTask)
			async.GET("/stats", s.getAsyncStats)
		}

		// 监控接口
		monitor := v1.Group("/monitor")
		{
			monitor.GET("/metrics", s.getSystemMetrics)
		}
	}

	// 监控接口
	s.engine.GET("/metrics", s.metrics)

	// WebSocket接口（预留）
	s.engine.GET("/ws", s.websocketHandler)

	s.logger.Info("路由设置完成")
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
		// 跳过健康检查、监控、认证、缓存和异步相关接口
		if c.Request.URL.Path == "/api/v1/health" ||
			c.Request.URL.Path == "/health" ||
			c.Request.URL.Path == "/health/detailed" ||
			c.Request.URL.Path == "/health/components" ||
			c.Request.URL.Path == "/metrics" ||
			c.Request.URL.Path == "/api/v1/health/detailed" ||
			c.Request.URL.Path == "/api/v1/metrics" ||
			c.Request.URL.Path == "/api/v1/health/components" ||
			c.Request.URL.Path == "/api/v1/players/register" ||
			c.Request.URL.Path == "/api/v1/players/login" ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/cache/") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/async/") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/monitor/") {
			c.Next()
			return
		}

		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			s.logger.Warn("缺少Authorization头", "path", c.Request.URL.Path)
			c.AbortWithStatusJSON(401, gin.H{
				"code":    401,
				"message": "缺少认证令牌",
			})
			return
		}

		// 从Authorization头提取令牌
		token, err := s.jwtService.ExtractTokenFromHeader(authHeader)
		if err != nil {
			s.logger.Warn("无效的Authorization头格式", "header", authHeader[:20]+"...")
			c.AbortWithStatusJSON(401, gin.H{
				"code":    401,
				"message": "无效的认证令牌格式",
			})
			return
		}

		// 验证JWT令牌
		claims, err := s.jwtService.ValidateToken(token)
		if err != nil {
			s.logger.Warn("JWT令牌验证失败", "error", err)
			c.AbortWithStatusJSON(401, gin.H{
				"code":    401,
				"message": "认证令牌无效或已过期",
			})
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", claims.UserID)
		c.Set("game_id", claims.GameID)
		c.Set("username", claims.Username)
		c.Set("token_id", claims.TokenID)

		s.logger.Debug("JWT认证成功", "user_id", claims.UserID, "path", c.Request.URL.Path)
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

// playerRegister 玩家注册
func (s *HTTPServer) playerRegister(c *gin.Context) {
	var req struct {
		GameID   string `json:"game_id" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// 调用玩家服务注册
	player, err := s.playerService.RegisterPlayer(req.GameID, req.Username, req.Password, req.Email, req.Phone)
	if err != nil {
		s.logger.Warn("玩家注册失败", "username", req.Username, "game_id", req.GameID, "error", err)
		bizErr := s.errorHandler.Handle(err, "注册失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	s.logger.Info("玩家注册成功", "user_id", player.UserID, "username", req.Username, "game_id", req.GameID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "注册成功",
		"data":    player,
	})
}

// playerLogin 玩家登录
func (s *HTTPServer) playerLogin(c *gin.Context) {
	var req struct {
		GameID   string `json:"game_id" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		DeviceID string `json:"device_id"`
		Platform string `json:"platform"`
		Version  string `json:"version"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// 设置默认值
	if req.DeviceID == "" {
		req.DeviceID = "web"
	}
	if req.Platform == "" {
		req.Platform = "web"
	}
	if req.Version == "" {
		req.Version = "1.0.0"
	}

	// 调用玩家服务登录
	result, err := s.playerService.LoginPlayerByUsername(req.Username, req.Password, req.GameID, req.DeviceID, req.Platform, req.Version)
	if err != nil {
		s.logger.Warn("玩家登录失败", "username", req.Username, "game_id", req.GameID, "error", err)
		bizErr := s.errorHandler.Handle(err, "登录失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// 生成JWT令牌
	tokenPair, err := s.jwtService.GenerateToken(result.User.UserID, req.GameID, result.User.Username)
	if err != nil {
		s.logger.Error("生成JWT令牌失败", "user_id", result.User.UserID, "error", err)
		bizErr := s.errorHandler.Handle(err, "令牌生成失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	s.logger.Info("玩家登录成功", "user_id", result.User.UserID, "username", req.Username, "game_id", req.GameID)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "登录成功",
		"data": gin.H{
			"user":       result.User,
			"session_id": result.SessionID,
			"token": gin.H{
				"access_token":  tokenPair.AccessToken,
				"refresh_token": tokenPair.RefreshToken,
				"token_type":    tokenPair.TokenType,
				"expires_in":    tokenPair.ExpiresIn,
				"expires_at":    tokenPair.ExpiresAt,
			},
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

	// 调用玩家服务获取信息
	player, err := s.playerService.GetPlayer(userID)
	if err != nil {
		s.logger.Warn("获取玩家信息失败", "user_id", userID, "error", err)
		bizErr := s.errorHandler.Handle(err, "获取玩家信息失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data":    player,
	})
}

// updatePlayer 更新玩家信息
func (s *HTTPServer) updatePlayer(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}

	if len(updates) == 0 {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "没有需要更新的字段",
		})
		return
	}

	// 调用玩家服务更新
	player, err := s.playerService.UpdatePlayer(userID, updates)
	if err != nil {
		s.logger.Warn("更新玩家信息失败", "user_id", userID, "error", err)
		bizErr := s.errorHandler.Handle(err, "更新玩家信息失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    player,
	})
}

// getItems 获取道具列表
func (s *HTTPServer) getItems(c *gin.Context) {
	userID := c.Query("user_id")
	gameID := c.Query("game_id")

	if userID == "" {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "缺少用户ID参数",
		})
		return
	}

	// 调用道具服务获取列表
	items, err := s.itemService.GetUserItems(userID, gameID)
	if err != nil {
		s.logger.Warn("获取道具列表失败", "user_id", userID, "game_id", gameID, "error", err)
		bizErr := s.errorHandler.Handle(err, "获取道具列表失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "获取成功",
		"data": gin.H{
			"items": items,
			"total": len(items),
		},
	})
}

// createItem 创建道具
func (s *HTTPServer) createItem(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		GameID   string `json:"game_id" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Quantity int    `json:"quantity" binding:"required"`
		Type     string `json:"type" binding:"required"`
		Category string `json:"category"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		bizErr := s.errorHandler.Handle(err, "参数绑定失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	// 调用道具服务创建道具
	item, err := s.itemService.CreateItem(req.UserID, req.GameID, req.Name, req.Type, req.Category, int64(req.Quantity))
	if err != nil {
		s.logger.Error("创建道具失败", "user_id", req.UserID, "game_id", req.GameID, "name", req.Name, "error", err)
		bizErr := s.errorHandler.Handle(err, "创建道具失败")
		c.JSON(bizErr.HTTPStatus, gin.H{
			"code":    bizErr.Code,
			"message": bizErr.Message,
		})
		return
	}

	c.JSON(201, gin.H{
		"code":    0,
		"message": "创建成功",
		"data": gin.H{
			"item_id": item.ItemID,
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

// ==================== 缓存相关Handler ====================

// setCache 设置缓存
func (s *HTTPServer) setCache(c *gin.Context) {
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "参数绑定失败",
			"error":   err.Error(),
		})
		return
	}

	err := s.cacheManager.Set(req.Key, []byte(req.Value))
	if err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "缓存设置失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "缓存设置成功",
	})
}

// getCache 获取缓存
func (s *HTTPServer) getCache(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "缺少key参数",
		})
		return
	}

	value, err := s.cacheManager.Get(key)
	if err != nil {
		if err.Error() == "cache miss" {
			c.JSON(404, gin.H{
				"code":    404,
				"message": "缓存未找到",
			})
			return
		}
		c.JSON(500, gin.H{
			"code":    500,
			"message": "操作失败",
			"error":   err.Error(),
		})
		return
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"value":   string(value),
	})
}

// setCacheJSON 设置JSON缓存
func (s *HTTPServer) setCacheJSON(c *gin.Context) {
	var req struct {
		Key   string      `json:"key" binding:"required"`
		Value interface{} `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "参数绑定失败",
			"error":   err.Error(),
		})
		return
	}

	err := s.cacheManager.SetJSON(req.Key, req.Value)
	if err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "JSON缓存设置失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "JSON缓存设置成功",
	})
}

// getCacheJSON 获取JSON缓存
func (s *HTTPServer) getCacheJSON(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "缺少key参数",
		})
		return
	}

	var value interface{}
	err := s.cacheManager.GetJSON(key, &value)
	if err != nil {
		if err.Error() == "cache miss" {
			c.JSON(404, gin.H{
				"code":    404,
				"message": "缓存未找到",
			})
			return
		}
		c.JSON(500, gin.H{
			"code":    500,
			"message": "操作失败",
			"error":   err.Error(),
		})
		return
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"value":   value,
	})
}

// deleteCache 删除缓存
func (s *HTTPServer) deleteCache(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "缺少key参数",
		})
		return
	}

	err := s.cacheManager.Delete(key)
	if err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "操作失败",
			"error":   err.Error(),
		})
		return
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "缓存删除成功",
	})
}

// existsCache 检查缓存是否存在
func (s *HTTPServer) existsCache(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "缺少key参数",
		})
		return
	}

	exists := s.cacheManager.Exists(key)
	c.JSON(200, gin.H{
		"success": true,
		"exists":  exists,
	})
}

// warmupCache 缓存预热
func (s *HTTPServer) warmupCache(c *gin.Context) {
	warmup := cache.NewDefaultWarmup(s.logger)
	err := s.cacheManager.WarmupCache(warmup)
	if err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "操作失败",
			"error":   err.Error(),
		})
		return
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "缓存预热完成",
	})
}

// getProtectionStats 获取缓存防护统计
func (s *HTTPServer) getProtectionStats(c *gin.Context) {
	stats := s.cacheManager.GetProtectionStats()
	c.JSON(200, gin.H{
		"success": true,
		"stats":   stats,
	})
}

// invalidateCache 缓存失效
func (s *HTTPServer) invalidateCache(c *gin.Context) {
	var req struct {
		Pattern string `json:"pattern"`
		Prefix  string `json:"prefix"`
		Keys    []string `json:"keys"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "操作失败",
			"error":   err.Error(),
		})
		return
		return
	}

	var err error
	if req.Pattern != "" {
		err = s.cacheManager.InvalidateByPattern(req.Pattern)
	} else if req.Prefix != "" {
		err = s.cacheManager.InvalidateByPrefix(req.Prefix)
	} else if len(req.Keys) > 0 {
		err = s.cacheManager.BatchInvalidate(req.Keys)
	} else {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "需要指定pattern、prefix或keys之一",
		})
		return
	}

	if err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "操作失败",
			"error":   err.Error(),
		})
		return
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "缓存失效完成",
	})
}

// ==================== 异步处理相关Handler ====================

// submitTask 提交异步任务
func (s *HTTPServer) submitTask(c *gin.Context) {
	var req struct {
		ID       string      `json:"id" binding:"required"`
		Type     string      `json:"type" binding:"required"`
		Priority int         `json:"priority"`
		Data     interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "操作失败",
			"error":   err.Error(),
		})
		return
		return
	}

	task := &async.BaseTask{
		ID:       req.ID,
		Type:     req.Type,
		Priority: req.Priority,
		Data:     req.Data,
	}

	err := s.taskScheduler.SubmitTask(task)
	if err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "操作失败",
			"error":   err.Error(),
		})
		return
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "任务提交成功",
		"task_id": req.ID,
	})
}

// getAsyncStats 获取异步处理统计
func (s *HTTPServer) getAsyncStats(c *gin.Context) {
	stats := s.taskScheduler.GetStats()
	c.JSON(200, gin.H{
		"success": true,
		"stats":   stats,
	})
}

// ==================== 监控相关Handler ====================

// getSystemMetrics 获取系统指标
func (s *HTTPServer) getSystemMetrics(c *gin.Context) {
	metrics := s.monitor.GetSystemMetrics()
	c.JSON(200, gin.H{
		"success": true,
		"metrics": metrics,
	})
}
