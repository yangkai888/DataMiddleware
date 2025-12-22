package dao

import (
	"fmt"
	"time"
)

// BaseModel 基础模型
type BaseModel struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Player 玩家模型
type Player struct {
	BaseModel
	UserID      string    `gorm:"uniqueIndex;size:64" json:"user_id"`       // 用户ID
	GameID      string    `gorm:"index;size:64" json:"game_id"`             // 游戏ID
	Username    string    `gorm:"uniqueIndex;size:64" json:"username"`      // 用户名
	Password    string    `gorm:"size:256" json:"-"`                        // 密码哈希（JSON中不输出）
	Email       string    `gorm:"size:128" json:"email"`                    // 邮箱
	Phone       string    `gorm:"size:32" json:"phone"`                     // 手机号
	Nickname    string    `gorm:"size:64" json:"nickname"`                  // 昵称
	Avatar      string    `gorm:"size:256" json:"avatar"`                   // 头像URL
	Level       int       `gorm:"default:1" json:"level"`                   // 等级
	Experience  int64     `gorm:"default:0" json:"experience"`              // 经验值
	Coins       int64     `gorm:"default:0" json:"coins"`                   // 金币
	Diamonds    int64     `gorm:"default:0" json:"diamonds"`                // 钻石
	Status      string    `gorm:"size:16;default:active" json:"status"`     // 状态: active, banned, deleted
	LastLoginAt *time.Time `json:"last_login_at"`                           // 最后登录时间
	LastLoginIP string    `gorm:"size:64" json:"last_login_ip"`             // 最后登录IP
	DeviceID    string    `gorm:"size:128" json:"device_id"`                // 设备ID
	Platform    string    `gorm:"size:16" json:"platform"`                  // 平台: ios, android, web
	Version     string    `gorm:"size:32" json:"version"`                   // 客户端版本
	ExtraData   string    `gorm:"type:text" json:"extra_data"`              // 额外数据(JSON)
}

// TableName 指定表名
func (Player) TableName() string {
	return "players"
}

// PlayerSession 玩家会话模型
type PlayerSession struct {
	BaseModel
	SessionID   string    `gorm:"uniqueIndex;size:128" json:"session_id"`   // 会话ID
	UserID      string    `gorm:"index;size:64" json:"user_id"`             // 用户ID
	GameID      string    `gorm:"index;size:64" json:"game_id"`             // 游戏ID
	Token       string    `gorm:"size:512" json:"token"`                    // JWT令牌
	IPAddress   string    `gorm:"size:64" json:"ip_address"`                // IP地址
	UserAgent   string    `gorm:"size:256" json:"user_agent"`               // 用户代理
	LoginAt     time.Time `json:"login_at"`                                 // 登录时间
	ExpireAt    time.Time `json:"expire_at"`                                // 过期时间
	IsActive    bool      `gorm:"default:true" json:"is_active"`            // 是否活跃
	DeviceID    string    `gorm:"size:128" json:"device_id"`                // 设备ID
}

// TableName 指定表名
func (PlayerSession) TableName() string {
	return "player_sessions"
}

// Item 道具模型
type Item struct {
	BaseModel
	ItemID      string `gorm:"uniqueIndex;size:64" json:"item_id"`       // 道具ID
	UserID      string `gorm:"index;size:64" json:"user_id"`             // 用户ID
	GameID      string `gorm:"index;size:64" json:"game_id"`             // 游戏ID
	Name        string `gorm:"size:128" json:"name"`                      // 道具名称
	Type        string `gorm:"size:32" json:"type"`                       // 道具类型: consumable, equipment, currency
	Category    string `gorm:"size:32" json:"category"`                  // 道具分类
	Rarity      string `gorm:"size:16;default:common" json:"rarity"`     // 稀有度: common, rare, epic, legendary
	Quantity    int64  `gorm:"default:1" json:"quantity"`                // 数量
	MaxQuantity int64  `gorm:"default:-1" json:"max_quantity"`           // 最大数量(-1表示无限制)
	IsBound     bool   `gorm:"default:false" json:"is_bound"`            // 是否绑定
	IsTradable  bool   `gorm:"default:true" json:"is_tradable"`          // 是否可交易
	ExpireAt    *time.Time `json:"expire_at"`                            // 过期时间
	Description string `gorm:"type:text" json:"description"`             // 描述
	IconURL     string `gorm:"size:256" json:"icon_url"`                 // 图标URL
	ExtraData   string `gorm:"type:text" json:"extra_data"`              // 额外数据(JSON)
}

// TableName 指定表名
func (Item) TableName() string {
	return "items"
}

// Order 订单模型
type Order struct {
	BaseModel
	OrderID       string    `gorm:"uniqueIndex;size:64" json:"order_id"`     // 订单ID
	UserID        string    `gorm:"index;size:64" json:"user_id"`            // 用户ID
	GameID        string    `gorm:"index;size:64" json:"game_id"`            // 游戏ID
	ProductID     string    `gorm:"size:64" json:"product_id"`               // 产品ID
	ProductName   string    `gorm:"size:128" json:"product_name"`            // 产品名称
	Amount        int64     `gorm:"not null" json:"amount"`                  // 金额(分)
	Currency      string    `gorm:"size:8;default:CNY" json:"currency"`      // 货币类型
	PaymentMethod string    `gorm:"size:32" json:"payment_method"`           // 支付方式
	Status        string    `gorm:"size:16;default:pending" json:"status"`   // 状态: pending, paid, cancelled, refunded
	PaymentAt     *time.Time `json:"payment_at"`                             // 支付时间
	RefundAt      *time.Time `json:"refund_at"`                             // 退款时间
	RefundAmount  int64     `json:"refund_amount"`                          // 退款金额
	TransactionID string    `gorm:"size:128" json:"transaction_id"`         // 第三方交易ID
	Channel       string    `gorm:"size:32" json:"channel"`                 // 支付渠道: alipay, wechat, etc.
	IP            string    `gorm:"size:64" json:"ip"`                      // 下单IP
	DeviceID      string    `gorm:"size:128" json:"device_id"`              // 设备ID
	ExtraData     string    `gorm:"type:text" json:"extra_data"`            // 额外数据(JSON)
}

