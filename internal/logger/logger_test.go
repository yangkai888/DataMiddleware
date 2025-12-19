package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"datamiddleware/pkg/types"

	"go.uber.org/zap/zapcore"
)

func TestInit(t *testing.T) {
	// 创建临时目录用于测试日志文件
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name   string
		config types.LoggerConfig
	}{
		{
			name: "控制台输出",
			config: types.LoggerConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		},
		{
			name: "文件输出",
			config: types.LoggerConfig{
				Level:  "debug",
				Format: "console",
				Output: "file",
				File: types.LogFileConfig{
					Path:       logFile,
					MaxSize:    1,
					MaxAge:     1,
					MaxBackups: 1,
					Compress:   false,
				},
			},
		},
		{
			name: "JSON格式",
			config: types.LoggerConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		},
		{
			name: "Console格式",
			config: types.LoggerConfig{
				Level:  "info",
				Format: "console",
				Output: "stdout",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := Init(tt.config)
			if err != nil {
				t.Fatalf("初始化日志失败: %v", err)
			}

			// 测试不同级别的日志
			logger.Info("测试信息日志", "key", "value")
			logger.Debug("测试调试日志")
			logger.Warn("测试警告日志")
			logger.Error("测试错误日志")

			// 测试格式化日志
			logger.Infof("测试格式化日志: %s", "参数")
			logger.Errorf("测试错误格式化日志: %v", err)

			// 对于stdout输出，跳过sync测试（stdout不支持sync）
			if tt.config.Output != "stdout" {
				if err := logger.Sync(); err != nil {
					t.Errorf("同步日志失败: %v", err)
				}
			}
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"debug", "debug"},
		{"DEBUG", "debug"},
		{"info", "info"},
		{"INFO", "info"},
		{"warn", "warn"},
		{"warning", "warn"},
		{"WARN", "warn"},
		{"error", "error"},
		{"ERROR", "error"},
		{"fatal", "fatal"},
		{"FATAL", "fatal"},
		{"panic", "panic"},
		{"PANIC", "panic"},
		{"invalid", "info"}, // 默认级别
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := getLogLevel(tt.input)
			if level.String() != tt.expected {
				t.Errorf("期望级别 %s，实际得到 %s", tt.expected, level.String())
			}
		})
	}
}

func TestGetEncoder(t *testing.T) {
	tests := []struct {
		format string
		isJSON bool
	}{
		{"json", true},
		{"JSON", true},
		{"console", false},
		{"CONSOLE", false},
		{"invalid", true}, // 默认JSON
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			encoder := getEncoder(tt.format)

			// 编码测试
			zapEntry := zapcore.Entry{
				Level:      zapcore.InfoLevel,
				Time:       time.Now(),
				LoggerName: "",
				Message:    "test message",
				Caller:     zapcore.EntryCaller{},
				Stack:      "",
			}
			buf, err := encoder.EncodeEntry(zapEntry, []zapcore.Field{zapcore.Field{Key: "test", Type: zapcore.StringType, String: "value"}})
			if err != nil {
				t.Errorf("编码失败: %v", err)
			}
			defer buf.Free()

			encoded := buf.String()

			// 检查是否为JSON格式
			isJSON := strings.Contains(encoded, `"level"`) && strings.Contains(encoded, `"msg"`)
			if isJSON != tt.isJSON {
				t.Errorf("期望JSON格式: %v，实际: %v，内容: %s", tt.isJSON, isJSON, encoded)
			}
		})
	}
}

func TestGetWriteSyncer(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name   string
		config types.LoggerConfig
	}{
		{
			name: "标准输出",
			config: types.LoggerConfig{
				Output: "stdout",
			},
		},
		{
			name: "标准错误输出",
			config: types.LoggerConfig{
				Output: "stderr",
			},
		},
		{
			name: "文件输出",
			config: types.LoggerConfig{
				Output: "file",
				File: types.LogFileConfig{
					Path: filepath.Join(tempDir, "test.log"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeSyncer := getWriteSyncer(tt.config)
			if writeSyncer == nil {
				t.Error("WriteSyncer 不能为空")
			}

			// 测试写入
			_, err := writeSyncer.Write([]byte("test log entry\n"))
			if err != nil {
				t.Errorf("写入失败: %v", err)
			}

			// 对于stdout/stderr，跳过sync测试
			if tt.config.Output != "stdout" && tt.config.Output != "stderr" {
				if err := writeSyncer.Sync(); err != nil {
					t.Errorf("同步失败: %v", err)
				}
			}
		})
	}
}

func TestFileOutput(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	config := types.LoggerConfig{
		Level:  "info",
		Format: "json",
		Output: "file",
		File: types.LogFileConfig{
			Path:       logFile,
			MaxSize:    1,
			MaxAge:     1,
			MaxBackups: 1,
			Compress:   false,
		},
	}

	logger, err := Init(config)
	if err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}

	// 写入一些日志
	for i := 0; i < 10; i++ {
		logger.Info("测试日志消息", "index", i)
	}

	// 同步
	logger.Sync()

	// 检查文件是否存在
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("日志文件未创建")
	}

	// 读取文件内容
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Errorf("读取日志文件失败: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "测试日志消息") {
		t.Error("日志文件中未找到期望的内容")
	}

	if !strings.Contains(contentStr, `"level":"INFO"`) {
		t.Error("日志文件中未找到正确的JSON格式")
	}
}
