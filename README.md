# JCP MCP Server

股票数据服务 MCP Server，提供实时行情、K线数据、盘口数据、龙虎榜、研报、财联社快讯、热点榜单等功能。

## 功能特性

- 股票实时行情查询
- K线数据（支持多种周期）
- 五档盘口数据
- 大盘指数
- 市场状态
- 龙虎榜数据
- 个股研报
- 财联社快讯
- 热点榜单（百度、抖音、B站）

## 快速开始

### 本地运行

```bash
# 克隆仓库
git clone https://github.com/kore-01/jcp-mcp-server.git
cd jcp-mcp-server

# 编译
go build -ldflags="-s -w" -o jcp-mcp-server main.go sse_server.go

# 运行（STDIO 模式）
./jcp-mcp-server

# 或 SSE 模式
./jcp-mcp-server -mode=sse
```

### Docker 部署

```bash
# 构建镜像
docker build -t jcp-mcp-server .

# 运行容器
docker run -d -p 8080:8080 --name jcp-mcp jcp-mcp-server
```

### 服务器部署

使用一键部署脚本：

```bash
# 下载并运行部署脚本
curl -fsSL https://raw.githubusercontent.com/kore-01/jcp-mcp-server/main/deploy/install.sh | bash
```

详细部署文档：[DEPLOY.md](./DEPLOY.md)

## 配置

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `MCP_MODE` | 运行模式：stdio 或 sse | `stdio` |
| `PORT` | SSE 模式监听端口 | `8080` |
| `BASE_URL` | SSE 服务基础 URL | `http://localhost:8080` |
| `LOG_LEVEL` | 日志级别 | `info` |

### MCP 客户端配置

#### OpenClaw

```json
{
  "mcpServers": {
    "jcp-stock": {
      "command": "/usr/local/bin/jcp-mcp-server",
      "args": [],
      "env": {}
    }
  }
}
```

#### Claude Desktop

```json
{
  "mcpServers": {
    "jcp-stock": {
      "command": "/usr/local/bin/jcp-mcp-server",
      "args": []
    }
  }
}
```

#### SSE 远程连接

```json
{
  "mcpServers": {
    "jcp-stock": {
      "url": "http://your-server:8080/sse"
    }
  }
}
```

## 可用工具

| 工具名 | 描述 |
|--------|------|
| `get_stock_realtime` | 获取股票实时行情 |
| `search_stocks` | 搜索股票 |
| `get_kline_data` | 获取K线数据 |
| `get_order_book` | 获取五档盘口 |
| `get_market_indices` | 获取大盘指数 |
| `get_market_status` | 获取市场状态 |
| `get_longhubang` | 获取龙虎榜数据 |
| `get_research_reports` | 获取个股研报 |
| `get_telegraphs` | 获取财联社快讯 |
| `get_baidu_hot` | 获取百度热搜 |
| `get_douyin_hot` | 获取抖音热搜 |
| `get_bilibili_hot` | 获取B站热门 |

## 文档

- [部署指南](./DEPLOY.md) - 详细的服务器部署教程
- [宝塔面板配置](./BT-PANEL.md) - 宝塔面板部署教程
- [OpenClaw 集成](./OPENCLAW-INTEGRATION.md) - OpenClaw 集成教程

## 技术栈

- Go 1.24+
- MCP Go SDK
- 新浪财经 API
- 东方财富 API
- 财联社 API

## License

MIT License
