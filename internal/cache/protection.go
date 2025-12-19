package cache

import (
	"fmt"
	"sync"
	"time"

	"datamiddleware/internal/logger"
	"datamiddleware/pkg/types"
)

// Protection 缓存防护器
type Protection struct {
	manager       *Manager
	logger        logger.Logger

	// 缓存穿透防护
	penetrationProtection *PenetrationProtection

	// 缓存雪崩防护
	avalancheProtection *AvalancheProtection

	// 缓存监控
	monitor *CacheMonitor
}

// NewProtection 创建缓存防护器
func NewProtection(manager *Manager, logger logger.Logger) *Protection {
	return &Protection{
		manager: manager,
		logger:  logger,
		penetrationProtection: NewPenetrationProtection(logger),
		avalancheProtection:   NewAvalancheProtection(logger),
		monitor:               NewCacheMonitor(logger),
	}
}

// GetWithProtection 带防护的缓存获取
func (p *Protection) GetWithProtection(key string) ([]byte, error) {
	// 记录监控数据
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		p.monitor.RecordGet(key, duration)
	}()

	// 缓存穿透防护：检查是否是已知的空值
	if p.penetrationProtection.IsBlockedKey(key) {
		p.logger.Debug("缓存穿透防护：阻止访问已知空值键", "key", key)
		return nil, types.ErrCacheMiss
	}

	// 缓存雪崩防护：检查是否需要熔断
	if p.avalancheProtection.ShouldBlock() {
		p.logger.Warn("缓存雪崩防护：触发熔断", "key", key)
		return nil, fmt.Errorf("cache avalanche protection activated")
	}

	// 正常获取缓存
	value, err := p.manager.Get(key)
	if err != nil {
		if err == types.ErrCacheMiss {
			// 记录空值，防止缓存穿透
			p.penetrationProtection.RecordMiss(key)
		}
		return nil, err
	}

	// 缓存雪崩防护：记录成功访问
	p.avalancheProtection.RecordSuccess()

	return value, nil
}

// SetWithProtection 带防护的缓存设置
func (p *Protection) SetWithProtection(key string, value []byte) error {
	// 记录监控数据
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		p.monitor.RecordSet(key, duration)
	}()

	// 清除穿透防护记录（因为现在有值了）
	p.penetrationProtection.ClearBlockedKey(key)

	// 正常设置缓存
	return p.manager.Set(key, value)
}

// GetStats 获取防护统计信息
func (p *Protection) GetStats() ProtectionStats {
	return ProtectionStats{
		PenetrationProtection: p.penetrationProtection.GetStats(),
		AvalancheProtection:   p.avalancheProtection.GetStats(),
		Monitor:               p.monitor.GetStats(),
	}
}

// ProtectionStats 防护统计信息
type ProtectionStats struct {
	PenetrationProtection PenetrationStats `json:"penetration_protection"`
	AvalancheProtection   AvalancheStats   `json:"avalanche_protection"`
	Monitor               MonitorStats     `json:"monitor"`
}

// PenetrationProtection 缓存穿透防护
type PenetrationProtection struct {
	blockedKeys map[string]time.Time
	mu          sync.RWMutex
	logger      logger.Logger

	// 配置
	blockDuration time.Duration // 阻止访问持续时间
	maxBlockedKeys int          // 最大阻止键数量
}

func NewPenetrationProtection(logger logger.Logger) *PenetrationProtection {
	return &PenetrationProtection{
		blockedKeys:    make(map[string]time.Time),
		logger:         logger,
		blockDuration:  5 * time.Minute,   // 默认5分钟
		maxBlockedKeys: 10000,             // 默认最多阻止1万个键
	}
}

// IsBlockedKey 检查键是否被阻止
func (p *PenetrationProtection) IsBlockedKey(key string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	blockTime, exists := p.blockedKeys[key]
	if !exists {
		return false
	}

	// 检查是否过期
	if time.Since(blockTime) > p.blockDuration {
		// 过期了，删除记录
		delete(p.blockedKeys, key)
		return false
	}

	return true
}

// RecordMiss 记录缓存未命中
func (p *PenetrationProtection) RecordMiss(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查是否达到最大限制
	if len(p.blockedKeys) >= p.maxBlockedKeys {
		p.logger.Warn("缓存穿透防护：达到最大阻止键数量限制", "max_keys", p.maxBlockedKeys)
		return
	}

	p.blockedKeys[key] = time.Now()
	p.logger.Debug("缓存穿透防护：记录空值键", "key", key)
}

// ClearBlockedKey 清除阻止的键
func (p *PenetrationProtection) ClearBlockedKey(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.blockedKeys, key)
}

// GetStats 获取统计信息
func (p *PenetrationProtection) GetStats() PenetrationStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 清理过期键
	validKeys := 0
	for key, blockTime := range p.blockedKeys {
		if time.Since(blockTime) <= p.blockDuration {
			validKeys++
		} else {
			delete(p.blockedKeys, key)
		}
	}

	return PenetrationStats{
		BlockedKeysCount: validKeys,
		BlockDuration:    p.blockDuration,
	}
}

// PenetrationStats 穿透防护统计
type PenetrationStats struct {
	BlockedKeysCount int           `json:"blocked_keys_count"`
	BlockDuration    time.Duration `json:"block_duration"`
}

// AvalancheProtection 缓存雪崩防护
type AvalancheProtection struct {
	mu             sync.RWMutex
	logger         logger.Logger

	// 统计数据
	totalRequests   int64
	failedRequests  int64
	lastFailureTime time.Time

	// 配置
	failureThreshold float64       // 失败率阈值 (0.0-1.0)
	minRequests      int64         // 最少请求数
	blockDuration    time.Duration // 熔断持续时间
}

