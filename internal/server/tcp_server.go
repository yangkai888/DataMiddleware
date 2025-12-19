package server

import (
	"fmt"
	"net"
	"sync"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/internal/protocol"
	"datamiddleware/pkg/types"
)

// TCPServer TCP服务器
type TCPServer struct {
	config      types.TCPConfig           `json:"config"`       // TCP配置
	connManager *protocol.ConnectionManager `json:"-"`           // 连接管理器
	logger      logger.Logger             `json:"-"`             // 日志器
	listener    net.Listener              `json:"-"`             // 监听器
	stopChan    chan struct{}             `json:"-"`             // 停止通道
	wg          sync.WaitGroup            `json:"-"`             // 等待组
	running     bool                      `json:"running"`      // 运行状态
	mu          sync.RWMutex              `json:"-"`             // 保护并发访问
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(config types.TCPConfig, log logger.Logger) *TCPServer {
	// 创建连接配置
	connConfig := types.ConnectionConfig{
		MaxConnections: config.MaxConnections,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		BufferSize:     8192, // 8KB缓冲区
		Heartbeat: types.HeartbeatConfig{
			Enabled:  true,
			Interval: 30 * time.Second, // 30秒心跳间隔
			Timeout:  90 * time.Second, // 90秒超时
			MaxMissed: 3,               // 最多丢失3次
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
	address := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
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

	// 关闭停止通道
	select {
	case s.stopChan <- struct{}{}:
	default:
	}

	// 关闭监听器
	if s.listener != nil {
		s.listener.Close()
	}

	// 停止连接管理器
	s.connManager.Stop()

	// 等待所有协程退出
	s.wg.Wait()

	s.running = false
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
		Running:           s.IsRunning(),
		ConnectionCount:   connStats.TotalConnections,
		GameConnections:   connStats.GameStats,
		UserConnections:   connStats.UserStats,
		StateConnections:  connStats.StateStats,
	}
}

// ServerStats 服务器统计信息
type ServerStats struct {
	Running           bool                              `json:"running"`             // 是否运行中
	ConnectionCount   int                               `json:"connection_count"`    // 连接总数
	GameConnections   map[string]int                   `json:"game_connections"`    // 按游戏分组的连接数
	UserConnections   map[string]int                   `json:"user_connections"`    // 按用户分组的连接数
	StateConnections  map[types.ConnectionState]int    `json:"state_connections"`   // 按状态分组的连接数
}

// acceptLoop 接受连接循环
func (s *TCPServer) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			// 设置接受超时
			s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))

			conn, err := s.listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// 超时，继续循环
					continue
				}
				// 其他错误，检查是否是服务器停止导致的
				select {
				case <-s.stopChan:
					return
				default:
					s.logger.Error("接受连接失败", "error", err)
					continue
				}
			}

			// 处理新连接
			s.handleConnection(conn)
		}
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

// handleConnectionLoop 处理连接循环
func (s *TCPServer) handleConnectionLoop(conn *protocol.Connection) {
	defer s.wg.Done()
	defer s.connManager.RemoveConnection(conn.ID)
	defer conn.Close()

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
				} else {
					s.logger.Error("读取消息失败", "conn_id", conn.ID, "error", err)
				}
				return
			}

			// 处理消息
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
		s.logger.Error("发送心跳回复失败", "conn_id", conn.ID, "error", err)
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
