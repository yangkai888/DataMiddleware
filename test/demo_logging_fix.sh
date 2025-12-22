#!/bin/bash

# 日志输出修复演示脚本
# 展示修复前后的日志输出差异

set -e

echo "🐛 DataMiddleware 日志输出问题修复演示"
echo "========================================"
echo

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY_PATH="$PROJECT_ROOT/bin/datamiddleware"

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[演示]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[成功]${NC} $1"
}

log_error() {
    echo -e "${RED}[问题]${NC} $1"
}

log_fix() {
    echo -e "${GREEN}[修复]${NC} $1"
}

# 检查二进制文件
if [[ ! -f "$BINARY_PATH" ]]; then
    log_error "DataMiddleware二进制文件不存在: $BINARY_PATH"
    echo "请先运行: go build -o bin/datamiddleware ./cmd/server"
    exit 1
fi

echo "📋 问题描述:"
echo "  测试脚本中设置了 DATAMIDDLEWARE_LOGGING_LEVEL=info"
echo "  这覆盖了配置文件中的 logger.level: debug"
echo "  导致DEBUG级别的详细日志被过滤掉"
echo

echo "🔍 日志级别说明:"
echo "  DEBUG: 显示所有日志，包括SQL查询、内部状态"
echo "  INFO:  只显示重要事件和服务状态"
echo "  WARN/ERROR: 只显示警告和错误"
echo

echo "🧪 演示日志输出差异:"
echo

# 演示1: 默认配置 (debug级别)
echo "1️⃣ 默认启动 (debug级别 - 配置文件设置):"
echo "   命令: ./bin/datamiddleware"
echo "   预期: 显示DEBUG、INFO、WARN、ERROR级别日志"
echo
timeout 3s $BINARY_PATH 2>&1 | head -12 | while read line; do
    if [[ $line == *"DEBUG"* ]]; then
        echo -e "   ${GREEN}DEBUG${NC}: $line"
    elif [[ $line == *"INFO"* ]]; then
        echo -e "   ${BLUE}INFO${NC}: $line"
    else
        echo -e "   $line"
    fi
done
echo

# 演示2: 环境变量覆盖为info级别 (问题重现)
echo "2️⃣ 环境变量设置为info级别 (问题重现):"
echo "   命令: DATAMIDDLEWARE_LOGGING_LEVEL=info ./bin/datamiddleware"
echo "   预期: 只显示INFO级别以上的日志，DEBUG日志被过滤"
echo
DATAMIDDLEWARE_LOGGING_LEVEL=info timeout 3s $BINARY_PATH 2>&1 | head -8 | while read line; do
    if [[ $line == *"DEBUG"* ]]; then
        echo -e "   ${RED}DEBUG (被过滤)${NC}: $line"
    elif [[ $line == *"INFO"* ]]; then
        echo -e "   ${BLUE}INFO${NC}: $line"
    else
        echo -e "   $line"
    fi
done
echo

# 演示3: 修复后的测试脚本
echo "3️⃣ 修复后的测试脚本 (debug级别):"
echo "   修复内容: export DATAMIDDLEWARE_LOGGING_LEVEL=debug"
echo "   预期: 现在会显示完整的DEBUG级别日志"
echo
DATAMIDDLEWARE_LOGGING_LEVEL=debug timeout 3s $BINARY_PATH 2>&1 | head -15 | while read line; do
    if [[ $line == *"DEBUG"* ]]; then
        echo -e "   ${GREEN}DEBUG${NC}: $line"
    elif [[ $line == *"INFO"* ]]; then
        echo -e "   ${BLUE}INFO${NC}: $line"
    else
        echo -e "   $line"
    fi
done
echo

echo "✅ 修复总结:"
echo "   🔧 问题: 测试脚本的环境变量覆盖了配置文件"
echo "   🛠️ 修复: 将环境变量从info改为debug级别"
echo "   📊 结果: 现在可以正常显示详细的程序日志"
echo

echo "📚 相关文件:"
echo "   📄 详细解释: docs/logging_issue_explanation.md"
echo "   🔧 修复脚本: test/limit_performance_test.sh"
echo "   🔧 修复脚本: test/functionality_comprehensive_test.sh"
echo

log_success "演示完成！现在您可以在测试过程中看到完整的程序日志了。"
