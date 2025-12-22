package monitor

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HTTPHandler 监控HTTP处理器
type HTTPHandler struct {
	monitor *Monitor
}

// NewHTTPHandler 创建HTTP处理器
func NewHTTPHandler(monitor *Monitor) *HTTPHandler {
	return &HTTPHandler{
		monitor: monitor,
	}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(engine *gin.Engine) {
	// 基础健康检查
	engine.GET("/health", h.healthCheck)
	engine.GET("/api/v1/health", h.healthCheck)

	// 详细健康检查
	engine.GET("/health/detailed", h.detailedHealthCheck)
	engine.GET("/api/v1/health/detailed", h.detailedHealthCheck)

	// 系统指标
	engine.GET("/metrics", h.systemMetrics)
	engine.GET("/api/v1/metrics", h.systemMetrics)

	// 组件健康状态
	engine.GET("/health/components", h.componentHealth)
	engine.GET("/api/v1/health/components", h.componentHealth)
}

// healthCheck 基础健康检查
func (h *HTTPHandler) healthCheck(c *gin.Context) {
	metrics := h.monitor.GetSystemMetrics()

	// 检查是否有严重问题
	hasCriticalIssues := false
	for _, component := range metrics.Components {
		if component.Status == "unhealthy" {
			hasCriticalIssues = true
			break
		}
	}

	status := "ok"
	if hasCriticalIssues {
		status = "warning"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"uptime":    metrics.Uptime,
	})
}

// detailedHealthCheck 详细健康检查
func (h *HTTPHandler) detailedHealthCheck(c *gin.Context) {
	metrics := h.monitor.GetSystemMetrics()

	// 整体健康状态
	overallStatus := "healthy"
	unhealthyCount := 0

	for _, component := range metrics.Components {
		if component.Status == "unhealthy" {
			unhealthyCount++
			overallStatus = "unhealthy"
		} else if component.Status == "unknown" && overallStatus == "healthy" {
			overallStatus = "warning"
		}
	}

	response := gin.H{
		"status":    overallStatus,
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"uptime":    metrics.Uptime,
		"system_metrics": gin.H{
			"total_requests":    metrics.TotalRequests,
			"active_requests":   metrics.ActiveRequests,
			"failed_requests":   metrics.FailedRequests,
			"avg_response_time": metrics.AvgResponseTime.String(),
			"goroutines":        metrics.Goroutines,
		},
		"memory": gin.H{
			"alloc_mb":       float64(metrics.Memory.Alloc) / 1024 / 1024,
			"total_alloc_mb": float64(metrics.Memory.TotalAlloc) / 1024 / 1024,
			"sys_mb":         float64(metrics.Memory.Sys) / 1024 / 1024,
			"heap_alloc_mb":  float64(metrics.Memory.HeapAlloc) / 1024 / 1024,
			"heap_sys_mb":    float64(metrics.Memory.HeapSys) / 1024 / 1024,
			"heap_idle_mb":   float64(metrics.Memory.HeapIdle) / 1024 / 1024,
			"heap_inuse_mb":  float64(metrics.Memory.HeapInuse) / 1024 / 1024,
			"heap_objects":   metrics.Memory.HeapObjects,
			"num_gc":         metrics.Memory.NumGC,
		},
		"components":      metrics.Components,
		"unhealthy_count": unhealthyCount,
	}

	c.JSON(http.StatusOK, response)
}

// systemMetrics 系统指标
func (h *HTTPHandler) systemMetrics(c *gin.Context) {
	metrics := h.monitor.GetSystemMetrics()
	customMetrics := h.monitor.GetAllCustomMetrics()

	response := gin.H{
		"timestamp": time.Now().Unix(),
		"system": gin.H{
			"uptime_seconds": metrics.Uptime,
			"goroutines":     metrics.Goroutines,
			"memory": gin.H{
				"alloc_bytes":       metrics.Memory.Alloc,
				"total_alloc_bytes": metrics.Memory.TotalAlloc,
				"sys_bytes":         metrics.Memory.Sys,
				"heap_alloc_bytes":  metrics.Memory.HeapAlloc,
				"heap_sys_bytes":    metrics.Memory.HeapSys,
				"heap_idle_bytes":   metrics.Memory.HeapIdle,
				"heap_inuse_bytes":  metrics.Memory.HeapInuse,
				"heap_objects":      metrics.Memory.HeapObjects,
				"num_gc":            metrics.Memory.NumGC,
			},
		},
		"requests": gin.H{
			"total":                metrics.TotalRequests,
			"active":               metrics.ActiveRequests,
			"failed":               metrics.FailedRequests,
			"avg_response_time_ns": metrics.AvgResponseTime.Nanoseconds(),
			"avg_response_time_ms": float64(metrics.AvgResponseTime.Nanoseconds()) / 1000000,
		},
		"components": metrics.Components,
		"custom":     customMetrics,
	}

	c.JSON(http.StatusOK, response)
}

