#!/bin/bash
# 自动部署 JCP MCP Server 到 GZ 服务器
# 服务器信息: Debian 12, IP: 193.112.101.212

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 服务器配置
SERVER_HOST="193.112.101.212"
SERVER_USER="root"
INSTALL_DIR="/opt/jcp-mcp-server"
SERVICE_NAME="jcp-mcp-server"

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

# 检查 SSH 连接
check_ssh() {
    info "检查 SSH 连接..."
    if ! ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_HOST}" "echo 'SSH OK'" > /dev/null 2>&1; then
        error "无法连接到服务器 ${SERVER_HOST}"
        error "请确保可以通过 SSH 访问: ssh ${SERVER_USER}@${SERVER_HOST}"
        exit 1
    fi
    success "SSH 连接正常"
}

# 部署函数
deploy() {
    info "开始部署 JCP MCP Server 到 ${SERVER_HOST}..."

    # 在服务器上执行部署命令
    ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_HOST}" << 'REMOTE_SCRIPT'
        set -e

        # 颜色定义
        GREEN='\033[0;32m'
        BLUE='\033[0;34m'
        NC='\033[0m'

        echo -e "${BLUE}[INFO]${NC} 更新系统包..."
        apt-get update -qq

        echo -e "${BLUE}[INFO]${NC} 安装依赖..."
        apt-get install -y -qq curl wget git

        # 检查并安装 Go
        if ! command -v go &> /dev/null; then
            echo -e "${BLUE}[INFO]${NC} 安装 Go..."
            wget -q https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
            tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
            rm go1.24.0.linux-amd64.tar.gz
            echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
            export PATH=$PATH:/usr/local/go/bin
        fi

        # 验证 Go 安装
        GO_VERSION=$(/usr/local/go/bin/go version 2>/dev/null || go version)
        echo -e "${GREEN}[SUCCESS]${NC} Go 版本: $GO_VERSION"

        # 克隆仓库
        echo -e "${BLUE}[INFO]${NC} 下载 JCP MCP Server..."
        if [ -d "/opt/jcp-mcp-server" ]; then
            cd /opt/jcp-mcp-server
            git pull origin main || true
        else
            git clone https://github.com/kore-01/jcp-mcp-server.git /opt/jcp-mcp-server
            cd /opt/jcp-mcp-server
        fi

        # 编译
        echo -e "${BLUE}[INFO]${NC} 编译程序..."
        /usr/local/go/bin/go mod download
        /usr/local/go/bin/go build -ldflags="-s -w" -o jcp-mcp-server main.go sse_server.go

        # 创建 systemd 服务
        echo -e "${BLUE}[INFO]${NC} 创建服务..."
        cat > /etc/systemd/system/jcp-mcp-server.service << 'EOF'
[Unit]
Description=JCP MCP Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/jcp-mcp-server
ExecStart=/opt/jcp-mcp-server/jcp-mcp-server -mode=sse
Restart=always
RestartSec=5
Environment="MCP_MODE=sse"
Environment="PORT=8080"
Environment="LOG_LEVEL=info"

[Install]
WantedBy=multi-user.target
EOF

        # 重载并启动服务
        systemctl daemon-reload
        systemctl enable jcp-mcp-server

        # 停止旧服务（如果存在）
        systemctl stop jcp-mcp-server 2>/dev/null || true

        # 启动新服务
        systemctl start jcp-mcp-server

        # 等待服务启动
        sleep 3

        # 检查服务状态
        if systemctl is-active --quiet jcp-mcp-server; then
            echo -e "${GREEN}[SUCCESS]${NC} 服务启动成功"
        else
            echo -e "${RED}[ERROR]${NC} 服务启动失败"
            journalctl -u jcp-mcp-server -n 20 --no-pager
            exit 1
        fi

        # 配置防火墙
        echo -e "${BLUE}[INFO]${NC} 配置防火墙..."
        if command -v ufw &> /dev/null; then
            ufw allow 8080/tcp >/dev/null 2>&1 || true
            echo -e "${GREEN}[SUCCESS]${NC} UFW 防火墙配置完成"
        fi

        iptables -C INPUT -p tcp --dport 8080 -j ACCEPT 2>/dev/null || \
            iptables -A INPUT -p tcp --dport 8080 -j ACCEPT

        # 测试服务
        echo -e "${BLUE}[INFO]${NC} 测试服务..."
        sleep 2
        if curl -s http://localhost:8080/health | grep -q "ok"; then
            echo -e "${GREEN}[SUCCESS]${NC} 服务运行正常"
        else
            echo -e "${YELLOW}[WARNING]${NC} 服务可能未完全启动，请稍后手动检查"
        fi

        # 打印信息
        echo ""
        echo "=========================================="
        echo -e "${GREEN}  JCP MCP Server 部署完成！${NC}"
        echo "=========================================="
        echo ""
        echo "服务地址:"
        echo "  - SSE: http://${SERVER_HOST}:8080/sse"
        echo "  - Health: http://${SERVER_HOST}:8080/health"
        echo ""
        echo "管理命令:"
        echo "  查看状态: systemctl status jcp-mcp-server"
        echo "  查看日志: journalctl -u jcp-mcp-server -f"
        echo "  重启服务: systemctl restart jcp-mcp-server"
        echo ""
        echo "=========================================="
