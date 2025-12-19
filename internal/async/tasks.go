package async

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"datamiddleware/internal/logger"
)

// LogTask 异步日志任务
type LogTask struct {
	BaseTask
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields"`
}

func NewLogTask(id string, level, message string, fields map[string]interface{}) *LogTask {
	return &LogTask{
		BaseTask: BaseTask{
			ID:       id,
			Type:     "log",
			Priority: 1, // 日志任务优先级较低
		},
		Level:   level,
		Message: message,
		Fields:  fields,
	}
}

func (t *LogTask) Execute(ctx context.Context) error {
	// 模拟异步日志处理
	// 在实际应用中，这里可能写入文件、发送到远程日志系统等

	// 将fields转换为JSON字符串用于记录
	fieldsJSON, _ := json.Marshal(t.Fields)

	fmt.Printf("[ASYNC LOG] %s [%s] %s %s\n",
		time.Now().Format("2006-01-02 15:04:05"),
		t.Level,
		t.Message,
		string(fieldsJSON))

	// 模拟处理时间
	time.Sleep(10 * time.Millisecond)

	return nil
}

// BusinessTask 异步业务任务
type BusinessTask struct {
	BaseTask
	Action   string                              `json:"action"`
	Params   map[string]interface{}              `json:"params"`
	Callback func(result interface{}, err error) `json:"-"` // 回调函数
}

func NewBusinessTask(id, action string, params map[string]interface{}, callback func(result interface{}, err error)) *BusinessTask {
	return &BusinessTask{
		BaseTask: BaseTask{
			ID:       id,
			Type:     "business",
			Priority: 5, // 业务任务优先级中等
		},
		Action:   action,
		Params:   params,
		Callback: callback,
	}
}

func (t *BusinessTask) Execute(ctx context.Context) error {
	// 模拟异步业务处理
	switch t.Action {
	case "user_login":
		return t.handleUserLogin(ctx)
	case "send_notification":
		return t.handleSendNotification(ctx)
	case "data_sync":
		return t.handleDataSync(ctx)
	default:
		return fmt.Errorf("unknown action: %s", t.Action)
	}
}

func (t *BusinessTask) handleUserLogin(ctx context.Context) error {
	userID, ok := t.Params["user_id"].(string)
	if !ok {
		return fmt.Errorf("missing user_id parameter")
	}

	// 模拟登录后处理：更新最后登录时间、发送欢迎消息等
	fmt.Printf("[BUSINESS] 处理用户登录: user_id=%s\n", userID)

	// 模拟异步处理时间
	time.Sleep(50 * time.Millisecond)

	// 调用回调函数
	if t.Callback != nil {
		t.Callback(map[string]interface{}{
			"user_id":    userID,
			"login_time": time.Now(),
			"status":     "success",
		}, nil)
	}

	return nil
}

func (t *BusinessTask) handleSendNotification(ctx context.Context) error {
	userID, ok := t.Params["user_id"].(string)
	if !ok {
		return fmt.Errorf("missing user_id parameter")
	}

	message, _ := t.Params["message"].(string)

	fmt.Printf("[BUSINESS] 发送通知: user_id=%s, message=%s\n", userID, message)

	// 模拟发送通知的时间
	time.Sleep(30 * time.Millisecond)

	if t.Callback != nil {
		t.Callback(map[string]string{
			"status":  "sent",
			"user_id": userID,
		}, nil)
	}

	return nil
}

func (t *BusinessTask) handleDataSync(ctx context.Context) error {
	table, ok := t.Params["table"].(string)
	if !ok {
		return fmt.Errorf("missing table parameter")
	}

	fmt.Printf("[BUSINESS] 数据同步: table=%s\n", table)

	// 模拟数据同步时间
	time.Sleep(100 * time.Millisecond)

	if t.Callback != nil {
		t.Callback(map[string]interface{}{
			"table":     table,
			"sync_time": time.Now(),
			"records":   1000,
		}, nil)
	}

	return nil
}