// componentHealth 组件健康状态
func (h *HTTPHandler) componentHealth(c *gin.Context) {
	metrics := h.monitor.GetSystemMetrics()

	// 支持查询参数过滤
	component := c.Query("component")
	if component != "" {
		if status, exists := metrics.Components[component]; exists {
			c.JSON(http.StatusOK, gin.H{
				"component": component,
				"status":    status,
			})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "组件不存在",
			"component": component,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"timestamp":  time.Now().Unix(),
		"components": metrics.Components,
	})
}

// PrometheusMetrics Prometheus格式的指标
func (h *HTTPHandler) PrometheusMetrics() string {
	metrics := h.monitor.GetSystemMetrics()

	var prometheusOutput string

	// 基础指标
	prometheusOutput += "# HELP datamiddleware_uptime_seconds 服务运行时间(秒)\n"
	prometheusOutput += "# TYPE datamiddleware_uptime_seconds gauge\n"
	prometheusOutput += "datamiddleware_uptime_seconds " + strconv.FormatInt(metrics.Uptime, 10) + "\n\n"

	prometheusOutput += "# HELP datamiddleware_goroutines 当前goroutine数量\n"
	prometheusOutput += "# TYPE datamiddleware_goroutines gauge\n"
	prometheusOutput += "datamiddleware_goroutines " + strconv.Itoa(metrics.Goroutines) + "\n\n"

	prometheusOutput += "# HELP datamiddleware_requests_total 总请求数\n"
	prometheusOutput += "# TYPE datamiddleware_requests_total counter\n"
	prometheusOutput += "datamiddleware_requests_total " + strconv.FormatInt(metrics.TotalRequests, 10) + "\n\n"

	prometheusOutput += "# HELP datamiddleware_requests_active 当前活跃请求数\n"
	prometheusOutput += "# TYPE datamiddleware_requests_active gauge\n"
	prometheusOutput += "datamiddleware_requests_active " + strconv.FormatInt(metrics.ActiveRequests, 10) + "\n\n"

	prometheusOutput += "# HELP datamiddleware_requests_failed 失败请求数\n"
	prometheusOutput += "# TYPE datamiddleware_requests_failed counter\n"
	prometheusOutput += "datamiddleware_requests_failed " + strconv.FormatInt(metrics.FailedRequests, 10) + "\n\n"

	// 内存指标
	prometheusOutput += "# HELP datamiddleware_memory_alloc_bytes 当前已分配内存(字节)\n"
	prometheusOutput += "# TYPE datamiddleware_memory_alloc_bytes gauge\n"
	prometheusOutput += "datamiddleware_memory_alloc_bytes " + strconv.FormatUint(metrics.Memory.Alloc, 10) + "\n\n"

	prometheusOutput += "# HELP datamiddleware_memory_heap_alloc_bytes 堆内存分配(字节)\n"
	prometheusOutput += "# TYPE datamiddleware_memory_heap_alloc_bytes gauge\n"
	prometheusOutput += "datamiddleware_memory_heap_alloc_bytes " + strconv.FormatUint(metrics.Memory.HeapAlloc, 10) + "\n\n"

	prometheusOutput += "# HELP datamiddleware_memory_sys_bytes 系统内存(字节)\n"
	prometheusOutput += "# TYPE datamiddleware_memory_sys_bytes gauge\n"
	prometheusOutput += "datamiddleware_memory_sys_bytes " + strconv.FormatUint(metrics.Memory.Sys, 10) + "\n\n"

	// 组件健康状态
	for name, status := range metrics.Components {
		healthValue := "0"
		if status.Status == "healthy" {
			healthValue = "1"
		}

		prometheusOutput += "# HELP datamiddleware_component_health 组件健康状态(1=健康,0=不健康)\n"
		prometheusOutput += "# TYPE datamiddleware_component_health gauge\n"
		prometheusOutput += "datamiddleware_component_health{component=\"" + name + "\"} " + healthValue + "\n"

		prometheusOutput += "# HELP datamiddleware_component_response_time_ms 组件响应时间(毫秒)\n"
		prometheusOutput += "# TYPE datamiddleware_component_response_time_ms gauge\n"
		prometheusOutput += "datamiddleware_component_response_time_ms{component=\"" + name + "\"} " + strconv.FormatInt(status.Response, 10) + "\n\n"
	}

	return prometheusOutput
}
