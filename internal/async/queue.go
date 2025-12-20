package async

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"datamiddleware/internal/logger"
)

// Task 异步任务接口
type Task interface {
	// Execute 执行任务
	Execute(ctx context.Context) error
	// GetID 获取任务ID
	GetID() string
	// GetType 获取任务类型
	GetType() string
	// GetPriority 获取任务优先级 (0-100, 越高优先级越大)
	GetPriority() int
}

// BaseTask 基础任务实现
type BaseTask struct {
	ID       string
	Type     string
	Priority int
	Data     interface{}
}

// Execute 执行任务
func (t *BaseTask) Execute(ctx context.Context) error {
	// 基础任务实现，打印任务信息
	fmt.Printf("执行任务: ID=%s, Type=%s, Priority=%d, Data=%v\n", t.ID, t.Type, t.Priority, t.Data)
	return nil
}

func (t *BaseTask) GetID() string {
	return t.ID
}

func (t *BaseTask) GetType() string {
	return t.Type
}

func (t *BaseTask) GetPriority() int {
	return t.Priority
}

// Queue 异步队列接口
type Queue interface {
	// Enqueue 添加任务到队列
	Enqueue(task Task) error
	// Dequeue 从队列获取任务
	Dequeue() (Task, error)
	// Size 获取队列大小
	Size() int
	// IsEmpty 检查队列是否为空
	IsEmpty() bool
	// Close 关闭队列
	Close() error
}

// PriorityQueue 优先级队列实现
type PriorityQueue struct {
	tasks   []Task
	mu      sync.RWMutex
	maxSize int
	logger  logger.Logger
}

func NewPriorityQueue(maxSize int, logger logger.Logger) *PriorityQueue {
	return &PriorityQueue{
		tasks:   make([]Task, 0, maxSize),
		maxSize: maxSize,
		logger:  logger,
	}
}

// Enqueue 添加任务到队列（按优先级插入）
func (q *PriorityQueue) Enqueue(task Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) >= q.maxSize {
		return fmt.Errorf("queue is full, max size: %d", q.maxSize)
	}

	// 找到插入位置（优先级高的在前面）
	insertIndex := len(q.tasks)
	for i, t := range q.tasks {
		if task.GetPriority() > t.GetPriority() {
			insertIndex = i
			break
		}
	}

	// 插入任务
	q.tasks = append(q.tasks, nil)
	copy(q.tasks[insertIndex+1:], q.tasks[insertIndex:])
	q.tasks[insertIndex] = task

	q.logger.Debug("任务已入队", "task_id", task.GetID(), "type", task.GetType(), "priority", task.GetPriority(), "queue_size", len(q.tasks))
	return nil
}

// Dequeue 从队列获取任务（获取最高优先级的任务）
func (q *PriorityQueue) Dequeue() (Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return nil, fmt.Errorf("queue is empty")
	}

	task := q.tasks[0]
	q.tasks = q.tasks[1:]

	q.logger.Debug("任务已出队", "task_id", task.GetID(), "type", task.GetType(), "remaining", len(q.tasks))
	return task, nil
}

// Size 获取队列大小
func (q *PriorityQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks)
}

// IsEmpty 检查队列是否为空
func (q *PriorityQueue) IsEmpty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks) == 0
}

// Close 关闭队列
func (q *PriorityQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tasks = nil
	q.logger.Info("优先级队列已关闭")
	return nil
}

// Worker 工作协程
type Worker struct {
	id       int
	queue    Queue
	logger   logger.Logger
	stopChan chan struct{}
	running  int32
}

