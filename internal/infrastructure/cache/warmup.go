package cache

import (
	"fmt"
	"time"

	"datamiddleware/internal/infrastructure/logging"
)

// DefaultWarmup 默认缓存预热器
type DefaultWarmup struct {
	logger logger.Logger
}

// NewDefaultWarmup 创建默认缓存预热器
func NewDefaultWarmup(logger logger.Logger) *DefaultWarmup {
	return &DefaultWarmup{
		logger: logger,
	}
}

// GetHotspotKeys 获取热点键列表
func (w *DefaultWarmup) GetHotspotKeys() []string {
	// 这里可以从数据库或配置文件中获取热点键列表
	// 目前返回一些示例键
	return []string{
		"hotkey:user:1",
		"hotkey:user:2",
		"hotkey:game:game1",
		"hotkey:game:game2",
	}
}

// LoadData 加载数据
func (w *DefaultWarmup) LoadData(keys []string) map[string][]byte {
	data := make(map[string][]byte)

	for _, key := range keys {
		// 这里可以从数据库或其他数据源加载实际数据
		// 目前生成一些示例数据
		switch {
		case key == "hotkey:user:1":
			data[key] = []byte(fmt.Sprintf(`{"user_id":"1","username":"user1","level":10,"timestamp":%d}`, time.Now().Unix()))
		case key == "hotkey:user:2":
			data[key] = []byte(fmt.Sprintf(`{"user_id":"2","username":"user2","level":15,"timestamp":%d}`, time.Now().Unix()))
		case key == "hotkey:game:game1":
			data[key] = []byte(fmt.Sprintf(`{"game_id":"game1","name":"游戏1","players":1000,"timestamp":%d}`, time.Now().Unix()))
		case key == "hotkey:game:game2":
			data[key] = []byte(fmt.Sprintf(`{"game_id":"game2","name":"游戏2","players":800,"timestamp":%d}`, time.Now().Unix()))
		default:
			// 生成默认数据
			data[key] = []byte(fmt.Sprintf(`{"key":"%s","value":"hot_data","timestamp":%d}`, key, time.Now().Unix()))
		}
	}

	w.logger.Info("缓存预热数据加载完成", "keys", len(keys), "data_count", len(data))
	return data
}