package logger

import (
	"os"
	"path/filepath"
	"strings"

	"datamiddleware/internal/common/types"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 日志接口
type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})

	// 带格式化的日志方法
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})

	// 同步日志缓冲区
	Sync() error
}

// ZapLogger zap日志实现
type ZapLogger struct {
	*zap.SugaredLogger
}

// Init 初始化日志系统
func Init(config types.LoggerConfig) (Logger, error) {
	// 创建日志编码器
	encoder := getEncoder(config.Format)

	// 设置日志级别
	level := getLogLevel(config.Level)

	// 创建多个核心
	var cores []zapcore.Core

	// 根据输出目标创建核心
	switch strings.ToLower(config.Output) {
	case "stdout":
		core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level)
		cores = append(cores, core)
	case "stderr":
		core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stderr), level)
		cores = append(cores, core)
	case "console":
		// console模式：同时输出到控制台和文件
		consoleCore := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level)
		cores = append(cores, consoleCore)

		if config.File.Path != "" {
			// 确保路径是绝对路径
			absPath, err := filepath.Abs(config.File.Path)
			if err != nil {
				absPath = config.File.Path
			}
			logDir := filepath.Dir(absPath)
			if err := os.MkdirAll(logDir, 0755); err == nil {
				lumberJackLogger := &lumberjack.Logger{
					Filename:   absPath,
					MaxSize:    config.File.MaxSize,
					MaxBackups: config.File.MaxBackups,
					MaxAge:     config.File.MaxAge,
					Compress:   config.File.Compress,
				}
				fileCore := zapcore.NewCore(encoder, zapcore.AddSync(lumberJackLogger), level)
				cores = append(cores, fileCore)
			}
		}
	case "file":
		// file模式：只输出到文件
		if config.File.Path != "" {
			absPath, err := filepath.Abs(config.File.Path)
			if err != nil {
				absPath = config.File.Path
			}
			logDir := filepath.Dir(absPath)
			if err := os.MkdirAll(logDir, 0755); err == nil {
				lumberJackLogger := &lumberjack.Logger{
					Filename:   absPath,
					MaxSize:    config.File.MaxSize,
					MaxBackups: config.File.MaxBackups,
					MaxAge:     config.File.MaxAge,
					Compress:   config.File.Compress,
				}
				fileCore := zapcore.NewCore(encoder, zapcore.AddSync(lumberJackLogger), level)
				cores = append(cores, fileCore)
			}
		}
	default:
		// 默认使用标准输出
		core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level)
		cores = append(cores, core)
	}

	// 使用NewTee组合多个核心
	core := zapcore.NewTee(cores...)

	// 创建日志器
	logger := zap.New(core,
		zap.AddCaller(),                       // 添加调用者信息
		zap.AddCallerSkip(1),                  // 跳过一层调用栈
		zap.AddStacktrace(zapcore.ErrorLevel), // Error级别及以上添加堆栈跟踪
	)

	// 创建SugaredLogger
	sugaredLogger := logger.Sugar()

	return &ZapLogger{
		SugaredLogger: sugaredLogger,
	}, nil
}

// getEncoder 获取编码器
func getEncoder(format string) zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()

	// 设置时间格式
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// 设置调用者信息格式
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// 设置日志级别格式
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// 根据格式选择编码器
	switch strings.ToLower(format) {
	case "console":
		return zapcore.NewConsoleEncoder(encoderConfig)
	case "json":
		fallthrough
	default:
		return zapcore.NewJSONEncoder(encoderConfig)
	}
}

// getLogLevel 获取日志级别
func getLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	case "panic":
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}

// Sync 同步日志缓冲区
func (l *ZapLogger) Sync() error {
	return l.SugaredLogger.Sync()
}
