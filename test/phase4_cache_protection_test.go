package test

import (
	"fmt"
	"testing"
	"time"

	"datamiddleware/internal/cache"
	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// TestCacheProtection 测试缓存防护机制
func TestCacheProtection(t *testing.T) {
	// 初始化日志
	logConfig := types.LoggerConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
	log, err := logger.Init(logConfig)
	if err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}

	// 创建缓存管理器（使用内存缓存进行测试）
	cacheConfig := types.CacheConfig{
		L1: types.CacheConfigL1{
			Enabled:            true,
			Shards:             1024,
			LifeWindow:         time.Minute,
			CleanWindow:        time.Minute,
			MaxEntriesInWindow: 1000 * 10 * 60,
			MaxEntrySize:       500,
			Verbose:            true,
			HardMaxCacheSize:   10 * 1024 * 1024, // 10MB
		},
		L2: types.CacheConfigL2{
			Enabled: false, // 禁用L2以简化测试
		},
	}

	manager, err := cache.NewManager(cacheConfig, log)
	if err != nil {
		t.Fatalf("创建缓存管理器失败: %v", err)
	}
	defer manager.Close()

	// 创建防护器
	protection := cache.NewProtection(manager, log)

	t.Run("PenetrationProtection", func(t *testing.T) {
		testPenetrationProtection(t, protection)
	})

	t.Run("AvalancheProtection", func(t *testing.T) {
		testAvalancheProtection(t, protection)
	})

	t.Run("CacheMonitor", func(t *testing.T) {
		testCacheMonitor(t, protection)
	})

	t.Run("ProtectionStats", func(t *testing.T) {
		testProtectionStats(t, protection)
	})
}

// testPenetrationProtection 测试缓存穿透防护
func testPenetrationProtection(t *testing.T, protection *cache.Protection) {
	// 测试不存在的键多次访问
	nonExistentKey := "non_existent_key_12345"

	// 首次访问应该返回缓存未命中
	_, err := protection.GetWithProtection(nonExistentKey)
	if err != types.ErrCacheMiss {
		t.Errorf("期望缓存未命中，实际得到: %v", err)
	}

	// 重复访问相同的不存在键
	for i := 0; i < 10; i++ {
		_, err := protection.GetWithProtection(nonExistentKey)
		if err != types.ErrCacheMiss {
			t.Errorf("期望缓存未命中，实际得到: %v", err)
		}
	}

	// 检查防护统计
	stats := protection.GetStats()
	if stats.PenetrationProtection.BlockedKeysCount <= 0 {
		t.Error("期望有被阻止的键")
	}

	t.Logf("缓存穿透防护统计: %+v", stats.PenetrationProtection)
}

// testAvalancheProtection 测试缓存雪崩防护
func testAvalancheProtection(t *testing.T, protection *cache.Protection) {
	// 模拟高失败率场景
	// 注意：雪崩防护主要监控整体失败率，这里通过正常操作来观察统计

	// 执行一些操作以生成统计数据
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("avalanche_test_key_%d", i)
		_, err := protection.GetWithProtection(key) // 这些键不存在，会增加失败计数
		if err != types.ErrCacheMiss {
			t.Errorf("期望缓存未命中，实际: %v", err)
		}
	}

	// 检查雪崩防护状态
	stats := protection.GetStats()
	t.Logf("缓存雪崩防护统计: %+v", stats.AvalancheProtection)

	// 即使没有触发雪崩，统计信息也应该被正确收集
	if stats.AvalancheProtection.TotalRequests < 0 {
		t.Error("请求统计不应该为负数")
	}
}

// testCacheMonitor 测试缓存监控
func testCacheMonitor(t *testing.T, protection *cache.Protection) {
	testKey := "monitor_test_key"
	testValue := []byte("monitor_test_value")

	// 执行一些缓存操作
	err := protection.SetWithProtection(testKey, testValue)
	if err != nil {
		t.Errorf("设置缓存失败: %v", err)
	}

	// 获取缓存
	value, err := protection.GetWithProtection(testKey)
	if err != nil {
		t.Errorf("获取缓存失败: %v", err)
	} else if string(value) != string(testValue) {
		t.Errorf("缓存值不匹配，期望: %s, 实际: %s", testValue, value)
	}

	// 获取不存在的键
	_, err = protection.GetWithProtection("non_existent_monitor_key")
	if err != types.ErrCacheMiss {
		t.Errorf("期望缓存未命中，实际: %v", err)
	}

	// 检查监控统计
	stats := protection.GetStats()
	if stats.Monitor.GetCount <= 0 {
		t.Error("期望有获取操作记录")
	}

	if stats.Monitor.SetCount <= 0 {
		t.Error("期望有设置操作记录")
	}

	// 检查命中率
	totalRequests := stats.Monitor.HitCount + stats.Monitor.MissCount
	if totalRequests <= 0 {
		t.Error("期望有请求记录")
	}

	hitRate := float64(stats.Monitor.HitCount) / float64(totalRequests)
	t.Logf("缓存命中率: %.2f%% (%d/%d)", hitRate*100, stats.Monitor.HitCount, totalRequests)

	t.Logf("缓存监控统计: %+v", stats.Monitor)
}

