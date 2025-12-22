package services

import (
	"fmt"
	"time"

	daoPkg "datamiddleware/internal/data/dao"
	loggingInfra "datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/common/types"
)

// ItemService 道具服务
type ItemService struct {
	dao    daoPkg.DAO
	logger loggingInfra.Logger
}

// NewItemService 创建道具服务
func NewItemService(dao daoPkg.DAO, log loggingInfra.Logger) *ItemService {
	return &ItemService{
		dao:    dao,
		logger: log,
	}
}

// CreateItem 创建道具
func (s *ItemService) CreateItem(userID, gameID, name, itemType, category string, quantity int64) (*types.Item, error) {
	// 生成道具ID
	itemID := s.generateItemID()

	item := &daoPkg.Item{
		ItemID:      itemID,
		UserID:      userID,
		GameID:      gameID,
		Name:        name,
		Type:        itemType,
		Category:    category,
		Rarity:      "common", // 默认稀有度
		Quantity:    quantity,
		MaxQuantity: -1, // 无限制
		IsBound:     false,
		IsTradable:  true,
		Description: fmt.Sprintf("%s 道具", name),
	}

	if err := s.dao.CreateItem(item); err != nil {
		s.logger.Error("创建道具失败", "item_id", itemID, "user_id", userID, "error", err)
		return nil, fmt.Errorf("创建道具失败: %w", err)
	}

	s.logger.Info("道具创建成功", "item_id", itemID, "user_id", userID, "name", name, "quantity", quantity)
	return s.convertToAPITypes(item), nil
}

// GetItem 获取道具信息
func (s *ItemService) GetItem(itemID string) (*types.Item, error) {
	item, err := s.dao.GetItemByID(itemID)
	if err != nil {
		s.logger.Error("获取道具信息失败", "item_id", itemID, "error", err)
		return nil, fmt.Errorf("获取道具信息失败: %w", err)
	}
	if item == nil {
		return nil, fmt.Errorf("道具不存在: %s", itemID)
	}

	return s.convertToAPITypes(item), nil
}

// GetUserItems 获取用户道具列表
func (s *ItemService) GetUserItems(userID, gameID string) ([]*types.Item, error) {
	items, err := s.dao.GetUserItems(userID, gameID)
	if err != nil {
		s.logger.Error("获取用户道具失败", "user_id", userID, "game_id", gameID, "error", err)
		return nil, fmt.Errorf("获取用户道具失败: %w", err)
	}

	// 转换为API类型
	apiItems := make([]*types.Item, len(items))
	for i, item := range items {
		apiItems[i] = s.convertToAPITypes(item)
	}

	return apiItems, nil
}

// UpdateItem 更新道具信息
func (s *ItemService) UpdateItem(itemID string, updates map[string]interface{}) (*types.Item, error) {
	item, err := s.dao.GetItemByID(itemID)
	if err != nil {
		s.logger.Error("获取道具信息失败", "item_id", itemID, "error", err)
		return nil, fmt.Errorf("获取道具信息失败: %w", err)
	}
	if item == nil {
		return nil, fmt.Errorf("道具不存在: %s", itemID)
	}

	// 应用更新
	if name, ok := updates["name"].(string); ok {
		item.Name = name
	}
	if description, ok := updates["description"].(string); ok {
		item.Description = description
	}
	if iconURL, ok := updates["icon_url"].(string); ok {
		item.IconURL = iconURL
	}

	if err := s.dao.UpdateItem(item); err != nil {
		s.logger.Error("更新道具信息失败", "item_id", itemID, "error", err)
		return nil, fmt.Errorf("更新道具信息失败: %w", err)
	}

	s.logger.Info("道具信息更新成功", "item_id", itemID)
	return s.convertToAPITypes(item), nil
}

// AddItemQuantity 增加道具数量
func (s *ItemService) AddItemQuantity(itemID string, quantity int64) error {
	if quantity <= 0 {
		return fmt.Errorf("增加数量必须大于0: %d", quantity)
	}

	if err := s.dao.AddItemQuantity(itemID, quantity); err != nil {
		s.logger.Error("增加道具数量失败", "item_id", itemID, "quantity", quantity, "error", err)
		return fmt.Errorf("增加道具数量失败: %w", err)
	}

	s.logger.Info("道具数量增加成功", "item_id", itemID, "quantity", quantity)
	return nil
}

// ConsumeItem 消耗道具
func (s *ItemService) ConsumeItem(itemID string, quantity int64) error {
	if quantity <= 0 {
		return fmt.Errorf("消耗数量必须大于0: %d", quantity)
	}

	if err := s.dao.ConsumeItem(itemID, quantity); err != nil {
		s.logger.Error("消耗道具失败", "item_id", itemID, "quantity", quantity, "error", err)
		return fmt.Errorf("消耗道具失败: %w", err)
	}

	s.logger.Info("道具消耗成功", "item_id", itemID, "quantity", quantity)
	return nil
}

