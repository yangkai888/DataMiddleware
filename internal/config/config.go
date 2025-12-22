package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"datamiddleware/internal/common/types"

	"github.com/spf13/viper"
)

// Init 初始化配置
func Init() (*types.Config, error) {
	// 获取配置路径
	configPath := getConfigPath()

	// 设置viper配置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)

	// 设置环境变量前缀
	viper.SetEnvPrefix("DATAMIDDLEWARE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置到结构体
	var cfg types.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 验证配置
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 监听配置文件变化
	viper.WatchConfig()

	return &cfg, nil
}

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	// 优先使用环境变量指定的路径
	if path := os.Getenv("DATAMIDDLEWARE_CONFIG_PATH"); path != "" {
		return path
	}

	// 默认使用configs目录
	workDir, _ := os.Getwd()
	return filepath.Join(workDir, "configs")
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 服务器默认配置
	viper.SetDefault("server.env", "dev")
	viper.SetDefault("server.http.host", "0.0.0.0")
	viper.SetDefault("server.http.port", 8080)
	viper.SetDefault("server.http.read_timeout", "30s")
	viper.SetDefault("server.http.write_timeout", "30s")
	viper.SetDefault("server.http.max_header_bytes", 1048576)

	viper.SetDefault("server.tcp.host", "0.0.0.0")
	viper.SetDefault("server.tcp.port", 9090)
	viper.SetDefault("server.tcp.max_connections", 10000)
	viper.SetDefault("server.tcp.read_timeout", "30s")
	viper.SetDefault("server.tcp.write_timeout", "30s")

	// 日志默认配置
	viper.SetDefault("logger.level", "info")
	viper.SetDefault("logger.format", "json")
	viper.SetDefault("logger.output", "stdout")
	viper.SetDefault("logger.file.path", "logs/datamiddleware.log")
	viper.SetDefault("logger.file.max_size", 100)
	viper.SetDefault("logger.file.max_age", 30)
	viper.SetDefault("logger.file.max_backups", 10)
	viper.SetDefault("logger.file.compress", true)

	// 数据库默认配置
	viper.SetDefault("database.primary.driver", "mysql")
	viper.SetDefault("database.primary.host", "localhost")
	viper.SetDefault("database.primary.port", 3306)
	viper.SetDefault("database.primary.username", "root")
	viper.SetDefault("database.primary.password", "")
	viper.SetDefault("database.primary.database", "datamiddleware")
	viper.SetDefault("database.primary.charset", "utf8mb4")
	viper.SetDefault("database.primary.max_open_conns", 100)
	viper.SetDefault("database.primary.max_idle_conns", 10)
	viper.SetDefault("database.primary.conn_max_lifetime", "300s")

	// Redis默认配置
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 2)
	viper.SetDefault("redis.conn_max_lifetime", "300s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")

	// 本地缓存默认配置
	viper.SetDefault("cache.local.size", 1000)
	viper.SetDefault("cache.local.ttl", 3600)

	// JWT默认配置
	viper.SetDefault("jwt.secret", "change-this-in-production")
	viper.SetDefault("jwt.expire", 86400)

	// 监控默认配置
	viper.SetDefault("monitor.enabled", true)
	viper.SetDefault("monitor.port", 9091)
	viper.SetDefault("monitor.path", "/metrics")

	// 健康检查默认配置
	viper.SetDefault("health.enabled", true)
	viper.SetDefault("health.path", "/health")
	viper.SetDefault("health.check_interval", "30s")
}

// validateConfig 验证配置
func validateConfig(cfg *types.Config) error {
	// 验证服务器环境
	if cfg.Server.Env != "dev" && cfg.Server.Env != "test" && cfg.Server.Env != "prod" {
		return fmt.Errorf("无效的服务器环境: %s", cfg.Server.Env)
	}

	// 验证端口范围
	if cfg.Server.HTTP.Port < 1 || cfg.Server.HTTP.Port > 65535 {
		return fmt.Errorf("无效的HTTP端口: %d", cfg.Server.HTTP.Port)
	}
	if cfg.Server.TCP.Port < 1 || cfg.Server.TCP.Port > 65535 {
		return fmt.Errorf("无效的TCP端口: %d", cfg.Server.TCP.Port)
	}

	// 验证日志级别
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, cfg.Logger.Level) {
		return fmt.Errorf("无效的日志级别: %s", cfg.Logger.Level)
	}

	// 验证数据库驱动
	validDrivers := []string{"mysql", "oracle"}
	if !contains(validDrivers, cfg.Database.Primary.Driver) {
		return fmt.Errorf("无效的数据库驱动: %s", cfg.Database.Primary.Driver)
	}

	// 验证游戏配置
	gameIDs := make(map[string]bool)
	for _, game := range cfg.Games {
		if game.ID == "" {
			return fmt.Errorf("游戏ID不能为空")
		}
		if gameIDs[game.ID] {
			return fmt.Errorf("重复的游戏ID: %s", game.ID)
		}
		gameIDs[game.ID] = true

		if game.TCPPort < 1 || game.TCPPort > 65535 {
			return fmt.Errorf("游戏 %s 的TCP端口无效: %d", game.ID, game.TCPPort)
		}
	}

	return nil
}

// contains 检查切片是否包含指定值
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetConfig 获取当前配置（用于热更新后的重新加载）
func GetConfig() (*types.Config, error) {
	var cfg types.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("重新解析配置失败: %w", err)
	}
	return &cfg, nil
}
