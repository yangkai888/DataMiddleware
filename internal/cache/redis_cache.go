package cache

import (
	"context"
	"fmt"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"

	"github.com/redis/go-redis/v9"
)

// RedisCache Redis缓存实现
type RedisCache struct {
	client *redis.Client
	logger logger.Logger
}

// NewRedisCache 创建Redis缓存
func NewRedisCache(config types.CacheConfigL2, logger logger.Logger) (*RedisCache, error) {
	if !config.Enabled {
		return &RedisCache{logger: logger}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConn,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("Redis连接失败: %w", err)
	}

	return &RedisCache{
		client: client,
		logger: logger,
	}, nil
}

// Get 获取缓存值
func (c *RedisCache) Get(key string) ([]byte, error) {
	if c.client == nil {
		return nil, types.ErrCacheDisabled
	}

	ctx := context.Background()
	value, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, types.ErrCacheMiss
		}
		return nil, err
	}

	return []byte(value), nil
}

// Set 设置缓存值
func (c *RedisCache) Set(key string, value []byte) error {
	if c.client == nil {
		return types.ErrCacheDisabled
	}

	ctx := context.Background()
	return c.client.Set(ctx, key, value, 0).Err()
}

// SetWithTTL 设置缓存值并指定TTL
func (c *RedisCache) SetWithTTL(key string, value []byte, ttl time.Duration) error {
	if c.client == nil {
		return types.ErrCacheDisabled
	}

	ctx := context.Background()
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Delete 删除缓存值
func (c *RedisCache) Delete(key string) error {
	if c.client == nil {
		return types.ErrCacheDisabled
	}

	ctx := context.Background()
	return c.client.Del(ctx, key).Err()
}

// Exists 检查键是否存在
func (c *RedisCache) Exists(key string) bool {
	if c.client == nil {
		return false
	}

	ctx := context.Background()
	count, err := c.client.Exists(ctx, key).Result()
	return err == nil && count > 0
}

// Clear 清空缓存
func (c *RedisCache) Clear() error {
	if c.client == nil {
		return types.ErrCacheDisabled
	}

	ctx := context.Background()
	return c.client.FlushDB(ctx).Err()
}

// Close 关闭缓存
func (c *RedisCache) Close() error {
	if c.client == nil {
		return nil
	}

	return c.client.Close()
}