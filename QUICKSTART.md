# JCP MCP Server 快速开始指南

本文档帮助你在 5 分钟内完成从代码到部署的全过程。

## 目录

1. [推送到 GitHub](#推送到-github)
2. [部署到服务器](#部署到服务器)
3. [配置 MCP 客户端](#配置-mcp-客户端)
4. [验证部署](#验证部署)

---

## 推送到 GitHub

### 步骤 1: 创建 GitHub 仓库

1. 访问 https://github.com/new
2. 仓库名称: `jcp-mcp-server`
3. 选择 **Public** 或 **Private**
4. 不要初始化 README（已存在）
5. 点击 **Create repository**

### 步骤 2: 推送代码

```bash
# 进入项目目录
cd D:\ai\jcp-mcp-server-github

# 初始化 git 仓库
git init

# 添加所有文件
git add .

# 提交
git commit -m "Initial commit: JCP MCP Server v1.1.0"

# 添加远程仓库（替换为你的用户名）
git remote add origin https://github.com/kore-01/jcp-mcp-server.git

# 推送
git push -u origin main
```

### 步骤 3: 验证推送

访问 `https://github.com/kore-01/jcp-mcp-server` 确认代码已推送。

---

## 部署到服务器

### 方式一：一键自动部署（推荐）

在本地运行部署脚本：

```bash
# 进入项目目录
cd D:\ai\jcp-mcp-server-github

# 运行部署脚本（需要配置 SSH）
bash deploy/deploy-to-gz.sh
```

按照提示选择：
- 输入 `1` 进行完整部署
- 脚本会自动连接服务器、安装依赖、编译、启动服务

### 方式二：GitHub Actions 自动部署

#### 配置 Secrets

1. 在 GitHub 仓库页面，点击 **Settings** → **Secrets and variables** → **Actions**
2. 添加以下 secrets：

| Secret 名称 | 值 |
|------------|-----|
| `SERVER_HOST` | `193.112.101.212` |
| `SERVER_USER` | `root` |
| `SERVER_PASSWORD` | 你的服务器密码 |
| `SERVER_SSH_KEY` | SSH 私钥（可选） |

#### 触发部署

每次推送到 `main` 分支会自动部署：

```bash
git push origin main
```

或者在 GitHub 页面：
1. 点击 **Actions** 标签
2. 选择 **Deploy to GZ Server**
3. 点击 **Run workflow**

### 方式三：手动部署到服务器

SSH 连接到服务器并执行：

```bash
# 连接服务器
ssh root@193.112.101.212

# 运行一键安装脚本
curl -fsSL https://raw.githubusercontent.com/kore-01/jcp-mcp-server/main/deploy/install.sh | bash
```

或者手动步骤：

```bash
# 连接服务器
ssh root@193.112.101.212

# 安装 Go
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 克隆仓库
git clone https://github.com/kore-01/jcp-mcp-server.git /opt/jcp-mcp-server
cd /opt/jcp-mcp-server

# 编译
go mod download
go build -ldflags="-s -w" -o jcp-mcp-server main.go sse_server.go

# 创建服务
cat > /etc/systemd/system/jcp-mcp-server.service <<'EOF'
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

[Install]
WantedBy=multi-user.target
EOF

# 启动服务
systemctl daemon-reload
systemctl enable jcp-mcp-server
systemctl start jcp-mcp-server

# 开放防火墙
ufw allow 8080/tcp
```

### 方式四：Docker 部署

```bash
ssh root@193.112.101.212

# 安装 Docker
curl -fsSL https://get.docker.com | bash

# 运行容器
docker run -d \
  --name jcp-mcp-server \
  --restart unless-stopped \
  -p 8080:8080 \
  ghcr.io/kore-01/jcp-mcp-server:latest
```

---

## 配置 MCP 客户端

### OpenClaw

编辑配置文件（位置：`~/.config/openclaw/mcp.json`）：

```json
{
  "mcpServers": {
    "jcp-stock": {
      "url": "http://193.112.101.212:8080/sse",
      "description": "JCP 股票数据服务"
    }
  }
}
```

重启 OpenClaw：

```bash
openclaw restart
```

### Claude Desktop

编辑配置文件：

- Windows: `%APPDATA%\Claude\settings.json`
- macOS: `~/Library/Application Support/Claude/settings.json`

```json
{
  "mcpServers": {
    "jcp-stock": {
      "url": "http://193.112.101.212:8080/sse"
    }
  }
}
```

重启 Claude Desktop。

### 其他 MCP 客户端

通用配置格式：

```json
{
  "mcpServers": {
    "jcp-stock": {
      "url": "http://193.112.101.212:8080/sse"
    }
  }
}
```

---

## 验证部署

### 1. 检查服务状态

```bash
ssh root@193.112.101.212 "systemctl status jcp-mcp-server"
```

### 2. 测试 HTTP 接口

```bash
# 健康检查
curl http://193.112.101.212:8080/health

# 预期返回
{"status":"ok","version":"1.1.0","time":"2026-03-16T12:00:00Z"}
```

### 3. 在 MCP 客户端中测试

向 OpenClaw 或 Claude 提问：

```
贵州茅台今天的股价是多少？
```

如果 MCP 客户端正确配置，它会自动调用 `get_stock_realtime` 工具。

---

## 管理命令

### 查看日志

```bash
ssh root@193.112.101.212 "journalctl -u jcp-mcp-server -f"
```

### 重启服务

```bash
ssh root@193.112.101.212 "systemctl restart jcp-mcp-server"
```

### 更新到最新版本

```bash
# 方式 1: 使用部署脚本
cd D:\ai\jcp-mcp-server-github
bash deploy/deploy-to-gz.sh
# 选择选项 2 (更新代码)

# 方式 2: 手动更新
ssh root@193.112.101.212 << 'EOF'
cd /opt/jcp-mcp-server
git pull
/usr/local/go/bin/go build -ldflags="-s -w" -o jcp-mcp-server main.go sse_server.go
systemctl restart jcp-mcp-server
EOF
```

---

## 故障排查

### 无法连接到服务器

```bash
# 测试 SSH
ssh root@193.112.101.212 echo "OK"

# 检查服务器是否在线
ping 193.112.101.212
```

### 服务无法启动

```bash
# 查看错误日志
ssh root@193.112.101.212 "journalctl -u jcp-mcp-server -n 50 --no-pager"
```

### 端口无法访问

```bash
# 检查防火墙
ssh root@193.112.101.212 "ufw status"

# 检查端口监听
ssh root@193.112.101.212 "netstat -tlnp | grep 8080"
```

### MCP 客户端无法连接

1. 确认服务运行：`curl http://193.112.101.212:8080/health`
2. 检查防火墙设置
3. 确认 MCP 配置格式正确

---

## 安全建议

### 1. 修改默认端口

编辑服务配置：

```bash
ssh root@193.112.101.212
systemctl edit jcp-mcp-server
```

添加：
```ini
[Service]
Environment="PORT=18080"
```

重启服务：
```bash
systemctl daemon-reload
systemctl restart jcp-mcp-server
```

更新防火墙：
```bash
ufw delete allow 8080/tcp
ufw allow 18080/tcp
```

### 2. 配置 Nginx + SSL

参考 [BT-PANEL.md](./BT-PANEL.md) 中的宝塔面板配置。

### 3. 限制访问 IP

在防火墙中只允许特定 IP 访问：

```bash
ufw allow from YOUR_IP to any port 8080
```

---

## 获取帮助

- GitHub Issues: https://github.com/kore-01/jcp-mcp-server/issues
- 部署文档: [DEPLOY.md](./DEPLOY.md)
- 宝塔面板: [BT-PANEL.md](./BT-PANEL.md)

---

## 下一步

1. ✅ 推送到 GitHub
2. ✅ 部署到服务器
3. ✅ 配置 MCP 客户端
4. ✅ 验证部署

现在你可以在 OpenClaw 或 Claude 中使用股票数据服务了！
