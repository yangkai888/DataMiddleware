# API设计规范

## 概述

数据中间件提供双协议API支持：HTTP REST API 和 TCP 二进制协议，满足不同场景的需求。

## 协议对比

| 特性 | HTTP REST API | TCP 二进制协议 |
|------|---------------|----------------|
| **延迟** | 中等 (10-100ms) | 极低 (< 1ms) |
| **吞吐量** | 高 (万级 TPS) | 极高 (十万级 TPS) |
| **连接方式** | 短连接/连接池 | 长连接/连接复用 |
| **消息大小** | 无限制 | 建议 < 64KB |
| **开发复杂度** | 简单 | 中等 |
| **适用场景** | Web应用、API集成 | 游戏服务器、实时通信 |

## HTTP REST API 设计

### 接口规范

#### URL命名规范
- 使用复数名词: `/api/v1/players`, `/api/v1/items`
- 层级关系: `/api/v1/players/{player_id}/items`
- 动作表示: `/api/v1/orders/{order_id}/cancel`

#### HTTP方法规范
```http
GET    /api/v1/players       # 查询玩家列表
GET    /api/v1/players/{id}  # 查询单个玩家
POST   /api/v1/players       # 创建玩家
PUT    /api/v1/players/{id}  # 更新玩家
DELETE /api/v1/players/{id}  # 删除玩家
```

#### 状态码规范
- **200**: 成功
- **201**: 创建成功
- **400**: 请求参数错误
- **401**: 未授权
- **403**: 权限不足
- **404**: 资源不存在
- **409**: 资源冲突
- **500**: 服务器内部错误

### 响应格式规范

