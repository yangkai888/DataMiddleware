# 数据中间件集群部署指南

## 📋 当前状态分析

### ✅ 已支持集群的基础特性
- **多实例运行**: 应用本身可以启动多个实例
- **数据库读写分离**: 支持主从数据库配置
- **Redis缓存**: 支持Redis集群模式
- **负载均衡就绪**: HTTP/TCP接口支持反向代理

### ❌ 当前不支持的集群特性
- 服务发现和注册
- 分布式会话管理
- 配置中心
- 分布式锁
- 健康检查和自动扩缩容

## 🏗️ 集群部署架构

### 推荐的集群架构

```
┌─────────────────┐    ┌─────────────────┐
│   Load Balancer │    │   Load Balancer │
│   (Nginx/HAProxy)│    │   (Nginx/HAProxy)│
└─────────┬───────┘    └─────────┬───────┘
          │                       │
          └─────────┬─────────────┘
                    │
        ┌───────────┼───────────┐
        │  Data Middleware      │
        │  Cluster Nodes        │
        │                       │
        │  ┌─────────┐ ┌─────┐  │
        │  │ Node 1  │ │ ... │  │
        │  │ Node 2  │ │ N   │  │
        │  └─────────┘ └─────┘  │
        └───────────────────────┘
                    │
        ┌───────────┼───────────┐
        │  Shared Infrastructure │
        │                       │
        │  ┌─────────┐ ┌─────┐  │
        │  │ Redis   │ │ DB  │  │
        │  │ Cluster │ │ HA  │  │
        │  └─────────┘ └─────┘  │
        └───────────────────────┘
```

## 🚀 集群部署方案

### 方案1: Docker Compose集群 (推荐用于开发/测试)

#### 1. 创建docker-compose.yml
```yaml
version: '3.8'

services:
  # 数据中间件节点1
  datamiddleware-1:
    build: .
    container_name: datamiddleware-1
    environment:
      - NODE_ID=1
      - HTTP_PORT=8081
      - TCP_PORT=9091
      - REDIS_HOST=redis
      - DB_HOST=mysql
    ports:
      - "8081:8080"
      - "9091:9090"
    depends_on:
      - redis
      - mysql
    networks:
      - datamiddleware-net

  # 数据中间件节点2
  datamiddleware-2:
    build: .
    container_name: datamiddleware-2
    environment:
      - NODE_ID=2
      - HTTP_PORT=8082
      - TCP_PORT=9092
      - REDIS_HOST=redis
      - DB_HOST=mysql
    ports:
      - "8082:8080"
      - "9092:9090"
    depends_on:
      - redis
      - mysql
    networks:
      - datamiddleware-net

  # Redis集群
  redis:
    image: redis:7-alpine
    container_name: datamiddleware-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    networks:
      - datamiddleware-net

  # MySQL主库
  mysql:
    image: mysql:8.0
    container_name: datamiddleware-mysql
    environment:
      MYSQL_ROOT_PASSWORD: MySQL@123456
      MYSQL_DATABASE: datamiddleware
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
    networks:
      - datamiddleware-net

  # Nginx负载均衡器
  nginx:
    image: nginx:alpine
    container_name: datamiddleware-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - datamiddleware-1
      - datamiddleware-2
    networks:
      - datamiddleware-net

volumes:
  redis_data:
  mysql_data:

networks:
  datamiddleware-net:
    driver: bridge
```

#### 2. 创建Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o datamiddleware ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/datamiddleware .
COPY --from=builder /app/configs ./configs
CMD ["./datamiddleware"]
```

#### 3. 创建Nginx配置文件
```nginx
events {
    worker_connections 1024;
}

