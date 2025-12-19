package protocol

import (
	"fmt"
	"net"
	"sync"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// ConnectionManager 连接管理器
type ConnectionManager struct {
	config        types.ConnectionConfig `json:"config"`         // 连接配置
	connections   map[string]*Connection `json:"-"`              // 连接映射
	mu            sync.RWMutex           `json:"-"`              // 保护并发访问
	logger        logger.Logger          `json:"-"`              // 日志器
	codec         Codec                  `json:"-"`              // 编解码器
	stopChan      chan struct{}          `json:"-"`              // 停止通道
	cleanupTicker *time.Ticker           `json:"-"`              // 清理定时器
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(config types.ConnectionConfig, codec Codec, log logger.Logger) *ConnectionManager {
	return &ConnectionManager{
		config:      config,
		connections: make(map[string]*Connection),
		logger:      log,
		codec:       codec,
		stopChan:    make(chan struct{}),
	}
}

// Start 启动连接管理器
func (cm *ConnectionManager) Start() {
	cm.logger.Info("连接管理器启动", "max_connections", cm.config.MaxConnections)

	// 启动清理协程
	if cm.config.CleanupInterval > 0 {
		cm.cleanupTicker = time.NewTicker(cm.config.CleanupInterval)
		go cm.cleanupLoop()
	}
}

// Stop 停止连接管理器
func (cm *ConnectionManager) Stop() {
	cm.logger.Info("连接管理器停止中...")

	// 停止清理协程
	if cm.cleanupTicker != nil {
		cm.cleanupTicker.Stop()
	}

	// 关闭停止通道
	select {
	case cm.stopChan <- struct{}{}:
	default:
	}

	// 关闭所有连接
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, conn := range cm.connections {
		if err := conn.Close(); err != nil {
			cm.logger.Error("关闭连接失败", "conn_id", id, "error", err)
		}
	}

	cm.connections = make(map[string]*Connection)
	cm.logger.Info("连接管理器已停止")
}

// AddConnection 添加连接
func (cm *ConnectionManager) AddConnection(conn net.Conn) (*Connection, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查连接数量限制
	if cm.config.MaxConnections > 0 && len(cm.connections) >= cm.config.MaxConnections {
		conn.Close()
		return nil, fmt.Errorf("连接数量已达到上限: %d", cm.config.MaxConnections)
	}

	// 创建连接包装器
	connection := NewConnection(conn, cm.config, cm.codec, cm.logger)

	// 添加到连接映射
	cm.connections[connection.ID] = connection

	// 启动连接
	connection.Start()

	cm.logger.Info("连接已添加", "conn_id", connection.ID, "total", len(cm.connections))
	return connection, nil
}

// RemoveConnection 移除连接
func (cm *ConnectionManager) RemoveConnection(connID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	_, exists := cm.connections[connID]
	if !exists {
		return
	}

	delete(cm.connections, connID)
	cm.logger.Info("连接已移除", "conn_id", connID, "remaining", len(cm.connections))
}

// GetConnection 获取连接
func (cm *ConnectionManager) GetConnection(connID string) (*Connection, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, exists := cm.connections[connID]
	return conn, exists
}

// GetAllConnections 获取所有连接
func (cm *ConnectionManager) GetAllConnections() map[string]*Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	connections := make(map[string]*Connection)
	for id, conn := range cm.connections {
		connections[id] = conn
	}
	return connections
}

// GetConnectionCount 获取连接数量
func (cm *ConnectionManager) GetConnectionCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections)
}

// GetConnectionsByGame 获取指定游戏的所有连接
func (cm *ConnectionManager) GetConnectionsByGame(gameID string) []*Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var connections []*Connection
	for _, conn := range cm.connections {
		if conn.Info.GameID == gameID {
			connections = append(connections, conn)
		}
	}
	return connections
}

// GetConnectionsByUser 获取指定用户的所有连接
func (cm *ConnectionManager) GetConnectionsByUser(userID string) []*Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var connections []*Connection
	for _, conn := range cm.connections {
		if conn.Info.UserID == userID {
			connections = append(connections, conn)
		}
	}
	return connections
}

// BroadcastToGame 广播消息到指定游戏的所有连接
func (cm *ConnectionManager) BroadcastToGame(gameID string, msg *types.Message) {
	connections := cm.GetConnectionsByGame(gameID)
	for _, conn := range connections {
		if err := conn.SendMessage(msg); err != nil {
			cm.logger.Error("广播消息失败", "conn_id", conn.ID, "error", err)
		}
	}
}

// BroadcastToUser 广播消息到指定用户的所有连接
func (cm *ConnectionManager) BroadcastToUser(userID string, msg *types.Message) {
	connections := cm.GetConnectionsByUser(userID)
	for _, conn := range connections {
		if err := conn.SendMessage(msg); err != nil {
			cm.logger.Error("广播消息失败", "conn_id", conn.ID, "error", err)
		}
	}
}

// GetStats 获取统计信息
func (cm *ConnectionManager) GetStats() ConnectionManagerStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := ConnectionManagerStats{
		TotalConnections: len(cm.connections),
		GameStats:        make(map[string]int),
		UserStats:        make(map[string]int),
		StateStats:       make(map[types.ConnectionState]int),
	}

	for _, conn := range cm.connections {
		// 按游戏统计
		if conn.Info.GameID != "" {
			stats.GameStats[conn.Info.GameID]++
		}

		// 按用户统计
		if conn.Info.UserID != "" {
			stats.UserStats[conn.Info.UserID]++
		}

		// 按状态统计
		stats.StateStats[conn.State]++
	}

	return stats
}

// ConnectionManagerStats 连接管理器统计信息
type ConnectionManagerStats struct {
	TotalConnections int                              `json:"total_connections"` // 总连接数
	GameStats        map[string]int                   `json:"game_stats"`        // 按游戏统计
	UserStats        map[string]int                   `json:"user_stats"`        // 按用户统计
	StateStats       map[types.ConnectionState]int    `json:"state_stats"`      // 按状态统计
}

// cleanupLoop 清理循环
func (cm *ConnectionManager) cleanupLoop() {
	for {
		select {
		case <-cm.cleanupTicker.C:
			cm.cleanup()
		case <-cm.stopChan:
			return
		}
	}
}

// cleanup 清理无效连接
func (cm *ConnectionManager) cleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var toRemove []string
	for id, conn := range cm.connections {
		// 检查连接状态
		if conn.State == types.StateClosed {
			toRemove = append(toRemove, id)
		}
	}

	// 移除无效连接
	for _, id := range toRemove {
		delete(cm.connections, id)
		cm.logger.Debug("清理无效连接", "conn_id", id)
	}

	if len(toRemove) > 0 {
		cm.logger.Info("连接清理完成", "removed", len(toRemove), "remaining", len(cm.connections))
	}
}
