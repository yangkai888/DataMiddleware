package errors

import (
	"errors"
	"net/http"
	"testing"

	"datamiddleware/internal/infrastructure/logging"
	"datamiddleware/pkg/constants"
	"datamiddleware/internal/common/types"
)

func TestBusinessError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *BusinessError
		expected string
	}{
		{
			name:     "简单错误",
			err:      New(constants.ErrCodeInvalidParam, "参数无效"),
			expected: "code=1101, message=参数无效",
		},
		{
			name: "带原因的错误",
			err: NewWithCause(constants.ErrCodeSystemInternal, "系统错误",
				errors.New("原始错误")),
			expected: "code=1001, message=系统错误, cause=原始错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("BusinessError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBusinessError_Unwrap(t *testing.T) {
	originalErr := errors.New("原始错误")
	bizErr := NewWithCause(constants.ErrCodeSystemInternal, "系统错误", originalErr)

	if unwrapped := bizErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("BusinessError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestNew(t *testing.T) {
	err := New(constants.ErrCodeDataNotFound, "数据未找到")

	if err.Code != constants.ErrCodeDataNotFound {
		t.Errorf("New() code = %v, want %v", err.Code, constants.ErrCodeDataNotFound)
	}
	if err.Message != "数据未找到" {
		t.Errorf("New() message = %v, want %v", err.Message, "数据未找到")
	}
	if err.Cause != nil {
		t.Errorf("New() cause should be nil")
	}
}

func TestNewWithCause(t *testing.T) {
	cause := errors.New("原始错误")
	err := NewWithCause(constants.ErrCodeSystemInternal, "系统错误", cause)

	if err.Cause != cause {
		t.Errorf("NewWithCause() cause = %v, want %v", err.Cause, cause)
	}
}

func TestNewWithDetails(t *testing.T) {
	details := map[string]interface{}{
		"field": "username",
		"value": "invalid",
	}
	err := NewWithDetails(constants.ErrCodeInvalidParam, "参数无效", details)

	if err.Details == nil {
		t.Fatal("NewWithDetails() details should not be nil")
	}
	if err.Details["field"] != "username" {
		t.Errorf("NewWithDetails() details[field] = %v, want %v", err.Details["field"], "username")
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("原始错误")
	err := Wrap(originalErr, constants.ErrCodeSystemInternal, "系统错误")

	if err == nil {
		t.Fatal("Wrap() should not return nil")
	}
	if err.Cause != originalErr {
		t.Errorf("Wrap() cause = %v, want %v", err.Cause, originalErr)
	}

	// 测试包装nil错误
	if Wrap(nil, constants.ErrCodeSystemInternal, "消息") != nil {
		t.Error("Wrap() with nil error should return nil")
	}
}

func TestGetHTTPStatus(t *testing.T) {
	tests := []struct {
		code     int
		expected int
	}{
		{constants.ErrCodeSuccess, http.StatusOK}, // 成功状态
		{constants.ErrCodeSystemInternal, http.StatusInternalServerError},
		{constants.ErrCodeUnauthorized, http.StatusUnauthorized},
		{constants.ErrCodePermissionDenied, http.StatusForbidden},
		{constants.ErrCodeUserNotFound, http.StatusNotFound},
		{constants.ErrCodeInvalidParam, http.StatusBadRequest},
		{constants.ErrCodeDataNotFound, http.StatusBadRequest},
		{99999, http.StatusInternalServerError}, // 未知错误码
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := New(tt.code, "test")
			if err.HTTPStatus != tt.expected {
				t.Errorf("getHTTPStatus(%d) = %v, want %v", tt.code, err.HTTPStatus, tt.expected)
			}
		})
	}
}

func TestErrorHandler_Handle(t *testing.T) {
	// 创建模拟logger
	log, _ := logger.Init(types.LoggerConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})

	handler := Init(log)

	// 测试处理nil错误
	if handler.Handle(nil, "test") != nil {
		t.Error("Handle() with nil error should return nil")
	}

	// 测试处理普通错误
	originalErr := errors.New("原始错误")
	result := handler.Handle(originalErr, "test context")

	if result == nil {
		t.Fatal("Handle() should return BusinessError")
	}
	if result.Code != constants.ErrCodeSystemInternal {
		t.Errorf("Handle() code = %v, want %v", result.Code, constants.ErrCodeSystemInternal)
	}
	if result.Cause != originalErr {
		t.Errorf("Handle() cause = %v, want %v", result.Cause, originalErr)
	}
}

func TestErrorHandler_HandleWithCode(t *testing.T) {
	log, _ := logger.Init(types.LoggerConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})

	handler := Init(log)

	originalErr := errors.New("原始错误")
	result := handler.HandleWithCode(originalErr, constants.ErrCodeInvalidParam, "参数错误", "test")

	if result == nil {
		t.Fatal("HandleWithCode() should return BusinessError")
	}
	if result.Code != constants.ErrCodeInvalidParam {
		t.Errorf("HandleWithCode() code = %v, want %v", result.Code, constants.ErrCodeInvalidParam)
	}
	if result.Message != "参数错误" {
		t.Errorf("HandleWithCode() message = %v, want %v", result.Message, "参数错误")
	}
}

func TestErrorHandler_GetErrorStats(t *testing.T) {
	log, _ := logger.Init(types.LoggerConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})

	handler := Init(log)

	// 记录一些错误
	handler.Handle(New(constants.ErrCodeInvalidParam, "错误1"), "ctx1")
	handler.Handle(New(constants.ErrCodeInvalidParam, "错误2"), "ctx2")
	handler.Handle(New(constants.ErrCodeDataNotFound, "错误3"), "ctx3")

	stats := handler.GetErrorStats()

	if stats[constants.ErrCodeInvalidParam] != 2 {
		t.Errorf("错误统计不正确: %v", stats)
	}
	if stats[constants.ErrCodeDataNotFound] != 1 {
		t.Errorf("错误统计不正确: %v", stats)
	}
}

func TestErrorHandler_ResetErrorStats(t *testing.T) {
	log, _ := logger.Init(types.LoggerConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	})

	handler := Init(log)

	// 记录错误
	handler.Handle(New(constants.ErrCodeInvalidParam, "错误"), "ctx")

	// 重置统计
	handler.ResetErrorStats()

	stats := handler.GetErrorStats()
	if len(stats) != 0 {
		t.Errorf("重置后统计应该为空: %v", stats)
	}
}

func TestIsBusinessError(t *testing.T) {
	if !IsBusinessError(New(constants.ErrCodeInvalidParam, "错误")) {
		t.Error("IsBusinessError() should return true for BusinessError")
	}

	if IsBusinessError(errors.New("普通错误")) {
		t.Error("IsBusinessError() should return false for regular error")
	}
}

func TestGetBusinessError(t *testing.T) {
	bizErr := New(constants.ErrCodeInvalidParam, "错误")

	if GetBusinessError(bizErr) != bizErr {
		t.Error("GetBusinessError() should return the BusinessError")
	}

	if GetBusinessError(errors.New("普通错误")) != nil {
		t.Error("GetBusinessError() should return nil for regular error")
	}
}
