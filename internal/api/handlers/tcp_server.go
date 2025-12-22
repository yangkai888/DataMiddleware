package server

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/protocol"
	"datamiddleware/internal/common/types"
)

// TCPServer TCP服务器
type TCPServer struct {
	config       types.ServerConfig          `json:"config"`        // 服务器配置（包含环境信息）
	connManager  *protocol.ConnectionManager `json:"-"`             // 连接管理器
	logger       logger.Logger               `json:"-"`             // 日志器
	listener     net.Listener                `json:"-"`             // 监听器
	stopChan     chan struct{}               `json:"-"`             // 停止通道
	wg           sync.WaitGroup              `json:"-"`             // 等待组
	running      bool                        `json:"running"`       // 运行状态
	shuttingDown bool                        `json:"shutting_down"` // 是否正在关闭
	mu           sync.RWMutex                `json:"-"`             // 保护并发访问
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(config types.ServerConfig, log logger.Logger) *TCPServer {
	// 创建连接配置
	connConfig := types.ConnectionConfig{
		MaxConnections: config.TCP.MaxConnections,
		ReadTimeout:    config.TCP.ReadTimeout,
		WriteTimeout:   config.TCP.WriteTimeout,
		BufferSize:     8192, // 8KB缓冲区
		Heartbeat: types.HeartbeatConfig{
			Enabled:   true,
			Interval:  30 * time.Second, // 30秒心跳间隔
			Timeout:   90 * time.Second, // 90秒超时
			MaxMissed: 3,                // 最多丢失3次
		},
		IdleTimeout:     300 * time.Second, // 5分钟空闲超时
		CleanupInterval: 60 * time.Second,  // 60秒清理间隔
	}

	// 创建编解码器
	codec := protocol.NewBinaryCodec() // 使用二进制编解码器获得更好性能

	// 创建连接管理器
	connManager := protocol.NewConnectionManager(connConfig, codec, log)

	return &TCPServer{
		config:      config,
		connManager: connManager,
		logger:      log,
		stopChan:    make(chan struct{}),
	}
}

// Start 启动TCP服务器
func (s *TCPServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("TCP服务器已在运行")
	}

	// 创建监听器
	address := fmt.Sprintf("%s:%d", s.config.TCP.Host, s.config.TCP.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("创建TCP监听器失败: %w", err)
	}

	s.listener = listener
	s.running = true

	// 启动连接管理器
	s.connManager.Start()

	s.logger.Info("TCP服务器启动", "address", address)

	// 启动接受连接的协程
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop 停止TCP服务器
func (s *TCPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("TCP服务器停止中...")

	// 标记为正在停止和关闭
	s.running = false
	s.shuttingDown = true

	// 先发送停止信号到所有协程，避免竞态条件
	close(s.stopChan)

	// 关闭监听器（这会让accept()立即返回错误）
	if s.listener != nil {
		s.listener.Close()
	}

	// 停止连接管理器
	s.connManager.Stop()

	// 等待所有协程退出
	s.wg.Wait()

	s.logger.Info("TCP服务器已停止")
	return nil
}

// IsRunning 检查服务器是否正在运行
func (s *TCPServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetStats 获取服务器统计信息
func (s *TCPServer) GetStats() ServerStats {
	connStats := s.connManager.GetStats()

	return ServerStats{
		Running:          s.IsRunning(),
		ConnectionCount:  connStats.TotalConnections,
		GameConnections:  connStats.GameStats,
		UserConnections:  connStats.UserStats,
		StateConnections: connStats.StateStats,
	}
}

// ServerStats 服务器统计信息
type ServerStats struct {
	Running          bool                          `json:"running"`           // 是否运行中
	ConnectionCount  int                           `json:"connection_count"`  // 连接总数
	GameConnections  map[string]int                `json:"game_connections"`  // 按游戏分组的连接数
	UserConnections  map[string]int                `json:"user_connections"`  // 按用户分组的连接数
	StateConnections map[types.ConnectionState]int `json:"state_connections"` // 按状态分组的连接数
}

// acceptLoop 接受连接循环
func (s *TCPServer) acceptLoop() {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("acceptLoop发生panic", "panic", r)
		}
		s.wg.Done()
	}()

	for {
		// 设置接受超时
		var timeout time.Duration
		select {
		case <-s.stopChan:
			// 服务器正在停止，设置很短的超时以便快速退出
			timeout = 10 * time.Millisecond
			s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))
		default:
			if s.config.Env == "dev" {
				timeout = 10 * time.Second // 开发模式10秒超时，减少日志噪音
			} else {
				timeout = 30 * time.Second // 生产模式30秒超时
			}
			s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))
		}

		conn, err := s.listener.Accept()

		// 检查是否收到停止信号
		select {
		case <-s.stopChan:
			s.logger.Debug("收到停止信号，退出accept循环")
			return
		default:
		}

		if err != nil {
			// 检查是否服务器正在关闭，如果是则不记录错误
			s.mu.RLock()
			isShuttingDown := s.shuttingDown
			s.mu.RUnlock()

			if isShuttingDown {
				s.logger.Debug("TCP服务器正在关闭，停止接受新连接")
				return
			}

			// 检查是否是服务器停止导致的错误
			select {
			case <-s.stopChan:
				s.logger.Debug("TCP服务器正在关闭，停止接受新连接")
				return
			default:
			}

			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// 超时是正常的，只有在调试模式下才记录
				if s.config.TCP.Debug {
					s.logger.Debug("TCP服务器等待连接超时，继续监听")
				}
				continue
			}

			// 检查是否是网络连接已关闭的错误（服务器正在关闭）
			if opErr, ok := err.(*net.OpError); ok {
				if opErr.Op == "accept" {
					// 检查是否是监听器关闭导致的错误
					if opErr.Err.Error() == "use of closed network connection" ||
						strings.Contains(opErr.Err.Error(), "closed") ||
						strings.Contains(opErr.Err.Error(), "shutdown") {
						s.logger.Debug("TCP监听器已关闭或服务器正在关闭，停止接受连接")
						return
					}
				}
			}

			// 对于accept相关的错误，在非调试模式下不记录（因为服务器关闭时会产生大量此类错误）
			// 只在调试模式下记录警告级别的信息
			if opErr, ok := err.(*net.OpError); ok && opErr.Op == "accept" {
				if s.config.TCP.Debug {
					s.logger.Debug("TCP服务器接受连接遇到非致命错误", "error", err)
				}
				// 非调试模式下完全不记录accept相关的错误，避免噪音
			}
			continue
		}

		// 处理新连接
		s.handleConnection(conn)
	}
}

