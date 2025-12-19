package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"datamiddleware/internal/database"
	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// PlayerService 玩家服务
type PlayerService struct {
	dao    database.DAO
	logger logger.Logger
}

// NewPlayerService 创建玩家服务
func NewPlayerService(dao database.DAO, log logger.Logger) *PlayerService {
	return &PlayerService{
		dao:    dao,
		logger: log,
	}
}

// RegisterPlayer 注册玩家
func (s *PlayerService) RegisterPlayer(gameID, username, email, phone string) (*types.Player, error) {
	// 检查用户名是否已存在
	existing, err := s.dao.GetPlayerByUsername(username)
	if err != nil {
		s.logger.Error("检查用户名是否存在失败", "username", username, "error", err)
		return nil, fmt.Errorf("检查用户名失败: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("用户名已存在: %s", username)
	}

	// 生成用户ID
	userID, err := s.generateUserID()
	if err != nil {
		s.logger.Error("生成用户ID失败", "error", err)
		return nil, fmt.Errorf("生成用户ID失败: %w", err)
	}

	// 创建玩家
	player := &database.Player{
		UserID:      userID,
		GameID:      gameID,
		Username:    username,
		Email:       email,
		Phone:       phone,
		Nickname:    username, // 默认昵称与用户名相同
		Level:       1,
		Experience:  0,
		Coins:       1000, // 初始金币
		Diamonds:    100,  // 初始钻石
		Status:      "active",
	}

	if err := s.dao.CreatePlayer(player); err != nil {
		s.logger.Error("创建玩家失败", "user_id", userID, "username", username, "error", err)
		return nil, fmt.Errorf("创建玩家失败: %w", err)
	}

	s.logger.Info("玩家注册成功", "user_id", userID, "username", username, "game_id", gameID)

	// 转换为API类型
	return s.convertToAPITypes(player), nil
}

// LoginPlayer 玩家登录
func (s *PlayerService) LoginPlayer(userID, gameID, deviceID, platform, version string) (*types.LoginResult, error) {
	// 获取玩家信息
	player, err := s.dao.GetPlayerByID(userID)
	if err != nil {
		s.logger.Error("获取玩家信息失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("获取玩家信息失败: %w", err)
	}
	if player == nil {
		return nil, fmt.Errorf("玩家不存在: %s", userID)
	}

	// 检查玩家状态
	if player.Status != "active" {
		return nil, fmt.Errorf("玩家账号状态异常: %s", player.Status)
	}

	// 更新登录信息
	now := time.Now()
	player.LastLoginAt = &now
	player.LastLoginIP = "" // TODO: 从请求中获取IP
	player.DeviceID = deviceID
	player.Platform = platform
	player.Version = version

	if err := s.dao.UpdatePlayer(player); err != nil {
		s.logger.Error("更新玩家登录信息失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("更新登录信息失败: %w", err)
	}

	// 生成会话令牌
	sessionID, err := s.generateSessionID()
	if err != nil {
		s.logger.Error("生成会话ID失败", "error", err)
		return nil, fmt.Errorf("生成会话ID失败: %w", err)
	}

	token := s.generateToken(sessionID)

	// 创建会话记录
	session := &database.PlayerSession{
		SessionID: sessionID,
		UserID:    userID,
		GameID:    gameID,
		Token:     token,
		IPAddress: "", // TODO: 从请求中获取
		UserAgent: "",
		LoginAt:   time.Now(),
		ExpireAt:  time.Now().Add(24 * time.Hour), // 24小时过期
		IsActive:  true,
		DeviceID:  deviceID,
	}

	if err := s.dao.CreateSession(session); err != nil {
		s.logger.Error("创建会话失败", "session_id", sessionID, "error", err)
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	s.logger.Info("玩家登录成功", "user_id", userID, "session_id", sessionID, "game_id", gameID)

	return &types.LoginResult{
		User:      s.convertToAPITypes(player),
		SessionID: sessionID,
		Token:     token,
		ExpiresAt: session.ExpireAt.Unix(),
	}, nil
}

// LogoutPlayer 玩家登出
func (s *PlayerService) LogoutPlayer(userID, sessionID string) error {
	// 使会话失效
	if err := s.dao.InvalidateSession(sessionID); err != nil {
		s.logger.Error("使会话失效失败", "session_id", sessionID, "error", err)
		return fmt.Errorf("登出失败: %w", err)
	}

	s.logger.Info("玩家登出成功", "user_id", userID, "session_id", sessionID)
	return nil
}

// GetPlayer 获取玩家信息
func (s *PlayerService) GetPlayer(userID string) (*types.Player, error) {
	player, err := s.dao.GetPlayerByID(userID)
	if err != nil {
		s.logger.Error("获取玩家信息失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("获取玩家信息失败: %w", err)
	}
	if player == nil {
		return nil, fmt.Errorf("玩家不存在: %s", userID)
	}

	return s.convertToAPITypes(player), nil
}

// UpdatePlayer 更新玩家信息
func (s *PlayerService) UpdatePlayer(userID string, updates map[string]interface{}) (*types.Player, error) {
	player, err := s.dao.GetPlayerByID(userID)
	if err != nil {
		s.logger.Error("获取玩家信息失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("获取玩家信息失败: %w", err)
	}
	if player == nil {
		return nil, fmt.Errorf("玩家不存在: %s", userID)
	}

	// 应用更新
	if nickname, ok := updates["nickname"].(string); ok {
		player.Nickname = nickname
	}
	if avatar, ok := updates["avatar"].(string); ok {
		player.Avatar = avatar
	}

	if err := s.dao.UpdatePlayer(player); err != nil {
		s.logger.Error("更新玩家信息失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("更新玩家信息失败: %w", err)
	}

	s.logger.Info("玩家信息更新成功", "user_id", userID)
	return s.convertToAPITypes(player), nil
}

// UpdatePlayerStats 更新玩家统计数据
func (s *PlayerService) UpdatePlayerStats(userID string, experience int64, coins int64, diamonds int64) (*types.Player, error) {
	player, err := s.dao.GetPlayerByID(userID)
	if err != nil {
		s.logger.Error("获取玩家信息失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("获取玩家信息失败: %w", err)
	}
	if player == nil {
		return nil, fmt.Errorf("玩家不存在: %s", userID)
	}

	// 更新统计数据
	player.Experience += experience
	player.Coins += coins
	player.Diamonds += diamonds

	// 检查等级升级
	newLevel := s.calculateLevel(player.Experience)
	if newLevel > player.Level {
		player.Level = newLevel
		s.logger.Info("玩家等级升级", "user_id", userID, "old_level", player.Level-newLevel, "new_level", newLevel)
	}

	// 确保数值不小于0
	if player.Coins < 0 {
		player.Coins = 0
	}
	if player.Diamonds < 0 {
		player.Diamonds = 0
	}

	if err := s.dao.UpdatePlayer(player); err != nil {
		s.logger.Error("更新玩家统计失败", "user_id", userID, "error", err)
		return nil, fmt.Errorf("更新玩家统计失败: %w", err)
	}

	s.logger.Debug("玩家统计更新成功", "user_id", userID, "exp", experience, "coins", coins, "diamonds", diamonds)
	return s.convertToAPITypes(player), nil
}

// ListPlayers 列出玩家
func (s *PlayerService) ListPlayers(gameID string, offset, limit int) ([]*types.Player, int64, error) {
	players, total, err := s.dao.ListPlayers(gameID, offset, limit)
	if err != nil {
		s.logger.Error("获取玩家列表失败", "game_id", gameID, "error", err)
		return nil, 0, fmt.Errorf("获取玩家列表失败: %w", err)
	}

	// 转换为API类型
	apiPlayers := make([]*types.Player, len(players))
	for i, player := range players {
		apiPlayers[i] = s.convertToAPITypes(player)
	}

	return apiPlayers, total, nil
}

// ValidateSession 验证会话
func (s *PlayerService) ValidateSession(sessionID string) (*types.Player, error) {
	session, err := s.dao.GetSessionByID(sessionID)
	if err != nil {
		s.logger.Error("获取会话失败", "session_id", sessionID, "error", err)
		return nil, fmt.Errorf("获取会话失败: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	if !session.IsActive {
		return nil, fmt.Errorf("会话已失效: %s", sessionID)
	}

	if time.Now().After(session.ExpireAt) {
		return nil, fmt.Errorf("会话已过期: %s", sessionID)
	}

	player, err := s.dao.GetPlayerByID(session.UserID)
	if err != nil {
		s.logger.Error("获取玩家信息失败", "user_id", session.UserID, "error", err)
		return nil, fmt.Errorf("获取玩家信息失败: %w", err)
	}
	if player == nil {
		return nil, fmt.Errorf("玩家不存在: %s", session.UserID)
	}

	return s.convertToAPITypes(player), nil
}

// CleanupExpiredSessions 清理过期会话
func (s *PlayerService) CleanupExpiredSessions() error {
	if err := s.dao.CleanupExpiredSessions(); err != nil {
		s.logger.Error("清理过期会话失败", "error", err)
		return fmt.Errorf("清理过期会话失败: %w", err)
	}

	s.logger.Info("过期会话清理完成")
	return nil
}

// Helper methods

func (s *PlayerService) generateUserID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "user_" + hex.EncodeToString(bytes), nil
}

func (s *PlayerService) generateSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "sess_" + hex.EncodeToString(bytes), nil
}

func (s *PlayerService) generateToken(sessionID string) string {
	// 简化的token生成，实际应该使用JWT
	return "token_" + sessionID
}

func (s *PlayerService) calculateLevel(experience int64) int {
	// 简化的等级计算公式
	level := 1
	expNeeded := int64(100)

	for experience >= expNeeded {
		level++
		expNeeded = int64(level * 100) // 每级需要level*100经验
	}

	return level
}

func (s *PlayerService) convertToAPITypes(player *database.Player) *types.Player {
	return &types.Player{
		UserID:      player.UserID,
		GameID:      player.GameID,
		Username:    player.Username,
		Email:       player.Email,
		Phone:       player.Phone,
		Nickname:    player.Nickname,
		Avatar:      player.Avatar,
		Level:       player.Level,
		Experience:  player.Experience,
		Coins:       player.Coins,
		Diamonds:    player.Diamonds,
		Status:      player.Status,
		LastLoginAt: player.LastLoginAt,
		CreatedAt:   player.CreatedAt,
		UpdatedAt:   player.UpdatedAt,
	}
}
