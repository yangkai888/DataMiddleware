package services

import (
	"fmt"
	"time"

	daoPkg "datamiddleware/internal/data/dao"
	loggingInfra "datamiddleware/internal/infrastructure/logging"
	"datamiddleware/internal/common/types"
)

// OrderService 订单服务
type OrderService struct {
	dao    daoPkg.DAO
	logger loggingInfra.Logger
}

// NewOrderService 创建订单服务
func NewOrderService(dao daoPkg.DAO, log loggingInfra.Logger) *OrderService {
	return &OrderService{
		dao:    dao,
		logger: log,
	}
}

// CreateOrder 创建订单
func (s *OrderService) CreateOrder(userID, gameID, productID, productName string, amount int64, currency, paymentMethod, channel, ip, deviceID string) (*types.Order, error) {
	// 生成订单ID
	orderID := s.generateOrderID()

	order := &daoPkg.Order{
		OrderID:       orderID,
		UserID:        userID,
		GameID:        gameID,
		ProductID:     productID,
		ProductName:   productName,
		Amount:        amount,
		Currency:      currency,
		PaymentMethod: paymentMethod,
		Status:        "pending",
		Channel:       channel,
		IP:            ip,
		DeviceID:      deviceID,
	}

	if err := s.dao.CreateOrder(order); err != nil {
		s.logger.Error("创建订单失败", "order_id", orderID, "user_id", userID, "error", err)
		return nil, fmt.Errorf("创建订单失败: %w", err)
	}

	s.logger.Info("订单创建成功", "order_id", orderID, "user_id", userID, "amount", amount, "currency", currency)
	return s.convertToAPITypes(order), nil
}

// GetOrder 获取订单信息
func (s *OrderService) GetOrder(orderID string) (*types.Order, error) {
	order, err := s.dao.GetOrderByID(orderID)
	if err != nil {
		s.logger.Error("获取订单信息失败", "order_id", orderID, "error", err)
		return nil, fmt.Errorf("获取订单信息失败: %w", err)
	}
	if order == nil {
		return nil, fmt.Errorf("订单不存在: %s", orderID)
	}

	return s.convertToAPITypes(order), nil
}

// GetUserOrders 获取用户订单列表
func (s *OrderService) GetUserOrders(userID, status string, offset, limit int) ([]*types.Order, int64, error) {
	orders, total, err := s.dao.GetUserOrders(userID, status, offset, limit)
	if err != nil {
		s.logger.Error("获取用户订单失败", "user_id", userID, "status", status, "error", err)
		return nil, 0, fmt.Errorf("获取用户订单失败: %w", err)
	}

	// 转换为API类型
	apiOrders := make([]*types.Order, len(orders))
	for i, order := range orders {
		apiOrders[i] = s.convertToAPITypes(order)
	}

	return apiOrders, total, nil
}

// ProcessPayment 处理支付
func (s *OrderService) ProcessPayment(orderID, transactionID string) (*types.Order, error) {
	order, err := s.dao.GetOrderByID(orderID)
	if err != nil {
		s.logger.Error("获取订单信息失败", "order_id", orderID, "error", err)
		return nil, fmt.Errorf("获取订单信息失败: %w", err)
	}
	if order == nil {
		return nil, fmt.Errorf("订单不存在: %s", orderID)
	}

	if order.Status != "pending" {
		return nil, fmt.Errorf("订单状态不允许支付: %s", order.Status)
	}

	// 更新订单状态
	now := time.Now()
	order.Status = "paid"
	order.PaymentAt = &now
	order.TransactionID = transactionID

	if err := s.dao.UpdateOrderStatus(orderID, "paid"); err != nil {
		s.logger.Error("更新订单状态失败", "order_id", orderID, "error", err)
		return nil, fmt.Errorf("更新订单状态失败: %w", err)
	}

	// TODO: 根据订单类型发放道具或货币
	// 这里应该调用道具服务或玩家服务来发放奖励

	s.logger.Info("订单支付成功", "order_id", orderID, "transaction_id", transactionID, "amount", order.Amount)
	return s.convertToAPITypes(order), nil
}

// CancelOrder 取消订单
func (s *OrderService) CancelOrder(orderID string) (*types.Order, error) {
	order, err := s.dao.GetOrderByID(orderID)
	if err != nil {
		s.logger.Error("获取订单信息失败", "order_id", orderID, "error", err)
		return nil, fmt.Errorf("获取订单信息失败: %w", err)
	}
	if order == nil {
		return nil, fmt.Errorf("订单不存在: %s", orderID)
	}

	if order.Status != "pending" {
		return nil, fmt.Errorf("订单状态不允许取消: %s", order.Status)
	}

	// 更新订单状态
	if err := s.dao.UpdateOrderStatus(orderID, "cancelled"); err != nil {
		s.logger.Error("取消订单失败", "order_id", orderID, "error", err)
		return nil, fmt.Errorf("取消订单失败: %w", err)
	}

	order.Status = "cancelled"
	s.logger.Info("订单取消成功", "order_id", orderID)
	return s.convertToAPITypes(order), nil
}

