package constants

// 错误码定义
// 错误码格式: XXXYYY
// XXX: 模块代码 (001-999)
// YYY: 具体错误代码 (001-999)

// 通用错误码 (001XXX)
const (
	// 成功
	ErrCodeSuccess = 0

	// 系统级错误
	ErrCodeSystemInternal    = 1001 // 系统内部错误
	ErrCodeConfigInvalid     = 1002 // 配置无效
	ErrCodeNetworkError      = 1003 // 网络错误
	ErrCodeTimeout           = 1004 // 超时
	ErrCodeResourceExhausted = 1005 // 资源耗尽
	ErrCodePermissionDenied  = 1006 // 权限不足
	ErrCodeUnauthorized      = 1007 // 未授权

	// 参数错误
	ErrCodeInvalidParam    = 1101 // 参数无效
	ErrCodeMissingParam    = 1102 // 缺少参数
	ErrCodeInvalidFormat   = 1103 // 格式无效
	ErrCodeOutOfRange      = 1104 // 超出范围

	// 数据错误
	ErrCodeDataNotFound    = 1201 // 数据未找到
	ErrCodeDataAlreadyExists = 1202 // 数据已存在
	ErrCodeDataCorrupted   = 1203 // 数据损坏
	ErrCodeDataInconsistent = 1204 // 数据不一致
)

// 服务器错误码 (002XXX)
const (
	ErrCodeServerStartFailed   = 2001 // 服务器启动失败
	ErrCodeServerStopFailed    = 2002 // 服务器停止失败
	ErrCodeConnectionFailed    = 2003 // 连接失败
	ErrCodeConnectionClosed    = 2004 // 连接已关闭
	ErrCodeProtocolError       = 2005 // 协议错误
	ErrCodeMessageTooLarge     = 2006 // 消息过大
)

// 数据库错误码 (003XXX)
const (
	ErrCodeDBConnectionFailed = 3001 // 数据库连接失败
	ErrCodeDBQueryFailed      = 3002 // 查询失败
	ErrCodeDBInsertFailed     = 3003 // 插入失败
	ErrCodeDBUpdateFailed     = 3004 // 更新失败
	ErrCodeDBDeleteFailed     = 3005 // 删除失败
	ErrCodeDBTransactionFailed = 3006 // 事务失败
	ErrCodeDBConstraintViolation = 3007 // 约束违反
)

// 用户相关错误码 (004XXX)
const (
	ErrCodeUserNotFound       = 4001 // 用户不存在
	ErrCodeUserAlreadyExists  = 4002 // 用户已存在
	ErrCodeUserDisabled       = 4003 // 用户已禁用
	ErrCodePasswordInvalid    = 4004 // 密码无效
	ErrCodeTokenInvalid       = 4005 // Token无效
	ErrCodeTokenExpired       = 4006 // Token过期
)

// 游戏相关错误码 (005XXX)
const (
	ErrCodeGameNotFound       = 5001 // 游戏不存在
	ErrCodeGameDisabled       = 5002 // 游戏已禁用
	ErrCodeGameServerFull     = 5003 // 游戏服务器已满
	ErrCodeGameInProgress     = 5004 // 游戏进行中
	ErrCodeGameFinished       = 5005 // 游戏已结束
)

// 道具相关错误码 (006XXX)
const (
	ErrCodeItemNotFound       = 6001 // 道具不存在
	ErrCodeItemInsufficient   = 6002 // 道具不足
	ErrCodeItemExpired        = 6003 // 道具过期
	ErrCodeItemLocked         = 6004 // 道具锁定
)

// 订单相关错误码 (007XXX)
const (
	ErrCodeOrderNotFound      = 7001 // 订单不存在
	ErrCodeOrderCancelled     = 7002 // 订单已取消
	ErrCodeOrderPaid          = 7003 // 订单已支付
	ErrCodeOrderExpired       = 7004 // 订单过期
	ErrCodePaymentFailed      = 7005 // 支付失败
)

// 缓存相关错误码 (008XXX)
const (
	ErrCodeCacheMiss          = 8001 // 缓存未命中
	ErrCodeCacheExpired       = 8002 // 缓存过期
	ErrCodeCacheInvalid       = 8003 // 缓存无效
)

// 业务逻辑错误码 (009XXX)
const (
	ErrCodeBusinessRuleViolation = 9001 // 业务规则违反
	ErrCodeOperationNotAllowed   = 9002 // 操作不允许
	ErrCodeStateInvalid          = 9003 // 状态无效
	ErrCodeQuotaExceeded         = 9004 // 配额超限
)