// CleanupTask 清理任务
type CleanupTask struct {
	BaseTask
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

func NewCleanupTask(id, resourceType, resourceID string) *CleanupTask {
	return &CleanupTask{
		BaseTask: BaseTask{
			ID:       id,
			Type:     "cleanup",
			Priority: 8, // 清理任务优先级较高
		},
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

func (t *CleanupTask) Execute(ctx context.Context) error {
	fmt.Printf("[CLEANUP] 清理资源: type=%s, id=%s\n", t.ResourceType, t.ResourceID)

	// 模拟清理操作
	switch t.ResourceType {
	case "temp_file":
		// 删除临时文件
		time.Sleep(20 * time.Millisecond)
	case "cache_entry":
		// 清理缓存条目
		time.Sleep(10 * time.Millisecond)
	case "session":
		// 清理过期会话
		time.Sleep(15 * time.Millisecond)
	default:
		time.Sleep(25 * time.Millisecond)
	}

	fmt.Printf("[CLEANUP] 资源清理完成: type=%s, id=%s\n", t.ResourceType, t.ResourceID)
	return nil
}

// AsyncManager 异步管理器
type AsyncManager struct {
	scheduler *TaskScheduler
	monitor   *QueueMonitor
	logger    logger.Logger
	running   bool
}

func NewAsyncManager(queueSize, workerCount int, logger logger.Logger) (*AsyncManager, error) {
	// 创建优先级队列
	queue := NewPriorityQueue(queueSize, logger)

	// 创建任务调度器
	scheduler := NewTaskScheduler(queue, workerCount, logger)

	// 创建队列监控器
	monitor := NewQueueMonitor(scheduler, 30*time.Second, logger)

	return &AsyncManager{
		scheduler: scheduler,
		monitor:   monitor,
		logger:    logger,
	}, nil
}

// Start 启动异步管理器
func (m *AsyncManager) Start() error {
	if m.running {
		return fmt.Errorf("async manager is already running")
	}

	// 启动调度器
	if err := m.scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// 启动监控器
	m.monitor.Start()

	m.running = true
	m.logger.Info("异步管理器已启动")
	return nil
}

// Stop 停止异步管理器
func (m *AsyncManager) Stop() error {
	if !m.running {
		return nil
	}

	// 停止监控器
	m.monitor.Stop()

	// 停止调度器
	if err := m.scheduler.Stop(); err != nil {
		return fmt.Errorf("failed to stop scheduler: %w", err)
	}

	m.running = false
	m.logger.Info("异步管理器已停止")
	return nil
}

// SubmitLogTask 提交日志任务
func (m *AsyncManager) SubmitLogTask(level, message string, fields map[string]interface{}) error {
	task := NewLogTask(fmt.Sprintf("log_%d", time.Now().UnixNano()), level, message, fields)
	return m.scheduler.SubmitTask(task)
}

// SubmitBusinessTask 提交业务任务
func (m *AsyncManager) SubmitBusinessTask(action string, params map[string]interface{}, callback func(result interface{}, err error)) error {
	task := NewBusinessTask(fmt.Sprintf("biz_%d", time.Now().UnixNano()), action, params, callback)
	return m.scheduler.SubmitTask(task)
}

// SubmitCleanupTask 提交清理任务
func (m *AsyncManager) SubmitCleanupTask(resourceType, resourceID string) error {
	task := NewCleanupTask(fmt.Sprintf("cleanup_%d", time.Now().UnixNano()), resourceType, resourceID)
	return m.scheduler.SubmitTask(task)
}

// GetStats 获取异步管理器统计信息
func (m *AsyncManager) GetStats() AsyncStats {
	return AsyncStats{
		Scheduler: m.scheduler.GetStats(),
		Running:   m.running,
	}
}

// AsyncStats 异步统计信息
type AsyncStats struct {
	Scheduler SchedulerStats `json:"scheduler"`
	Running   bool           `json:"running"`
}
