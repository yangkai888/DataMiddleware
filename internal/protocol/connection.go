package protocol

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/common/types"
)

// isConnectionClosedError 检查是否是连接关闭相关的错误
func isConnectionClosedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "use of closed network connection")
}

// Connection TCP连接包装器
type Connection struct {
	ID               string                 `json:"id"`                // 连接ID
	Conn             net.Conn               `json:"-"`                 // 底层TCP连接
	State            types.ConnectionState  `json:"state"`             // 连接状态
	Info             types.ConnectionInfo   `json:"info"`              // 连接信息
	Config           types.ConnectionConfig `json:"-"`                 // 连接配置
	Codec            Codec                  `json:"-"`                 // 编解码器
	Logger           logger.Logger          `json:"-"`                 // 日志器
	heartbeatTimer   *time.Timer            `json:"-"`                 // 心跳定时器
	closeChan        chan struct{}          `json:"-"`                 // 关闭通道
	lastHeartbeat    time.Time              `json:"last_heartbeat"`    // 最后心跳时间
	missedHeartbeats int64                  `json:"missed_heartbeats"` // 连续丢失心跳次数
	readBuffer       []byte                 `json:"-"`                 // 读缓冲区，用于处理TCP粘包分包
	mu               sync.RWMutex           `json:"-"`                 // 保护并发访问
}

// NewConnection 创建新连接
func NewConnection(conn net.Conn, config types.ConnectionConfig, codec Codec, log logger.Logger) *Connection {
	id := generateConnectionID()
	now := time.Now()

	c := &Connection{
		ID:    id,
		Conn:  conn,
		State: types.StateConnecting,
		Info: types.ConnectionInfo{
			ID:           id,
			RemoteAddr:   conn.RemoteAddr().String(),
			LocalAddr:    conn.LocalAddr().String(),
			State:        types.StateConnecting,
			ConnectedAt:  now,
			LastActivity: now,
		},
		Config:           config,
		Codec:            codec,
		Logger:           log,
		closeChan:        make(chan struct{}),
		lastHeartbeat:    now,
		missedHeartbeats: 0,
	}

	// 设置连接超时
	if config.ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))
	}
	if config.WriteTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
	}

	return c
}

// Start 启动连接
func (c *Connection) Start() {
	c.setState(types.StateConnected)
	c.Logger.Info("TCP连接已建立", "conn_id", c.ID, "remote_addr", c.Info.RemoteAddr)

	// 启动心跳检测
	if c.Config.Heartbeat.Enabled {
		go c.heartbeatLoop()
	}

	// 启动空闲检测
	if c.Config.IdleTimeout > 0 {
		go c.idleCheckLoop()
	}
}

// Close 关闭连接
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.State == types.StateClosed {
		return nil
	}

	c.setState(types.StateClosing)

	// 停止心跳定时器
	if c.heartbeatTimer != nil {
		c.heartbeatTimer.Stop()
	}

	// 关闭底层连接
	if err := c.Conn.Close(); err != nil {
		c.Logger.Error("关闭TCP连接失败", "conn_id", c.ID, "error", err)
	}

	// 发送关闭信号
	select {
	case c.closeChan <- struct{}{}:
	default:
	}

	c.setState(types.StateClosed)
	c.Logger.Info("TCP连接已关闭", "conn_id", c.ID)

	return nil
}