// handleConnection 处理新连接
func (s *TCPServer) handleConnection(conn net.Conn) {
	// 添加连接到管理器
	connection, err := s.connManager.AddConnection(conn)
	if err != nil {
		s.logger.Error("添加连接失败", "remote_addr", conn.RemoteAddr(), "error", err)
		conn.Close()
		return
	}

	// 启动连接处理协程
	s.wg.Add(1)
	go s.handleConnectionLoop(connection)
}

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
		strings.Contains(errStr, "EOF")
}

// handleConnectionLoop 处理连接循环
func (s *TCPServer) handleConnectionLoop(conn *protocol.Connection) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("handleConnectionLoop发生panic", "conn_id", conn.ID, "panic", r)
		}
		s.wg.Done()
		s.connManager.RemoveConnection(conn.ID)
		conn.Close()
	}()

	s.logger.Info("开始处理连接", "conn_id", conn.ID)

	for {
		select {
		case <-s.stopChan:
			return
		default:
			// 读取消息
			msg, err := conn.ReadMessage()
			if err != nil {
				if err == protocol.ErrConnectionClosed {
					s.logger.Info("连接已关闭", "conn_id", conn.ID)
				} else if isConnectionClosedError(err) {
					s.logger.Debug("连接被客户端关闭", "conn_id", conn.ID, "error", err)
				} else {
					s.logger.Error("读取消息失败", "conn_id", conn.ID, "error", err)
				}
				return
			}

			// 处理消息
			s.logger.Debug("处理消息", "conn_id", conn.ID, "type", msg.Header.Type, "seq", msg.Header.SequenceID)
			s.handleMessage(conn, msg)
		}
	}
}

