package logger

import (
	"os"
	"path/filepath"
	"strings"

	"datamiddleware/pkg/types"

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

	// 创建日志输出器
	writeSyncer := getWriteSyncer(config)

	// 设置日志级别
	level := getLogLevel(config.Level)

	// 创建核心
	core := zapcore.NewCore(encoder, writeSyncer, level)

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

// getWriteSyncer 获取输出器
func getWriteSyncer(config types.LoggerConfig) zapcore.WriteSyncer {
	var writeSyncers []zapcore.WriteSyncer

	// 根据输出目标添加输出器
	switch strings.ToLower(config.Output) {
	case "stdout":
		writeSyncers = append(writeSyncers, zapcore.AddSync(os.Stdout))
	case "stderr":
		writeSyncers = append(writeSyncers, zapcore.AddSync(os.Stderr))
	case "file":
		// 创建日志目录
		logDir := filepath.Dir(config.File.Path)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			// 如果创建目录失败，使用标准输出
			writeSyncers = append(writeSyncers, zapcore.AddSync(os.Stdout))
		} else {
			// 使用lumberjack进行日志轮转
			lumberJackLogger := &lumberjack.Logger{
				Filename:   config.File.Path,
				MaxSize:    config.File.MaxSize,    // 单个文件最大尺寸，MB
				MaxBackups: config.File.MaxBackups, // 保留的最大旧文件数量
				MaxAge:     config.File.MaxAge,     // 保留的最大天数
				Compress:   config.File.Compress,   // 是否压缩
			}
			writeSyncers = append(writeSyncers, zapcore.AddSync(lumberJackLogger))
		}
	default:
		// 默认使用标准输出
		writeSyncers = append(writeSyncers, zapcore.AddSync(os.Stdout))
	}

	// 如果配置为同时输出到文件和控制台
	if config.Output == "both" {
		writeSyncers = append(writeSyncers, zapcore.AddSync(os.Stdout))
		if config.File.Path != "" {
			logDir := filepath.Dir(config.File.Path)
			if err := os.MkdirAll(logDir, 0755); err == nil {
				lumberJackLogger := &lumberjack.Logger{
					Filename:   config.File.Path,
					MaxSize:    config.File.MaxSize,
					MaxBackups: config.File.MaxBackups,
					MaxAge:     config.File.MaxAge,
					Compress:   config.File.Compress,
				}
				writeSyncers = append(writeSyncers, zapcore.AddSync(lumberJackLogger))
			}
		}
	}

	// 返回组合输出器
	return zapcore.NewMultiWriteSyncer(writeSyncers...)
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
