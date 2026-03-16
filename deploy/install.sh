#!/bin/bash
# JCP MCP Server 一键安装脚本
# 支持 Debian/Ubuntu/CentOS

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
REPO="kore-01/jcp-mcp-server"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="jcp-mcp-server"
CONFIG_DIR="/etc/jcp-mcp-server"
DATA_DIR="/var/lib/jcp-mcp-server"
LOG_DIR="/var/log/jcp-mcp-server"

# 默认配置
DEFAULT_MODE="sse"
DEFAULT_PORT="8080"

# 打印信息
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查是否为 root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "请使用 root 权限运行此脚本"
        exit 1
    fi
}

# 检测系统类型
detect_os() {
    if [[ -f /etc/debian_version ]]; then
        OS="debian"
        info "检测到 Debian/Ubuntu 系统"
    elif [[ -f /etc/redhat-release ]]; then
        OS="centos"
        info "检测到 CentOS/RHEL 系统"
    else
        error "不支持的操作系统"
        exit 1
    fi
}

# 安装依赖
install_dependencies() {
    info "安装依赖..."

    if [[ "$OS" == "debian" ]]; then
        apt-get update
        apt-get install -y curl wget systemd
    elif [[ "$OS" == "centos" ]]; then
        yum install -y curl wget systemd
    fi

    success "依赖安装完成"
}

# 获取最新版本
get_latest_version() {
    info "获取最新版本..."

    LATEST_VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$LATEST_VERSION" ]]; then
        error "无法获取最新版本，请检查网络连接"
        exit 1
    fi

    info "最新版本: $LATEST_VERSION"
}

# 下载二进制文件
download_binary() {
    info "下载 JCP MCP Server..."

    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            SUFFIX="linux-amd64"
            ;;
        aarch64)
            SUFFIX="linux-arm64"
            ;;
        *)
            error "不支持的架构: $ARCH"
            exit 1
            ;;
    esac

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/jcp-mcp-server-${SUFFIX}"

    info "下载地址: $DOWNLOAD_URL"

    if ! wget -q --show-progress -O "${INSTALL_DIR}/jcp-mcp-server" "$DOWNLOAD_URL"; then
        error "下载失败"
        exit 1
    fi

    chmod +x "${INSTALL_DIR}/jcp-mcp-server"
    success "下载完成"
}

# 创建目录结构
create_directories() {
    info "创建目录结构..."

    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"

    success "目录创建完成"
}

# 创建配置文件
create_config() {
    info "创建配置文件..."

    cat > "${CONFIG_DIR}/env" <<EOF
# JCP MCP Server 配置
MCP_MODE=${DEFAULT_MODE}
PORT=${DEFAULT_PORT}
BASE_URL=http://0.0.0.0:${DEFAULT_PORT}
LOG_LEVEL=info
EOF

    success "配置文件创建完成"
}

# 创建 Systemd 服务
create_systemd_service() {
    info "创建 Systemd 服务..."

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=JCP MCP Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${DATA_DIR}
EnvironmentFile=${CONFIG_DIR}/env
ExecStart=${INSTALL_DIR}/jcp-mcp-server -mode=\${MCP_MODE}
Restart=always
RestartSec=5
StandardOutput=append:${LOG_DIR}/jcp-mcp-server.log
StandardError=append:${LOG_DIR}/jcp-mcp-server.error.log

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    success "Systemd 服务创建完成"
}

# 启动服务
start_service() {
    info "启动 JCP MCP Server..."

    systemctl enable "$SERVICE_NAME"
    systemctl start "$SERVICE_NAME"

    sleep 2

    if systemctl is-active --quiet "$SERVICE_NAME"; then
        success "服务启动成功"
    else
        error "服务启动失败，请检查日志"
        journalctl -u "$SERVICE_NAME" -n 20
        exit 1
    fi
}

# 配置防火墙
configure_firewall() {
    info "配置防火墙..."

    if command -v ufw &> /dev/null; then
        ufw allow "$DEFAULT_PORT/tcp" || true
        success "UFW 防火墙配置完成"
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-port="$DEFAULT_PORT/tcp" || true
        firewall-cmd --reload || true
        success "Firewalld 配置完成"
    else
        warning "未检测到防火墙，请手动开放端口 $DEFAULT_PORT"
    fi
}

# 打印安装信息
print_info() {
    echo ""
    echo "=========================================="
    echo "  JCP MCP Server 安装完成！"
    echo "=========================================="
    echo ""
    echo -e "${GREEN}版本:${NC} $LATEST_VERSION"
    echo -e "${GREEN}安装路径:${NC} ${INSTALL_DIR}/jcp-mcp-server"
    echo -e "${GREEN}配置文件:${NC} ${CONFIG_DIR}/env"
    echo -e "${GREEN}日志文件:${NC} ${LOG_DIR}/"
    echo -e "${GREEN}服务状态:${NC} $(systemctl is-active $SERVICE_NAME)"
    echo ""
    echo -e "${YELLOW}服务管理命令:${NC}"
    echo "  启动: systemctl start $SERVICE_NAME"
    echo "  停止: systemctl stop $SERVICE_NAME"
    echo "  重启: systemctl restart $SERVICE_NAME"
    echo "  状态: systemctl status $SERVICE_NAME"
    echo "  日志: journalctl -u $SERVICE_NAME -f"
    echo ""
    echo -e "${YELLOW}访问地址:${NC}"
    echo "  SSE: http://$(curl -s ifconfig.me):$DEFAULT_PORT/sse"
    echo "  Health: http://$(curl -s ifconfig.me):$DEFAULT_PORT/health"
    echo ""
    echo -e "${YELLOW}MCP 客户端配置:${NC}"
    cat <<'MCP_CONFIG'
{
  "mcpServers": {
    "jcp-stock": {
      "url": "http://YOUR_SERVER_IP:8080/sse"
    }
  }
}
MCP_CONFIG
    echo ""
    echo "=========================================="
}

# 主函数
main() {
    echo "=========================================="
    echo "  JCP MCP Server 安装程序"
    echo "=========================================="
    echo ""

    check_root
    detect_os
    install_dependencies
    get_latest_version
    download_binary
    create_directories
    create_config
    create_systemd_service
    configure_firewall
    start_service
    print_info
}

# 运行主函数
main "$@"
