# 数据中间件API文档

## 概述

数据中间件为游戏服务器和前端应用提供统一的API接口，支持玩家管理、道具交易、订单处理等核心业务功能。

**API版本**: v1.0.0
**数据格式**: JSON
**认证方式**: JWT Token (HTTP) / 会话认证 (TCP)

## 接口协议

### HTTP REST API
**基础URL**: `http://localhost:8080`
- **适用场景**: Web应用、前端应用、第三方集成
- **协议特点**: RESTful设计、JSON数据格式、JWT认证
- **端口**: 8080

### TCP 二进制协议
**服务器地址**: `localhost:9090`
- **适用场景**: 游戏服务器、实时通信、高并发场景
- **协议特点**: 二进制协议、高性能、低延迟、连接复用
- **端口**: 9090

## 认证

### 获取Token
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "player123",
  "password": "password123"
}
```

**响应**:
```json
{
  "code": 200,
  "message": "登录成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expire_at": "2024-12-31T23:59:59Z",
    "user": {
      "id": 123,
      "username": "player123",
      "level": 10
    }
  }
}
```

### 使用Token
在请求头中添加Authorization字段：
```
Authorization: Bearer {token}
```

## TCP协议详解

### 协议概述

TCP协议采用二进制格式，提供高性能的实时通信能力。主要用于游戏服务器之间的数据交换。

**协议版本**: v1.0
**消息格式**: 二进制
**编解码方式**: 支持JSON和二进制编解码
**连接管理**: 支持心跳检测、连接池、自动重连

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

### 消息类型

#### 基础消息类型
| 类型值 | 名称 | 说明 |
|--------|------|------|
| 0x0001 | Heartbeat | 心跳消息 |
| 0x0002 | Handshake | 握手认证 |

#### 游戏数据消息类型
| 类型值 | 名称 | 说明 |
|--------|------|------|
| 0x1001 | PlayerLogin | 玩家登录 |
| 0x1002 | PlayerLogout | 玩家登出 |
| 0x1003 | PlayerData | 玩家数据同步 |
| 0x1004 | ItemOperation | 道具操作 |
| 0x1005 | OrderOperation | 订单操作 |

#### 系统消息类型
| 类型值 | 名称 | 说明 |
|--------|------|------|
| 0x2001 | Error | 错误消息 |
| 0x2002 | Ping | 连接测试 |
| 0x2003 | Pong | 连接响应 |

### 消息标志 (Flags)

| 位标志 | 值 | 说明 |
|--------|-----|------|
| Compressed | 0x01 | 消息体经过gzip压缩 |
| Encrypted | 0x02 | 消息体经过加密 |
| NeedResponse | 0x04 | 需要服务器响应 |
| Async | 0x08 | 异步消息，不需要立即响应 |

### TCP接口示例

#### 1. 连接建立和握手
```javascript
// 客户端连接到服务器
const client = new TCPClient('localhost', 9090);

// 发送握手消息
const handshakeMessage = {
  header: {
    version: 1,
    type: 0x0002,  // Handshake
    flags: 0x04,   // NeedResponse
    sequence_id: 1,
    game_id: "game1",
    user_id: "",
    timestamp: Date.now(),
    body_length: handshakeBody.length,
    checksum: calculateCRC32(handshakeBody)
  },
  body: JSON.stringify({
    client_version: "1.0.0",
    supported_protocols: ["json", "binary"],
    capabilities: ["compression", "encryption"]
  })
};
```

#### 2. 玩家登录
```javascript
// 发送玩家登录消息
const loginMessage = {
  header: {
    version: 1,
    type: 0x1001,  // PlayerLogin
    flags: 0x04,   // NeedResponse
    sequence_id: 2,
    game_id: "game1",
    user_id: "",
    timestamp: Date.now(),
    body_length: loginBody.length,
    checksum: calculateCRC32(loginBody)
  },
  body: JSON.stringify({
    username: "player123",
    password: "hashed_password",
    client_info: {
      version: "1.0.0",
      platform: "ios"
    }
  })
};
```

#### 3. 道具操作
```javascript
// 发送道具使用消息
const itemUseMessage = {
  header: {
    version: 1,
    type: 0x1004,  // ItemOperation
    flags: 0x04,   // NeedResponse
    sequence_id: 3,
    game_id: "game1",
    user_id: "player123",
    timestamp: Date.now(),
    body_length: itemBody.length,
    checksum: calculateCRC32(itemBody)
  },
  body: JSON.stringify({
    action: "use",
    item_id: 1001,
    quantity: 1,
    target: null
  })
};
```

#### 4. 心跳保持
```javascript
// 定期发送心跳消息
setInterval(() => {
  const heartbeatMessage = {
    header: {
      version: 1,
      type: 0x0001,  // Heartbeat
      flags: 0x00,   // 不需要响应
      sequence_id: ++sequenceId,
      game_id: "game1",
      user_id: "player123",
      timestamp: Date.now(),
      body_length: 0,
      checksum: 0
    },
    body: null
  };
  client.send(heartbeatMessage);
}, 30000); // 30秒间隔
```

### TCP错误码

| 错误码 | 说明 |
|--------|------|
| 0xE001 | 协议版本不支持 |
| 0xE002 | 消息格式错误 |
| 0xE003 | 认证失败 |
| 0xE004 | 权限不足 |
| 0xE005 | 资源不存在 |
| 0xE006 | 服务器内部错误 |
| 0xE007 | 连接超时 |
| 0xE008 | 消息过大 |

### TCP连接管理

#### 连接池配置
```yaml
tcp:
  max_connections: 10000    # 最大连接数
  read_timeout: 30s         # 读取超时
  write_timeout: 30s        # 写入超时
  idle_timeout: 300s        # 空闲超时
