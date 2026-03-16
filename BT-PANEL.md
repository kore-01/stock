# 宝塔面板部署教程

本文档详细介绍如何在宝塔面板中部署 JCP MCP Server。

## 环境要求

- 宝塔面板 7.0+
- Debian 12 / Ubuntu 20.04+ / CentOS 7+
- 已安装 Docker（推荐）或 Supervisor

## 服务器信息

- **IP**: 193.112.101.212
- **系统**: Debian 12
- **SSH**: `ssh root@193.112.101.212`

---

## 方式一：Docker 部署（推荐）

### 步骤 1: 安装 Docker

1. 登录宝塔面板
2. 进入 **软件商店**
3. 搜索 **Docker**
4. 安装 **Docker管理器 3.0+**

或者通过 SSH 安装：

```bash
ssh root@193.112.101.212
curl -fsSL https://get.docker.com | bash
systemctl enable docker
systemctl start docker
```

### 步骤 2: 部署容器

通过 SSH 执行：

```bash
ssh root@193.112.101.212

# 拉取镜像
docker pull ghcr.io/kore-01/jcp-mcp-server:latest

# 创建并启动容器
docker run -d \
  --name jcp-mcp-server \
  --restart unless-stopped \
  -p 8080:8080 \
  -e MCP_MODE=sse \
  -e PORT=8080 \
  -e LOG_LEVEL=info \
  ghcr.io/kore-01/jcp-mcp-server:latest

# 查看运行状态
docker ps

# 查看日志
docker logs -f jcp-mcp-server
```

### 步骤 3: 配置防火墙

在宝塔面板中：

1. 点击左侧 **安全**
2. 在 **放行端口** 中添加：
   - 端口：`8080`
   - 备注：`JCP MCP Server`
3. 点击 **放行**

或者通过 SSH：

```bash
# 使用 UFW
ufw allow 8080/tcp

# 或使用 iptables
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
```

### 步骤 4: 验证部署

```bash
curl http://193.112.101.212:8080/health
```

应返回：
```json
{"status":"ok","version":"1.1.0","time":"2026-03-16T12:00:00Z"}
```

---

## 方式二：宝塔网站 + Nginx 反向代理

### 步骤 1: 创建网站

1. 宝塔面板 → **网站** → **添加站点**
2. 填写信息：
   - **域名**: `mcp.yourdomain.com`（替换为你的域名）
   - **根目录**: `/www/wwwroot/mcp`（任意）
   - **PHP 版本**: **纯静态**
3. 点击 **提交**

### 步骤 2: 部署后端服务

使用 Docker 部署到本地端口 8080：

```bash
ssh root@193.112.101.212

docker run -d \
  --name jcp-mcp-server \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -e MCP_MODE=sse \
  ghcr.io/kore-01/jcp-mcp-server:latest
```

### 步骤 3: 配置反向代理

1. 宝塔面板 → **网站** → 找到刚创建的网站
2. 点击 **设置**
3. 选择 **反向代理** 标签
4. 点击 **添加反向代理**
5. 填写：
   - **代理名称**: `jcp-mcp`
   - **代理目标**: `http://127.0.0.1:8080`
   - **发送域名**: `$host`
6. 点击 **确定**

### 步骤 4: 配置 SSL（可选但推荐）

1. 在网站设置中选择 **SSL**
2. 选择 **Let's Encrypt**
3. 勾选域名，点击 **申请**
4. 开启 **强制 HTTPS**

### 步骤 5: 使用 HTTPS 访问

```json
{
  "mcpServers": {
    "jcp-stock": {
      "url": "https://mcp.yourdomain.com/sse"
    }
  }
}
```

---

## 方式三：宝塔 Supervisor 部署

### 步骤 1: 安装 Supervisor

1. 宝塔面板 → **软件商店**
2. 搜索 **Supervisor**
3. 安装 **Supervisor管理器**

### 步骤 2: 下载程序

```bash
ssh root@193.112.101.212

# 创建目录
mkdir -p /www/jcp-mcp-server
cd /www/jcp-mcp-server

# 下载最新版本
wget https://github.com/kore-01/jcp-mcp-server/releases/latest/download/jcp-mcp-server-linux-amd64

# 重命名并授权
mv jcp-mcp-server-linux-amd64 jcp-mcp-server
chmod +x jcp-mcp-server

# 创建环境变量文件
cat > .env <<EOF
MCP_MODE=sse
PORT=8080
LOG_LEVEL=info
EOF
```