// SendMessage 发送消息
func (c *Connection) SendMessage(msg *types.Message) error {
	c.mu.RLock()
	if c.State != types.StateConnected && c.State != types.StateAuthenticated {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	// 编码消息
	data, err := c.Codec.Encode(msg)
	if err != nil {
		c.Logger.Error("编码消息失败", "conn_id", c.ID, "error", err)
		return err
	}

	// 发送数据
	if c.Config.WriteTimeout > 0 {
		c.Conn.SetWriteDeadline(time.Now().Add(c.Config.WriteTimeout))
	}

	n, err := c.Conn.Write(data)
	if err != nil {
		// 检查是否是连接关闭相关的错误
		if isConnectionClosedError(err) {
			c.Logger.Debug("连接已关闭，发送消息失败", "conn_id", c.ID, "error", err)
		} else {
			c.Logger.Error("发送消息失败", "conn_id", c.ID, "error", err)
		}
		atomic.AddInt64(&c.Info.BytesSent, int64(n))
		atomic.AddInt64(&c.Info.MessagesSent, 1)
		c.updateActivity()
		return err
	}

	atomic.AddInt64(&c.Info.BytesSent, int64(n))
	atomic.AddInt64(&c.Info.MessagesSent, 1)
	c.updateActivity()

	c.Logger.Debug("发送消息成功", "conn_id", c.ID, "type", msg.Header.Type, "size", n)
	return nil
}

// ReadMessage 读取消息，支持TCP粘包分包处理
func (c *Connection) ReadMessage() (*types.Message, error) {
	c.mu.RLock()
	if c.State != types.StateConnected && c.State != types.StateAuthenticated {
		c.mu.RUnlock()
		return nil, ErrConnectionClosed
	}
	c.mu.RUnlock()

	// 循环读取直到解析出完整消息
	for {
		// 如果缓冲区有足够数据，先尝试解析
		if len(c.readBuffer) >= 28 { // 最小消息长度
			msg, _, err := c.tryDecodeMessage()
			if err == nil {
				// 成功解析消息
				atomic.AddInt64(&c.Info.MessagesReceived, 1)
				c.updateActivity()
				return msg, nil
			}
			// 如果不是数据不足的错误，返回错误
			if !isInsufficientDataError(err) {
				c.Logger.Error("解码消息失败", "conn_id", c.ID, "error", err, "buffer_size", len(c.readBuffer))
				return nil, err
			}
			// 数据不足或校验和错误，继续读取更多数据
		}

		// 设置读取超时
		if c.Config.ReadTimeout > 0 {
			c.Conn.SetReadDeadline(time.Now().Add(c.Config.ReadTimeout))
		}

		// 读取更多数据
		buffer := make([]byte, c.Config.BufferSize)
		n, err := c.Conn.Read(buffer)
		if err != nil {
			return nil, err
		}

		// 累积到缓冲区
		c.readBuffer = append(c.readBuffer, buffer[:n]...)
		atomic.AddInt64(&c.Info.BytesReceived, int64(n))
	}
}

// tryDecodeMessage 尝试从缓冲区解码消息
func (c *Connection) tryDecodeMessage() (*types.Message, int, error) {
	if len(c.readBuffer) == 0 {
		return nil, 0, fmt.Errorf("缓冲区为空")
	}

	// 尝试解码消息
	msg, consumed, err := c.Codec.Decode(c.readBuffer)
	if err != nil {
		return nil, consumed, err
	}

	// 从缓冲区移除已处理的字节
	if consumed < len(c.readBuffer) {
		c.readBuffer = c.readBuffer[consumed:]
	} else {
		c.readBuffer = nil
	}

	return msg, consumed, nil
}

// isInsufficientDataError 检查是否是数据不足的错误
func isInsufficientDataError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "数据长度不足") ||
		strings.Contains(errStr, "insufficient data") ||
		strings.Contains(errStr, "无法解析")
}

// Authenticate 认证连接
func (c *Connection) Authenticate(gameID, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Info.GameID = gameID
	c.Info.UserID = userID
	c.setState(types.StateAuthenticated)

	c.Logger.Info("连接已认证", "conn_id", c.ID, "game_id", gameID, "user_id", userID)
}

// IsAuthenticated 检查是否已认证
func (c *Connection) IsAuthenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.State == types.StateAuthenticated
}

// UpdateHeartbeat 更新心跳时间
func (c *Connection) UpdateHeartbeat() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastHeartbeat = time.Now()
	atomic.StoreInt64(&c.missedHeartbeats, 0)
	c.updateActivity()
}

// GetStats 获取连接统计信息
func (c *Connection) GetStats() types.ConnectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := c.Info
	info.BytesReceived = atomic.LoadInt64(&c.Info.BytesReceived)
	info.BytesSent = atomic.LoadInt64(&c.Info.BytesSent)
	info.MessagesReceived = atomic.LoadInt64(&c.Info.MessagesReceived)
	info.MessagesSent = atomic.LoadInt64(&c.Info.MessagesSent)

	return info
}

// Private methods

func (c *Connection) setState(state types.ConnectionState) {
	c.State = state
	c.Info.State = state
}

func (c *Connection) updateActivity() {
	now := time.Now()
	c.Info.LastActivity = now
}

func (c *Connection) heartbeatLoop() {
	ticker := time.NewTicker(c.Config.Heartbeat.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkHeartbeat()
		case <-c.closeChan:
			return
		}
	}
}

func (c *Connection) checkHeartbeat() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.State != types.StateConnected && c.State != types.StateAuthenticated {
		return
	}

	// 检查是否超时
	if time.Since(c.lastHeartbeat) > c.Config.Heartbeat.Timeout {
		atomic.AddInt64(&c.missedHeartbeats, 1)

		if atomic.LoadInt64(&c.missedHeartbeats) >= int64(c.Config.Heartbeat.MaxMissed) {
			c.Logger.Warn("心跳超时，关闭连接", "conn_id", c.ID, "missed", atomic.LoadInt64(&c.missedHeartbeats))
			go c.Close()
			return
		}
	}

	// 重置missedHeartbeats计数（如果有活动）
	atomic.StoreInt64(&c.missedHeartbeats, 0)
}

func (c *Connection) idleCheckLoop() {
	ticker := time.NewTicker(c.Config.IdleTimeout / 4) // 每1/4空闲时间检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkIdle()
		case <-c.closeChan:
			return
		}
	}
}

func (c *Connection) checkIdle() {
	c.mu.RLock()
	if c.State != types.StateConnected && c.State != types.StateAuthenticated {
		c.mu.RUnlock()
		return
	}

	if time.Since(c.Info.LastActivity) > c.Config.IdleTimeout {
		c.mu.RUnlock()
		c.Logger.Warn("连接空闲超时，关闭连接", "conn_id", c.ID)
		go c.Close()
		return
	}
	c.mu.RUnlock()
}

// generateConnectionID 生成连接ID
func generateConnectionID() string {
	return fmt.Sprintf("conn_%d_%d", time.Now().Unix(), time.Now().UnixNano()%1000000)
}

// Errors
var (
	ErrConnectionClosed = errors.New("连接已关闭")
)