// TableName 指定表名
func (Order) TableName() string {
	return "orders"
}

// Game 游戏模型
type Game struct {
	BaseModel
	GameID      string `gorm:"uniqueIndex;size:64" json:"game_id"`       // 游戏ID
	Name        string `gorm:"size:128" json:"name"`                      // 游戏名称
	Description string `gorm:"type:text" json:"description"`             // 游戏描述
	Version     string `gorm:"size:32" json:"version"`                   // 版本号
	Status      string `gorm:"size:16;default:active" json:"status"`     // 状态: active, maintenance, offline
	Category    string `gorm:"size:32" json:"category"`                  // 分类
	IconURL     string `gorm:"size:256" json:"icon_url"`                 // 图标URL
	BannerURL   string `gorm:"size:256" json:"banner_url"`               // 横幅URL
	MinVersion  string `gorm:"size:32" json:"min_version"`               // 最低版本要求
	IsVisible   bool   `gorm:"default:true" json:"is_visible"`           // 是否可见
	SortOrder   int    `gorm:"default:0" json:"sort_order"`              // 排序顺序
	ExtraData   string `gorm:"type:text" json:"extra_data"`              // 额外数据(JSON)
}

// TableName 指定表名
func (Game) TableName() string {
	return "games"
}

// GameStats 游戏统计模型
type GameStats struct {
	BaseModel
	GameID              string    `gorm:"uniqueIndex;size:64" json:"game_id"` // 游戏ID
	Date                time.Time `gorm:"index" json:"date"`                  // 统计日期
	ActiveUsers         int64     `json:"active_users"`                       // 活跃用户数
	NewUsers            int64     `json:"new_users"`                          // 新增用户数
	TotalUsers          int64     `json:"total_users"`                        // 总用户数
	LoginCount          int64     `json:"login_count"`                        // 登录次数
	PlayTime            int64     `json:"play_time"`                          // 总游戏时长(分钟)
	Revenue             int64     `json:"revenue"`                            // 营收(分)
	OrderCount          int64     `json:"order_count"`                        // 订单数
	ItemPurchaseCount   int64     `json:"item_purchase_count"`               // 道具购买数
	ItemConsumptionCount int64    `json:"item_consumption_count"`            // 道具消耗数
}

// TableName 指定表名
func (GameStats) TableName() string {
	return "game_stats"
}

// SystemLog 系统日志模型
type SystemLog struct {
	BaseModel
	Level       string    `gorm:"size:16;index" json:"level"`        // 日志级别
	Message     string    `gorm:"type:text" json:"message"`           // 日志消息
	Source      string    `gorm:"size:64;index" json:"source"`        // 日志来源
	UserID      string    `gorm:"size:64;index" json:"user_id"`       // 关联用户ID
	GameID      string    `gorm:"size:64;index" json:"game_id"`       // 关联游戏ID
	IPAddress   string    `gorm:"size:64" json:"ip_address"`          // IP地址
	UserAgent   string    `gorm:"size:256" json:"user_agent"`         // 用户代理
	RequestID   string    `gorm:"size:64;index" json:"request_id"`    // 请求ID
	Action      string    `gorm:"size:64;index" json:"action"`        // 操作类型
	Resource    string    `gorm:"size:128" json:"resource"`           // 资源类型
	ResourceID  string    `gorm:"size:64" json:"resource_id"`         // 资源ID
	OldValue    string    `gorm:"type:text" json:"old_value"`         // 旧值
	NewValue    string    `gorm:"type:text" json:"new_value"`         // 新值
	ExtraData   string    `gorm:"type:text" json:"extra_data"`        // 额外数据
	ErrorCode   string    `gorm:"size:32" json:"error_code"`          // 错误码
	ErrorMsg    string    `gorm:"type:text" json:"error_msg"`         // 错误信息
	Duration    int64     `json:"duration"`                           // 耗时(毫秒)
}

// TableName 指定表名
func (SystemLog) TableName() string {
	return "system_logs"
}

// AutoMigrate 自动迁移数据库表结构
func (db *Database) AutoMigrate() error {
	models := []interface{}{
		&Player{},
		&PlayerSession{},
		&Item{},
		&Order{},
		&Game{},
		&GameStats{},
		&SystemLog{},
	}

	if db.master != nil {
		if err := db.master.AutoMigrate(models...); err != nil {
			return fmt.Errorf("主库表结构迁移失败: %w", err)
		}
		db.log.Info("主库表结构迁移完成")
	}

	for i, slave := range db.slaves {
		if err := slave.AutoMigrate(models...); err != nil {
			return fmt.Errorf("从库%d表结构迁移失败: %w", i, err)
		}
		db.log.Info("从库表结构迁移完成", "index", i)
	}

	return nil
}
