# DataMiddleware 环境安装指南

## 概述

`setup-environment.sh` 是一个一键安装脚本，用于自动配置 DataMiddleware 项目运行所需的环境。

## 支持的系统

- Ubuntu 18.04+
- Debian 9+
- WSL (Windows Subsystem for Linux)

## 安装内容

脚本将自动安装以下组件：

- **Go 语言环境** (1.23+)
- **make 构建工具**
- **Redis 缓存服务器** (监听 6379 端口)
- **MySQL/MariaDB 数据库** (监听 3306 端口)
- **项目 Go 依赖包**

## 数据库配置

安装完成后，数据库将被配置为：

- **数据库名**: `datamiddleware`
- **用户名**: `root`
- **密码**: `MySQL@123456`
- **字符集**: `utf8mb4`

## Redis 配置

### 开发环境配置

项目提供了专门的Redis开发环境配置：

```bash
# 使用项目提供的开发环境启动脚本
./scripts/start-redis-dev.sh start

# 查看状态
./scripts/start-redis-dev.sh status

# 停止服务
./scripts/start-redis-dev.sh stop
```

### 数据文件管理

为了避免Redis数据文件污染代码库：

1. **数据文件位置**: Redis数据文件存储在 `/tmp/redis-dev/` 目录
2. **gitignore规则**: 项目已配置忽略 `*.rdb` 和 `*.aof` 文件
3. **开发配置**: 开发环境默认禁用持久化

### 配置文件

- **开发环境**: `configs/redis.conf` - 开发专用配置
- **数据隔离**: 确保数据文件不会出现在项目目录中

## 使用方法

### 基本安装

```bash
# 切换到项目根目录
cd /path/to/DataMiddleware

# 运行安装脚本
./docs/setup-environment.sh
```

### 高级选项

```bash
# 显示帮助信息
./docs/setup-environment.sh --help

# 跳过数据库安装
./docs/setup-environment.sh --skip-db

# 跳过Redis安装
./docs/setup-environment.sh --skip-redis

# 跳过验证步骤
./docs/setup-environment.sh --no-verify
```

## 安装过程

脚本执行时会：

1. 检查系统环境和权限
2. 更新包管理器
3. 安装 Go 语言环境
4. 安装 make 工具
5. 安装并配置 Redis
6. 安装并配置 MySQL/MariaDB
7. 下载项目依赖
8. 验证所有组件

## 验证安装

安装完成后，脚本会自动验证：

- ✅ Go 版本检查
- ✅ make 工具可用性
- ✅ Redis 服务运行状态
- ✅ MySQL 数据库连接
- ✅ 项目编译测试

## 后续操作

安装成功后，您可以：

```bash
# 启动服务
make run

# 开发模式（热重载）
make dev

# 运行测试
make test

# 查看所有可用命令
make help
```

## 故障排除

### 权限问题

如果遇到权限错误，请确保：

1. 当前用户有 sudo 权限
2. 不要使用 root 用户运行脚本

### 端口冲突

如果 6379 (Redis) 或 3306 (MySQL) 端口被占用：

1. 停止占用端口的服务
2. 或修改配置文件中的端口设置

### 网络问题

如果 Go 模块下载失败：

1. 检查网络连接
2. 配置 Go 代理：

```bash
export GOPROXY=https://goproxy.cn,direct
go env -w GOPROXY=https://goproxy.cn,direct
```

### 服务启动问题

在 WSL 环境中，如果 systemctl 不可用，脚本会自动使用直接启动方式。

## 配置文件

安装完成后，请检查以下配置文件：

- `configs/config.yaml` - 主要配置文件
- Redis 和 MySQL 的默认配置会自动适配

## 技术支持

如果安装过程中遇到问题，请：

1. 查看终端输出的错误信息
2. 检查系统日志：`journalctl -u redis-server` 和 `journalctl -u mariadb`
3. 提交 Issue 到项目仓库

## 许可证

本脚本遵循项目的 MIT 许可证。