http {
    upstream datamiddleware_backend {
        server datamiddleware-1:8080;
        server datamiddleware-2:8080;
    }

    server {
        listen 80;
        
        location / {
            proxy_pass http://datamiddleware_backend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
        
        # 健康检查
        location /health {
            proxy_pass http://datamiddleware_backend;
        }
    }
}
```

#### 4. 部署命令
```bash
# 构建并启动集群
docker-compose up -d --build

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 停止集群
docker-compose down
```

### 方案2: Kubernetes集群部署 (生产环境推荐)

#### 1. 创建Kubernetes部署文件

**datamiddleware-deployment.yaml**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: datamiddleware
spec:
  replicas: 3
  selector:
    matchLabels:
      app: datamiddleware
  template:
    metadata:
      labels:
        app: datamiddleware
    spec:
      containers:
      - name: datamiddleware
        image: your-registry/datamiddleware:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: tcp
        env:
        - name: REDIS_HOST
          value: "redis-cluster"
        - name: DB_HOST
          value: "mysql-cluster"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

**datamiddleware-service.yaml**
```yaml
apiVersion: v1
kind: Service
metadata:
  name: datamiddleware-service
spec:
  selector:
    app: datamiddleware
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: tcp
    port: 9090
    targetPort: 9090
  type: LoadBalancer
```

#### 2. 部署到Kubernetes
```bash
# 部署应用
kubectl apply -f datamiddleware-deployment.yaml
kubectl apply -f datamiddleware-service.yaml

# 查看部署状态
kubectl get pods
kubectl get services

# 查看日志
kubectl logs -f deployment/datamiddleware

# 扩缩容
kubectl scale deployment datamiddleware --replicas=5
```

### 方案3: 传统服务器集群部署

#### 1. 服务器准备
```bash
# 假设有3台服务器: node1, node2, node3
# 每台服务器上部署一个应用实例

# 在每台服务器上:
git clone https://github.com/yangkai888/DataMiddleware.git
cd DataMiddleware
make build-linux

# 创建配置文件 (为每个节点设置不同端口)
cp configs/config.yaml configs/config-node1.yaml
# 修改端口配置...
```

#### 2. 使用Supervisor管理进程
```ini
# /etc/supervisor/conf.d/datamiddleware.conf
[program:datamiddleware]
directory=/opt/datamiddleware
command=/opt/datamiddleware/datamiddleware_unix
autostart=true
autorestart=true
stdout_logfile=/var/log/datamiddleware.log
stderr_logfile=/var/log/datamiddleware.err
environment=NODE_ID=1,HTTP_PORT=8081,TCP_PORT=9091
```

#### 3. Nginx负载均衡配置
```nginx
upstream datamiddleware_cluster {
    server node1:8081;
    server node2:8082;
    server node3:8083;
}

server {
    listen 80;
    location / {
        proxy_pass http://datamiddleware_cluster;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## ⚠️ 重要注意事项

### 当前限制
1. **无服务发现**: 节点间无法自动发现对方
2. **无分布式锁**: 无法协调分布式操作
3. **会话不共享**: 用户会话无法在节点间共享
4. **配置不统一**: 各节点配置需要手动同步

### 扩展建议
要实现完整的集群功能，建议后续开发：
1. **服务注册中心** (如Consul, etcd)
2. **分布式配置中心** (如Apollo, Nacos)
3. **分布式锁** (如Redis, ZooKeeper)
4. **会话共享** (如Redis Session Store)

## 🧪 集群功能测试

### 测试负载均衡
```bash
# 并发请求测试
for i in {1..100}; do
  curl -s "http://localhost/health" &
done

# 查看各节点日志，确认请求分发
docker-compose logs datamiddleware-1
docker-compose logs datamiddleware-2
```

### 测试故障转移
```bash
# 停止一个节点
docker-compose stop datamiddleware-1

# 继续发送请求，确认其他节点正常工作
curl -s "http://localhost/health"
```

## 📊 集群性能预期

| 部署规模 | QPS预期 | 内存使用 | CPU使用 |
|---------|--------|---------|-------|
| 单机 | 3,000+ | 256MB | 30% |
| 3节点集群 | 8,000+ | 768MB | 45% |
| 5节点集群 | 15,000+ | 1.2GB | 60% |
| 10节点集群 | 25,000+ | 2.5GB | 70% |

## 🎯 总结

**当前状态**: 支持基础的多实例部署，但缺少完整的集群协调功能

**推荐方案**: 
- **开发/测试环境**: 使用Docker Compose集群方案
- **生产环境**: 使用Kubernetes进行容器化集群部署

**扩展建议**: 如需完整的集群功能，建议后续开发服务发现、配置中心等分布式组件。

**立即可用的**: 通过上述方案，可以快速搭建一个基础的负载均衡集群，显著提升整体性能和可用性。