#### 成功响应
```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    // 具体数据
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

#### 错误响应
```json
{
  "code": 400,
  "message": "参数错误",
  "details": "player_id不能为空",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

#### 分页响应
```json
{
  "code": 200,
  "data": {
    "items": [...],
    "pagination": {
      "page": 1,
      "size": 20,
      "total": 100,
      "total_pages": 5
    }
  }
}
```

## TCP 二进制协议设计

### 协议概述

TCP协议采用二进制格式，提供高性能的实时通信能力。主要用于游戏服务器之间的数据交换。

**协议版本**: v1.0
**消息格式**: 二进制头部 + JSON/二进制消息体
**性能特点**: 低延迟 (< 1ms)、高吞吐 (> 10万 TPS)
**连接特性**: 长连接复用、心跳检测、自动重连

### 消息格式

#### 消息头结构 (24字节)
```c
struct MessageHeader {
    uint8  version;      // 协议版本 (当前: 1)
    uint16 message_type; // 消息类型
    uint8  flags;        // 消息标志
    uint32 sequence_id;  // 序列号
    uint32 game_id_len;  // 游戏ID长度
    uint32 user_id_len;  // 用户ID长度
    int64  timestamp;    // 时间戳
    uint32 body_length;  // 消息体长度
    uint32 checksum;     // CRC32校验和
}
```

#### 消息体
- **格式**: JSON 或 二进制 (根据消息类型)
- **编码**: UTF-8 (JSON) 或 自定义二进制格式
- **压缩**: 可选gzip压缩 (FlagCompressed)

### 消息类型定义

#### 基础消息类型 (0x0000-0x0FFF)
| 类型值 | 名称 | 说明 | 方向 |
|--------|------|------|------|
| 0x0001 | Heartbeat | 心跳消息 | 双向 |
| 0x0002 | Handshake | 握手认证 | 客户端→服务器 |
| 0x0003 | HandshakeAck | 握手确认 | 服务器→客户端 |
| 0x0004 | Ping | 连接测试 | 双向 |
| 0x0005 | Pong | 连接响应 | 双向 |

#### 玩家相关消息 (0x1000-0x1FFF)
| 类型值 | 名称 | 说明 | 方向 |
|--------|------|------|------|
| 0x1001 | PlayerLogin | 玩家登录 | 客户端→服务器 |
| 0x1002 | PlayerLogout | 玩家登出 | 客户端→服务器 |
| 0x1003 | PlayerDataSync | 玩家数据同步 | 双向 |
| 0x1004 | PlayerUpdate | 玩家信息更新 | 双向 |
| 0x1005 | PlayerQuery | 玩家信息查询 | 客户端→服务器 |

#### 道具相关消息 (0x2000-0x2FFF)
| 类型值 | 名称 | 说明 | 方向 |
|--------|------|------|------|
| 0x2001 | ItemQuery | 道具查询 | 客户端→服务器 |
| 0x2002 | ItemUse | 道具使用 | 客户端→服务器 |
| 0x2003 | ItemTransfer | 道具转移 | 客户端→服务器 |
| 0x2004 | ItemUpdate | 道具更新通知 | 服务器→客户端 |

#### 订单相关消息 (0x3000-0x3FFF)
| 类型值 | 名称 | 说明 | 方向 |
|--------|------|------|------|
| 0x3001 | OrderCreate | 创建订单 | 客户端→服务器 |
| 0x3002 | OrderQuery | 查询订单 | 客户端→服务器 |
| 0x3003 | OrderCancel | 取消订单 | 客户端→服务器 |
| 0x3004 | OrderPay | 订单支付 | 客户端→服务器 |
| 0x3005 | OrderUpdate | 订单状态更新 | 服务器→客户端 |

#### 系统消息类型 (0xF000-0xFFFF)
| 类型值 | 名称 | 说明 | 方向 |
|--------|------|------|------|
| 0xF001 | Error | 错误消息 | 服务器→客户端 |
| 0xF002 | SystemBroadcast | 系统广播 | 服务器→客户端 |
| 0xF003 | Maintenance | 维护通知 | 服务器→客户端 |

### 消息标志 (Flags)

| 位标志 | 值 | 说明 |
|--------|-----|------|
| None | 0x00 | 无特殊标志 |
| Compressed | 0x01 | 消息体经过gzip压缩 |
| Encrypted | 0x02 | 消息体经过加密 |
| NeedResponse | 0x04 | 需要服务器响应 |
| Async | 0x08 | 异步消息，不需要立即响应 |
| Urgent | 0x10 | 紧急消息，优先处理 |

### 序列号管理

#### 序列号规则
- **客户端**: 奇数序列号 (1, 3, 5, ...)
- **服务器**: 偶数序列号 (2, 4, 6, ...)
- **范围**: 0-4294967295，循环使用
- **用途**: 保证消息有序性和去重

#### 响应匹配
```go
// 请求消息
request := Message{
    Header: MessageHeader{
        SequenceID: 1,  // 客户端序列号
        Flags: FlagNeedResponse,
    }
}

// 响应消息
response := Message{
    Header: MessageHeader{
        SequenceID: 2,  // 服务器序列号
        // 其他字段基于请求
    }
}
```

### 连接管理

#### 连接建立流程
1. **TCP连接建立**
2. **握手认证**
   ```json
   // 客户端发送
   {
     "client_version": "1.0.0",
     "supported_protocols": ["json", "binary"],
     "capabilities": ["compression", "encryption"],
     "game_id": "game1",
     "auth_token": "xxx"
   }
   ```
3. **服务器确认**
   ```json
   {
     "server_version": "1.0.0",
     "session_id": "session_123",
     "heartbeat_interval": 30,
     "max_message_size": 65536
   }
   ```

#### 心跳机制
- **间隔**: 默认30秒
- **超时**: 90秒
- **重试**: 最多3次
- **断开**: 连续失败后断开连接

#### 自动重连
- **检测**: 连接断开后立即重连
- **退避**: 指数退避算法 (1s, 2s, 4s, 8s...)
- **上限**: 最大60秒间隔
- **恢复**: 重连成功后恢复会话状态

### 错误处理

#### 错误码体系
- **网络错误**: 0xE001-0xE0FF
- **协议错误**: 0xE101-0xE1FF
- **认证错误**: 0xE201-0xE2FF
- **业务错误**: 0xE301-0xE3FF
- **系统错误**: 0xE401-0xE4FF

#### 常见错误码
| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| 0xE001 | 网络连接失败 | 检查网络连接，重试 |
| 0xE002 | 协议版本不支持 | 升级客户端版本 |
| 0xE003 | 认证失败 | 重新登录 |
| 0xE004 | 权限不足 | 检查用户权限 |
| 0xE005 | 消息格式错误 | 检查消息格式 |
| 0xE006 | 服务器内部错误 | 稍后重试 |

### 性能优化

#### 消息压缩
- **算法**: gzip (推荐) / lz4 (速度优先)
- **阈值**: 消息体 > 1KB 时压缩
- **比率**: 通常可达到 70-90% 压缩率

#### 批量发送
- **合并**: 将多个小消息合并发送
- **延迟**: 累计 10ms 或 5个消息后发送
- **分包**: 大消息自动分包传输

#### 连接池
- **复用**: 长连接复用，减少建连开销
- **负载均衡**: 多连接间的负载均衡
- **健康检查**: 定期检查连接状态

### 安全性设计

#### 传输安全
- **TLS加密**: 可选的TLS 1.3加密
- **消息加密**: 敏感消息体加密
- **证书验证**: 双向证书验证

#### 认证授权
- **会话认证**: 连接级别的身份验证
- **权限控制**: 基于角色的访问控制
- **操作审计**: 敏感操作的审计日志

#### 防攻击措施
- **频率限制**: 连接级别的请求频率控制
- **消息大小限制**: 防止DOS攻击
- **连接数限制**: 防止资源耗尽

### 监控指标

#### 连接指标
- 活跃连接数
- 连接建立/断开次数
- 连接持续时间分布

#### 消息指标
- 消息收发数量
- 消息大小分布
- 响应时间分布
- 错误率统计

#### 性能指标
- 吞吐量 (TPS)
- 延迟分布 (P50/P95/P99)
- CPU/内存使用率
- 网络带宽使用

## 客户端实现指南

### TCP客户端实现

#### 连接管理
```go
type TCPClient struct {
    conn     net.Conn
    encoder  *protocol.BinaryCodec
    sequence uint32
    session  string
}

func (c *TCPClient) Connect(addr string) error {
    // 建立TCP连接
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        return err
    }
    c.conn = conn

    // 执行握手
    return c.handshake()
}
```

#### 消息发送
```go
func (c *TCPClient) SendMessage(msgType types.MessageType, body interface{}) error {
    // 构造消息
    message := &types.Message{
        Header: types.MessageHeader{
            Version:    types.ProtocolVersion,
            Type:       msgType,
            Flags:      types.FlagNeedResponse,
            SequenceID: atomic.AddUint32(&c.sequence, 2), // 客户端用奇数
            GameID:     c.gameID,
            UserID:     c.userID,
            Timestamp:  time.Now().Unix(),
        },
        Body: body,
    }

    // 编码发送
    data, err := c.encoder.Encode(message)
    if err != nil {
        return err
    }

    return c.sendData(data)
}
```

#### 消息接收
```go
func (c *TCPClient) receiveLoop() {
    buffer := make([]byte, 65536)
    for {
        n, err := c.conn.Read(buffer)
        if err != nil {
            // 处理错误
            break
        }

        // 解码消息
        message, consumed, err := c.encoder.Decode(buffer[:n])
        if err != nil {
            // 处理解码错误
            continue
        }

        // 处理消息
        c.handleMessage(message)

        // 处理剩余数据
        if consumed < n {
            // 处理粘包
        }
    }
}
```

### 错误处理策略

#### 重连机制
```go
func (c *TCPClient) reconnect() {
    backoff := time.Second
    maxBackoff := time.Minute

    for {
        err := c.Connect(c.addr)
        if err == nil {
            return // 重连成功
        }

        log.Printf("重连失败: %v, %v后重试", err, backoff)
        time.Sleep(backoff)

        backoff *= 2
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
    }
}
```

#### 超时处理
```go
func (c *TCPClient) SendWithTimeout(message *types.Message, timeout time.Duration) (*types.Message, error) {
    // 发送消息
    err := c.SendMessage(message.Header.Type, message.Body)
    if err != nil {
        return nil, err
    }

    // 等待响应
    return c.waitForResponse(message.Header.SequenceID+1, timeout)
}
```

## 版本兼容性

### 协议版本管理
- **版本号**: 消息头中的version字段
- **兼容性**: 向后兼容，低版本客户端可连接高版本服务器
- **升级策略**: 平滑升级，支持多版本共存

### 扩展机制
- **保留字段**: 消息头中预留扩展字段
- **可选字段**: 消息体中支持可选字段
- **功能标志**: 通过flags字段启用新功能

## 最佳实践

### 性能优化
1. **连接复用**: 避免频繁建连，使用长连接
2. **消息压缩**: 大消息启用压缩，减少带宽
3. **批量发送**: 小消息批量发送，提高效率
4. **异步处理**: 非关键消息使用异步发送

### 可靠性保证
1. **心跳保活**: 启用心跳机制，及时检测连接状态
2. **自动重连**: 实现自动重连机制，提高可用性
3. **消息重发**: 重要消息实现重发机制
4. **错误处理**: 完善的错误处理和恢复机制

### 安全加固
1. **传输加密**: 敏感数据启用TLS加密
2. **身份认证**: 严格的身份认证流程
3. **权限控制**: 细粒度的权限控制
4. **审计日志**: 敏感操作的审计记录

### 监控告警
1. **连接监控**: 实时监控连接状态和数量
2. **性能监控**: 监控吞吐量和响应时间
3. **错误监控**: 监控错误率和错误类型
4. **容量监控**: 监控资源使用情况