// TransferItem 道具转移（交易）
func (s *ItemService) TransferItem(itemID, fromUserID, toUserID string, quantity int64) error {
	if quantity <= 0 {
		return fmt.Errorf("转移数量必须大于0: %d", quantity)
	}

	// 检查道具是否存在且属于fromUser
	item, err := s.dao.GetItemByID(itemID)
	if err != nil {
		s.logger.Error("获取道具信息失败", "item_id", itemID, "error", err)
		return fmt.Errorf("获取道具信息失败: %w", err)
	}
	if item == nil {
		return fmt.Errorf("道具不存在: %s", itemID)
	}
	if item.UserID != fromUserID {
		return fmt.Errorf("道具不属于指定用户")
	}
	if item.Quantity < quantity {
		return fmt.Errorf("道具数量不足: 需要%d，拥有%d", quantity, item.Quantity)
	}
	if !item.IsTradable {
		return fmt.Errorf("道具不可交易")
	}

	// 检查接收方是否已有此道具
	toUserItems, err := s.dao.GetUserItems(toUserID, item.GameID)
	if err != nil {
		s.logger.Error("获取接收方道具失败", "to_user_id", toUserID, "error", err)
		return fmt.Errorf("获取接收方道具失败: %w", err)
	}

	// 查找接收方是否已有相同道具
	var existingItem *daoPkg.Item
	for _, userItem := range toUserItems {
		if userItem.Name == item.Name && userItem.Type == item.Type {
			existingItem = userItem
			break
		}
	}

	// 开始事务处理
	// 1. 减少发送方道具数量
	if err := s.dao.ConsumeItem(itemID, quantity); err != nil {
		s.logger.Error("减少发送方道具数量失败", "item_id", itemID, "quantity", quantity, "error", err)
		return fmt.Errorf("减少发送方道具数量失败: %w", err)
	}

	// 2. 增加接收方道具数量
	if existingItem != nil {
		// 增加现有道具数量
		if err := s.dao.AddItemQuantity(existingItem.ItemID, quantity); err != nil {
			s.logger.Error("增加接收方道具数量失败", "item_id", existingItem.ItemID, "quantity", quantity, "error", err)
			// 回滚：恢复发送方道具数量
			s.dao.AddItemQuantity(itemID, quantity)
			return fmt.Errorf("增加接收方道具数量失败: %w", err)
		}
	} else {
		// 创建新道具给接收方
		newItemID := s.generateItemID()
		newItem := &daoPkg.Item{
			ItemID:      newItemID,
			UserID:      toUserID,
			GameID:      item.GameID,
			Name:        item.Name,
			Type:        item.Type,
			Category:    item.Category,
			Rarity:      item.Rarity,
			Quantity:    quantity,
			MaxQuantity: item.MaxQuantity,
			IsBound:     item.IsBound,
			IsTradable:  item.IsTradable,
			ExpireAt:    item.ExpireAt,
			Description: item.Description,
			IconURL:     item.IconURL,
		}

		if err := s.dao.CreateItem(newItem); err != nil {
			s.logger.Error("创建接收方道具失败", "new_item_id", newItemID, "error", err)
			// 回滚：恢复发送方道具数量
			s.dao.AddItemQuantity(itemID, quantity)
			return fmt.Errorf("创建接收方道具失败: %w", err)
		}
	}

	s.logger.Info("道具转移成功", "item_id", itemID, "from_user", fromUserID, "to_user", toUserID, "quantity", quantity)
	return nil
}

// DeleteItem 删除道具
func (s *ItemService) DeleteItem(itemID string) error {
	if err := s.dao.DeleteItem(itemID); err != nil {
		s.logger.Error("删除道具失败", "item_id", itemID, "error", err)
		return fmt.Errorf("删除道具失败: %w", err)
	}

	s.logger.Info("道具删除成功", "item_id", itemID)
	return nil
}

// BatchCreateItems 批量创建道具
func (s *ItemService) BatchCreateItems(userID, gameID string, itemRequests []types.ItemRequest) ([]*types.Item, error) {
	if len(itemRequests) == 0 {
		return []*types.Item{}, nil
	}

	items := make([]*types.Item, 0, len(itemRequests))

	for _, req := range itemRequests {
		item, err := s.CreateItem(userID, gameID, req.Name, req.Type, req.Category, req.Quantity)
		if err != nil {
			s.logger.Error("批量创建道具失败", "user_id", userID, "item_name", req.Name, "error", err)
			return nil, fmt.Errorf("创建道具 %s 失败: %w", req.Name, err)
		}
		items = append(items, item)
	}

	s.logger.Info("批量创建道具成功", "user_id", userID, "count", len(items))
	return items, nil
}

// ValidateItemOwnership 验证道具所有权
func (s *ItemService) ValidateItemOwnership(itemID, userID string) error {
	item, err := s.dao.GetItemByID(itemID)
	if err != nil {
		return fmt.Errorf("验证道具所有权失败: %w", err)
	}
	if item == nil {
		return fmt.Errorf("道具不存在: %s", itemID)
	}
	if item.UserID != userID {
		return fmt.Errorf("道具不属于指定用户")
	}
	return nil
}

// CheckItemExpiration 检查道具过期
func (s *ItemService) CheckItemExpiration() ([]*types.Item, error) {
	// 这里需要DAO层支持按过期时间查询
	// 暂时返回空切片
	s.logger.Debug("检查道具过期")
	return []*types.Item{}, nil
}

// Helper methods

func (s *ItemService) generateItemID() string {
	return fmt.Sprintf("item_%d", time.Now().UnixNano())
}

func (s *ItemService) convertToAPITypes(item *daoPkg.Item) *types.Item {
	return &types.Item{
		ItemID:      item.ItemID,
		UserID:      item.UserID,
		GameID:      item.GameID,
		Name:        item.Name,
		Type:        item.Type,
		Category:    item.Category,
		Rarity:      item.Rarity,
		Quantity:    item.Quantity,
		MaxQuantity: item.MaxQuantity,
		IsBound:     item.IsBound,
		IsTradable:  item.IsTradable,
		ExpireAt:    item.ExpireAt,
		Description: item.Description,
		IconURL:     item.IconURL,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}
