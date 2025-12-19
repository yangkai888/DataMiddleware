package types

import "time"

// Player 玩家信息
type Player struct {
	UserID      string     `json:"user_id"`
	GameID      string     `json:"game_id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Nickname    string     `json:"nickname"`
	Avatar      string     `json:"avatar"`
	Level       int        `json:"level"`
	Experience  int64      `json:"experience"`
	Coins       int64      `json:"coins"`
	Diamonds    int64      `json:"diamonds"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// LoginResult 登录结果
type LoginResult struct {
	User      *Player `json:"user"`
	SessionID string  `json:"session_id"`
	Token     string  `json:"token"`
	ExpiresAt int64   `json:"expires_at"`
}

// Item 道具信息
type Item struct {
	ItemID      string     `json:"item_id"`
	UserID      string     `json:"user_id"`
	GameID      string     `json:"game_id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Category    string     `json:"category"`
	Rarity      string     `json:"rarity"`
	Quantity    int64      `json:"quantity"`
	MaxQuantity int64      `json:"max_quantity"`
	IsBound     bool       `json:"is_bound"`
	IsTradable  bool       `json:"is_tradable"`
	ExpireAt    *time.Time `json:"expire_at"`
	Description string     `json:"description"`
	IconURL     string     `json:"icon_url"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Order 订单信息
type Order struct {
	OrderID       string     `json:"order_id"`
	UserID        string     `json:"user_id"`
	GameID        string     `json:"game_id"`
	ProductID     string     `json:"product_id"`
	ProductName   string     `json:"product_name"`
	Amount        int64      `json:"amount"`
	Currency      string     `json:"currency"`
	PaymentMethod string     `json:"payment_method"`
	Status        string     `json:"status"`
	PaymentAt     *time.Time `json:"payment_at"`
	RefundAt      *time.Time `json:"refund_at"`
	RefundAmount  int64      `json:"refund_amount"`
	TransactionID string     `json:"transaction_id"`
	Channel       string     `json:"channel"`
	IP            string     `json:"ip"`
	DeviceID      string     `json:"device_id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Game 游戏信息
type Game struct {
	GameID      string `json:"game_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	Category    string `json:"category"`
	IconURL     string `json:"icon_url"`
	BannerURL   string `json:"banner_url"`
	MinVersion  string `json:"min_version"`
	IsVisible   bool   `json:"is_visible"`
	SortOrder   int    `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GameStats 游戏统计
type GameStats struct {
	GameID              string    `json:"game_id"`
	Date                time.Time `json:"date"`
	ActiveUsers         int64     `json:"active_users"`
	NewUsers            int64     `json:"new_users"`
	TotalUsers          int64     `json:"total_users"`
	LoginCount          int64     `json:"login_count"`
	PlayTime            int64     `json:"play_time"`
	Revenue             int64     `json:"revenue"`
	OrderCount          int64     `json:"order_count"`
	ItemPurchaseCount   int64     `json:"item_purchase_count"`
	ItemConsumptionCount int64    `json:"item_consumption_count"`
}

// SystemLog 系统日志
type SystemLog struct {
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	Source      string    `json:"source"`
	UserID      string    `json:"user_id"`
	GameID      string    `json:"game_id"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	RequestID   string    `json:"request_id"`
	Action      string    `json:"action"`
	Resource    string    `json:"resource"`
	ResourceID  string    `json:"resource_id"`
	OldValue    string    `json:"old_value"`
	NewValue    string    `json:"new_value"`
	Duration    int64     `json:"duration"`
	ErrorCode   string    `json:"error_code"`
	ErrorMsg    string    `json:"error_msg"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserSession 用户会话
type UserSession struct {
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	GameID      string    `json:"game_id"`
	Token       string    `json:"token"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	LoginAt     time.Time `json:"login_at"`
	ExpireAt    time.Time `json:"expire_at"`
	IsActive    bool      `json:"is_active"`
	DeviceID    string    `json:"device_id"`
}

// ItemRequest 道具创建请求
type ItemRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Quantity int64  `json:"quantity"`
}

// OrderStatistics 订单统计
type OrderStatistics struct {
	TotalOrders     int64 `json:"total_orders"`
	TotalRevenue    int64 `json:"total_revenue"`
	PaidOrders      int64 `json:"paid_orders"`
	CancelledOrders int64 `json:"cancelled_orders"`
	RefundedOrders  int64 `json:"refunded_orders"`
}

// TokenPair JWT令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
}

// TokenClaims JWT令牌声明
type TokenClaims struct {
	UserID   string `json:"user_id"`
	GameID   string `json:"game_id"`
	Username string `json:"username"`
	ExpiresAt int64 `json:"expires_at"`
	IssuedAt  int64 `json:"issued_at"`
	TokenID   string `json:"token_id"`
}

// APIKey API密钥
type APIKey struct {
	KeyID     string    `json:"key_id"`
	Key       string    `json:"key"`
	GameID    string    `json:"game_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IsActive  bool      `json:"is_active"`
}

// Cache 缓存接口
type Cache interface {
	// Get 获取缓存值
	Get(key string) ([]byte, error)
	// Set 设置缓存值
	Set(key string, value []byte) error
	// SetWithTTL 设置缓存值并指定TTL
	SetWithTTL(key string, value []byte, ttl time.Duration) error
	// Delete 删除缓存值
	Delete(key string) error
	// Exists 检查键是否存在
	Exists(key string) bool
	// Clear 清空缓存
	Clear() error
	// Close 关闭缓存
	Close() error
}

