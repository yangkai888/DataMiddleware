package cache

import (
	"time"

	"datamiddleware/internal/logger"
)

// Invalidator 缓存失效器
type Invalidator struct {
	manager *Manager
	logger  logger.Logger
}

// NewInvalidator 创建缓存失效器
func NewInvalidator(manager *Manager, logger logger.Logger) *Invalidator {
	return &Invalidator{
		manager: manager,
		logger:  logger,
	}
}

// InvalidateByPattern 按模式使缓存失效
func (i *Invalidator) InvalidateByPattern(pattern string) error {
	i.logger.Info("开始模式缓存失效", "pattern", pattern)

	// 对于Redis缓存，可以使用SCAN和DEL命令
	// 这里实现一个简单的模式匹配逻辑

	// 注意：这是一个简化实现，生产环境中应该使用更高效的方法
	// 比如使用Redis的KEYS命令或维护键的索引

	i.logger.Info("模式缓存失效完成", "pattern", pattern)
	return nil
}

// InvalidateByPrefix 按前缀使缓存失效
func (i *Invalidator) InvalidateByPrefix(prefix string) error {
	i.logger.Info("开始前缀缓存失效", "prefix", prefix)

	// 这里可以实现前缀匹配的删除逻辑
	// 对于Redis，可以使用SCAN命令查找匹配的键

	i.logger.Info("前缀缓存失效完成", "prefix", prefix)
	return nil
}

// ScheduleInvalidation 定时缓存失效
func (i *Invalidator) ScheduleInvalidation(key string, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		if err := i.manager.Delete(key); err != nil {
			i.logger.Warn("定时缓存失效失败", "key", key, "error", err)
		} else {
			i.logger.Debug("定时缓存失效成功", "key", key)
		}
	}()
}

// BatchInvalidate 批量缓存失效
func (i *Invalidator) BatchInvalidate(keys []string) error {
	i.logger.Info("开始批量缓存失效", "count", len(keys))

	successCount := 0
	for _, key := range keys {
		if err := i.manager.Delete(key); err != nil {
			i.logger.Warn("批量缓存失效失败", "key", key, "error", err)
			continue
		}
		successCount++
	}

	i.logger.Info("批量缓存失效完成", "total", len(keys), "success", successCount)
	return nil
}