func NewAvalancheProtection(logger logger.Logger) *AvalancheProtection {
	return &AvalancheProtection{
		logger:           logger,
		failureThreshold: 0.5,          // 50%失败率
		minRequests:      10,           // 最少10个请求
		blockDuration:    time.Minute,  // 熔断1分钟
	}
}

// ShouldBlock 检查是否应该熔断
func (p *AvalancheProtection) ShouldBlock() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 检查是否在熔断期内
	if time.Since(p.lastFailureTime) < p.blockDuration {
		return true
	}

	// 检查失败率
	if p.totalRequests < p.minRequests {
		return false
	}

	failureRate := float64(p.failedRequests) / float64(p.totalRequests)
	return failureRate > p.failureThreshold
}

// RecordSuccess 记录成功请求
func (p *AvalancheProtection) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalRequests++

	// 重置失败统计（如果成功了）
	if p.failedRequests > 0 {
		p.failedRequests = 0
		p.lastFailureTime = time.Time{} // 重置熔断时间
	}
}

// RecordFailure 记录失败请求
func (p *AvalancheProtection) RecordFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalRequests++
	p.failedRequests++
	p.lastFailureTime = time.Now()
}

// GetStats 获取统计信息
func (p *AvalancheProtection) GetStats() AvalancheStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var failureRate float64
	if p.totalRequests > 0 {
		failureRate = float64(p.failedRequests) / float64(p.totalRequests)
	}

	return AvalancheStats{
		TotalRequests:  p.totalRequests,
		FailedRequests: p.failedRequests,
		FailureRate:    failureRate,
		IsBlocked:      p.ShouldBlock(),
		BlockDuration:  p.blockDuration,
	}
}

// AvalancheStats 雪崩防护统计
type AvalancheStats struct {
	TotalRequests  int64         `json:"total_requests"`
	FailedRequests int64         `json:"failed_requests"`
	FailureRate    float64       `json:"failure_rate"`
	IsBlocked      bool          `json:"is_blocked"`
	BlockDuration  time.Duration `json:"block_duration"`
}

// CacheMonitor 缓存监控器
type CacheMonitor struct {
	mu     sync.RWMutex
	logger logger.Logger

	// 统计数据
	getCount      int64
	setCount      int64
	hitCount      int64
	missCount     int64
	totalGetTime  time.Duration
	totalSetTime  time.Duration
	errors        int64

	// 最近的慢查询
	slowQueries []SlowQuery
}

type SlowQuery struct {
	Key      string
	Operation string
	Duration time.Duration
	Time     time.Time
}

func NewCacheMonitor(logger logger.Logger) *CacheMonitor {
	return &CacheMonitor{
		logger:      logger,
		slowQueries: make([]SlowQuery, 0, 100), // 保留最近100个慢查询
	}
}

// RecordGet 记录Get操作
func (m *CacheMonitor) RecordGet(key string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getCount++
	m.totalGetTime += duration

	// 记录慢查询 (超过100ms)
	if duration > 100*time.Millisecond {
		m.recordSlowQuery("GET", key, duration)
	}
}

// RecordSet 记录Set操作
func (m *CacheMonitor) RecordSet(key string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.setCount++
	m.totalSetTime += duration

	// 记录慢查询 (超过50ms)
	if duration > 50*time.Millisecond {
		m.recordSlowQuery("SET", key, duration)
	}
}

// RecordHit 记录缓存命中
func (m *CacheMonitor) RecordHit() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hitCount++
}

// RecordMiss 记录缓存未命中
func (m *CacheMonitor) RecordMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.missCount++
}

// RecordError 记录错误
func (m *CacheMonitor) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errors++
}

func (m *CacheMonitor) recordSlowQuery(operation, key string, duration time.Duration) {
	query := SlowQuery{
		Key:       key,
		Operation: operation,
		Duration:  duration,
		Time:      time.Now(),
	}

	m.slowQueries = append(m.slowQueries, query)

	// 保持最近100个
	if len(m.slowQueries) > 100 {
		m.slowQueries = m.slowQueries[1:]
	}

	m.logger.Warn("缓存慢查询", "operation", operation, "key", key, "duration", duration)
}

// GetStats 获取监控统计
func (m *CacheMonitor) GetStats() MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var avgGetTime, avgSetTime time.Duration
	if m.getCount > 0 {
		avgGetTime = m.totalGetTime / time.Duration(m.getCount)
	}
	if m.setCount > 0 {
		avgSetTime = m.totalSetTime / time.Duration(m.setCount)
	}

	var hitRate float64
	totalRequests := m.hitCount + m.missCount
	if totalRequests > 0 {
		hitRate = float64(m.hitCount) / float64(totalRequests)
	}

	return MonitorStats{
		GetCount:     m.getCount,
		SetCount:     m.setCount,
		HitCount:     m.hitCount,
		MissCount:    m.missCount,
		HitRate:      hitRate,
		AvgGetTime:   avgGetTime,
		AvgSetTime:   avgSetTime,
		ErrorCount:   m.errors,
		SlowQueries:  len(m.slowQueries),
	}
}

// MonitorStats 监控统计信息
type MonitorStats struct {
	GetCount    int64         `json:"get_count"`
	SetCount    int64         `json:"set_count"`
	HitCount    int64         `json:"hit_count"`
	MissCount   int64         `json:"miss_count"`
	HitRate     float64       `json:"hit_rate"`
	AvgGetTime  time.Duration `json:"avg_get_time"`
	AvgSetTime  time.Duration `json:"avg_set_time"`
	ErrorCount  int64         `json:"error_count"`
	SlowQueries int           `json:"slow_queries"`
}