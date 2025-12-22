#!/bin/bash

# Redis数据文件清理脚本
# 用于清理开发环境中的Redis数据文件，避免污染代码库

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 默认配置
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REDIS_DATA_DIR="/tmp/redis-dev"
REDIS_PORT=6379

# 查找项目目录中的Redis文件
find_redis_files() {
    local files=()
    while IFS= read -r -d '' file; do
        files+=("$file")
    done < <(find "$PROJECT_ROOT" -name "*.rdb" -o -name "*.aof" -o -name "dump.rdb" -o -name "appendonly.aof" 2>/dev/null)

    echo "${files[@]}"
}

# 清理项目目录中的Redis文件
clean_project_redis_files() {
    log_info "扫描项目目录中的Redis数据文件..."

    local redis_files=($(find_redis_files))

    if [[ ${#redis_files[@]} -eq 0 ]]; then
        log_success "项目目录中没有发现Redis数据文件"
        return 0
    fi

    log_warn "发现 ${#redis_files[@]} 个Redis数据文件:"

    for file in "${redis_files[@]}"; do
        local size=$(du -h "$file" 2>/dev/null | cut -f1)
        echo "  - $file ($size)"
    done

    echo
    read -p "是否删除这些文件? (y/N): " -r
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        for file in "${redis_files[@]}"; do
            rm -f "$file"
            log_info "已删除: $file"
        done
        log_success "Redis数据文件清理完成"
    else
        log_info "跳过文件删除"
    fi
}

# 清理临时Redis数据目录
clean_temp_redis_data() {
    if [[ -d "$REDIS_DATA_DIR" ]]; then
        log_info "清理临时Redis数据目录: $REDIS_DATA_DIR"

        local size=$(du -sh "$REDIS_DATA_DIR" 2>/dev/null | cut -f1)
        log_info "数据目录大小: $size"

        read -p "是否删除临时数据目录? (y/N): " -r
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$REDIS_DATA_DIR"
            log_success "临时Redis数据目录已清理"
        else
            log_info "保留临时数据目录"
        fi
    else
        log_info "临时Redis数据目录不存在: $REDIS_DATA_DIR"
    fi
}

# 检查Redis服务状态
check_redis_status() {
    log_info "检查Redis服务状态..."

    if redis-cli -p $REDIS_PORT ping &>/dev/null; then
        log_success "Redis服务运行正常 (端口: $REDIS_PORT)"
        return 0
    else
        log_warn "Redis服务未运行 (端口: $REDIS_PORT)"
        return 1
    fi
}

# 显示帮助信息
show_help() {
    echo "Redis数据文件清理工具"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -p, --project    清理项目目录中的Redis文件"
    echo "  -t, --temp       清理临时Redis数据目录"
    echo "  -a, --all        清理所有Redis相关文件"
    echo "  -s, --status     检查Redis服务状态"
    echo "  -h, --help       显示帮助信息"
    echo ""
    echo "默认行为: 显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 -a          # 清理所有Redis文件"
    echo "  $0 -p -t       # 分别清理项目和临时文件"
    echo "  $0 -s          # 检查Redis状态"
}

# 主函数
main() {
    if [[ $# -eq 0 ]]; then
        show_help
        exit 0
    fi

    while [[ $# -gt 0 ]]; do
        case $1 in
            -p|--project)
                clean_project_redis_files
                shift
                ;;
            -t|--temp)
                clean_temp_redis_data
                shift
                ;;
            -a|--all)
                clean_project_redis_files
                clean_temp_redis_data
                shift
                ;;
            -s|--status)
                check_redis_status
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                echo "使用 '$0 --help' 查看帮助信息"
                exit 1
                ;;
        esac
    done

    log_success "Redis数据文件清理任务完成"
}

# 执行主函数
main "$@"
