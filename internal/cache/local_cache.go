package cache

import (
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"

	"github.com/allegro/bigcache/v3"
)

// LocalCache 本地缓存实现
type LocalCache struct {
	cache  *bigcache.BigCache
	logger logger.Logger
}

// NewLocalCache 创建本地缓存
func NewLocalCache(config types.CacheConfigL1, logger logger.Logger) (*LocalCache, error) {
	if !config.Enabled {
		return &LocalCache{logger: logger}, nil
	}

	bigcacheConfig := bigcache.Config{
		Shards:             config.Shards,
		LifeWindow:         config.LifeWindow,
		CleanWindow:        config.CleanWindow,
		MaxEntriesInWindow: config.MaxEntriesInWindow,
		MaxEntrySize:       config.MaxEntrySize,
		Verbose:            config.Verbose,
		HardMaxCacheSize:   config.HardMaxCacheSize,
	}

	cache, err := bigcache.NewBigCache(bigcacheConfig)
	if err != nil {
		return nil, err
	}

	return &LocalCache{
		cache:  cache,
		logger: logger,
	}, nil
}

// Get 获取缓存值
func (c *LocalCache) Get(key string) ([]byte, error) {
	if c.cache == nil {
		return nil, types.ErrCacheDisabled
	}

	value, err := c.cache.Get(key)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			return nil, types.ErrCacheMiss
		}
		return nil, err
	}

	return value, nil
}

// Set 设置缓存值
func (c *LocalCache) Set(key string, value []byte) error {
	if c.cache == nil {
		return types.ErrCacheDisabled
	}

	return c.cache.Set(key, value)
}

// SetWithTTL 设置缓存值并指定TTL
func (c *LocalCache) SetWithTTL(key string, value []byte, ttl time.Duration) error {
	if c.cache == nil {
		return types.ErrCacheDisabled
	}

	return c.cache.Set(key, value)
}

// Delete 删除缓存值
func (c *LocalCache) Delete(key string) error {
	if c.cache == nil {
		return types.ErrCacheDisabled
	}

	return c.cache.Delete(key)
}

// Exists 检查键是否存在
func (c *LocalCache) Exists(key string) bool {
	if c.cache == nil {
		return false
	}

	_, err := c.cache.Get(key)
	return err != bigcache.ErrEntryNotFound
}

// Clear 清空缓存
func (c *LocalCache) Clear() error {
	if c.cache == nil {
		return types.ErrCacheDisabled
	}

	return c.cache.Reset()
}

// Close 关闭缓存
func (c *LocalCache) Close() error {
	if c.cache == nil {
		return nil
	}

	return c.cache.Close()
}