```

#### 心跳配置
```yaml
heartbeat:
  enabled: true
  interval: 30s    # 心跳间隔
  timeout: 90s     # 心跳超时
  max_missed: 3    # 最大丢失次数
```

### TCP性能特性

- **高并发**: 支持数万个并发连接
- **低延迟**: 网络延迟 < 1ms
- **高吞吐**: 消息处理能力 > 10万 TPS
- **内存优化**: 对象池复用，GC友好
- **自动恢复**: 断线重连，心跳检测

## HTTP REST API

### 获取玩家信息
```http
GET /api/v1/players/{player_id}
Authorization: Bearer {token}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "id": 123,
    "username": "player123",
    "level": 25,
    "exp": 12500,
    "gold": 5000,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

### 更新玩家信息
```http
PUT /api/v1/players/{player_id}
Authorization: Bearer {token}
Content-Type: application/json

{
  "level": 26,
  "exp": 13000
}
```

### 注册新玩家
```http
POST /api/v1/players
Content-Type: application/json

{
  "username": "newplayer",
  "password": "securepass123",
  "email": "player@example.com"
}
```

## 道具管理API

### 获取玩家道具列表
```http
GET /api/v1/players/{player_id}/items
Authorization: Bearer {token}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "items": [
      {
        "id": 1001,
        "item_id": 1,
        "name": "魔法剑",
        "quantity": 1,
        "quality": "rare"
      },
      {
        "id": 1002,
        "item_id": 2,
        "name": "生命药水",
        "quantity": 50,
        "quality": "common"
      }
    ]
  }
}
```

### 使用道具
```http
POST /api/v1/players/{player_id}/items/{item_id}/use
Authorization: Bearer {token}
Content-Type: application/json

{
  "quantity": 1
}
```

### 转移道具
```http
POST /api/v1/players/{player_id}/items/{item_id}/transfer
Authorization: Bearer {token}
Content-Type: application/json

{
  "target_player_id": 456,
  "quantity": 10
}
```

## 订单管理API

### 创建订单
```http
POST /api/v1/orders
Authorization: Bearer {token}
Content-Type: application/json

{
  "player_id": 123,
  "item_id": 1,
  "quantity": 1,
  "price": 100,
  "order_type": "buy"
}
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "order_id": "order_1234567890",
    "status": "pending",
    "created_at": "2024-01-01T12:00:00Z"
  }
}
```

### 获取订单详情
```http
GET /api/v1/orders/{order_id}
Authorization: Bearer {token}
```

### 取消订单
```http
DELETE /api/v1/orders/{order_id}
Authorization: Bearer {token}
```

### 处理支付
```http
POST /api/v1/orders/{order_id}/pay
Authorization: Bearer {token}
Content-Type: application/json

{
  "payment_method": "gold",
  "amount": 100
}
```

## 游戏路由API

### 获取支持的游戏列表
```http
GET /api/v1/games
```

**响应**:
```json
{
  "code": 200,
  "data": {
    "games": [
      {
        "id": "game1",
        "name": "游戏1",
        "status": "active",
        "player_count": 10000
      },
      {
        "id": "game2",
        "name": "游戏2",
        "status": "maintenance",
        "player_count": 5000
      }
    ]
  }
}
```

### 游戏特定API调用
```http
POST /api/v1/games/{game_id}/{action}
Authorization: Bearer {token}
Content-Type: application/json

{
  "player_id": 123,
  "params": {
    "action": "level_up",
    "target_level": 26
  }
}
```

## 监控和健康检查API

### 健康检查
```http
GET /health
```

**响应**:
```json
{
  "status": "ok",
  "timestamp": 1735689600,
  "uptime": 3600,
  "version": "1.0.0"
}
```

### 系统指标
```http
GET /metrics
Authorization: Bearer {admin_token}
```

**响应**:
```json
{
  "cpu_usage": 45.5,
  "memory_usage": 67.8,
  "active_connections": 1250,
  "goroutines": 150,
  "heap_alloc": 104857600
}
```

### 缓存统计
```http
GET /api/v1/cache/stats
Authorization: Bearer {admin_token}
```

## 错误码说明

| 错误码 | 说明 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未授权访问 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 500 | 服务器内部错误 |
| 503 | 服务不可用 |

### 业务错误码

