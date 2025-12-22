package dao

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"datamiddleware/internal/infrastructure/logging"
)

// DAO 数据访问对象接口
type DAO interface {
	// 玩家相关
	CreatePlayer(player *Player) error
	GetPlayerByID(userID string) (*Player, error)
	GetPlayerByUsername(username string) (*Player, error)
	UpdatePlayer(player *Player) error
	DeletePlayer(userID string) error
	ListPlayers(gameID string, offset, limit int) ([]*Player, int64, error)

	// 会话相关
	CreateSession(session *PlayerSession) error
	GetSessionByID(sessionID string) (*PlayerSession, error)
	GetActiveSessions(userID string) ([]*PlayerSession, error)
	UpdateSession(session *PlayerSession) error
	InvalidateSession(sessionID string) error
	CleanupExpiredSessions() error

	// 道具相关
	CreateItem(item *Item) error
	GetItemByID(itemID string) (*Item, error)
	GetUserItems(userID string, gameID string) ([]*Item, error)
	UpdateItem(item *Item) error
	DeleteItem(itemID string) error
	ConsumeItem(itemID string, quantity int64) error
	AddItemQuantity(itemID string, quantity int64) error

	// 订单相关
	CreateOrder(order *Order) error
	GetOrderByID(orderID string) (*Order, error)
	GetUserOrders(userID string, status string, offset, limit int) ([]*Order, int64, error)
	UpdateOrderStatus(orderID string, status string) error
	GetOrdersByStatus(status string, offset, limit int) ([]*Order, int64, error)

	// 游戏相关
	CreateGame(game *Game) error
	GetGameByID(gameID string) (*Game, error)
	ListGames(offset, limit int) ([]*Game, int64, error)
	UpdateGame(game *Game) error
	DeleteGame(gameID string) error

	// 统计相关
	CreateGameStats(stats *GameStats) error
	GetGameStats(gameID string, date time.Time) (*GameStats, error)
	UpdateGameStats(gameID string, date time.Time, updates map[string]interface{}) error

	// 日志相关
	CreateSystemLog(logEntry *SystemLog) error
	ListSystemLogs(filters map[string]interface{}, offset, limit int) ([]*SystemLog, int64, error)
}

// daoImpl DAO实现
type daoImpl struct {
	db     *Database
	logger logger.Logger
}

// NewDAO 创建DAO实例
func NewDAO(db *Database, logger logger.Logger) DAO {
	return &daoImpl{
		db:     db,
		logger: logger,
	}
}

// CreatePlayer 创建玩家
func (d *daoImpl) CreatePlayer(player *Player) error {
	result := d.db.Master().Create(player)
	if result.Error != nil {
		d.logger.Error("创建玩家失败", "user_id", player.UserID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("创建玩家成功", "user_id", player.UserID, "id", player.ID)
	return nil
}

// GetPlayerByID 根据ID获取玩家
func (d *daoImpl) GetPlayerByID(userID string) (*Player, error) {
	var player Player
	result := d.db.Slave().Where("user_id = ?", userID).First(&player)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		d.logger.Error("获取玩家失败", "user_id", userID, "error", result.Error)
		return nil, result.Error
	}
	return &player, nil
}

// GetPlayerByUsername 根据用户名获取玩家
func (d *daoImpl) GetPlayerByUsername(username string) (*Player, error) {
	var player Player
	result := d.db.Slave().Where("username = ?", username).First(&player)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		d.logger.Error("获取玩家失败", "username", username, "error", result.Error)
		return nil, result.Error
	}
	return &player, nil
}

