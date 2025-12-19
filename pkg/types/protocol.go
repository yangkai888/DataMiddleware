package types

import (
	"time"
)

// ProtocolVersion 协议版本
const ProtocolVersion = 1

// MessageType 消息类型
type MessageType uint16

const (
	// 基础消息类型
	MessageTypeHeartbeat MessageType = 0x0001 // 心跳
	MessageTypeHandshake MessageType = 0x0002 // 握手

	// 游戏数据消息类型
	MessageTypePlayerLogin    MessageType = 0x1001 // 玩家登录
	MessageTypePlayerLogout   MessageType = 0x1002 // 玩家登出
	MessageTypePlayerData     MessageType = 0x1003 // 玩家数据
	MessageTypeItemOperation  MessageType = 0x1004 // 道具操作
	MessageTypeOrderOperation MessageType = 0x1005 // 订单操作

	// 系统消息类型
	MessageTypeError      MessageType = 0x2001 // 错误消息
	MessageTypePing       MessageType = 0x2002 // ping
	MessageTypePong       MessageType = 0x2003 // pong
)

// MessageFlag 消息标志
type MessageFlag uint8

const (
	FlagNone         MessageFlag = 0x00 // 无特殊标志
	FlagCompressed  MessageFlag = 0x01 // 压缩
	FlagEncrypted   MessageFlag = 0x02 // 加密
	FlagNeedResponse MessageFlag = 0x04 // 需要响应
	FlagAsync       MessageFlag = 0x08 // 异步消息
)

// MessageHeader 消息头
type MessageHeader struct {
	Version     uint8       `json:"version"`      // 协议版本
	Type        MessageType `json:"type"`         // 消息类型
	Flags       MessageFlag `json:"flags"`        // 消息标志
	SequenceID  uint32      `json:"sequence_id"`  // 序列号
	GameID      string      `json:"game_id"`      // 游戏ID
	UserID      string      `json:"user_id"`      // 用户ID
	Timestamp   int64       `json:"timestamp"`    // 时间戳
	BodyLength  uint32      `json:"body_length"`  // 消息体长度
	Checksum    uint32      `json:"checksum"`     // 校验和
}

// Message 完整消息
type Message struct {
	Header MessageHeader `json:"header"`
	Body   []byte        `json:"body"`
}

// ConnectionState 连接状态
type ConnectionState int

const (
	StateConnecting ConnectionState = iota // 连接中
	StateConnected                         // 已连接
	StateAuthenticated                     // 已认证
	StateClosing                           // 关闭中
	StateClosed                            // 已关闭
)

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	ID              string          `json:"id"`               // 连接ID
	RemoteAddr      string          `json:"remote_addr"`      // 远程地址
	LocalAddr       string          `json:"local_addr"`       // 本地地址
	State           ConnectionState `json:"state"`            // 连接状态
	ConnectedAt     time.Time       `json:"connected_at"`     // 连接时间
	LastActivity    time.Time       `json:"last_activity"`    // 最后活动时间
	GameID          string          `json:"game_id"`          // 游戏ID
	UserID          string          `json:"user_id"`          // 用户ID
	BytesReceived   int64           `json:"bytes_received"`   // 接收字节数
	BytesSent       int64           `json:"bytes_sent"`       // 发送字节数
	MessagesReceived int64          `json:"messages_received"` // 接收消息数
	MessagesSent    int64           `json:"messages_sent"`     // 发送消息数
}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Enabled  bool          `json:"enabled"`   // 是否启用心跳
	Interval time.Duration `json:"interval"`  // 心跳间隔
	Timeout  time.Duration `json:"timeout"`   // 心跳超时时间
	MaxMissed int          `json:"max_missed"` // 最大连续丢失次数
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	MaxConnections   int             `json:"max_connections"`   // 最大连接数
	ReadTimeout      time.Duration   `json:"read_timeout"`      // 读取超时
	WriteTimeout     time.Duration   `json:"write_timeout"`     // 写入超时
	BufferSize       int             `json:"buffer_size"`       // 缓冲区大小
	Heartbeat        HeartbeatConfig `json:"heartbeat"`         // 心跳配置
	IdleTimeout      time.Duration   `json:"idle_timeout"`      // 空闲超时
	CleanupInterval  time.Duration   `json:"cleanup_interval"`  // 清理间隔
}

// Request 业务请求
type Request struct {
	ID       string      `json:"id"`       // 请求ID
	Type     MessageType `json:"type"`     // 请求类型
	GameID   string      `json:"game_id"`  // 游戏ID
	UserID   string      `json:"user_id"`  // 用户ID
	Data     interface{} `json:"data"`     // 请求数据
	Timeout  time.Duration `json:"-"`      // 超时时间
}

// Response 业务响应
type Response struct {
	ID        string      `json:"id"`         // 响应ID
	Code      int         `json:"code"`       // 响应码
	Message   string      `json:"message"`    // 响应消息
	Data      interface{} `json:"data"`       // 响应数据
	Timestamp int64       `json:"timestamp"`  // 时间戳
}

// Handler 消息处理器接口
type Handler interface {
	Handle(connID string, req *Request) (*Response, error)
}

// Middleware 中间件接口
type Middleware interface {
	Process(connID string, msg *Message) (*Message, error)
}