| 错误码 | 说明 |
|--------|------|
| 1001 | 用户不存在 |
| 1002 | 用户已存在 |
| 1003 | 密码错误 |
| 2001 | 道具不足 |
| 2002 | 道具不存在 |
| 3001 | 余额不足 |
| 3002 | 订单不存在 |

## 数据格式

### 玩家对象
```json
{
  "id": "integer",
  "username": "string",
  "email": "string",
  "level": "integer",
  "exp": "integer",
  "gold": "integer",
  "created_at": "string (ISO 8601)",
  "updated_at": "string (ISO 8601)"
}
```

### 道具对象
```json
{
  "id": "integer",
  "item_id": "integer",
  "name": "string",
  "description": "string",
  "quantity": "integer",
  "quality": "string",
  "durability": "integer",
  "created_at": "string (ISO 8601)"
}
```

### 订单对象
```json
{
  "id": "string",
  "player_id": "integer",
  "item_id": "integer",
  "quantity": "integer",
  "price": "integer",
  "status": "string",
  "order_type": "string",
  "created_at": "string (ISO 8601)",
  "updated_at": "string (ISO 8601)"
}
```

## 限制和配额

- **API调用频率**: 每个IP每分钟最多1000次请求
- **文件上传大小**: 最大10MB
- **数据分页**: 默认每页20条，最大每页100条
- **Token过期时间**: 24小时
- **会话超时**: 30分钟无活动自动登出

## 协议选择指南

### HTTP vs TCP 对比

| 特性 | HTTP REST API | TCP 二进制协议 |
|------|---------------|----------------|
| **延迟** | 中等 (10-100ms) | 极低 (< 1ms) |
| **吞吐量** | 高 (万级 TPS) | 极高 (十万级 TPS) |
| **连接方式** | 短连接/连接池 | 长连接/连接复用 |
| **消息大小** | 无限制 | 建议 < 64KB |
| **开发复杂度** | 简单 | 中等 |
| **适用场景** | Web应用、API集成 | 游戏服务器、实时通信 |

### 选择建议

#### 使用HTTP API的情况：
- Web前端应用
- 第三方系统集成
- 管理后台操作
- 非实时性业务

#### 使用TCP协议的情况：
- 游戏服务器通信
- 实时数据同步
- 高频数据交换
- 对延迟敏感的场景

## 客户端SDK

### TCP客户端示例 (Node.js)
```javascript
class GameClient {
  constructor(host, port) {
    this.host = host;
    this.port = port;
    this.socket = null;
    this.sequenceId = 0;
    this.connected = false;
  }

  async connect() {
    return new Promise((resolve, reject) => {
      this.socket = new net.Socket();

      this.socket.connect(this.port, this.host, () => {
        this.connected = true;
        this.startHeartbeat();
        resolve();
      });

      this.socket.on('error', reject);
      this.socket.on('data', (data) => this.handleMessage(data));
    });
  }

  sendMessage(type, body, flags = 0x04) {
    const message = {
      header: {
        version: 1,
        type: type,
        flags: flags,
        sequence_id: ++this.sequenceId,
        game_id: "game1",
        user_id: this.userId || "",
        timestamp: Date.now(),
        body_length: body ? Buffer.byteLength(JSON.stringify(body)) : 0,
        checksum: 0 // 计算CRC32
      },
      body: body
    };

    // 编码并发送消息
    const encodedMessage = this.encodeMessage(message);
    this.socket.write(encodedMessage);
  }

  handleMessage(data) {
    const message = this.decodeMessage(data);
    // 处理响应消息
    this.emit('message', message);
  }
}
```

### HTTP客户端示例 (JavaScript)
```javascript
class ApiClient {
  constructor(baseUrl) {
    this.baseUrl = baseUrl;
    this.token = null;
  }

  async login(username, password) {
    const response = await fetch(`${this.baseUrl}/api/v1/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });

    const data = await response.json();
    if (data.code === 200) {
      this.token = data.data.token;
    }
    return data;
  }

  async getPlayerInfo(playerId) {
    const response = await fetch(`${this.baseUrl}/api/v1/players/${playerId}`, {
      headers: {
        'Authorization': `Bearer ${this.token}`
      }
    });
    return response.json();
  }
}
```

## 故障排除

### TCP连接问题
- **连接超时**: 检查服务器地址和端口配置
- **认证失败**: 确认握手消息格式正确
- **消息乱序**: 检查序列号管理和重传机制
- **性能下降**: 检查网络延迟和服务器负载

### HTTP请求问题
- **401错误**: 检查JWT token是否有效
- **403错误**: 确认用户权限设置
- **500错误**: 查看服务器日志定位问题
- **超时问题**: 检查网络连接和服务器响应时间

## 更新日志

### v1.0.0 (2024-12-22)
- ✅ 初始版本发布
- ✅ HTTP REST API: 玩家管理、道具交易、订单处理
- ✅ TCP 二进制协议: 高性能实时通信
- ✅ JWT认证和会话管理
- ✅ 多游戏路由支持
- ✅ 缓存和监控功能
- ✅ 双协议完整文档