// UpdatePlayer 更新玩家
func (d *daoImpl) UpdatePlayer(player *Player) error {
	result := d.db.Master().Save(player)
	if result.Error != nil {
		d.logger.Error("更新玩家失败", "user_id", player.UserID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("更新玩家成功", "user_id", player.UserID)
	return nil
}

// DeletePlayer 删除玩家
func (d *daoImpl) DeletePlayer(userID string) error {
	result := d.db.Master().Where("user_id = ?", userID).Delete(&Player{})
	if result.Error != nil {
		d.logger.Error("删除玩家失败", "user_id", userID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("删除玩家成功", "user_id", userID, "affected", result.RowsAffected)
	return nil
}

// ListPlayers 列出玩家
func (d *daoImpl) ListPlayers(gameID string, offset, limit int) ([]*Player, int64, error) {
	var players []*Player
	var total int64

	query := d.db.Slave().Model(&Player{})
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		d.logger.Error("获取玩家总数失败", "error", err)
		return nil, 0, err
	}

	// 获取数据
	if err := query.Offset(offset).Limit(limit).Find(&players).Error; err != nil {
		d.logger.Error("获取玩家列表失败", "error", err)
		return nil, 0, err
	}

	return players, total, nil
}

// CreateSession 创建会话
func (d *daoImpl) CreateSession(session *PlayerSession) error {
	result := d.db.Master().Create(session)
	if result.Error != nil {
		d.logger.Error("创建会话失败", "session_id", session.SessionID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("创建会话成功", "session_id", session.SessionID)
	return nil
}

// GetSessionByID 根据ID获取会话
func (d *daoImpl) GetSessionByID(sessionID string) (*PlayerSession, error) {
	var session PlayerSession
	result := d.db.Slave().Where("session_id = ?", sessionID).First(&session)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		d.logger.Error("获取会话失败", "session_id", sessionID, "error", result.Error)
		return nil, result.Error
	}
	return &session, nil
}

// GetActiveSessions 获取用户活跃会话
func (d *daoImpl) GetActiveSessions(userID string) ([]*PlayerSession, error) {
	var sessions []*PlayerSession
	result := d.db.Slave().Where("user_id = ? AND is_active = ?", userID, true).Find(&sessions)
	if result.Error != nil {
		d.logger.Error("获取活跃会话失败", "user_id", userID, "error", result.Error)
		return nil, result.Error
	}
	return sessions, nil
}

// UpdateSession 更新会话
func (d *daoImpl) UpdateSession(session *PlayerSession) error {
	result := d.db.Master().Save(session)
	if result.Error != nil {
		d.logger.Error("更新会话失败", "session_id", session.SessionID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("更新会话成功", "session_id", session.SessionID)
	return nil
}

// InvalidateSession 使会话失效
func (d *daoImpl) InvalidateSession(sessionID string) error {
	result := d.db.Master().Model(&PlayerSession{}).Where("session_id = ?", sessionID).Update("is_active", false)
	if result.Error != nil {
		d.logger.Error("使会话失效失败", "session_id", sessionID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("使会话失效成功", "session_id", sessionID)
	return nil
}

// CleanupExpiredSessions 清理过期会话
func (d *daoImpl) CleanupExpiredSessions() error {
	result := d.db.Master().Where("expire_at < ? OR is_active = ?", time.Now(), false).Delete(&PlayerSession{})
	if result.Error != nil {
		d.logger.Error("清理过期会话失败", "error", result.Error)
		return result.Error
	}
	d.logger.Debug("清理过期会话成功", "affected", result.RowsAffected)
	return nil
}

// CreateItem 创建道具
func (d *daoImpl) CreateItem(item *Item) error {
	result := d.db.Master().Create(item)
	if result.Error != nil {
		d.logger.Error("创建道具失败", "item_id", item.ItemID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("创建道具成功", "item_id", item.ItemID)
	return nil
}

// GetItemByID 根据ID获取道具
func (d *daoImpl) GetItemByID(itemID string) (*Item, error) {
	var item Item
	result := d.db.Slave().Where("item_id = ?", itemID).First(&item)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		d.logger.Error("获取道具失败", "item_id", itemID, "error", result.Error)
		return nil, result.Error
	}
	return &item, nil
}

// GetUserItems 获取用户道具
func (d *daoImpl) GetUserItems(userID string, gameID string) ([]*Item, error) {
	var items []*Item
	query := d.db.Slave().Where("user_id = ?", userID)
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}

	result := query.Find(&items)
	if result.Error != nil {
		d.logger.Error("获取用户道具失败", "user_id", userID, "error", result.Error)
		return nil, result.Error
	}
	return items, nil
}

// UpdateItem 更新道具
func (d *daoImpl) UpdateItem(item *Item) error {
	result := d.db.Master().Save(item)
	if result.Error != nil {
		d.logger.Error("更新道具失败", "item_id", item.ItemID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("更新道具成功", "item_id", item.ItemID)
	return nil
}

// DeleteItem 删除道具
func (d *daoImpl) DeleteItem(itemID string) error {
	result := d.db.Master().Where("item_id = ?", itemID).Delete(&Item{})
	if result.Error != nil {
		d.logger.Error("删除道具失败", "item_id", itemID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("删除道具成功", "item_id", itemID, "affected", result.RowsAffected)
	return nil
}

// ConsumeItem 消耗道具
func (d *daoImpl) ConsumeItem(itemID string, quantity int64) error {
	result := d.db.Master().Model(&Item{}).Where("item_id = ? AND quantity >= ?", itemID, quantity).Update("quantity", gorm.Expr("quantity - ?", quantity))
	if result.Error != nil {
		d.logger.Error("消耗道具失败", "item_id", itemID, "quantity", quantity, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("道具数量不足")
	}
	d.logger.Debug("消耗道具成功", "item_id", itemID, "quantity", quantity)
	return nil
}

// AddItemQuantity 增加道具数量
func (d *daoImpl) AddItemQuantity(itemID string, quantity int64) error {
	result := d.db.Master().Model(&Item{}).Where("item_id = ?", itemID).Update("quantity", gorm.Expr("quantity + ?", quantity))
	if result.Error != nil {
		d.logger.Error("增加道具数量失败", "item_id", itemID, "quantity", quantity, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("增加道具数量成功", "item_id", itemID, "quantity", quantity)
	return nil
}

// CreateOrder 创建订单
func (d *daoImpl) CreateOrder(order *Order) error {
	result := d.db.Master().Create(order)
	if result.Error != nil {
		d.logger.Error("创建订单失败", "order_id", order.OrderID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("创建订单成功", "order_id", order.OrderID)
	return nil
}

// GetOrderByID 根据ID获取订单
func (d *daoImpl) GetOrderByID(orderID string) (*Order, error) {
	var order Order
	result := d.db.Slave().Where("order_id = ?", orderID).First(&order)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		d.logger.Error("获取订单失败", "order_id", orderID, "error", result.Error)
		return nil, result.Error
	}
	return &order, nil
}

// GetUserOrders 获取用户订单
func (d *daoImpl) GetUserOrders(userID string, status string, offset, limit int) ([]*Order, int64, error) {
	var orders []*Order
	var total int64

	query := d.db.Slave().Model(&Order{}).Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		d.logger.Error("获取用户订单总数失败", "user_id", userID, "error", err)
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&orders).Error; err != nil {
		d.logger.Error("获取用户订单列表失败", "user_id", userID, "error", err)
		return nil, 0, err
	}

	return orders, total, nil
}

// UpdateOrderStatus 更新订单状态
func (d *daoImpl) UpdateOrderStatus(orderID string, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == "paid" {
		updates["payment_at"] = time.Now()
	}

	result := d.db.Master().Model(&Order{}).Where("order_id = ?", orderID).Updates(updates)
	if result.Error != nil {
		d.logger.Error("更新订单状态失败", "order_id", orderID, "status", status, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("更新订单状态成功", "order_id", orderID, "status", status)
	return nil
}

// GetOrdersByStatus 根据状态获取订单
func (d *daoImpl) GetOrdersByStatus(status string, offset, limit int) ([]*Order, int64, error) {
	var orders []*Order
	var total int64

	query := d.db.Slave().Model(&Order{}).Where("status = ?", status)

	if err := query.Count(&total).Error; err != nil {
		d.logger.Error("获取订单总数失败", "status", status, "error", err)
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&orders).Error; err != nil {
		d.logger.Error("获取订单列表失败", "status", status, "error", err)
		return nil, 0, err
	}

	return orders, total, nil
}

// CreateGame 创建游戏
func (d *daoImpl) CreateGame(game *Game) error {
	result := d.db.Master().Create(game)
	if result.Error != nil {
		d.logger.Error("创建游戏失败", "game_id", game.GameID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("创建游戏成功", "game_id", game.GameID)
	return nil
}

// GetGameByID 根据ID获取游戏
func (d *daoImpl) GetGameByID(gameID string) (*Game, error) {
	var game Game
	result := d.db.Slave().Where("game_id = ?", gameID).First(&game)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		d.logger.Error("获取游戏失败", "game_id", gameID, "error", result.Error)
		return nil, result.Error
	}
	return &game, nil
}

// ListGames 列出游戏
func (d *daoImpl) ListGames(offset, limit int) ([]*Game, int64, error) {
	var games []*Game
	var total int64

	if err := d.db.Slave().Model(&Game{}).Count(&total).Error; err != nil {
		d.logger.Error("获取游戏总数失败", "error", err)
		return nil, 0, err
	}

	if err := d.db.Slave().Offset(offset).Limit(limit).Order("sort_order ASC, created_at DESC").Find(&games).Error; err != nil {
		d.logger.Error("获取游戏列表失败", "error", err)
		return nil, 0, err
	}

	return games, total, nil
}

// UpdateGame 更新游戏
func (d *daoImpl) UpdateGame(game *Game) error {
	result := d.db.Master().Save(game)
	if result.Error != nil {
		d.logger.Error("更新游戏失败", "game_id", game.GameID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("更新游戏成功", "game_id", game.GameID)
	return nil
}

// DeleteGame 删除游戏
func (d *daoImpl) DeleteGame(gameID string) error {
	result := d.db.Master().Where("game_id = ?", gameID).Delete(&Game{})
	if result.Error != nil {
		d.logger.Error("删除游戏失败", "game_id", gameID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("删除游戏成功", "game_id", gameID, "affected", result.RowsAffected)
	return nil
}

// CreateGameStats 创建游戏统计
func (d *daoImpl) CreateGameStats(stats *GameStats) error {
	result := d.db.Master().Create(stats)
	if result.Error != nil {
		d.logger.Error("创建游戏统计失败", "game_id", stats.GameID, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("创建游戏统计成功", "game_id", stats.GameID, "date", stats.Date)
	return nil
}

// GetGameStats 获取游戏统计
func (d *daoImpl) GetGameStats(gameID string, date time.Time) (*GameStats, error) {
	var stats GameStats
	result := d.db.Slave().Where("game_id = ? AND date = ?", gameID, date).First(&stats)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		d.logger.Error("获取游戏统计失败", "game_id", gameID, "date", date, "error", result.Error)
		return nil, result.Error
	}
	return &stats, nil
}

// UpdateGameStats 更新游戏统计
func (d *daoImpl) UpdateGameStats(gameID string, date time.Time, updates map[string]interface{}) error {
	result := d.db.Master().Model(&GameStats{}).Where("game_id = ? AND date = ?", gameID, date).Updates(updates)
	if result.Error != nil {
		d.logger.Error("更新游戏统计失败", "game_id", gameID, "date", date, "error", result.Error)
		return result.Error
	}
	d.logger.Debug("更新游戏统计成功", "game_id", gameID, "date", date)
	return nil
}

// CreateSystemLog 创建系统日志
func (d *daoImpl) CreateSystemLog(logEntry *SystemLog) error {
	result := d.db.Master().Create(logEntry)
	if result.Error != nil {
		d.logger.Error("创建系统日志失败", "error", result.Error)
		return result.Error
	}
	return nil
}

// ListSystemLogs 列出系统日志
func (d *daoImpl) ListSystemLogs(filters map[string]interface{}, offset, limit int) ([]*SystemLog, int64, error) {
	var logs []*SystemLog
	var total int64

	query := d.db.Slave().Model(&SystemLog{})

	// 应用过滤条件
	for key, value := range filters {
		query = query.Where(fmt.Sprintf("%s = ?", key), value)
	}

	if err := query.Count(&total).Error; err != nil {
		d.logger.Error("获取系统日志总数失败", "error", err)
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&logs).Error; err != nil {
		d.logger.Error("获取系统日志列表失败", "error", err)
		return nil, 0, err
	}

	return logs, total, nil
}
