package services

import (
	"encoding/json"
	"fmt"

	"datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/common/types"
)

// GameHandler 游戏处理器实现
type GameHandler struct {
	playerService *PlayerService
	itemService   *ItemService
	orderService  *OrderService
	logger        logger.Logger
	gameID        string
}

// NewGameHandler 创建游戏处理器
func NewGameHandler(gameID string, playerService *PlayerService, itemService *ItemService, orderService *OrderService, log logger.Logger) *GameHandler {
	return &GameHandler{
		playerService: playerService,
		itemService:   itemService,
		orderService:  orderService,
		logger:        log,
		gameID:        gameID,
	}
}

// Handle 处理游戏请求
func (h *GameHandler) Handle(gameID string, req *types.Request) (*types.Response, error) {
	h.logger.Debug("处理游戏请求", "game_id", gameID, "type", req.Type, "user_id", req.UserID)

	switch req.Type {
	case types.MessageTypePlayerLogin:
		return h.handlePlayerLogin(req)
	case types.MessageTypePlayerLogout:
		return h.handlePlayerLogout(req)
	case types.MessageTypeItemOperation:
		return h.handleItemOperation(req)
	case types.MessageTypeOrderOperation:
		return h.handleOrderOperation(req)
	default:
		return &types.Response{
			ID:        req.ID,
			Code:      4001,
			Message:   "不支持的消息类型",
			Timestamp: req.Timestamp,
		}, nil
	}
}

// GetSupportedMessageTypes 获取支持的消息类型
func (h *GameHandler) GetSupportedMessageTypes() []types.MessageType {
	return []types.MessageType{
		types.MessageTypePlayerLogin,
		types.MessageTypePlayerLogout,
		types.MessageTypeItemOperation,
		types.MessageTypeOrderOperation,
	}
}

// GetName 获取处理器名称
func (h *GameHandler) GetName() string {
	return fmt.Sprintf("GameHandler-%s", h.gameID)
}

// handlePlayerLogin 处理玩家登录
func (h *GameHandler) handlePlayerLogin(req *types.Request) (*types.Response, error) {
	var loginReq struct {
		UserID   string `json:"user_id"`
		DeviceID string `json:"device_id"`
		Platform string `json:"platform"`
		Version  string `json:"version"`
	}

	data, ok := req.Data.([]byte)
	if !ok {
		return &types.Response{
			ID:        req.ID,
			Code:      4002,
			Message:   "请求数据格式错误",
			Timestamp: req.Timestamp,
		}, nil
	}
	if err := json.Unmarshal(data, &loginReq); err != nil {
		h.logger.Error("解析登录请求失败", "error", err)
		return &types.Response{
			ID:        req.ID,
			Code:      4002,
			Message:   "请求数据格式错误",
			Timestamp: req.Timestamp,
		}, nil
	}

	// 调用玩家服务登录
	result, err := h.playerService.LoginPlayer(loginReq.UserID, req.GameID, loginReq.DeviceID, loginReq.Platform, loginReq.Version)
	if err != nil {
		h.logger.Error("玩家登录失败", "user_id", loginReq.UserID, "error", err)
		return &types.Response{
			ID:        req.ID,
			Code:      5001,
			Message:   fmt.Sprintf("登录失败: %v", err),
			Timestamp: req.Timestamp,
		}, nil
	}

	return &types.Response{
		ID:        req.ID,
		Code:      0,
		Message:   "登录成功",
		Data:      result,
		Timestamp: req.Timestamp,
	}, nil
}