// RefundOrder 退款订单
func (s *OrderService) RefundOrder(orderID string, refundAmount int64) (*types.Order, error) {
	order, err := s.dao.GetOrderByID(orderID)
	if err != nil {
		s.logger.Error("获取订单信息失败", "order_id", orderID, "error", err)
		return nil, fmt.Errorf("获取订单信息失败: %w", err)
	}
	if order == nil {
		return nil, fmt.Errorf("订单不存在: %s", orderID)
	}

	if order.Status != "paid" {
		return nil, fmt.Errorf("订单状态不允许退款: %s", order.Status)
	}

	if refundAmount > order.Amount {
		return nil, fmt.Errorf("退款金额不能超过订单金额: %d > %d", refundAmount, order.Amount)
	}

	// 更新订单状态
	now := time.Now()
	order.Status = "refunded"
	order.RefundAt = &now
	order.RefundAmount = refundAmount

	// 这里需要更新数据库中的退款信息
	// 由于DAO层没有专门的退款更新方法，这里暂时使用UpdateOrderStatus
	if err := s.dao.UpdateOrderStatus(orderID, "refunded"); err != nil {
		s.logger.Error("退款订单失败", "order_id", orderID, "error", err)
		return nil, fmt.Errorf("退款订单失败: %w", err)
	}

	// TODO: 回收已发放的道具或货币

	s.logger.Info("订单退款成功", "order_id", orderID, "refund_amount", refundAmount)
	return s.convertToAPITypes(order), nil
}

// GetOrdersByStatus 根据状态获取订单
func (s *OrderService) GetOrdersByStatus(status string, offset, limit int) ([]*types.Order, int64, error) {
	orders, total, err := s.dao.GetOrdersByStatus(status, offset, limit)
	if err != nil {
		s.logger.Error("获取订单列表失败", "status", status, "error", err)
		return nil, 0, fmt.Errorf("获取订单列表失败: %w", err)
	}

	// 转换为API类型
	apiOrders := make([]*types.Order, len(orders))
	for i, order := range orders {
		apiOrders[i] = s.convertToAPITypes(order)
	}

	return apiOrders, total, nil
}

// ValidateOrderOwnership 验证订单所有权
func (s *OrderService) ValidateOrderOwnership(orderID, userID string) error {
	order, err := s.dao.GetOrderByID(orderID)
	if err != nil {
		return fmt.Errorf("验证订单所有权失败: %w", err)
	}
	if order == nil {
		return fmt.Errorf("订单不存在: %s", orderID)
	}
	if order.UserID != userID {
		return fmt.Errorf("订单不属于指定用户")
	}
	return nil
}

// CalculateTotalRevenue 计算总营收
func (s *OrderService) CalculateTotalRevenue(gameID string, startDate, endDate time.Time) (int64, error) {
	// 这里需要DAO层支持按时间和游戏ID查询订单统计
	// 暂时返回0
	s.logger.Debug("计算营收", "game_id", gameID, "start", startDate, "end", endDate)
	return 0, nil
}

// GetOrderStatistics 获取订单统计
func (s *OrderService) GetOrderStatistics(gameID string, startDate, endDate time.Time) (*types.OrderStatistics, error) {
	// 这里需要DAO层支持订单统计查询
	// 暂时返回空统计
	s.logger.Debug("获取订单统计", "game_id", gameID, "start", startDate, "end", endDate)

	return &types.OrderStatistics{
		TotalOrders:    0,
		TotalRevenue:   0,
		PaidOrders:     0,
		CancelledOrders: 0,
		RefundedOrders: 0,
	}, nil
}

// Helper methods

func (s *OrderService) generateOrderID() string {
	return fmt.Sprintf("order_%d", time.Now().UnixNano())
}

func (s *OrderService) convertToAPITypes(order *daoPkg.Order) *types.Order {
	return &types.Order{
		OrderID:       order.OrderID,
		UserID:        order.UserID,
		GameID:        order.GameID,
		ProductID:     order.ProductID,
		ProductName:   order.ProductName,
		Amount:        order.Amount,
		Currency:      order.Currency,
		PaymentMethod: order.PaymentMethod,
		Status:        order.Status,
		PaymentAt:     order.PaymentAt,
		RefundAt:      order.RefundAt,
		RefundAmount:  order.RefundAmount,
		TransactionID: order.TransactionID,
		Channel:       order.Channel,
		IP:            order.IP,
		DeviceID:      order.DeviceID,
		CreatedAt:     order.CreatedAt,
		UpdatedAt:     order.UpdatedAt,
	}
}
