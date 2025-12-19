package utils

import "errors"

// 工具库错误定义
var (
	// ErrPoolExhausted 对象池耗尽
	ErrPoolExhausted = errors.New("对象池已耗尽")
	// ErrInvalidInput 无效输入
	ErrInvalidInput = errors.New("无效输入")
	// ErrTimeout 超时
	ErrTimeout = errors.New("操作超时")
	// ErrNotFound 未找到
	ErrNotFound = errors.New("未找到")
)