// handlePlayerLogout 处理玩家登出
func (h *GameHandler) handlePlayerLogout(req *types.Request) (*types.Response, error) {
	var logoutReq struct {
		UserID    string `json:"user_id"`
		SessionID string `json:"session_id"`
	}

	data, ok := req.Data.([]byte)
	if !ok {
		return &types.Response{
			ID:        req.ID,
			Code:      4002,
			Message:   "请求数据格式错误",
			Timestamp: req.Timestamp,
		}, nil
	}
	if err := json.Unmarshal(data, &logoutReq); err != nil {
		h.logger.Error("解析登出请求失败", "error", err)
		return &types.Response{
			ID:        req.ID,
			Code:      4002,
			Message:   "请求数据格式错误",
			Timestamp: req.Timestamp,
		}, nil
	}

	// 调用玩家服务登出
	if err := h.playerService.LogoutPlayer(logoutReq.UserID, logoutReq.SessionID); err != nil {
		h.logger.Error("玩家登出失败", "user_id", logoutReq.UserID, "error", err)
		return &types.Response{
			ID:        req.ID,
			Code:      5002,
			Message:   fmt.Sprintf("登出失败: %v", err),
			Timestamp: req.Timestamp,
		}, nil
	}

	return &types.Response{
		ID:        req.ID,
		Code:      0,
		Message:   "登出成功",
		Timestamp: req.Timestamp,
	}, nil
}

// handleItemOperation 处理道具操作
func (h *GameHandler) handleItemOperation(req *types.Request) (*types.Response, error) {
	var itemReq struct {
		UserID     string `json:"user_id"`
		Operation  string `json:"operation"` // create, consume, transfer
		ItemID     string `json:"item_id,omitempty"`
		Name       string `json:"name,omitempty"`
		Type       string `json:"type,omitempty"`
		Category   string `json:"category,omitempty"`
		Quantity   int64  `json:"quantity,omitempty"`
		ToUserID   string `json:"to_user_id,omitempty"`
	}

	data, ok := req.Data.([]byte)
	if !ok {
		return &types.Response{
			ID:        req.ID,
			Code:      4002,
			Message:   "请求数据格式错误",
			Timestamp: req.Timestamp,
		}, nil
	}
	if err := json.Unmarshal(data, &itemReq); err != nil {
		h.logger.Error("解析道具操作请求失败", "error", err)
		return &types.Response{
			ID:        req.ID,
			Code:      4002,
			Message:   "请求数据格式错误",
			Timestamp: req.Timestamp,
		}, nil
	}

	switch itemReq.Operation {
	case "create":
		item, err := h.itemService.CreateItem(itemReq.UserID, req.GameID, itemReq.Name, itemReq.Type, itemReq.Category, itemReq.Quantity)
		if err != nil {
			h.logger.Error("创建道具失败", "user_id", itemReq.UserID, "error", err)
			return &types.Response{
				ID:        req.ID,
				Code:      5003,
				Message:   fmt.Sprintf("创建道具失败: %v", err),
				Timestamp: req.Timestamp,
			}, nil
		}
		return &types.Response{
			ID:        req.ID,
			Code:      0,
			Message:   "创建道具成功",
			Data:      item,
			Timestamp: req.Timestamp,
		}, nil

	case "consume":
		if err := h.itemService.ConsumeItem(itemReq.ItemID, itemReq.Quantity); err != nil {
			h.logger.Error("消耗道具失败", "item_id", itemReq.ItemID, "error", err)
			return &types.Response{
				ID:        req.ID,
				Code:      5004,
				Message:   fmt.Sprintf("消耗道具失败: %v", err),
				Timestamp: req.Timestamp,
			}, nil
		}
		return &types.Response{
			ID:        req.ID,
			Code:      0,
			Message:   "消耗道具成功",
			Timestamp: req.Timestamp,
		}, nil

	case "transfer":
		if err := h.itemService.TransferItem(itemReq.ItemID, itemReq.UserID, itemReq.ToUserID, itemReq.Quantity); err != nil {
			h.logger.Error("转移道具失败", "item_id", itemReq.ItemID, "error", err)
			return &types.Response{
				ID:        req.ID,
				Code:      5005,
				Message:   fmt.Sprintf("转移道具失败: %v", err),
				Timestamp: req.Timestamp,
			}, nil
		}
		return &types.Response{
			ID:        req.ID,
			Code:      0,
			Message:   "转移道具成功",
			Timestamp: req.Timestamp,
		}, nil

	default:
		return &types.Response{
			ID:        req.ID,
			Code:      4003,
			Message:   "不支持的道具操作",
			Timestamp: req.Timestamp,
		}, nil
	}
}

