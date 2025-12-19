package cache

import (
	"encoding/json"
	"fmt"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// Manager 缓存管理器
type Manager struct {
	l1          types.Cache    // L1本地缓存
	l2          types.Cache    // L2 Redis缓存
	invalidator *Invalidator   // 缓存失效器
	logger      logger.Logger
}

// NewManager 创建缓存管理器
func NewManager(config types.CacheConfig, logger logger.Logger) (*Manager, error) {
	manager := &Manager{
		logger: logger,
	}

	// 初始化L1本地缓存
	l1Cache, err := NewLocalCache(config.L1, logger)
	if err != nil {
		return nil, fmt.Errorf("初始化L1缓存失败: %w", err)
	}
	manager.l1 = l1Cache

	// 初始化L2 Redis缓存
	l2Cache, err := NewRedisCache(config.L2, logger)
	if err != nil {
		logger.Warn("L2缓存初始化失败，将使用L1缓存", "error", err)
		// L2缓存失败不影响启动
	}
	manager.l2 = l2Cache

	// 初始化缓存失效器
	manager.invalidator = NewInvalidator(manager, logger)

	return manager, nil
}

// Get 获取缓存值
func (m *Manager) Get(key string) ([]byte, error) {
	// 先查L1缓存
	if m.l1 != nil {
		if value, err := m.l1.Get(key); err == nil {
			m.logger.Debug("L1缓存命中", "key", key)
			return value, nil
		} else if err != types.ErrCacheMiss {
			m.logger.Warn("L1缓存查询失败", "key", key, "error", err)
		}
	}

	// L1未命中，查L2缓存
	if m.l2 != nil {
		if value, err := m.l2.Get(key); err == nil {
			m.logger.Debug("L2缓存命中", "key", key)
			// 同步到L1缓存
			if m.l1 != nil {
				if err := m.l1.Set(key, value); err != nil {
					m.logger.Warn("同步到L1缓存失败", "key", key, "error", err)
				}
			}
			return value, nil
		} else if err != types.ErrCacheMiss {
			m.logger.Warn("L2缓存查询失败", "key", key, "error", err)
		}
	}

	return nil, types.ErrCacheMiss
}

// Set 设置缓存值
func (m *Manager) Set(key string, value []byte) error {
	// 设置L1缓存
	if m.l1 != nil {
		if err := m.l1.Set(key, value); err != nil && err != types.ErrCacheDisabled {
			m.logger.Warn("L1缓存设置失败", "key", key, "error", err)
		}
	}

	// 设置L2缓存
	if m.l2 != nil {
		if err := m.l2.Set(key, value); err != nil && err != types.ErrCacheDisabled {
			m.logger.Warn("L2缓存设置失败", "key", key, "error", err)
		}
	}

	return nil
}

// SetWithTTL 设置缓存值并指定TTL
func (m *Manager) SetWithTTL(key string, value []byte, ttl time.Duration) error {
	// 设置L1缓存
	if m.l1 != nil {
		if err := m.l1.SetWithTTL(key, value, ttl); err != nil && err != types.ErrCacheDisabled {
			m.logger.Warn("L1缓存设置失败", "key", key, "error", err)
		}
	}

	// 设置L2缓存
	if m.l2 != nil {
		if err := m.l2.SetWithTTL(key, value, ttl); err != nil && err != types.ErrCacheDisabled {
			m.logger.Warn("L2缓存设置失败", "key", key, "error", err)
		}
	}

	return nil
}

// Delete 删除缓存值
func (m *Manager) Delete(key string) error {
	// 删除L1缓存
	if m.l1 != nil {
		if err := m.l1.Delete(key); err != nil && err != types.ErrCacheDisabled {
			m.logger.Warn("L1缓存删除失败", "key", key, "error", err)
		}
	}

	// 删除L2缓存
	if m.l2 != nil {
		if err := m.l2.Delete(key); err != nil && err != types.ErrCacheDisabled {
			m.logger.Warn("L2缓存删除失败", "key", key, "error", err)
		}
	}

	return nil
}

// Exists 检查键是否存在
func (m *Manager) Exists(key string) bool {
	// 先查L1缓存
	if m.l1 != nil && m.l1.Exists(key) {
		return true
	}

	// 再查L2缓存
	if m.l2 != nil && m.l2.Exists(key) {
		return true
	}

	return false
}

// Clear 清空缓存
func (m *Manager) Clear() error {
	// 清空L1缓存
	if m.l1 != nil {
		if err := m.l1.Clear(); err != nil && err != types.ErrCacheDisabled {
			m.logger.Warn("L1缓存清空失败", "error", err)
		}
	}

	// 清空L2缓存
	if m.l2 != nil {
		if err := m.l2.Clear(); err != nil && err != types.ErrCacheDisabled {
			m.logger.Warn("L2缓存清空失败", "error", err)
		}
	}

	return nil
}

// Close 关闭缓存管理器
func (m *Manager) Close() error {
	var errs []error

	// 关闭L1缓存
	if m.l1 != nil {
		if err := m.l1.Close(); err != nil {
			errs = append(errs, fmt.Errorf("L1缓存关闭失败: %w", err))
		}
	}

	// 关闭L2缓存
	if m.l2 != nil {
		if err := m.l2.Close(); err != nil {
			errs = append(errs, fmt.Errorf("L2缓存关闭失败: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("缓存关闭失败: %v", errs)
	}

	return nil
}

// GetStats 获取缓存统计信息
func (m *Manager) GetStats() CacheStats {
	stats := CacheStats{
		L1Enabled: m.l1 != nil,
		L2Enabled: m.l2 != nil,
	}

	// 这里可以添加更详细的统计信息
	// 比如命中率、缓存大小等

	return stats
}

// CacheStats 缓存统计信息
type CacheStats struct {
	L1Enabled bool `json:"l1_enabled"`
	L2Enabled bool `json:"l2_enabled"`
	// 可以添加更多统计字段
}

// SetJSON 设置JSON对象到缓存
func (m *Manager) SetJSON(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}
	return m.Set(key, data)
}

// SetJSONWithTTL 设置JSON对象到缓存并指定TTL
func (m *Manager) SetJSONWithTTL(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}
	return m.SetWithTTL(key, data, ttl)
}

// GetJSON 从缓存获取并反序列化为JSON对象
func (m *Manager) GetJSON(key string, value interface{}) error {
	data, err := m.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, value)
}

// Preload 预加载热点数据到缓存
func (m *Manager) Preload(data map[string][]byte) error {
	m.logger.Info("开始缓存预热", "count", len(data))

	successCount := 0
	for key, value := range data {
		if err := m.Set(key, value); err != nil {
			m.logger.Warn("缓存预热失败", "key", key, "error", err)
			continue
		}
		successCount++
	}

	m.logger.Info("缓存预热完成", "total", len(data), "success", successCount)
	return nil
}

// Invalidate 使缓存失效（支持模式匹配）
func (m *Manager) Invalidate(pattern string) error {
	// 对于Redis缓存，支持键模式删除
	if m.l2 != nil {
		// 这里可以实现更复杂的失效策略
		// 目前先记录日志，实际应用中可以扩展
		m.logger.Info("缓存失效模式", "pattern", pattern)
	}

	// 对于本地缓存，由于bigcache不支持模式删除，
	// 可以考虑实现一个键前缀匹配的逻辑

	return nil
}

// Warmup 缓存预热接口
type Warmup interface {
	// GetHotspotKeys 获取热点键列表
	GetHotspotKeys() []string
	// LoadData 加载数据
	LoadData(keys []string) map[string][]byte
}

// WarmupCache 缓存预热
func (m *Manager) WarmupCache(warmer Warmup) error {
	m.logger.Info("开始缓存预热流程")

	// 获取热点键
	hotKeys := warmer.GetHotspotKeys()
	if len(hotKeys) == 0 {
		m.logger.Info("没有热点数据需要预热")
		return nil
	}

	// 加载数据
	data := warmer.LoadData(hotKeys)
	if len(data) == 0 {
		m.logger.Warn("预热数据加载为空")
		return nil
	}

	// 预加载到缓存
	return m.Preload(data)
}

// InvalidateByPattern 按模式使缓存失效
func (m *Manager) InvalidateByPattern(pattern string) error {
	return m.invalidator.InvalidateByPattern(pattern)
}

// InvalidateByPrefix 按前缀使缓存失效
func (m *Manager) InvalidateByPrefix(prefix string) error {
	return m.invalidator.InvalidateByPrefix(prefix)
}

// ScheduleInvalidation 定时缓存失效
func (m *Manager) ScheduleInvalidation(key string, delay time.Duration) {
	m.invalidator.ScheduleInvalidation(key, delay)
}

// BatchInvalidate 批量缓存失效
func (m *Manager) BatchInvalidate(keys []string) error {
	return m.invalidator.BatchInvalidate(keys)
}