### 步骤 3: 添加守护进程

1. 宝塔面板 → **Supervisor管理器**
2. 点击 **添加守护进程**
3. 填写：
   - **名称**: `jcp-mcp-server`
   - **启动用户**: `root`
   - **运行目录**: `/www/jcp-mcp-server`
   - **启动命令**: `./jcp-mcp-server -mode=sse`
4. 点击 **确定**

### 步骤 4: 配置防火墙

同上，放行端口 8080。

---

## 方式四：Docker Compose（高级）

### 步骤 1: 创建 Compose 文件

```bash
ssh root@193.112.101.212

mkdir -p /www/jcp-mcp-server
cd /www/jcp-mcp-server

cat > docker-compose.yml <<EOF
version: '3.8'

services:
  jcp-mcp-server:
    image: ghcr.io/kore-01/jcp-mcp-server:latest
    container_name: jcp-mcp-server
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - MCP_MODE=sse
      - PORT=8080
      - LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s

  # 可选：Nginx 反向代理
  nginx:
    image: nginx:alpine
    container_name: jcp-mcp-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./ssl:/etc/nginx/ssl
    depends_on:
      - jcp-mcp-server
EOF
```

### 步骤 2: 启动服务

```bash
cd /www/jcp-mcp-server
docker-compose up -d
```

### 步骤 3: 在宝塔中管理

1. 宝塔面板 → **Docker**
2. 可以看到运行的容器
3. 可以进行启动、停止、重启、查看日志等操作

---

## 常用管理命令

### Docker 管理

```bash
# 查看容器状态
docker ps

# 查看日志
docker logs -f jcp-mcp-server

# 重启容器
docker restart jcp-mcp-server

# 停止容器
docker stop jcp-mcp-server

# 删除容器
docker rm -f jcp-mcp-server

# 进入容器
docker exec -it jcp-mcp-server sh
```

### 更新版本

```bash
# 拉取最新镜像
docker pull ghcr.io/kore-01/jcp-mcp-server:latest

# 重启容器
docker restart jcp-mcp-server

# 或重新创建容器
docker stop jcp-mcp-server
docker rm jcp-mcp-server
docker run -d \
  --name jcp-mcp-server \
  --restart unless-stopped \
  -p 8080:8080 \
  ghcr.io/kore-01/jcp-mcp-server:latest
```

### 查看资源使用

```bash
# Docker 资源使用
docker stats jcp-mcp-server

# 系统资源
docker system df
```

---

## MCP 客户端配置示例

### OpenClaw 配置

编辑 `~/.config/openclaw/mcp.json`：

```json
{
  "mcpServers": {
    "jcp-stock": {
      "url": "http://193.112.101.212:8080/sse",
      "description": "宝塔服务器 JCP 股票服务"
    }
  }
}
```

### Claude Desktop 配置

编辑 `%APPDATA%\Claude\settings.json`：

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

## 故障排查

### 容器无法启动

```bash
# 查看错误日志
docker logs jcp-mcp-server

# 检查端口占用
netstat -tlnp | grep 8080

# 查看容器详情
docker inspect jcp-mcp-server
```

### 无法访问服务

```bash
# 本地测试
curl http://localhost:8080/health

# 检查防火墙
ufw status

# 检查宝塔防火墙
bt default
```

### 性能问题

```bash
# 查看资源使用
docker stats

# 限制容器资源
docker update --memory=512m --cpus=1 jcp-mcp-server
```

---

## 安全建议

1. **修改默认端口**: 不要使用 8080，改为随机高端口
2. **配置防火墙**: 只开放必要的端口
3. **使用 HTTPS**: 配置 SSL 证书
4. **定期更新**: 及时更新 Docker 镜像
5. **监控日志**: 定期检查访问日志

---

## 宝塔面板插件推荐

- **Docker管理器**: 管理 Docker 容器
- **Supervisor管理器**: 进程守护管理
- **Nginx防火墙**: Web 应用防火墙
- **系统防火墙**: 系统级防火墙管理

---

## 联系支持

- GitHub Issues: https://github.com/kore-01/jcp-mcp-server/issues
- 邮箱: your-email@example.com
