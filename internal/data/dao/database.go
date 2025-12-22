package dao

import (
	"fmt"
	"time"

	"datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/common/types"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// Database 数据库管理器
type Database struct {
	config   types.DatabaseConfig `json:"config"`    // 数据库配置
	master   *gorm.DB             `json:"-"`         // 主库连接
	slaves   []*gorm.DB           `json:"-"`         // 从库连接列表
	log      logger.Logger        `json:"-"`         // 日志器
	isClosed bool                 `json:"is_closed"` // 是否已关闭
}

// NewDatabase 创建数据库管理器
func NewDatabase(config types.DatabaseConfig, log logger.Logger) *Database {
	return &Database{
		config:   config,
		slaves:   make([]*gorm.DB, 0),
		log:      log,
		isClosed: false,
	}
}

// Connect 连接数据库
func (db *Database) Connect() error {
	if db.isClosed {
		return fmt.Errorf("数据库已关闭")
	}

	// 连接主库
	if err := db.connectMaster(); err != nil {
		return fmt.Errorf("连接主库失败: %w", err)
	}

	// 连接从库（可选，如果失败不影响主库连接）
	if err := db.connectSlaves(); err != nil {
		db.log.Warn("从库连接失败，将使用主库进行读写操作", "error", err)
	}

	db.log.Info("数据库连接成功",
		"master_connected", db.master != nil,
		"slaves_count", len(db.slaves))

	return nil
}

// connectMaster 连接主库
func (db *Database) connectMaster() error {
	config := db.config.Primary

	// 构建DSN
	dsn, err := db.buildDSN(config)
	if err != nil {
		return err
	}

	// 创建GORM配置
	gormConfig := &gorm.Config{
		Logger: gormLogger.New(
			&gormLogAdapter{log: db.log},
			gormLogger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  gormLogger.Info,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	}

	// 根据驱动类型连接数据库
	var dialector gorm.Dialector
	switch config.Driver {
	case "mysql":
		dialector = mysql.Open(dsn)
	default:
		return fmt.Errorf("不支持的数据库驱动: %s", config.Driver)
	}

	// 连接数据库
	master, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 配置连接池
	sqlDB, err := master.DB()
	if err != nil {
		return fmt.Errorf("获取底层数据库连接失败: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	db.master = master
	db.log.Info("主库连接成功", "driver", config.Driver, "host", config.Host)
	return nil
}

// connectSlaves 连接从库
func (db *Database) connectSlaves() error {
	for i, config := range db.config.Replica {
		dsn, err := db.buildDSN(config)
		if err != nil {
			return fmt.Errorf("构建从库%d DSN失败: %w", i, err)
		}

		gormConfig := &gorm.Config{
			Logger: gormLogger.New(
				&gormLogAdapter{log: db.log},
				gormLogger.Config{
					SlowThreshold:             time.Second,
					LogLevel:                  gormLogger.Info,
					IgnoreRecordNotFoundError: true,
					Colorful:                  false,
				},
			),
		}

		var dialector gorm.Dialector
		switch config.Driver {
		case "mysql":
			dialector = mysql.Open(dsn)
		default:
			return fmt.Errorf("不支持的数据库驱动: %s", config.Driver)
		}

		slave, err := gorm.Open(dialector, gormConfig)
		if err != nil {
			return fmt.Errorf("连接从库%d失败: %w", i, err)
		}

		sqlDB, err := slave.DB()
		if err != nil {
			return fmt.Errorf("获取从库%d底层连接失败: %w", i, err)
		}

		sqlDB.SetMaxOpenConns(config.MaxOpenConns)
		sqlDB.SetMaxIdleConns(config.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)

		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("从库%d连接测试失败: %w", i, err)
		}

		db.slaves = append(db.slaves, slave)
		db.log.Info("从库连接成功", "index", i, "driver", config.Driver, "host", config.Host)
	}

	return nil
}

// buildDSN 构建数据库连接字符串
func (db *Database) buildDSN(config types.DBConfig) (string, error) {
	switch config.Driver {
	case "mysql":
		// MySQL DSN格式: username:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True&loc=Local
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			config.Username,
			config.Password,
			config.Host,
			config.Port,
			config.Database,
			config.Charset,
		)
		return dsn, nil

	default:
		return "", fmt.Errorf("不支持的数据库驱动: %s", config.Driver)
	}
}

// Master 获取主库连接
func (db *Database) Master() *gorm.DB {
	return db.master
}

// Slave 获取从库连接（轮询）
func (db *Database) Slave() *gorm.DB {
	if len(db.slaves) == 0 {
		return db.master // 如果没有从库，返回主库
	}

	// 简单的轮询策略，可以根据需要改进为更复杂的负载均衡
	index := int(time.Now().UnixNano()) % len(db.slaves)
	return db.slaves[index]
}

// Slaves 获取所有从库连接
func (db *Database) Slaves() []*gorm.DB {
	return db.slaves
}

// HealthCheck 健康检查
func (db *Database) HealthCheck() error {
	if db.isClosed {
		return fmt.Errorf("数据库已关闭")
	}

	// 检查主库
	if db.master != nil {
		sqlDB, err := db.master.DB()
		if err != nil {
			return fmt.Errorf("获取主库连接失败: %w", err)
		}

		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("主库连接异常: %w", err)
		}
	}

	// 检查从库
	for i, slave := range db.slaves {
		sqlDB, err := slave.DB()
		if err != nil {
			return fmt.Errorf("获取从库%d连接失败: %w", i, err)
		}

		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("从库%d连接异常: %w", i, err)
		}
	}

	return nil
}

