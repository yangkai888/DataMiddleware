package monitor

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"datamiddleware/internal/cache"
	"datamiddleware/internal/logger"

	"github.com/redis/go-redis/v9"
)

// DatabaseHealthChecker 数据库健康检查器
type DatabaseHealthChecker struct {
	db     *sql.DB
	logger logger.Logger
}

func NewDatabaseHealthChecker(db *sql.DB, logger logger.Logger) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		db:     db,
		logger: logger,
	}
}

func (c *DatabaseHealthChecker) Name() string {
	return "database"
}

func (c *DatabaseHealthChecker) Check(ctx context.Context) HealthStatus {
	start := time.Now()

	if c.db == nil {
		return HealthStatus{
			Status:    "unknown",
			Message:   "数据库未配置",
			Timestamp: time.Now(),
			Response:  time.Since(start).Milliseconds(),
		}
	}

	// 执行简单的健康检查查询
	err := c.db.PingContext(ctx)
	response := time.Since(start).Milliseconds()

	if err != nil {
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("数据库连接失败: %v", err),
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	return HealthStatus{
		Status:    "healthy",
		Message:   "数据库连接正常",
		Timestamp: time.Now(),
		Response:  response,
	}
}

// CacheHealthChecker 缓存健康检查器
type CacheHealthChecker struct {
	cache  cache.Manager
	logger logger.Logger
}

func NewCacheHealthChecker(cache cache.Manager, logger logger.Logger) *CacheHealthChecker {
	return &CacheHealthChecker{
		cache:  cache,
		logger: logger,
	}
}

func (c *CacheHealthChecker) Name() string {
	return "cache"
}

func (c *CacheHealthChecker) Check(ctx context.Context) HealthStatus {
	start := time.Now()

	// 测试缓存读写
	testKey := "__health_check__"
	testValue := []byte(fmt.Sprintf("health_check_%d", time.Now().Unix()))

	// 尝试写入
	err := c.cache.Set(testKey, testValue)
	if err != nil {
		response := time.Since(start).Milliseconds()
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("缓存写入失败: %v", err),
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	// 尝试读取
	readValue, err := c.cache.Get(testKey)
	if err != nil {
		response := time.Since(start).Milliseconds()
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("缓存读取失败: %v", err),
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	// 验证数据一致性
	if string(readValue) != string(testValue) {
		response := time.Since(start).Milliseconds()
		return HealthStatus{
			Status:    "unhealthy",
			Message:   "缓存数据不一致",
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	// 清理测试数据
	c.cache.Delete(testKey)

	response := time.Since(start).Milliseconds()
	return HealthStatus{
		Status:    "healthy",
		Message:   "缓存服务正常",
		Timestamp: time.Now(),
		Response:  response,
	}
}

// RedisHealthChecker Redis健康检查器
type RedisHealthChecker struct {
	client *redis.Client
	logger logger.Logger
}

func NewRedisHealthChecker(client *redis.Client, logger logger.Logger) *RedisHealthChecker {
	return &RedisHealthChecker{
		client: client,
		logger: logger,
	}
}

func (c *RedisHealthChecker) Name() string {
	return "redis"
}

func (c *RedisHealthChecker) Check(ctx context.Context) HealthStatus {
	start := time.Now()

	if c.client == nil {
		return HealthStatus{
			Status:    "unknown",
			Message:   "Redis未配置",
			Timestamp: time.Now(),
			Response:  time.Since(start).Milliseconds(),
		}
	}

	// 执行PING命令
	pong, err := c.client.Ping(ctx).Result()
	response := time.Since(start).Milliseconds()

	if err != nil {
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Redis连接失败: %v", err),
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	if pong != "PONG" {
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Redis响应异常: %s", pong),
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	return HealthStatus{
		Status:    "healthy",
		Message:   "Redis连接正常",
		Timestamp: time.Now(),
		Response:  response,
	}
}

// HTTPHealthChecker HTTP服务健康检查器
type HTTPHealthChecker struct {
	url    string
	logger logger.Logger
}

func NewHTTPHealthChecker(url string, logger logger.Logger) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		url:    url,
		logger: logger,
	}
}

func (c *HTTPHealthChecker) Name() string {
	return "http_server"
}

func (c *HTTPHealthChecker) Check(ctx context.Context) HealthStatus {
	start := time.Now()

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		response := time.Since(start).Milliseconds()
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("创建请求失败: %v", err),
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	resp, err := client.Do(req)
	response := time.Since(start).Milliseconds()

	if err != nil {
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("HTTP请求失败: %v", err),
			Timestamp: time.Now(),
			Response:  response,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("HTTP状态码异常: %d", resp.StatusCode),
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	return HealthStatus{
		Status:    "healthy",
		Message:   "HTTP服务正常",
		Timestamp: time.Now(),
		Response:  response,
	}
}

// OverallHealthChecker 整体健康检查器
type OverallHealthChecker struct {
	checkers []HealthChecker
	logger   logger.Logger
}

func NewOverallHealthChecker(logger logger.Logger) *OverallHealthChecker {
	return &OverallHealthChecker{
		checkers: make([]HealthChecker, 0),
		logger:   logger,
	}
}

func (c *OverallHealthChecker) AddChecker(checker HealthChecker) {
	c.checkers = append(c.checkers, checker)
}

func (c *OverallHealthChecker) Name() string {
	return "overall"
}

func (c *OverallHealthChecker) Check(ctx context.Context) HealthStatus {
	start := time.Now()
	totalResponse := int64(0)
	unhealthyCount := 0
	messages := make([]string, 0)

	for _, checker := range c.checkers {
		status := checker.Check(ctx)
		totalResponse += status.Response

		if status.Status != "healthy" {
			unhealthyCount++
			messages = append(messages, fmt.Sprintf("%s: %s", checker.Name(), status.Message))
		}
	}

	response := time.Since(start).Milliseconds()

	if unhealthyCount == 0 {
		return HealthStatus{
			Status:    "healthy",
			Message:   "所有组件正常",
			Timestamp: time.Now(),
			Response:  response,
		}
	}

	message := fmt.Sprintf("%d/%d 个组件异常", unhealthyCount, len(c.checkers))
	if len(messages) > 0 {
		message += ": " + messages[0] // 只显示第一个异常消息
	}

	return HealthStatus{
		Status:    "unhealthy",
		Message:   message,
		Timestamp: time.Now(),
		Response:  response,
	}
}
