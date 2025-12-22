#!/bin/bash

# DataMiddleware 项目一键安装脚本
# 用于自动安装和配置项目运行所需的环境
# 支持: Ubuntu/Debian 系统

set -e  # 遇到错误立即退出

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

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查命令是否存在
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 检查是否为root用户
check_root() {
    if [[ $EUID -eq 0 ]]; then
        log_error "请不要使用root用户运行此脚本"
        exit 1
    fi
}

# 更新包管理器
update_packages() {
    log_info "更新包管理器..."
    sudo apt update
    sudo apt upgrade -y
}

# 安装Go语言环境
install_go() {
    if command_exists go; then
        local current_version=$(go version | awk '{print $3}' | sed 's/go//')
        log_info "Go 已安装，版本: $current_version"
        return
    fi

    log_info "安装Go语言环境..."
    sudo apt install -y golang-go

    # 验证安装
    if command_exists go; then
        local version=$(go version)
        log_success "Go 安装成功: $version"
    else
        log_error "Go 安装失败"
        exit 1
    fi
}

# 安装make工具
install_make() {
    if command_exists make; then
        log_info "make 已安装"
        return
    fi

    log_info "安装make工具..."
    sudo apt install -y make
    log_success "make 安装成功"
}

# 安装Redis
install_redis() {
    if command_exists redis-server && pgrep -x "redis-server" >/dev/null; then
        log_info "Redis 已安装并运行"
        return
    fi

    log_info "安装Redis服务器..."
    sudo apt install -y redis-server

    # 启动Redis服务
    log_info "启动Redis服务..."
    if command_exists systemctl; then
        sudo systemctl start redis-server 2>/dev/null || true
        sudo systemctl enable redis-server 2>/dev/null || true
    else
        # WSL环境下直接启动
        redis-server --daemonize yes
    fi

    # 验证Redis
    sleep 2
    if redis-cli ping | grep -q "PONG"; then
        log_success "Redis 安装并启动成功"
    else
        log_error "Redis 启动失败"
        exit 1
    fi
}

# 安装MySQL/MariaDB
install_mysql() {
    if command_exists mysql && pgrep -f "mariadbd\|mysqld" >/dev/null; then
        log_info "MySQL/MariaDB 已安装并运行"
        return
    fi

    log_info "安装MariaDB服务器..."
    sudo apt install -y mariadb-server

    # 启动MariaDB服务
    log_info "启动MariaDB服务..."
    if command_exists systemctl; then
        sudo systemctl start mariadb 2>/dev/null || true
        sudo systemctl enable mariadb 2>/dev/null || true
    else
        # WSL环境下直接启动
        if command_exists mariadbd; then
            mariadbd --user=mysql --socket=/run/mysqld/mysqld.sock &
            sleep 3
        fi
    fi

    # 配置数据库
    log_info "配置数据库..."
    setup_database

    log_success "MySQL/MariaDB 安装并配置成功"
}

# 配置数据库
setup_database() {
    # 停止服务以安全模式启动
    pkill mariadbd 2>/dev/null || true
    sleep 2

    # 以跳过权限表模式启动
    log_info "重置数据库root密码..."
    mariadbd --user=mysql --skip-grant-tables --socket=/run/mysqld/mysqld.sock &
    sleep 3

    # 重置密码并创建数据库
    mysql -u root --socket=/run/mysqld/mysqld.sock -e "
        FLUSH PRIVILEGES;
        ALTER USER 'root'@'localhost' IDENTIFIED BY 'MySQL@123456';
        CREATE DATABASE IF NOT EXISTS datamiddleware;
    " 2>/dev/null || true

    # 停止安全模式
    pkill mariadbd
    sleep 2

    # 正常启动服务
    mariadbd --user=mysql --socket=/run/mysqld/mysqld.sock &
    sleep 3

    # 验证数据库连接
    if mysql -u root -pMySQL@123456 -e "SELECT 1;" >/dev/null 2>&1; then
        log_success "数据库配置成功"
    else
        log_error "数据库配置失败"
        exit 1
    fi
}

# 安装Go项目依赖
install_go_dependencies() {
    log_info "安装Go项目依赖..."

    cd "$(dirname "$0")/.."

    if [[ ! -f "go.mod" ]]; then
        log_error "未找到go.mod文件，请确保在项目根目录运行此脚本"
        exit 1
    fi

    go mod download
    go mod tidy

    log_success "Go依赖安装成功"
}

# 验证安装
verify_installation() {
    log_info "验证安装..."

    local errors=0

    # 检查Go
    if ! command_exists go; then
        log_error "Go 未安装"
        ((errors++))
    else
        log_success "Go: $(go version)"
    fi

    # 检查make
    if ! command_exists make; then
        log_error "make 未安装"
        ((errors++))
    else
        log_success "make: 已安装"
    fi

    # 检查Redis
    if ! redis-cli ping | grep -q "PONG"; then
        log_error "Redis 未运行"
        ((errors++))
    else
        log_success "Redis: 运行中"
    fi

    # 检查MySQL
    if ! mysql -u root -pMySQL@123456 -e "SELECT 1;" >/dev/null 2>&1; then
        log_error "MySQL 连接失败"
        ((errors++))
    else
        log_success "MySQL: 连接正常"
    fi

    # 检查项目编译
    if ! go build -v ./cmd/server >/dev/null 2>&1; then
        log_error "项目编译失败"
        ((errors++))
    else
        log_success "项目: 编译成功"
    fi

    if [[ $errors -eq 0 ]]; then
        log_success "所有组件验证通过！"
        echo
        echo "========================================"
        echo "🎉 环境安装完成！"
        echo "========================================"
        echo "现在您可以运行以下命令："
        echo "  make run      # 启动服务"
        echo "  make dev      # 开发模式（热重载）"
        echo "  make test     # 运行测试"
        echo "========================================"
    else
        log_error "安装验证失败，请检查上述错误信息"
        exit 1
    fi
}

# 显示帮助信息
show_help() {
    cat << EOF
DataMiddleware 环境一键安装脚本

用法:
    $0 [选项]

选项:
    -h, --help      显示此帮助信息
    --no-verify     跳过安装验证
    --skip-db       跳过数据库安装
    --skip-redis    跳过Redis安装

示例:
    $0              # 完整安装
    $0 --skip-db    # 跳过数据库安装

此脚本将安装：
- Go 语言环境
- make 构建工具
- Redis 缓存服务器
- MySQL/MariaDB 数据库
- 项目Go依赖包

EOF
}

# 主函数
main() {
    local skip_verify=false
    local skip_db=false
    local skip_redis=false

    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            --no-verify)
                skip_verify=true
                ;;
            --skip-db)
                skip_db=true
                ;;
            --skip-redis)
                skip_redis=true
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
        shift
    done

    echo "========================================"
    echo "🚀 DataMiddleware 环境安装脚本"
    echo "========================================"

    check_root

    update_packages
    install_go
    install_make

    if [[ $skip_redis != true ]]; then
        install_redis
    fi

    if [[ $skip_db != true ]]; then
        install_mysql
    fi

    install_go_dependencies

    if [[ $skip_verify != true ]]; then
        verify_installation
    fi

    log_success "安装脚本执行完成！"
}

# 运行主函数
main "$@"