REMOTE_SCRIPT

    success "部署完成！"
    echo ""
    echo "访问地址:"
    echo "  SSE: http://${SERVER_HOST}:8080/sse"
    echo "  Health: http://${SERVER_HOST}:8080/health"
    echo ""
    echo "MCP 客户端配置:"
    echo '  {'
    echo '    "mcpServers": {'
    echo '      "jcp-stock": {'
    echo "        \"url\": \"http://${SERVER_HOST}:8080/sse\""
    echo '      }'
    echo '    }'
    echo '  }'
}

# 显示菜单
show_menu() {
    echo ""
    echo "=========================================="
    echo "  JCP MCP Server 部署工具"
    echo "=========================================="
    echo ""
    echo "目标服务器: ${SERVER_USER}@${SERVER_HOST}"
    echo ""
    echo "请选择操作:"
    echo "  1. 完整部署 (克隆→编译→启动)"
    echo "  2. 仅更新代码 (拉取最新代码并重启)"
    echo "  3. 查看服务状态"
    echo "  4. 查看日志"
    echo "  5. 重启服务"
    echo "  6. 卸载服务"
    echo "  0. 退出"
    echo ""
    echo "=========================================="
}

# 更新代码
update_code() {
    info "更新代码..."
    ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_HOST}" << 'EOF'
        cd /opt/jcp-mcp-server
        git pull origin main
        /usr/local/go/bin/go build -ldflags="-s -w" -o jcp-mcp-server main.go sse_server.go
        systemctl restart jcp-mcp-server
        echo "更新完成！"
EOF
}

# 查看状态
view_status() {
    ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_HOST}" "systemctl status jcp-mcp-server --no-pager"
}

# 查看日志
view_logs() {
    ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_HOST}" "journalctl -u jcp-mcp-server -f -n 100"
}

# 重启服务
restart_service() {
    info "重启服务..."
    ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_HOST}" "systemctl restart jcp-mcp-server"
    success "服务已重启"
}

# 卸载服务
uninstall() {
    warning "这将卸载 JCP MCP Server 并删除所有数据！"
    read -p "确定要继续吗? (yes/no): " confirm
    if [ "$confirm" = "yes" ]; then
        ssh -o StrictHostKeyChecking=no "${SERVER_USER}@${SERVER_HOST}" << 'EOF'
            systemctl stop jcp-mcp-server
            systemctl disable jcp-mcp-server
            rm -f /etc/systemd/system/jcp-mcp-server.service
            systemctl daemon-reload
            rm -rf /opt/jcp-mcp-server
            ufw delete allow 8080/tcp 2>/dev/null || true
            echo "卸载完成"
EOF
        success "卸载完成"
    else
        info "已取消"
    fi
}

# 主函数
main() {
    # 检查参数
    if [ "$1" = "--auto" ]; then
        check_ssh
        deploy
        exit 0
    fi

    # 显示菜单循环
    while true; do
        show_menu
        read -p "请输入选项 [0-6]: " choice

        case $choice in
            1)
                check_ssh
                deploy
                ;;
            2)
                update_code
                ;;
            3)
                view_status
                ;;
            4)
                view_logs
                ;;
            5)
                restart_service
                ;;
            6)
                uninstall
                ;;
            0)
                echo "再见!"
                exit 0
                ;;
            *)
                error "无效选项"
                ;;
        esac

        echo ""
        read -p "按回车键继续..."
    done
}

main "$@"
