# DataMiddleware 测试指南

## 概述

DataMiddleware项目提供了完整的测试套件，包括单元测试、集成测试和验收测试。本指南介绍如何运行各种测试来验证项目功能。

## 测试依赖工具

运行测试需要以下工具：

- **curl**: HTTP请求工具，用于API测试
- **jq**: JSON处理工具，用于解析API响应
- **netcat (nc)**: 网络连接工具，用于TCP测试

### 安装依赖工具

```bash
# Ubuntu/Debian系统
apt update && apt install -y curl jq netcat-traditional

# 或者使用一键安装脚本
./docs/setup-environment.sh
```

## 测试脚本概览

### 1. 一键完整测试脚本
```bash
# 运行所有Phase的完整功能测试
./run-all-tests.sh
```
此脚本会：
- 检查依赖工具
- 自动启动所需服务 (Redis, MySQL, 应用服务器)
- 按顺序运行所有Phase测试
- 生成详细的测试报告
- 自动清理服务

### 2. Phase专项测试脚本

#### Phase 1: 基础框架测试
```bash
# 编译和基础模块测试
go build -v ./cmd/server
go test -v ./internal/config/...
go test -v ./internal/logger/...
```

#### Phase 2: 协议和数据层测试
```bash
# TCP连接测试
nc -z localhost 9090

# HTTP健康检查
curl -s http://localhost:8080/health | jq .

# 数据库连接测试
mysql -u root -pMySQL@123456 -e "SELECT 1;"

# Redis连接测试
redis-cli ping
```

#### Phase 3: 业务逻辑层测试
```bash
# 完整的业务功能测试
bash test/phase3_complete_test.sh
```
测试内容：
- 玩家注册登录
- JWT认证
- 道具管理
- 订单处理
- TCP游戏路由

#### Phase 4: 缓存和基础设施测试
```bash
# 缓存和监控功能测试
bash test/phase4_validation_simple.sh
```
测试内容：
- 多级缓存 (L1/L2)
- 缓存防护机制
- 异步处理系统
- 监控和健康检查

#### Phase 5: 高并发优化测试
```bash
# 内存优化测试
go test -v ./test/phase5_memory_test.go

# 协程池测试
go test -v ./test/phase5_goroutine_pool_test.go

# 连接池测试
go test -v ./test/phase5_connection_pool_test.go

# 性能基准测试
go test -v ./test/phase5_performance_benchmarks.go
```

## 手动测试示例

### 1. 启动服务
```bash
# 启动Redis
redis-server --daemonize yes

# 启动MySQL
mariadbd --user=mysql --socket=/run/mysqld/mysqld.sock &

# 启动应用服务器
./server &
```

### 2. API功能测试

#### 健康检查
```bash
curl -s http://localhost:8080/health | jq .
```

#### 用户注册
```bash
curl -X POST http://localhost:8080/api/v1/players/register \
  -H "Content-Type: application/json" \
  -d '{
    "game_id": "game1",
    "username": "testuser",
    "password": "password123",
    "email": "test@example.com"
  }' | jq .
```

#### 用户登录
```bash
curl -X POST http://localhost:8080/api/v1/players/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "password123"
  }' | jq .
```

#### 获取用户信息 (需要JWT token)
```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/players/user_d4f4ce2e1994fd7e | jq .
```

### 3. 缓存功能测试

#### 设置缓存
```bash
curl -X POST http://localhost:8080/api/v1/cache/set \
  -H "Content-Type: application/json" \
  -d '{
    "key": "test_key",
    "value": "test_value",
    "ttl": 300
  }' | jq .
```

#### 获取缓存
```bash
curl "http://localhost:8080/api/v1/cache/get?key=test_key" | jq .
```

### 4. 异步任务测试

#### 提交异步任务
```bash
curl -X POST http://localhost:8080/api/v1/async/task \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "priority": 1,
    "payload": {"to": "user@example.com", "subject": "Test"}
  }' | jq .
```

#### 查看异步队列状态
```bash
curl http://localhost:8080/api/v1/async/stats | jq .
```

## 性能测试

### 连接并发测试
```bash
# TCP连接并发测试
go run test/tcp_performance_test.go

# HTTP QPS测试
go run test/performance_test.go
```

### 内存和协程测试
```bash
# 内存使用情况
go test -bench=. ./test/phase5_memory_test.go

# 协程池性能
go test -bench=. ./test/phase5_goroutine_pool_test.go
```

## 持续集成 (CI/CD)

### GitHub Actions 示例
```yaml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Setup Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'

    - name: Install dependencies
      run: |
        apt update
        apt install -y curl jq netcat-traditional redis-server mariadb-server

    - name: Run tests
      run: ./run-all-tests.sh
```

## 测试覆盖率

### 生成测试覆盖率报告
```bash
# 运行测试并生成覆盖率报告
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# 查看覆盖率报告
go tool cover -html=coverage.out -o coverage.html

# 查看覆盖率统计
go tool cover -func=coverage.out
```

## 故障排除

### 常见问题

1. **测试脚本找不到命令**
   ```bash
   # 安装缺失的工具
   apt install -y curl jq netcat-traditional
   ```

2. **服务启动失败**
   ```bash
   # 检查端口占用
   netstat -tlnp | grep :8080
   netstat -tlnp | grep :9090

   # 检查服务状态
   ps aux | grep redis
   ps aux | grep mysql
   ```

3. **数据库连接失败**
   ```bash
   # 检查MySQL状态
   systemctl status mariadb

   # 手动启动MySQL
   mariadbd --user=mysql --socket=/run/mysqld/mysqld.sock &
   ```

4. **Redis连接失败**
   ```bash
   # 检查Redis状态
   redis-cli ping

   # 启动Redis
   redis-server --daemonize yes
   ```

### 调试技巧

- 查看详细日志：`tail -f server.log`
- 检查API响应：使用 `curl -v` 查看完整请求响应
- 数据库调试：`mysql -u root -pMySQL@123456 -e "SHOW PROCESSLIST;"`
- Redis调试：`redis-cli monitor`

## 测试策略

### 单元测试
- 覆盖核心业务逻辑
- 使用mock替代外部依赖
- 测试边界条件和异常情况

### 集成测试
- 测试模块间协作
- 验证数据流完整性
- 确保API契约正确

### 验收测试
- 端到端功能验证
- 性能基准测试
- 生产环境模拟

## 贡献指南

1. **编写测试**: 新功能必须包含相应测试
2. **测试覆盖**: 保持代码覆盖率 > 70%
3. **CI通过**: 所有测试必须在CI环境中通过
4. **文档更新**: 测试变更需要更新相关文档

---

## 📊 测试结果解读

运行 `./run-all-tests.sh` 后的典型输出：

```
========================================
🎉 DataMiddleware 完整功能测试报告
========================================

📊 Phase功能验证结果:
✅ Phase 1: 基础框架搭建 - 100%完成
✅ Phase 2: 协议层和数据层 - 100%完成
✅ Phase 3: 业务逻辑层 - 100%完成
✅ Phase 4: 缓存和基础设施 - 100%完成
✅ Phase 5: 高并发优化 - 100%完成

🚀 项目状态: 生产就绪
📈 测试统计: 5/5 个Phase测试通过
```

**🎯 测试全部通过 = 项目功能完整，生产就绪！**
