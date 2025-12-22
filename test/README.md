# 测试目录结构说明

## 概述

本目录包含数据中间件项目的完整测试套件，按照功能和类型进行了合理的组织。

## 目录结构

```
test/
├── async/                 # 异步处理测试
│   └── async_test.go     # 异步任务队列和调度器测试
├── benchmarks/           # 性能基准测试
│   ├── performance_test.go          # 完整性能测试套件
│   ├── performance_benchmark.go     # 性能基准测试
│   └── high_concurrency_benchmark.go # 高并发基准测试
├── concurrency/          # 并发测试
│   ├── ultimate_concurrency_test.go # 终极并发测试
│   ├── run_concurrency_test.go     # 并发测试运行器
│   ├── simple_perf.go              # 简单性能测试
│   └── stability_test.sh           # 稳定性测试脚本
├── demos/                # 演示和示例代码
│   ├── async_demo.go               # 异步处理演示
│   ├── benchmark_demo.go           # 基准测试演示
│   ├── goroutine_pool_demo.go      # 协程池演示
│   └── memory_demo.go              # 内存优化演示
├── phases/               # 开发阶段测试（历史记录）
│   ├── phase3_*.go/md/sh           # 第三阶段测试
│   ├── phase4_*.go/md/sh           # 第四阶段测试
│   └── phase5_*.go/md/sh           # 第五阶段测试
├── tcp/                  # TCP协议测试
│   ├── tcp_test.go                 # TCP协议基础测试
│   ├── tcp_client_test.go          # TCP客户端测试
│   └── tcp_performance_test.go     # TCP性能测试
├── e2e/                  # 端到端测试（预留）
├── integration/          # 集成测试（预留）
└── unit/                 # 单元测试（预留）
```

## 测试分类说明

### 🔄 async - 异步处理测试
- **用途**: 测试异步任务队列、优先级调度、错误处理
- **文件**: `async_test.go` - 异步功能完整验证

### 📊 benchmarks - 性能基准测试
- **用途**: 提供标准化的性能基准测试
- **文件**:
  - `performance_test.go` - 完整的性能测试套件
  - `performance_benchmark.go` - 性能基准测试
  - `high_concurrency_benchmark.go` - 高并发场景测试

### ⚡ concurrency - 并发测试
- **用途**: 测试并发处理能力、资源竞争、死锁预防
- **文件**:
  - `ultimate_concurrency_test.go` - 极限并发测试
  - `run_concurrency_test.go` - 并发测试执行器
  - `simple_perf.go` - 基础性能验证
  - `stability_test.sh` - 长时间稳定性测试

### 🎬 demos - 演示和示例
- **用途**: 展示各项功能的使用方法和最佳实践
- **文件**: 各种功能演示代码，适合学习和参考

### 📚 phases - 开发阶段记录
- **用途**: 保存各开发阶段的测试和验证记录
- **内容**: 按phase3/4/5组织的开发历史
- **说明**: 这些是项目开发过程中的重要记录，保留用于追溯

### 🔌 tcp - TCP协议测试
- **用途**: 测试TCP二进制协议的编解码、连接管理、心跳机制
- **文件**:
  - `tcp_test.go` - 基础TCP协议测试
  - `tcp_client_test.go` - TCP客户端功能测试
  - `tcp_performance_test.go` - TCP性能和压力测试

### 🧪 e2e/integration/unit - 标准测试目录
- **用途**: 预留给标准的端到端测试、集成测试、单元测试
- **状态**: 当前为空，未来可按需填充

## 使用指南

### 运行特定类型测试

```bash
# 异步测试
go run test/async/async_test.go

# 性能基准测试
go run test/benchmarks/performance_test.go

# TCP协议测试
go run test/tcp/tcp_test.go

# 并发测试
go run test/concurrency/ultimate_concurrency_test.go
```

### 运行演示代码

```bash
# 查看各项功能演示
go run test/demos/async_demo.go
go run test/demos/memory_demo.go
```

### 运行阶段测试

```bash
# 运行特定阶段的测试脚本
bash test/phases/phase5_validation_test.sh
```

## 注意事项

1. **演示文件**: `demos/` 目录下的文件主要用于学习和演示，不是正式测试
2. **阶段文件**: `phases/` 目录保存历史记录，可用于问题追溯
3. **性能测试**: benchmarks和concurrency目录下的测试可能消耗较多资源
4. **重复文件**: 已清理所有重复文件，每个功能保留最佳实现版本

## 维护建议

- 新增测试时请按照功能分类放入相应目录
- 定期清理不再需要的演示文件
- 性能测试文件定期更新以反映最新功能
- 阶段文件可按需归档到历史记录中
