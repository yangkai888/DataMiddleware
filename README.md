# 数据中间件 (DataMiddleware)

基于Go语言开发的高性能数据中间件系统，支持多游戏数据处理和分发。

## 功能特性

- 🚀 高性能TCP/HTTP服务器
- 🎮 多游戏数据路由
- 📊 数据库连接池管理
- 💾 多级缓存体系
- 🔐 JWT用户认证
- 📈 性能监控和健康检查
- 🐳 容器化部署支持

## 快速开始

### 环境要求

- Go 1.21+
- Linux/macOS/Windows

### 安装依赖

```bash
make deps
```

### 运行服务

```bash
make run
```

### 开发模式（热重载）

```bash
make dev
```

## 项目结构

```
datamiddleware/
├── cmd/server/          # 主程序入口
├── internal/            # 内部包
│   ├── config/          # 配置管理
│   ├── logger/          # 日志系统
│   ├── errors/          # 错误处理
│   └── utils/           # 工具库
├── pkg/                 # 公共包
│   ├── types/           # 类型定义
│   └── constants/       # 常量定义
├── configs/             # 配置文件
├── scripts/             # 构建脚本
├── docs/                # 项目文档
└── test/                # 测试文件
```

## 配置说明

配置文件位于 `configs/` 目录下，支持YAML格式。

## 构建和部署

### 本地构建

```bash
make build
```

### 交叉编译（Linux）

```bash
make build-linux
```

### 运行测试

```bash
make test
```

### 生成测试覆盖率报告

```bash
make test-coverage
```

## 开发规范

- 代码格式化：`make fmt`
- 代码检查：`make lint`
- 提交前请确保测试通过

## 许可证

[MIT License](LICENSE)

## 贡献

欢迎提交Issue和Pull Request！

## 联系我们

如有问题，请通过Issue联系我们。
