package router

import (
	"encoding/json"
	"fmt"
	"sync"

	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// GameHandler 游戏处理器接口
type GameHandler interface {
	// Handle 处理游戏请求
	Handle(gameID string, req *types.Request) (*types.Response, error)

	// GetSupportedMessageTypes 获取支持的消息类型
	GetSupportedMessageTypes() []types.MessageType

	// GetName 获取处理器名称
	GetName() string
}

// Router 路由器
type Router struct {
	handlers map[string]GameHandler // gameID -> handler
	mu       sync.RWMutex
	logger   logger.Logger
}

// NewRouter 创建路由器
func NewRouter(log logger.Logger) *Router {
	return &Router{
		handlers: make(map[string]GameHandler),
		logger:   log,
	}
}

// RegisterHandler 注册游戏处理器
func (r *Router) RegisterHandler(gameID string, handler GameHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[gameID]; exists {
		return fmt.Errorf("游戏处理器已存在: %s", gameID)
	}

	r.handlers[gameID] = handler
	r.logger.Info("游戏处理器已注册", "game_id", gameID, "handler", handler.GetName())
	return nil
}

// UnregisterHandler 注销游戏处理器
func (r *Router) UnregisterHandler(gameID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if handler, exists := r.handlers[gameID]; exists {
		delete(r.handlers, gameID)
		r.logger.Info("游戏处理器已注销", "game_id", gameID, "handler", handler.GetName())
	}
}

// Route 路由请求
func (r *Router) Route(req *types.Request) (*types.Response, error) {
	r.mu.RLock()
	handler, exists := r.handlers[req.GameID]
	r.mu.RUnlock()

	if !exists {
		r.logger.Warn("未找到游戏处理器", "game_id", req.GameID, "user_id", req.UserID)
		return &types.Response{
			ID:        req.ID,
			Code:      4004, // 游戏不存在
			Message:   "游戏不存在或未启用",
			Timestamp: req.Timestamp,
		}, nil
	}

	// 检查处理器是否支持此消息类型
	if !r.supportsMessageType(handler, req.Type) {
		r.logger.Warn("处理器不支持消息类型",
			"game_id", req.GameID,
			"handler", handler.GetName(),
			"message_type", req.Type)

		return &types.Response{
			ID:        req.ID,
			Code:      4005, // 不支持的消息类型
			Message:   "处理器不支持此消息类型",
			Timestamp: req.Timestamp,
		}, nil
	}

	// 处理请求
	r.logger.Debug("路由请求到处理器",
		"game_id", req.GameID,
		"handler", handler.GetName(),
		"message_type", req.Type,
		"user_id", req.UserID)

	return handler.Handle(req.GameID, req)
}

// GetRegisteredGames 获取已注册的游戏列表
func (r *Router) GetRegisteredGames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	games := make([]string, 0, len(r.handlers))
	for gameID := range r.handlers {
		games = append(games, gameID)
	}
	return games
}

// GetHandler 获取游戏处理器
func (r *Router) GetHandler(gameID string) (GameHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[gameID]
	return handler, exists
}

// supportsMessageType 检查处理器是否支持消息类型
func (r *Router) supportsMessageType(handler GameHandler, msgType types.MessageType) bool {
	supportedTypes := handler.GetSupportedMessageTypes()
	for _, t := range supportedTypes {
		if t == msgType {
			return true
		}
	}
	return false
}

// TCPMessageHandler TCP消息处理器接口
type TCPMessageHandler interface {
	// HandleTCPMessage 处理TCP消息
	HandleTCPMessage(connID string, msg *types.Message) (*types.Message, error)
}

// HTTPRequestHandler HTTP请求处理器接口
type HTTPRequestHandler interface {
	// HandleHTTPRequest 处理HTTP请求
	HandleHTTPRequest(c interface{}) // c 是Gin的Context，这里用interface{}避免导入依赖
}

// MessageRouter 消息路由器（结合TCP和HTTP路由）
type MessageRouter struct {
	gameRouter *Router
	tcpRouter  TCPMessageHandler
	httpRouter HTTPRequestHandler
	logger     logger.Logger
}

// NewMessageRouter 创建消息路由器
func NewMessageRouter(log logger.Logger) *MessageRouter {
	return &MessageRouter{
		gameRouter: NewRouter(log),
		logger:     log,
	}
}

// SetTCPRouter 设置TCP路由器
func (mr *MessageRouter) SetTCPRouter(router TCPMessageHandler) {
	mr.tcpRouter = router
}

// SetHTTPRouter 设置HTTP路由器
func (mr *MessageRouter) SetHTTPRouter(router HTTPRequestHandler) {
	mr.httpRouter = router
}

// GetGameRouter 获取游戏路由器
func (mr *MessageRouter) GetGameRouter() *Router {
	return mr.gameRouter
}

// RegisterGameHandler 注册游戏处理器
func (mr *MessageRouter) RegisterGameHandler(gameID string, handler GameHandler) error {
	return mr.gameRouter.RegisterHandler(gameID, handler)
}

// RouteTCPMessage 路由TCP消息
func (mr *MessageRouter) RouteTCPMessage(connID string, msg *types.Message) (*types.Message, error) {
	if mr.tcpRouter != nil {
		return mr.tcpRouter.HandleTCPMessage(connID, msg)
	}

	// 默认处理：转换为业务请求并路由
	req := &types.Request{
		ID:      fmt.Sprintf("%s_%d", connID, msg.Header.SequenceID),
		Type:    msg.Header.Type,
		GameID:  msg.Header.GameID,
		UserID:  msg.Header.UserID,
		Data:    msg.Body,
	}

	resp, err := mr.gameRouter.Route(req)
	if err != nil {
		return nil, err
	}

	// 转换为TCP响应消息
	return mr.createTCPResponse(msg, resp), nil
}

// createTCPResponse 创建TCP响应消息
func (mr *MessageRouter) createTCPResponse(reqMsg *types.Message, resp *types.Response) *types.Message {
	respMsg := &types.Message{
		Header: types.MessageHeader{
			Version:     reqMsg.Header.Version,
			Type:        reqMsg.Header.Type, // 响应类型与请求类型相同
			Flags:       types.FlagNone,
			SequenceID:  reqMsg.Header.SequenceID,
			GameID:      reqMsg.Header.GameID,
			UserID:      reqMsg.Header.UserID,
			Timestamp:   resp.Timestamp,
			BodyLength:  0, // 稍后计算
		},
	}

	// 序列化响应数据
	respData := map[string]interface{}{
		"id":        resp.ID,
		"code":      resp.Code,
		"message":   resp.Message,
		"data":      resp.Data,
		"timestamp": resp.Timestamp,
	}
	bodyData, _ := json.Marshal(respData)
	respMsg.Body = bodyData
	respMsg.Header.BodyLength = uint32(len(bodyData))

	return respMsg
}