// handleMessage 处理消息
func (s *TCPServer) handleMessage(conn *protocol.Connection, msg *types.Message) {
	s.logger.Debug("收到消息", "conn_id", conn.ID, "type", msg.Header.Type, "seq", msg.Header.SequenceID)

	switch msg.Header.Type {
	case types.MessageTypeHeartbeat:
		s.handleHeartbeat(conn, msg)
	case types.MessageTypeHandshake:
		s.handleHandshake(conn, msg)
	case types.MessageTypePlayerLogin:
		s.handlePlayerLogin(conn, msg)
	case types.MessageTypePlayerLogout:
		s.handlePlayerLogout(conn, msg)
	default:
		s.handleUnknownMessage(conn, msg)
	}
}

// handleHeartbeat 处理心跳消息
func (s *TCPServer) handleHeartbeat(conn *protocol.Connection, msg *types.Message) {
	conn.UpdateHeartbeat()

	// 回复心跳
	response := protocol.CreateHeartbeatMessage(msg.Header.SequenceID)
	if err := conn.SendMessage(response); err != nil {
		// 检查是否是连接被客户端重置的正常情况
		if isConnectionClosedError(err) {
			s.logger.Debug("客户端已断开，跳过心跳回复", "conn_id", conn.ID)
		} else {
			s.logger.Error("发送心跳回复失败", "conn_id", conn.ID, "error", err)
		}
	}
}

// handleHandshake 处理握手消息
func (s *TCPServer) handleHandshake(conn *protocol.Connection, msg *types.Message) {
	// 解析握手数据
	gameID := msg.Header.GameID
	userID := msg.Header.UserID

	if gameID == "" || userID == "" {
		s.logger.Warn("握手失败：缺少游戏ID或用户ID", "conn_id", conn.ID)
		errorMsg := protocol.CreateErrorMessage(4001, "缺少游戏ID或用户ID", msg.Header.SequenceID)
		conn.SendMessage(errorMsg)
		return
	}

	// 认证连接
	conn.Authenticate(gameID, userID)

	s.logger.Info("握手成功", "conn_id", conn.ID, "game_id", gameID, "user_id", userID)

	// 回复握手成功
	response := protocol.CreateHandshakeMessage(gameID, userID, msg.Header.SequenceID)
	conn.SendMessage(response)
}

// handlePlayerLogin 处理玩家登录
func (s *TCPServer) handlePlayerLogin(conn *protocol.Connection, msg *types.Message) {
	if !conn.IsAuthenticated() {
		errorMsg := protocol.CreateErrorMessage(4002, "连接未认证", msg.Header.SequenceID)
		conn.SendMessage(errorMsg)
		return
	}

	s.logger.Info("玩家登录", "conn_id", conn.ID, "game_id", conn.Info.GameID, "user_id", conn.Info.UserID)

	// TODO: 调用业务逻辑处理玩家登录
	// 这里暂时只记录日志，实际实现会调用业务服务
}

// handlePlayerLogout 处理玩家登出
func (s *TCPServer) handlePlayerLogout(conn *protocol.Connection, msg *types.Message) {
	if !conn.IsAuthenticated() {
		errorMsg := protocol.CreateErrorMessage(4002, "连接未认证", msg.Header.SequenceID)
		conn.SendMessage(errorMsg)
		return
	}

	s.logger.Info("玩家登出", "conn_id", conn.ID, "game_id", conn.Info.GameID, "user_id", conn.Info.UserID)

	// TODO: 调用业务逻辑处理玩家登出
	// 这里暂时只记录日志，实际实现会调用业务服务
}

// handleUnknownMessage 处理未知消息
func (s *TCPServer) handleUnknownMessage(conn *protocol.Connection, msg *types.Message) {
	s.logger.Warn("收到未知消息类型", "conn_id", conn.ID, "type", msg.Header.Type)

	errorMsg := protocol.CreateErrorMessage(4003, "未知消息类型", msg.Header.SequenceID)
	conn.SendMessage(errorMsg)
}
