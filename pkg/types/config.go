package types

import (
	"time"
)

// Config 总配置结构体
type Config struct {
	Server   ServerConfig   `mapstructure:"server" yaml:"server"`
	Logger   LoggerConfig   `mapstructure:"logger" yaml:"logger"`
	Database DatabaseConfig `mapstructure:"database" yaml:"database"`
	Redis    RedisConfig    `mapstructure:"redis" yaml:"redis"`
	Cache    CacheConfig    `mapstructure:"cache" yaml:"cache"`
	JWT      JWTConfig      `mapstructure:"jwt" yaml:"jwt"`
	Games    []GameConfig   `mapstructure:"games" yaml:"games"`
	Monitor  MonitorConfig  `mapstructure:"monitor" yaml:"monitor"`
	Health   HealthConfig   `mapstructure:"health" yaml:"health"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Env  string      `mapstructure:"env" yaml:"env"`
	HTTP HTTPConfig  `mapstructure:"http" yaml:"http"`
	TCP  TCPConfig   `mapstructure:"tcp" yaml:"tcp"`
}

// HTTPConfig HTTP服务器配置
type HTTPConfig struct {
	Host           string        `mapstructure:"host" yaml:"host"`
	Port           int           `mapstructure:"port" yaml:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout" yaml:"write_timeout"`
	MaxHeaderBytes int           `mapstructure:"max_header_bytes" yaml:"max_header_bytes"`
}

// TCPConfig TCP服务器配置
type TCPConfig struct {
	Host           string        `mapstructure:"host" yaml:"host"`
	Port           int           `mapstructure:"port" yaml:"port"`
	MaxConnections int           `mapstructure:"max_connections" yaml:"max_connections"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout" yaml:"write_timeout"`
	Debug          bool          `mapstructure:"debug" yaml:"debug"` // 是否显示调试信息
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level  string     `mapstructure:"level" yaml:"level"`
	Format string     `mapstructure:"format" yaml:"format"`
	Output string     `mapstructure:"output" yaml:"output"`
	File   LogFileConfig `mapstructure:"file" yaml:"file"`
}

// LogFileConfig 日志文件配置
type LogFileConfig struct {
	Path       string `mapstructure:"path" yaml:"path"`
	MaxSize    int    `mapstructure:"max_size" yaml:"max_size"`
	MaxAge     int    `mapstructure:"max_age" yaml:"max_age"`
	MaxBackups int    `mapstructure:"max_backups" yaml:"max_backups"`
	Compress   bool   `mapstructure:"compress" yaml:"compress"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Primary  DBConfig     `mapstructure:"primary" yaml:"primary"`
	Replica  []DBConfig   `mapstructure:"replica" yaml:"replica"`
}

// DBConfig 数据库连接配置
type DBConfig struct {
	Driver          string        `mapstructure:"driver" yaml:"driver"`
	Host            string        `mapstructure:"host" yaml:"host"`
	Port            int           `mapstructure:"port" yaml:"port"`
	Username        string        `mapstructure:"username" yaml:"username"`
	Password        string        `mapstructure:"password" yaml:"password"`
	Database        string        `mapstructure:"database" yaml:"database"`
	Charset         string        `mapstructure:"charset" yaml:"charset"`
	MaxOpenConns    int           `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host            string        `mapstructure:"host" yaml:"host"`
	Port            int           `mapstructure:"port" yaml:"port"`
	Password        string        `mapstructure:"password" yaml:"password"`
	DB              int           `mapstructure:"db" yaml:"db"`
	PoolSize        int           `mapstructure:"pool_size" yaml:"pool_size"`
	MinIdleConns    int           `mapstructure:"min_idle_conns" yaml:"min_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout" yaml:"write_timeout"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Local LocalCacheConfig `mapstructure:"local" yaml:"local"`
}

// LocalCacheConfig 本地缓存配置
type LocalCacheConfig struct {
	Size int `mapstructure:"size" yaml:"size"`
	TTL  int `mapstructure:"ttl" yaml:"ttl"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret string `mapstructure:"secret" yaml:"secret"`
	Expire int    `mapstructure:"expire" yaml:"expire"`
}

// GameConfig 游戏配置
type GameConfig struct {
	ID         string `mapstructure:"id" yaml:"id"`
	Name       string `mapstructure:"name" yaml:"name"`
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled"`
	TCPPort    int    `mapstructure:"tcp_port" yaml:"tcp_port"`
	HTTPPrefix string `mapstructure:"http_prefix" yaml:"http_prefix"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled"`
	Port    int    `mapstructure:"port" yaml:"port"`
	Path    string `mapstructure:"path" yaml:"path"`
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	Enabled       bool          `mapstructure:"enabled" yaml:"enabled"`
	Path          string        `mapstructure:"path" yaml:"path"`
	CheckInterval time.Duration `mapstructure:"check_interval" yaml:"check_interval"`
}
