#!/bin/bash

# Redis开发环境启动脚本
# 使用项目配置文件启动Redis，避免污染代码库

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
REDIS_CONF="configs/redis.conf"
REDIS_PORT=6379
REDIS_DATA_DIR="/tmp/redis-dev"

# 检查Redis是否已安装
check_redis() {
    if ! command -v redis-server &> /dev/null; then
        log_error "Redis未安装，请先安装Redis："
        echo "  Ubuntu/Debian: sudo apt install redis-server"
        echo "  CentOS/RHEL: sudo yum install redis"
        exit 1
    fi
}

# 创建数据目录
setup_data_dir() {
    if [[ ! -d "$REDIS_DATA_DIR" ]]; then
        mkdir -p "$REDIS_DATA_DIR"
        log_info "创建Redis数据目录: $REDIS_DATA_DIR"
    fi
}

# 检查端口是否被占用
check_port() {
    if lsof -Pi :$REDIS_PORT -sTCP:LISTEN -t >/dev/null; then
        log_error "端口 $REDIS_PORT 已被占用"
        echo "请检查是否有其他Redis实例在运行："
        echo "  ps aux | grep redis"
        echo "  sudo netstat -tlnp | grep :$REDIS_PORT"
        exit 1
    fi
}

# 启动Redis
start_redis() {
    log_info "启动Redis开发服务器..."
    log_info "配置文件: $REDIS_CONF"
    log_info "数据目录: $REDIS_DATA_DIR"
    log_info "端口: $REDIS_PORT"

    # 设置环境变量，确保Redis数据文件不会在项目目录生成
    export REDIS_DATA_DIR="$REDIS_DATA_DIR"

    # 启动Redis服务器
    redis-server "$REDIS_CONF" \
        --port $REDIS_PORT \
        --dir "$REDIS_DATA_DIR" \
        --dbfilename "dump-dev.rdb" \
        --logfile "$REDIS_DATA_DIR/redis-dev.log"

    # 等待Redis启动
    sleep 2

    # 验证Redis是否启动成功
    if redis-cli -p $REDIS_PORT ping &>/dev/null; then
        log_success "Redis开发服务器启动成功"
        log_info "连接信息: redis://localhost:$REDIS_PORT"
        log_info "数据文件位置: $REDIS_DATA_DIR/dump-dev.rdb"
        log_info "日志文件位置: $REDIS_DATA_DIR/redis-dev.log"
    else
        log_error "Redis启动失败"
        exit 1
    fi
}

# 停止Redis
stop_redis() {
    log_info "停止Redis开发服务器..."
    if redis-cli -p $REDIS_PORT shutdown &>/dev/null; then
        log_success "Redis开发服务器已停止"
    else
        log_warn "Redis可能未在运行"
    fi
}

# 显示帮助信息
show_help() {
    echo "Redis开发环境管理脚本"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  start    启动Redis开发服务器"
    echo "  stop     停止Redis开发服务器"
    echo "  restart  重启Redis开发服务器"
    echo "  status   查看Redis状态"
    echo "  clean    清理Redis数据文件"
    echo "  help     显示帮助信息"
    echo ""
    echo "配置:"
    echo "  配置文件: $REDIS_CONF"
    echo "  数据目录: $REDIS_DATA_DIR"
    echo "  端口: $REDIS_PORT"
}

# 主函数
main() {
    case "${1:-start}" in
        start)
            check_redis
            setup_data_dir
            check_port
            start_redis
            ;;
        stop)
            stop_redis
            ;;
        restart)
            stop_redis
            sleep 2
            check_redis
            setup_data_dir
            check_port
            start_redis
            ;;
        status)
            if redis-cli -p $REDIS_PORT ping &>/dev/null; then
                log_success "Redis运行正常 (端口: $REDIS_PORT)"
                redis-cli -p $REDIS_PORT info server | grep -E "(redis_version|uptime_in_seconds|connected_clients)"
            else
                log_error "Redis未运行 (端口: $REDIS_PORT)"
            fi
            ;;
        clean)
            log_info "清理Redis数据文件..."
            rm -rf "$REDIS_DATA_DIR"
            log_success "Redis数据文件已清理"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知命令: $1"
            echo "使用 '$0 help' 查看可用命令"
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"
