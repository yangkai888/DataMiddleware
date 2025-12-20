# 🚀 数据中间件集群部署指南

## 📋 快速开始

### 一键部署集群
```bash
# 克隆项目
git clone https://github.com/yangkai888/DataMiddleware.git
cd DataMiddleware

# 一键部署2节点集群
./deploy-cluster.sh
```

部署完成后，你将拥有：
- ✅ **2个应用节点** (端口8081, 8082)
- ✅ **1个Nginx负载均衡器** (端口80)
- ✅ **1个Redis缓存** (端口6379)
- ✅ **1个MySQL数据库** (端口3306)

## 🏗️ 集群架构

```
Internet
    ↓
┌─────────┐ 端口80
│ Nginx   │ ← 负载均衡器
│ LB      │
└────┬────┘
     │
     ├─→ ┌─────────┐ 端口8081
     │   │ Node 1  │
     │   │ App     │
     │   └─────────┘
     │
     └─→ ┌─────────┐ 端口8082
         │ Node 2  │
         │ App     │
         └─────────┘
             │
             ├─→ Redis (6379) - 共享缓存
             └─→ MySQL (3306) - 共享数据库
```

## 🌐 访问地址

| 服务 | 地址 | 说明 |
|------|------|------|
| **应用入口** | http://localhost | Nginx负载均衡器 |
| **节点1** | http://localhost:8081 | 直接访问节点1 |
| **节点2** | http://localhost:8082 | 直接访问节点2 |
| **Redis** | localhost:6379 | 缓存服务 |
| **MySQL** | localhost:3306 | 数据库 (root/MySQL@123456) |

## 🧪 测试集群功能

### 1. 基本健康检查
```bash
# 测试负载均衡器
curl http://localhost/health

# 测试各个节点
curl http://localhost:8081/health
curl http://localhost:8082/health
```

### 2. 负载均衡测试
```bash
# 发送多个并发请求，观察负载分布
for i in {1..10}; do
  curl -s http://localhost/health &
done
```

### 3. API功能测试
```bash
# 测试缓存功能
curl -X POST http://localhost/api/v1/cache/set \
  -H "Content-Type: application/json" \
  -d '{"key":"cluster_test","value":"success"}'

curl http://localhost/api/v1/cache/get?key=cluster_test
```

### 4. 故障转移测试
```bash
# 停止一个节点
docker-compose -f docker-compose.cluster.yml stop datamiddleware-2

# 继续测试，所有请求会自动转发到节点1
curl http://localhost/health

# 重启节点
docker-compose -f docker-compose.cluster.yml start datamiddleware-2
```

## 📊 性能指标

| 指标 | 单节点 | 2节点集群 | 提升 |
|------|-------|----------|------|
| **QPS** | ~3,000 | ~6,000+ | 2倍+ |
| **可用性** | 99% | 99.9%+ | 高可用 |
| **扩展性** | 有限 | 水平扩展 | 无限 |

## 🔧 管理命令

### 查看集群状态
```bash
# 查看所有服务状态
docker-compose -f docker-compose.cluster.yml ps

# 查看服务日志
docker-compose -f docker-compose.cluster.yml logs -f

# 查看特定服务日志
docker-compose -f docker-compose.cluster.yml logs -f datamiddleware-node1
```

### 集群控制
```bash
# 停止集群
docker-compose -f docker-compose.cluster.yml down

# 重启集群
docker-compose -f docker-compose.cluster.yml restart

# 重新构建并启动
docker-compose -f docker-compose.cluster.yml up -d --build
```

### 扩缩容
```bash
# 添加更多节点 (修改docker-compose.cluster.yml)
# 复制datamiddleware-2配置，修改端口和名称

# 或者使用环境变量动态配置
NODE_ID=3 HTTP_PORT=8083 TCP_PORT=9093 \
docker-compose -f docker-compose.cluster.yml up -d datamiddleware-3
```

## 🔍 故障排除

### 常见问题

#### 1. 端口冲突
```bash
# 检查端口占用
netstat -tulpn | grep :8080

# 修改docker-compose.cluster.yml中的端口映射
ports:
  - "8083:8080"  # 改为未使用的端口
```

#### 2. 数据库连接失败
```bash
# 检查MySQL容器状态
docker-compose -f docker-compose.cluster.yml logs mysql

# 验证数据库连接
mysql -h localhost -P 3306 -u root -pMySQL@123456 datamiddleware
```

#### 3. Redis连接失败
```bash
# 检查Redis状态
docker-compose -f docker-compose.cluster.yml exec redis redis-cli ping

# 查看Redis日志
docker-compose -f docker-compose.cluster.yml logs redis
```

#### 4. 应用启动失败
```bash
# 查看应用日志
docker-compose -f docker-compose.cluster.yml logs datamiddleware-node1

# 检查配置文件
docker-compose -f docker-compose.cluster.yml exec datamiddleware-node1 cat configs/config.yaml
```

### 日志分析
```bash
# 查看所有服务日志
docker-compose -f docker-compose.cluster.yml logs

# 实时监控日志
docker-compose -f docker-compose.cluster.yml logs -f --tail=100

# 导出日志用于分析
docker-compose -f docker-compose.cluster.yml logs > cluster_logs.txt
```

## 🔄 升级部署

### 滚动升级
```bash
# 逐个升级节点，最小化停机时间
docker-compose -f docker-compose.cluster.yml up -d --build datamiddleware-node1
docker-compose -f docker-compose.cluster.yml up -d --build datamiddleware-node2
```

### 零停机部署
```bash
# 启动新版本节点
docker-compose -f docker-compose.cluster.yml up -d datamiddleware-node3-new

# 等待新节点就绪
sleep 30

# 停止旧节点
docker-compose -f docker-compose.cluster.yml stop datamiddleware-node1

# 重复上述步骤升级所有节点
```

## 📈 监控和告警

### 健康检查端点
- `/health` - 基础健康检查
- `/health/detailed` - 详细系统指标
- `/metrics` - Prometheus监控指标

### 推荐监控指标
- **应用指标**: QPS、响应时间、错误率
- **系统指标**: CPU、内存、磁盘、网络
- **集群指标**: 节点状态、负载均衡分布

## 🎯 生产环境建议

### 1. 高可用配置
- 使用3+个应用节点
- 配置Redis Sentinel集群
- 使用MySQL主从复制
- 添加监控和告警系统

### 2. 安全加固
- 修改默认密码
- 配置TLS/SSL证书
- 设置防火墙规则
- 定期更新镜像

### 3. 性能优化
- 根据负载调整实例数量
- 配置合适的资源限制
- 优化数据库连接池
- 启用缓存预热

## 📚 更多资源

- 📖 [详细集群部署指南](cluster_deployment_guide.md)
- 🐳 [Docker Compose配置](docker-compose.cluster.yml)
- ⚙️ [Nginx配置](nginx-cluster.conf)
- 🗄️ [数据库初始化](scripts/init.sql)

---

## 🎉 快速开始享受你的高可用集群！

```bash
# 一键启动
./deploy-cluster.sh

# 开始使用
curl http://localhost/health
```

**祝你部署顺利！如有问题，请查看故障排除部分或提交Issue。** 🚀