func NewWorker(id int, queue Queue, logger logger.Logger) *Worker {
	return &Worker{
		id:       id,
		queue:    queue,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Start 启动工作协程
func (w *Worker) Start() {
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		return // 已经在运行
	}

	go w.run()
	w.logger.Info("工作协程已启动", "worker_id", w.id)
}

// Stop 停止工作协程
func (w *Worker) Stop() {
	if !atomic.CompareAndSwapInt32(&w.running, 1, 0) {
		return // 没有在运行
	}

	close(w.stopChan)
	w.logger.Info("工作协程已停止", "worker_id", w.id)
}

// IsRunning 检查工作协程是否在运行
func (w *Worker) IsRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

func (w *Worker) run() {
	ticker := time.NewTicker(100 * time.Millisecond) // 每100ms检查一次队列
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			w.logger.Debug("工作协程接收到停止信号", "worker_id", w.id)
			return
		case <-ticker.C:
			// 尝试获取任务
			if task, err := w.queue.Dequeue(); err == nil {
				// 执行任务
				ctx := context.Background()
				startTime := time.Now()

				func() {
					defer func() {
						if r := recover(); r != nil {
							w.logger.Error("任务执行发生panic", "task_id", task.GetID(), "panic", r)
						}
					}()

					if err := task.Execute(ctx); err != nil {
						w.logger.Error("任务执行失败", "task_id", task.GetID(), "error", err)
					} else {
						duration := time.Since(startTime)
						w.logger.Debug("任务执行成功", "task_id", task.GetID(), "duration", duration)
					}
				}()
			}
		}
	}
}

// TaskScheduler 任务调度器
type TaskScheduler struct {
	queue    Queue
	workers  []*Worker
	logger   logger.Logger
	stopChan chan struct{}
	running  int32
	mu       sync.RWMutex
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler(queue Queue, workerCount int, logger logger.Logger) *TaskScheduler {
	workers := make([]*Worker, workerCount)
	for i := 0; i < workerCount; i++ {
		workers[i] = NewWorker(i+1, queue, logger)
	}

	return &TaskScheduler{
		queue:    queue,
		workers:  workers,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Start 启动调度器
func (s *TaskScheduler) Start() error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("scheduler is already running")
	}

	// 启动所有工作协程
	for _, worker := range s.workers {
		worker.Start()
	}

	s.logger.Info("任务调度器已启动", "worker_count", len(s.workers))
	return nil
}

// Stop 停止调度器
func (s *TaskScheduler) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return nil // 没有在运行
	}

	close(s.stopChan)

	// 停止所有工作协程
	for _, worker := range s.workers {
		worker.Stop()
	}

	s.logger.Info("任务调度器已停止")
	return nil
}

// SubmitTask 提交任务
func (s *TaskScheduler) SubmitTask(task Task) error {
	return s.queue.Enqueue(task)
}

// GetStats 获取调度器统计信息
func (s *TaskScheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SchedulerStats{
		Running:     atomic.LoadInt32(&s.running) == 1,
		WorkerCount: len(s.workers),
		QueueSize:   s.queue.Size(),
	}

	runningWorkers := 0
	for _, worker := range s.workers {
		if worker.IsRunning() {
			runningWorkers++
		}
	}
	stats.RunningWorkers = runningWorkers

	return stats
}

// SchedulerStats 调度器统计信息
type SchedulerStats struct {
	Running        bool `json:"running"`
	WorkerCount    int  `json:"worker_count"`
	RunningWorkers int  `json:"running_workers"`
	QueueSize      int  `json:"queue_size"`
}

// QueueMonitor 队列监控器
type QueueMonitor struct {
	scheduler *TaskScheduler
	logger    logger.Logger
	interval  time.Duration
	stopChan  chan struct{}
	running   int32
}

// NewQueueMonitor 创建队列监控器
func NewQueueMonitor(scheduler *TaskScheduler, interval time.Duration, logger logger.Logger) *QueueMonitor {
	return &QueueMonitor{
		scheduler: scheduler,
		logger:    logger,
		interval:  interval,
		stopChan:  make(chan struct{}),
	}
}

// Start 启动监控
func (m *QueueMonitor) Start() {
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return
	}

	go m.monitor()
	m.logger.Info("队列监控器已启动", "interval", m.interval)
}

// Stop 停止监控
func (m *QueueMonitor) Stop() {
	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		return
	}

	close(m.stopChan)
	m.logger.Info("队列监控器已停止")
}

func (m *QueueMonitor) monitor() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			stats := m.scheduler.GetStats()

			// 记录监控信息
			m.logger.Info("队列监控",
				"running", stats.Running,
				"workers", stats.WorkerCount,
				"running_workers", stats.RunningWorkers,
				"queue_size", stats.QueueSize)

			// 预警检查
			if stats.QueueSize > 1000 {
				m.logger.Warn("队列积压严重", "queue_size", stats.QueueSize)
			}

			if stats.RunningWorkers < stats.WorkerCount/2 {
				m.logger.Warn("工作协程不足", "running", stats.RunningWorkers, "total", stats.WorkerCount)
			}
		}
	}
}
