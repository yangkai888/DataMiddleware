package config

import (
	"os"
	"path/filepath"
	"testing"

	"datamiddleware/pkg/types"

	"github.com/spf13/viper"
)

func TestInit(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	configContent := `
server:
  env: dev
  http:
    host: "127.0.0.1"
    port: 8080
  tcp:
    host: "127.0.0.1"
    port: 9090

logger:
  level: info
  format: json
  output: stdout

database:
  primary:
    driver: mysql
    host: localhost
    port: 3306
    username: root
    password: test
    database: test
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("创建临时配置文件失败: %v", err)
	}

	// 设置环境变量指向临时配置目录
	oldConfigPath := os.Getenv("DATAMIDDLEWARE_CONFIG_PATH")
	os.Setenv("DATAMIDDLEWARE_CONFIG_PATH", tempDir)
	defer func() {
		if oldConfigPath != "" {
			os.Setenv("DATAMIDDLEWARE_CONFIG_PATH", oldConfigPath)
		} else {
			os.Unsetenv("DATAMIDDLEWARE_CONFIG_PATH")
		}
	}()

	// 测试配置初始化
	cfg, err := Init()
	if err != nil {
		t.Fatalf("配置初始化失败: %v", err)
	}

	// 验证配置值
	if cfg.Server.Env != "dev" {
		t.Errorf("期望环境为dev，实际为%s", cfg.Server.Env)
	}
	if cfg.Server.HTTP.Host != "127.0.0.1" {
		t.Errorf("期望HTTP主机为127.0.0.1，实际为%s", cfg.Server.HTTP.Host)
	}
	if cfg.Server.HTTP.Port != 8080 {
		t.Errorf("期望HTTP端口为8080，实际为%d", cfg.Server.HTTP.Port)
	}
	if cfg.Logger.Level != "info" {
		t.Errorf("期望日志级别为info，实际为%s", cfg.Logger.Level)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *types.Config
		wantErr bool
	}{
		{
			name: "有效配置",
			config: &types.Config{
				Server: types.ServerConfig{
					Env: "dev",
					HTTP: types.HTTPConfig{
						Host: "127.0.0.1",
						Port: 8080,
					},
					TCP: types.TCPConfig{
						Host: "127.0.0.1",
						Port: 9090,
					},
				},
				Logger: types.LoggerConfig{
					Level: "info",
				},
				Database: types.DatabaseConfig{
					Primary: types.DBConfig{
						Driver: "mysql",
					},
				},
				Games: []types.GameConfig{
					{ID: "game1", TCPPort: 9101},
				},
			},
			wantErr: false,
		},
		{
			name: "无效环境",
			config: &types.Config{
				Server: types.ServerConfig{
					Env: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "无效HTTP端口",
			config: &types.Config{
				Server: types.ServerConfig{
					Env: "dev",
					HTTP: types.HTTPConfig{
						Port: 99999,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "重复游戏ID",
			config: &types.Config{
				Server: types.ServerConfig{Env: "dev"},
				Games: []types.GameConfig{
					{ID: "game1", TCPPort: 9101},
					{ID: "game1", TCPPort: 9102},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	// 清除所有设置
	// 注意：这里只是测试默认值设置，不进行实际验证
	// 因为viper是全局的，测试可能会互相影响
	setDefaults()

	// 验证一些关键默认值
	if viper.GetString("server.env") != "dev" {
		t.Errorf("期望默认环境为dev")
	}
	if viper.GetInt("server.http.port") != 8080 {
		t.Errorf("期望默认HTTP端口为8080")
	}
	if viper.GetString("logger.level") != "info" {
		t.Errorf("期望默认日志级别为info")
	}
}

func TestGetConfigPath(t *testing.T) {
	// 测试环境变量指定的路径
	os.Setenv("DATAMIDDLEWARE_CONFIG_PATH", "/custom/path")
	defer os.Unsetenv("DATAMIDDLEWARE_CONFIG_PATH")

	path := getConfigPath()
	if path != "/custom/path" {
		t.Errorf("期望配置路径为/custom/path，实际为%s", path)
	}

	// 测试默认路径
	os.Unsetenv("DATAMIDDLEWARE_CONFIG_PATH")
	path = getConfigPath()
	expected := filepath.Join(os.Getenv("PWD"), "configs")
	if path != expected {
		t.Errorf("期望默认配置路径为%s，实际为%s", expected, path)
	}
}