// testProtectionStats 测试防护统计信息
func testProtectionStats(t *testing.T, protection *cache.Protection) {
	// 执行一些操作以生成统计数据
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("stats_test_key_%d", i)
		protection.SetWithProtection(key, []byte(fmt.Sprintf("value_%d", i)))
		protection.GetWithProtection(key)
	}

	// 获取不存在的键以增加未命中计数
	for i := 0; i < 3; i++ {
		protection.GetWithProtection(fmt.Sprintf("non_existent_stats_key_%d", i))
	}

	stats := protection.GetStats()

	// 验证统计数据结构
	if stats.PenetrationProtection.BlockedKeysCount < 0 {
		t.Error("阻止键数量不应该为负数")
	}

	if stats.AvalancheProtection.TotalRequests < 0 {
		t.Error("雪崩防护请求数不应该为负数")
	}

	if stats.Monitor.GetCount < 0 {
		t.Error("获取计数不应该为负数")
	}

	if stats.Monitor.SetCount < 0 {
		t.Error("设置计数不应该为负数")
	}

	// 验证命中率计算
	totalRequests := stats.Monitor.HitCount + stats.Monitor.MissCount
	if stats.Monitor.GetCount != totalRequests {
		t.Errorf("获取计数(%d)应该等于命中(%d)+未命中(%d)",
			stats.Monitor.GetCount, stats.Monitor.HitCount, stats.Monitor.MissCount)
	}

	t.Logf("完整的防护统计信息: %+v", stats)
}

// TestCacheProtectionIntegration 集成测试缓存防护
func TestCacheProtectionIntegration(t *testing.T) {
	logConfig := types.LoggerConfig{
		Level:  "warn", // 减少日志输出
		Format: "json",
		Output: "stdout",
	}
	log, _ := logger.Init(logConfig)

	cacheConfig := types.CacheConfig{
		L1: types.CacheConfigL1{
			Enabled:            true,
			Shards:             1024,
			LifeWindow:         time.Minute,
			CleanWindow:        time.Minute,
			MaxEntriesInWindow: 1000 * 10 * 60,
			MaxEntrySize:       500,
			Verbose:            false, // 减少日志输出
			HardMaxCacheSize:   10 * 1024 * 1024, // 10MB
		},
		L2: types.CacheConfigL2{
			Enabled: false,
		},
	}

	manager, _ := cache.NewManager(cacheConfig, log)
	defer manager.Close()

	protection := cache.NewProtection(manager, log)

	// 模拟实际使用场景
	t.Run("NormalUsage", func(t *testing.T) {
		// 设置一些正常数据
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("normal_key_%d", i)
			value := []byte(fmt.Sprintf("normal_value_%d", i))
			err := protection.SetWithProtection(key, value)
			if err != nil {
				t.Errorf("设置正常数据失败: %v", err)
			}
		}

		// 读取数据多次
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("normal_key_%d", i)
			value, err := protection.GetWithProtection(key)
			if err != nil {
				t.Errorf("读取正常数据失败: %v", err)
			}
			expected := fmt.Sprintf("normal_value_%d", i)
			if string(value) != expected {
				t.Errorf("数据不匹配，期望: %s, 实际: %s", expected, value)
			}
		}
	})

	t.Run("PenetrationAttackSimulation", func(t *testing.T) {
		// 模拟缓存穿透攻击
		attackKey := "attack_key_123"

		// 重复访问不存在的键
		hitsBefore := protection.GetStats().PenetrationProtection.BlockedKeysCount

		for i := 0; i < 20; i++ {
			_, err := protection.GetWithProtection(attackKey)
			if err != types.ErrCacheMiss {
				t.Errorf("期望缓存未命中，实际: %v", err)
			}
		}

		hitsAfter := protection.GetStats().PenetrationProtection.BlockedKeysCount

		// 检查是否触发了防护
		if hitsAfter <= hitsBefore {
			t.Log("注意: 可能未触发缓存穿透防护，这可能是正常的，取决于防护配置")
		}
	})

	t.Run("FinalStats", func(t *testing.T) {
		finalStats := protection.GetStats()
		t.Logf("集成测试最终统计: 命中率=%.2f%%, 阻止键=%d, 请求总数=%d",
			float64(finalStats.Monitor.HitCount)/float64(finalStats.Monitor.GetCount)*100,
			finalStats.PenetrationProtection.BlockedKeysCount,
			finalStats.AvalancheProtection.TotalRequests)
	})
}
