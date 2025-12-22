#!/bin/bash

# 数据中间件构建脚本
# 用于构建和打包应用程序

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
BUILD_DIR="bin"
OUTPUT_NAME="datamiddleware"
GOOS=${GOOS:-"linux"}
GOARCH=${GOARCH:-"amd64"}
VERSION=${VERSION:-"dev"}
COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--output)
            OUTPUT_NAME="$2"
            shift 2
            ;;
        --os)
            GOOS="$2"
            shift 2
            ;;
        --arch)
            GOARCH="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -d|--dir)
            BUILD_DIR="$2"
            shift 2
            ;;
        --clean)
            CLEAN=true
            shift
            ;;
        -h|--help)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  -o, --output NAME     输出文件名 (默认: datamiddleware)"
            echo "  --os OS               目标操作系统 (默认: linux)"
            echo "  --arch ARCH           目标架构 (默认: amd64)"
            echo "  -v, --version VER     版本号 (默认: dev)"
            echo "  -d, --dir DIR         构建目录 (默认: build)"
            echo "  --clean               清理构建目录"
            echo "  -h, --help            显示帮助信息"
            exit 0
            ;;
        *)
            log_error "未知选项: $1"
            echo "使用 $0 --help 查看帮助信息"
            exit 1
            ;;
    esac
done

# 清理函数
cleanup() {
    if [[ "$CLEAN" == "true" ]]; then
        log_info "清理构建目录: $BUILD_DIR"
        rm -rf "$BUILD_DIR"
    fi
}

# 主构建函数
main() {
    log_info "开始构建数据中间件..."
    log_info "构建信息:"
    echo "  输出文件名: $OUTPUT_NAME"
    echo "  目标平台: $GOOS/$GOARCH"
    echo "  版本: $VERSION"
    echo "  提交: $COMMIT"
    echo "  构建时间: $BUILD_TIME"

    # 检查Go环境
    if ! command -v go &> /dev/null; then
        log_error "Go未安装或不在PATH中"
        exit 1
    fi

    # 显示Go版本
    GO_VERSION=$(go version)
    log_info "Go版本: $GO_VERSION"

    # 设置构建环境变量
    export GOOS=$GOOS
    export GOARCH=$GOARCH
    export CGO_ENABLED=0

    # 清理旧的构建目录
    rm -rf "$BUILD_DIR"
    mkdir -p "$BUILD_DIR"

    # 下载依赖
    log_info "下载依赖..."
    go mod download
    go mod tidy

    # 构建主程序
    log_info "编译主程序..."
    go build \
        -ldflags "-X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildTime=$BUILD_TIME" \
        -o "$BUILD_DIR/$OUTPUT_NAME" \
        ./cmd/server

    if [[ $? -ne 0 ]]; then
        log_error "编译失败"
        exit 1
    fi

    # 复制配置文件
    log_info "复制配置文件..."
    cp -r configs "$BUILD_DIR/"
    cp README.md "$BUILD_DIR/"

    # 创建启动脚本
    cat > "$BUILD_DIR/start.sh" << EOF
#!/bin/bash
# 数据中间件启动脚本

set -e

# 默认配置
CONFIG_FILE="configs/config.yaml"
LOG_LEVEL="info"

# 解析命令行参数
while [[ \$# -gt 0 ]]; do
    case \$1 in
        -c|--config)
            CONFIG_FILE="\$2"
            shift 2
            ;;
        -l|--log-level)
            LOG_LEVEL="\$2"
            shift 2
            ;;
        -h|--help)
            echo "用法: \$0 [选项]"
            echo ""
            echo "选项:"
            echo "  -c, --config FILE    配置文件路径 (默认: configs/config.yaml)"
            echo "  -l, --log-level LEVEL 日志级别 (默认: info)"
            echo "  -h, --help           显示帮助信息"
            exit 0
            ;;
        *)
            echo "未知选项: \$1"
            echo "使用 \$0 --help 查看帮助信息"
            exit 1
            ;;
    esac
done

# 检查配置文件是否存在
if [[ ! -f "\$CONFIG_FILE" ]]; then
    echo "错误: 配置文件不存在: \$CONFIG_FILE"
    exit 1
fi

# 设置环境变量
export LOG_LEVEL="\$LOG_LEVEL"

# 启动应用
echo "启动数据中间件..."
echo "配置文件: \$CONFIG_FILE"
echo "日志级别: \$LOG_LEVEL"

exec ./datamiddleware
EOF

    chmod +x "$BUILD_DIR/start.sh"

    # 生成构建信息文件
    cat > "$BUILD_DIR/build.info" << EOF
构建信息:
版本: $VERSION
提交: $COMMIT
构建时间: $BUILD_TIME
Go版本: $GO_VERSION
目标平台: $GOOS/$GOARCH
EOF

    log_success "构建完成!"
    log_info "构建产物位于: $BUILD_DIR/"
    ls -la "$BUILD_DIR/"

    # 显示构建摘要
    BINARY_SIZE=$(du -h "$BUILD_DIR/$OUTPUT_NAME" | cut -f1)
    log_info "二进制文件大小: $BINARY_SIZE"
}

# 设置退出时的清理
trap cleanup EXIT

# 执行主函数
main "$@"
