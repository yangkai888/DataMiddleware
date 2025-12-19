package errors

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"datamiddleware/internal/logger"
	"datamiddleware/pkg/constants"
)

// BusinessError 业务错误
type BusinessError struct {
	Code       int                    `json:"code"`              // 错误码
	Message    string                 `json:"message"`           // 错误信息
	Details    map[string]interface{} `json:"details,omitempty"` // 详细信息
	Cause      error                  `json:"-"`                 // 原始错误
	HTTPStatus int                    `json:"-"`                 // HTTP状态码
}

// Error 实现error接口
func (e *BusinessError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("code=%d, message=%s, cause=%v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("code=%d, message=%s", e.Code, e.Message)
}

// Unwrap 实现errors.Unwrap接口
func (e *BusinessError) Unwrap() error {
	return e.Cause
}

// New 创建新的业务错误
func New(code int, message string) *BusinessError {
	return &BusinessError{
		Code:       code,
		Message:    message,
		HTTPStatus: getHTTPStatus(code),
	}
}

// NewWithCause 创建带有原因的业务错误
func NewWithCause(code int, message string, cause error) *BusinessError {
	return &BusinessError{
		Code:       code,
		Message:    message,
		Cause:      cause,
		HTTPStatus: getHTTPStatus(code),
	}
}

// NewWithDetails 创建带有详细信息的业务错误
func NewWithDetails(code int, message string, details map[string]interface{}) *BusinessError {
	return &BusinessError{
		Code:       code,
		Message:    message,
		Details:    details,
		HTTPStatus: getHTTPStatus(code),
	}
}

// Wrap 包装现有错误
func Wrap(err error, code int, message string) *BusinessError {
	if err == nil {
		return nil
	}

	// 如果已经是BusinessError，直接返回
	if bizErr, ok := err.(*BusinessError); ok {
		return bizErr
	}

	return &BusinessError{
		Code:       code,
		Message:    message,
		Cause:      err,
		HTTPStatus: getHTTPStatus(code),
	}
}

// getHTTPStatus 根据错误码获取HTTP状态码
func getHTTPStatus(code int) int {
	// 根据错误码前缀判断HTTP状态码
	switch {
	case code == 0: // 成功
		return http.StatusOK
	case code >= 1000 && code < 1100: // 系统级错误
		switch code {
		case constants.ErrCodeUnauthorized:
			return http.StatusUnauthorized
		case constants.ErrCodePermissionDenied:
			return http.StatusForbidden
		case constants.ErrCodeTimeout:
			return http.StatusRequestTimeout
		case constants.ErrCodeResourceExhausted:
			return http.StatusTooManyRequests
		default:
			return http.StatusInternalServerError
		}
	case code >= 1100 && code < 1300: // 参数和数据错误
		return http.StatusBadRequest
	case code >= 2000 && code < 3000: // 服务器错误
		return http.StatusInternalServerError
	case code >= 3000 && code < 4000: // 数据库错误
		return http.StatusInternalServerError
	case code >= 4000 && code < 5000: // 用户相关错误
		switch code {
		case constants.ErrCodeUserNotFound:
			return http.StatusNotFound
		case constants.ErrCodeTokenInvalid, constants.ErrCodeTokenExpired:
			return http.StatusUnauthorized
		default:
			return http.StatusBadRequest
		}
	case code >= 5000 && code < 6000: // 游戏相关错误
		return http.StatusBadRequest
	case code >= 6000 && code < 7000: // 道具相关错误
		return http.StatusBadRequest
	case code >= 7000 && code < 8000: // 订单相关错误
		return http.StatusBadRequest
	case code >= 8000 && code < 9000: // 缓存相关错误
		return http.StatusInternalServerError
	case code >= 9000 && code < 10000: // 业务逻辑错误
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// ErrorHandler 错误处理器
type ErrorHandler struct {
	logger      logger.Logger
	errorStats  map[int]*int64 // 错误统计
	enableStats bool
}

// Init 初始化错误处理器
func Init(log logger.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger:      log,
		errorStats:  make(map[int]*int64),
		enableStats: true,
	}
}

// Handle 处理错误
func (h *ErrorHandler) Handle(err error, context string) *BusinessError {
	if err == nil {
		return nil
	}

	// 如果已经是BusinessError，直接返回
	if bizErr, ok := err.(*BusinessError); ok {
		h.recordError(bizErr.Code)
		h.logger.Error("业务错误", "error", err, "context", context)
		return bizErr
	}

	// 包装为业务错误
	wrappedErr := Wrap(err, constants.ErrCodeSystemInternal, "系统内部错误")
	h.recordError(wrappedErr.Code)
	h.logger.Error("系统错误", "error", err, "context", context)

	return wrappedErr
}

// HandleWithCode 使用指定错误码处理错误
func (h *ErrorHandler) HandleWithCode(err error, code int, message string, context string) *BusinessError {
	if err == nil {
		return nil
	}

	wrappedErr := Wrap(err, code, message)
	h.recordError(wrappedErr.Code)
	h.logger.Error("业务错误", "error", wrappedErr, "context", context)

	return wrappedErr
}

// recordError 记录错误统计
func (h *ErrorHandler) recordError(code int) {
	if !h.enableStats {
		return
	}

	counter, exists := h.errorStats[code]
	if !exists {
		counter = new(int64)
		h.errorStats[code] = counter
	}
	atomic.AddInt64(counter, 1)
}

// GetErrorStats 获取错误统计
func (h *ErrorHandler) GetErrorStats() map[int]int64 {
	stats := make(map[int]int64)
	for code, counter := range h.errorStats {
		stats[code] = atomic.LoadInt64(counter)
	}
	return stats
}

// ResetErrorStats 重置错误统计
func (h *ErrorHandler) ResetErrorStats() {
	h.errorStats = make(map[int]*int64)
}

// IsBusinessError 判断是否为业务错误
func IsBusinessError(err error) bool {
	_, ok := err.(*BusinessError)
	return ok
}

// GetBusinessError 获取业务错误
func GetBusinessError(err error) *BusinessError {
	if bizErr, ok := err.(*BusinessError); ok {
		return bizErr
	}
	return nil
}