// Stats 获取数据库统计信息
func (db *Database) Stats() DatabaseStats {
	stats := DatabaseStats{
		HasMaster:  db.master != nil,
		SlaveCount: len(db.slaves),
		IsClosed:   db.isClosed,
	}

	// 获取主库统计
	if db.master != nil {
		if sqlDB, err := db.master.DB(); err == nil {
			sqlStats := sqlDB.Stats()
			stats.MasterStats = &DBStats{
				OpenConnections:   sqlStats.OpenConnections,
				InUse:             sqlStats.InUse,
				Idle:              sqlStats.Idle,
				WaitCount:         sqlStats.WaitCount,
				WaitDuration:      sqlStats.WaitDuration,
				MaxIdleClosed:     sqlStats.MaxIdleClosed,
				MaxLifetimeClosed: sqlStats.MaxLifetimeClosed,
			}
		}
	}

	// 获取从库统计
	stats.SlaveStats = make([]*DBStats, len(db.slaves))
	for i, slave := range db.slaves {
		if sqlDB, err := slave.DB(); err == nil {
			sqlStats := sqlDB.Stats()
			stats.SlaveStats[i] = &DBStats{
				OpenConnections:   sqlStats.OpenConnections,
				InUse:             sqlStats.InUse,
				Idle:              sqlStats.Idle,
				WaitCount:         sqlStats.WaitCount,
				WaitDuration:      sqlStats.WaitDuration,
				MaxIdleClosed:     sqlStats.MaxIdleClosed,
				MaxLifetimeClosed: sqlStats.MaxLifetimeClosed,
			}
		}
	}

	return stats
}

// Close 关闭数据库连接
func (db *Database) Close() error {
	if db.isClosed {
		return nil
	}

	db.isClosed = true

	// 关闭主库
	if db.master != nil {
		if sqlDB, err := db.master.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				db.log.Error("关闭主库连接失败", "error", err)
			}
		}
	}

	// 关闭从库
	for i, slave := range db.slaves {
		if sqlDB, err := slave.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				db.log.Error("关闭从库连接失败", "index", i, "error", err)
			}
		}
	}

	db.log.Info("数据库连接已关闭")
	return nil
}

// DatabaseStats 数据库统计信息
type DatabaseStats struct {
	HasMaster   bool       `json:"has_master"`   // 是否有主库
	SlaveCount  int        `json:"slave_count"`  // 从库数量
	IsClosed    bool       `json:"is_closed"`    // 是否已关闭
	MasterStats *DBStats   `json:"master_stats"` // 主库统计
	SlaveStats  []*DBStats `json:"slave_stats"`  // 从库统计
}

// DBStats 数据库连接统计
type DBStats struct {
	OpenConnections   int           `json:"open_connections"`    // 打开的连接数
	InUse             int           `json:"in_use"`              // 正在使用的连接数
	Idle              int           `json:"idle"`                // 空闲连接数
	WaitCount         int64         `json:"wait_count"`          // 等待次数
	WaitDuration      time.Duration `json:"wait_duration"`       // 等待总时长
	MaxIdleClosed     int64         `json:"max_idle_closed"`     // 因空闲超时关闭的连接数
	MaxLifetimeClosed int64         `json:"max_lifetime_closed"` // 因生命周期超时关闭的连接数
}

// gormLogAdapter GORM日志适配器
type gormLogAdapter struct {
	log logger.Logger
}

func (l *gormLogAdapter) Printf(format string, args ...interface{}) {
	l.log.Debug(fmt.Sprintf(format, args...))
}