// handleOrderOperation 处理订单操作
func (h *GameHandler) handleOrderOperation(req *types.Request) (*types.Response, error) {
	var orderReq struct {
		UserID        string `json:"user_id"`
		Operation     string `json:"operation"` // create, pay, cancel
		OrderID       string `json:"order_id,omitempty"`
		ProductID     string `json:"product_id,omitempty"`
		ProductName   string `json:"product_name,omitempty"`
		Amount        int64  `json:"amount,omitempty"`
		Currency      string `json:"currency,omitempty"`
		PaymentMethod string `json:"payment_method,omitempty"`
		Channel       string `json:"channel,omitempty"`
		TransactionID string `json:"transaction_id,omitempty"`
	}

	data, ok := req.Data.([]byte)
	if !ok {
		return &types.Response{
			ID:        req.ID,
			Code:      4002,
			Message:   "请求数据格式错误",
			Timestamp: req.Timestamp,
		}, nil
	}
	if err := json.Unmarshal(data, &orderReq); err != nil {
		h.logger.Error("解析订单操作请求失败", "error", err)
		return &types.Response{
			ID:        req.ID,
			Code:      4002,
			Message:   "请求数据格式错误",
			Timestamp: req.Timestamp,
		}, nil
	}

	switch orderReq.Operation {
	case "create":
		order, err := h.orderService.CreateOrder(
			orderReq.UserID, req.GameID, orderReq.ProductID, orderReq.ProductName,
			orderReq.Amount, orderReq.Currency, orderReq.PaymentMethod,
			orderReq.Channel, "", "", // IP和DeviceID暂时为空
		)
		if err != nil {
			h.logger.Error("创建订单失败", "user_id", orderReq.UserID, "error", err)
			return &types.Response{
				ID:        req.ID,
				Code:      5006,
				Message:   fmt.Sprintf("创建订单失败: %v", err),
				Timestamp: req.Timestamp,
			}, nil
		}
		return &types.Response{
			ID:        req.ID,
			Code:      0,
			Message:   "创建订单成功",
			Data:      order,
			Timestamp: req.Timestamp,
		}, nil

	case "pay":
		order, err := h.orderService.ProcessPayment(orderReq.OrderID, orderReq.TransactionID)
		if err != nil {
			h.logger.Error("处理支付失败", "order_id", orderReq.OrderID, "error", err)
			return &types.Response{
				ID:        req.ID,
				Code:      5007,
				Message:   fmt.Sprintf("处理支付失败: %v", err),
				Timestamp: req.Timestamp,
			}, nil
		}
		return &types.Response{
			ID:        req.ID,
			Code:      0,
			Message:   "支付成功",
			Data:      order,
			Timestamp: req.Timestamp,
		}, nil

	case "cancel":
		order, err := h.orderService.CancelOrder(orderReq.OrderID)
		if err != nil {
			h.logger.Error("取消订单失败", "order_id", orderReq.OrderID, "error", err)
			return &types.Response{
				ID:        req.ID,
				Code:      5008,
				Message:   fmt.Sprintf("取消订单失败: %v", err),
				Timestamp: req.Timestamp,
			}, nil
		}
		return &types.Response{
			ID:        req.ID,
			Code:      0,
			Message:   "取消订单成功",
			Data:      order,
			Timestamp: req.Timestamp,
		}, nil

	default:
		return &types.Response{
			ID:        req.ID,
			Code:      4003,
			Message:   "不支持的订单操作",
			Timestamp: req.Timestamp,
		}, nil
	}
}
