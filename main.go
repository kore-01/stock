// JCP MCP Server - 股票数据服务 MCP 服务器
// 提供实时行情、K线数据、盘口数据、热点榜单等功能
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	version         = "1.1.0"
	sinaStockURL    = "http://hq.sinajs.cn/rn=%d&list=%s"
	sinaKLineURL    = "http://quotes.sina.cn/cn/api/json_v2.php/CN_MarketDataService.getKLineData?symbol=%s&scale=%s&ma=5,10,20&datalen=%d"
	baiduHotURL     = "https://top.baidu.com/api/board?platform=wise&tab=realtime"
	douyinHotURL    = "https://www.douyin.com/aweme/v1/web/hot/search/list/"
	bilibiliHotURL  = "https://api.bilibili.com/x/web-interface/ranking/v2?rid=0&type=all"
	lhbListBaseURL  = "https://datacenter-web.eastmoney.com/api/data/v1/get?sortColumns=TRADE_DATE,BILLBOARD_NET_AMT&sortTypes=-1,-1&pageSize=%d&pageNumber=%d&reportName=RPT_DAILYBILLBOARD_DETAILSNEW&columns=SECURITY_CODE,SECUCODE,SECURITY_NAME_ABBR,TRADE_DATE,EXPLAIN,CLOSE_PRICE,CHANGE_RATE,BILLBOARD_NET_AMT,BILLBOARD_BUY_AMT,BILLBOARD_SELL_AMT,BILLBOARD_DEAL_AMT,ACCUM_AMOUNT,DEAL_NET_RATIO,DEAL_AMOUNT_RATIO,TURNOVERRATE,EXPLANATION,D1_CLOSE_ADJCHRATE,D2_CLOSE_ADJCHRATE,D5_CLOSE_ADJCHRATE,D10_CLOSE_ADJCHRATE&source=WEB&client=WEB"
	reportAPI       = "https://reportapi.eastmoney.com/report/list"
	clsTelegraphURL = "https://www.cls.cn/telegraph"
)

var (
	sinaStockRegex = regexp.MustCompile(`var hq_str_(\w+)="([^"]*)"`)
	httpClient     = &http.Client{Timeout: 10 * time.Second}
)

// Stock 股票数据
type Stock struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Change      float64 `json:"change"`
	ChangeRate  float64 `json:"changeRate"`
	Volume      int64   `json:"volume"`
	Turnover    float64 `json:"turnover"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	PrevClose   float64 `json:"prevClose"`
	MarketCap   float64 `json:"marketCap"`
	PeRatio     float64 `json:"peRatio"`
	PbRatio     float64 `json:"pbRatio"`
}

// KLineData K线数据
type KLineData struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
	MA5    float64 `json:"ma5,omitempty"`
	MA10   float64 `json:"ma10,omitempty"`
	MA20   float64 `json:"ma20,omitempty"`
}

// StockSearchResult 股票搜索结果
type StockSearchResult struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Market string `json:"market"`
}

// OrderBook 盘口数据
type OrderBook struct {
	Code       string           `json:"code"`
	BuyOrders  []OrderBookItem  `json:"buyOrders"`
	SellOrders []OrderBookItem  `json:"sellOrders"`
}

// OrderBookItem 盘口条目
type OrderBookItem struct {
	Price  float64 `json:"price"`
	Volume int     `json:"volume"`
}

// HotItem 热点条目
type HotItem struct {
	Rank        int    `json:"rank"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Heat        int    `json:"heat"`
	Platform    string `json:"platform"`
	PlatformCN  string `json:"platformCN"`
}

// MarketIndex 大盘指数
type MarketIndex struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	ChangeRate float64 `json:"changeRate"`
}

// LongHuBangItem 龙虎榜条目
type LongHuBangItem struct {
	TradeDate         string  `json:"tradeDate"`
	SecurityCode      string  `json:"securityCode"`
	SecurityName      string  `json:"securityName"`
	ClosePrice        float64 `json:"closePrice"`
	ChangeRate        float64 `json:"changeRate"`
	BillboardNetAmt   float64 `json:"billboardNetAmt"`
	BillboardBuyAmt   float64 `json:"billboardBuyAmt"`
	BillboardSellAmt  float64 `json:"billboardSellAmt"`
	BillboardDealAmt  float64 `json:"billboardDealAmt"`
	DealNetRatio      float64 `json:"dealNetRatio"`
	TurnoverRate      float64 `json:"turnoverRate"`
	Explanation       string  `json:"explanation"`
	D1CloseAdjChgRate float64 `json:"d1CloseAdjChgRate"`
	D5CloseAdjChgRate float64 `json:"d5CloseAdjChgRate"`
}

// ResearchReport 个股研报
type ResearchReport struct {
	Title              string `json:"title"`
	StockName          string `json:"stockName"`
	StockCode          string `json:"stockCode"`
	OrgSName           string `json:"orgSName"`
	PublishDate        string `json:"publishDate"`
	PredictThisYearEps string `json:"predictThisYearEps"`
	PredictThisYearPe  string `json:"predictThisYearPe"`
	IndvInduName       string `json:"indvInduName"`
	EmRatingName       string `json:"emRatingName"`
	Researcher         string `json:"researcher"`
	InfoCode           string `json:"infoCode"`
}

// Telegraph 财联社快讯
type Telegraph struct {
	Time    string `json:"time"`
	Content string `json:"content"`
	URL     string `json:"url"`
}

func main() {
	// 解析命令行参数
	var mode string
	flag.StringVar(&mode, "mode", "stdio", "运行模式: stdio 或 sse")
	flag.Parse()

	// 检查环境变量
	if envMode := os.Getenv("MCP_MODE"); envMode != "" {
		mode = envMode
	}

	// 根据模式启动
	switch mode {
	case "sse":
		log.Println("启动 SSE 模式服务器...")
		runSSEServer()
	case "stdio":
		fallthrough
	default:
		log.Println("启动 STDIO 模式服务器...")
		runStdioServer()
	}
}

// runStdioServer 启动 STDIO 模式服务器
func runStdioServer() {
	// 加载环境变量
	godotenv.Load()

	// 创建 MCP Server
	s := server.NewMCPServer(
		"JCP Stock MCP Server",
		version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	// 注册 Tools
	registerStockTools(s)
	registerKLineTools(s)
	registerOrderBookTools(s)
	registerHotTrendTools(s)
	registerMarketTools(s)
	registerLongHuBangTools(s)
	registerResearchReportTools(s)
	registerTelegraphTools(s)

	// 启动 stdio 服务器
	log.Printf("JCP MCP Server v%s starting...", version)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// registerStockTools 注册股票相关 Tools
func registerStockTools(s *server.MCPServer) {
	// 1. 获取股票实时数据
	stockTool := mcp.NewTool("get_stock_realtime",
		mcp.WithDescription("获取股票实时行情数据，支持多只股票同时查询"),
		mcp.WithString("codes",
			mcp.Required(),
			mcp.Description("股票代码列表，逗号分隔，如：sh600519,sz000001"),
		),
	)
	s.AddTool(stockTool, handleGetStockRealtime)

	// 2. 搜索股票
	searchTool := mcp.NewTool("search_stocks",
		mcp.WithDescription("根据名称或代码搜索股票"),
		mcp.WithString("keyword",
			mcp.Required(),
			mcp.Description("搜索关键词，如：茅台、600519"),
		),
	)
	s.AddTool(searchTool, handleSearchStocks)
}

// registerKLineTools 注册K线相关 Tools
func registerKLineTools(s *server.MCPServer) {
	klineTool := mcp.NewTool("get_kline_data",
		mcp.WithDescription("获取股票K线数据，支持多种周期"),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("股票代码，如：sh600519"),
		),
		mcp.WithString("period",
			mcp.Required(),
			mcp.Description("K线周期：1m(1分钟), 5m, 15m, 30m, 60m, day(日线), week(周线), month(月线)"),
			mcp.DefaultString("day"),
		),
		mcp.WithNumber("days",
			mcp.Description("获取天数，默认60天"),
			mcp.DefaultNumber(60),
		),
	)
	s.AddTool(klineTool, handleGetKLineData)
}

// registerOrderBookTools 注册盘口相关 Tools
func registerOrderBookTools(s *server.MCPServer) {
	orderbookTool := mcp.NewTool("get_order_book",
		mcp.WithDescription("获取股票五档盘口数据"),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("股票代码，如：sh600519"),
		),
	)
	s.AddTool(orderbookTool, handleGetOrderBook)
}

// registerHotTrendTools 注册热点相关 Tools
func registerHotTrendTools(s *server.MCPServer) {
	// 1. 百度热搜
	baiduTool := mcp.NewTool("get_baidu_hot",
		mcp.WithDescription("获取百度热搜榜单"),
		mcp.WithNumber("limit",
			mcp.Description("返回条数，默认20"),
			mcp.DefaultNumber(20),
		),
	)
	s.AddTool(baiduTool, handleGetBaiduHot)

	// 2. 抖音热搜
	douyinTool := mcp.NewTool("get_douyin_hot",
		mcp.WithDescription("获取抖音热搜榜单"),
		mcp.WithNumber("limit",
			mcp.Description("返回条数，默认20"),
			mcp.DefaultNumber(20),
		),
	)
	s.AddTool(douyinTool, handleGetDouyinHot)

	// 3. B站热搜
	bilibiliTool := mcp.NewTool("get_bilibili_hot",
		mcp.WithDescription("获取Bilibili热搜榜单"),
		mcp.WithNumber("limit",
			mcp.Description("返回条数，默认20"),
			mcp.DefaultNumber(20),
		),
	)
	s.AddTool(bilibiliTool, handleGetBilibiliHot)
}

// registerMarketTools 注册市场相关 Tools
func registerMarketTools(s *server.MCPServer) {
	// 1. 大盘指数
	indexTool := mcp.NewTool("get_market_indices",
		mcp.WithDescription("获取大盘指数行情（上证指数、深证成指、创业板指）"),
	)
	s.AddTool(indexTool, handleGetMarketIndices)

	// 2. 市场状态
	statusTool := mcp.NewTool("get_market_status",
		mcp.WithDescription("获取当前市场状态（交易中/休市等）"),
	)
	s.AddTool(statusTool, handleGetMarketStatus)
}

// handleGetStockRealtime 处理获取股票实时数据
func handleGetStockRealtime(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	codes, ok := request.Params.Arguments["codes"].(string)
	if !ok || codes == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: "股票代码不能为空"}},
			IsError: true,
		}, nil
	}

	stocks, err := fetchStockRealTime(codes)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取股票数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(stocks, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// handleSearchStocks 处理搜索股票
func handleSearchStocks(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	keyword, ok := request.Params.Arguments["keyword"].(string)
	if !ok || keyword == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: "搜索关键词不能为空"}},
			IsError: true,
		}, nil
	}

	// 从内置股票列表中搜索
	results := searchStocksFromEmbedded(keyword)

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// handleGetKLineData 处理获取K线数据
func handleGetKLineData(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, _ := request.Params.Arguments["code"].(string)
	period, _ := request.Params.Arguments["period"].(string)
	days, _ := request.Params.Arguments["days"].(float64)

	if code == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: "股票代码不能为空"}},
			IsError: true,
		}, nil
	}
	if period == "" {
		period = "day"
	}
	if days == 0 {
		days = 60
	}

	klines, err := fetchKLineData(code, period, int(days))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取K线数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(klines, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// handleGetOrderBook 处理获取盘口数据
func handleGetOrderBook(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, ok := request.Params.Arguments["code"].(string)
	if !ok || code == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: "股票代码不能为空"}},
			IsError: true,
		}, nil
	}

	orderbook, err := fetchOrderBook(code)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取盘口数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(orderbook, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// handleGetBaiduHot 处理获取百度热搜
func handleGetBaiduHot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit, _ := request.Params.Arguments["limit"].(float64)
	if limit == 0 {
		limit = 20
	}

	items, err := fetchBaiduHot(int(limit))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取百度热搜失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// handleGetDouyinHot 处理获取抖音热搜
func handleGetDouyinHot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit, _ := request.Params.Arguments["limit"].(float64)
	if limit == 0 {
		limit = 20
	}

	items, err := fetchDouyinHot(int(limit))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取抖音热搜失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// handleGetBilibiliHot 处理获取B站热搜
func handleGetBilibiliHot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit, _ := request.Params.Arguments["limit"].(float64)
	if limit == 0 {
		limit = 20
	}

	items, err := fetchBilibiliHot(int(limit))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取B站热搜失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// handleGetMarketIndices 处理获取大盘指数
func handleGetMarketIndices(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	indices, err := fetchMarketIndices()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取大盘指数失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(indices, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// handleGetMarketStatus 处理获取市场状态
func handleGetMarketStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status := getMarketStatus()

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// fetchStockRealTime 从新浪获取股票实时数据
func fetchStockRealTime(codes string) ([]Stock, error) {
	url := fmt.Sprintf(sinaStockURL, time.Now().UnixNano(), codes)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseSinaStockData(string(body))
}

// parseSinaStockData 解析新浪股票数据
func parseSinaStockData(data string) ([]Stock, error) {
	var stocks []Stock
	matches := sinaStockRegex.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 3 || match[2] == "" {
			continue
		}
		parts := strings.Split(match[2], ",")
		if len(parts) < 32 {
			continue
		}

		stock := parseStockFields(match[1], parts)
		stocks = append(stocks, stock)
	}
	return stocks, nil
}

// parseStockFields 解析股票字段
func parseStockFields(code string, parts []string) Stock {
	price, _ := strconv.ParseFloat(parts[3], 64)
	prevClose, _ := strconv.ParseFloat(parts[2], 64)
	open, _ := strconv.ParseFloat(parts[1], 64)
	high, _ := strconv.ParseFloat(parts[4], 64)
	low, _ := strconv.ParseFloat(parts[5], 64)
	volume, _ := strconv.ParseInt(parts[8], 10, 64)
	turnover, _ := strconv.ParseFloat(parts[9], 64)

	change := price - prevClose
	changeRate := 0.0
	if prevClose > 0 {
		changeRate = change / prevClose * 100
	}

	return Stock{
		Code:       code,
		Name:       strings.TrimSpace(parts[0]),
		Price:      price,
		Change:     change,
		ChangeRate: changeRate,
		Volume:     volume,
		Turnover:   turnover,
		Open:       open,
		High:       high,
		Low:        low,
		PrevClose:  prevClose,
	}
}

// fetchKLineData 获取K线数据
func fetchKLineData(code, period string, days int) ([]KLineData, error) {
	scale := periodToScale(period)
	url := fmt.Sprintf(sinaKLineURL, code, scale, days)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseKLineData(string(body))
}

// periodToScale 转换周期到新浪格式
func periodToScale(period string) string {
	switch period {
	case "1m":
		return "1"
	case "5m":
		return "5"
	case "15m":
		return "15"
	case "30m":
		return "30"
	case "60m":
		return "60"
	case "week":
		return "week"
	case "month":
		return "month"
	default:
		return "240"
	}
}

// parseKLineData 解析K线数据
func parseKLineData(data string) ([]KLineData, error) {
	// 移除不必要的字符
	data = strings.TrimPrefix(data, "(")
	data = strings.TrimSuffix(data, ")")

	var rawData []map[string]interface{}
	if err := json.Unmarshal([]byte(data), &rawData); err != nil {
		return nil, err
	}

	var klines []KLineData
	for _, item := range rawData {
		kline := KLineData{
			Date:   getString(item, "day"),
			Open:   getFloat(item, "open"),
			High:   getFloat(item, "high"),
			Low:    getFloat(item, "low"),
			Close:  getFloat(item, "close"),
			Volume: int64(getFloat(item, "volume")),
		}
		if ma, ok := item["ma5"].(float64); ok {
			kline.MA5 = ma
		}
		if ma, ok := item["ma10"].(float64); ok {
			kline.MA10 = ma
		}
		if ma, ok := item["ma20"].(float64); ok {
			kline.MA20 = ma
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

// fetchOrderBook 获取盘口数据
func fetchOrderBook(code string) (*OrderBook, error) {
	stocks, err := fetchStockRealTime(code)
	if err != nil {
		return nil, err
	}
	if len(stocks) == 0 {
		return nil, fmt.Errorf("未找到股票: %s", code)
	}

	// 重新获取完整数据包含盘口
	url := fmt.Sprintf(sinaStockURL, time.Now().UnixNano(), code)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Referer", "http://finance.sina.com.cn")
	resp, _ := httpClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	return parseOrderBookData(code, string(body))
}

// parseOrderBookData 解析盘口数据
func parseOrderBookData(code, data string) (*OrderBook, error) {
	matches := sinaStockRegex.FindAllStringSubmatch(data, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("解析数据失败")
	}

	parts := strings.Split(matches[0][2], ",")
	if len(parts) < 32 {
		return nil, fmt.Errorf("数据格式错误")
	}

	orderbook := &OrderBook{Code: code}

	// 解析买盘 (第11-20位)
	for i := 0; i < 5; i++ {
		priceIdx := 11 + i*2
		volIdx := 12 + i*2
		price, _ := strconv.ParseFloat(parts[priceIdx], 64)
		vol, _ := strconv.Atoi(parts[volIdx])
		if price > 0 {
			orderbook.BuyOrders = append(orderbook.BuyOrders, OrderBookItem{
				Price:  price,
				Volume: vol,
			})
		}
	}

	// 解析卖盘 (第21-30位)
	for i := 0; i < 5; i++ {
		priceIdx := 21 + i*2
		volIdx := 22 + i*2
		price, _ := strconv.ParseFloat(parts[priceIdx], 64)
		vol, _ := strconv.Atoi(parts[volIdx])
		if price > 0 {
			orderbook.SellOrders = append(orderbook.SellOrders, OrderBookItem{
				Price:  price,
				Volume: vol,
			})
		}
	}

	return orderbook, nil
}

// fetchBaiduHot 获取百度热搜
func fetchBaiduHot(limit int) ([]HotItem, error) {
	resp, err := httpClient.Get(baiduHotURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Cards []struct {
				Content []struct {
					Content []struct {
						Word  string `json:"word"`
						URL   string `json:"url"`
						Index int    `json:"index"`
					} `json:"content"`
				} `json:"content"`
			} `json:"cards"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var items []HotItem
	for _, card := range result.Data.Cards {
		for _, content := range card.Content {
			for _, item := range content.Content {
				items = append(items, HotItem{
					Rank:       item.Index,
					Title:      item.Word,
					URL:        item.URL,
					Heat:       100 - item.Index,
					Platform:   "baidu",
					PlatformCN: "百度热搜",
				})
				if len(items) >= limit {
					break
				}
			}
			if len(items) >= limit {
				break
			}
		}
		if len(items) >= limit {
			break
		}
	}

	return items, nil
}

// fetchDouyinHot 获取抖音热搜
func fetchDouyinHot(limit int) ([]HotItem, error) {
	req, _ := http.NewRequest("GET", douyinHotURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			WordList []struct {
				Word     string `json:"word"`
				Position int    `json:"position"`
			} `json:"word_list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var items []HotItem
	for _, item := range result.Data.WordList {
		items = append(items, HotItem{
			Rank:       item.Position,
			Title:      item.Word,
			Heat:       100 - item.Position,
			Platform:   "douyin",
			PlatformCN: "抖音热搜",
		})
		if len(items) >= limit {
			break
		}
	}

	return items, nil
}

// fetchBilibiliHot 获取B站热搜
func fetchBilibiliHot(limit int) ([]HotItem, error) {
	resp, err := httpClient.Get(bilibiliHotURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			List []struct {
				Title string `json:"title"`
				Bvid  string `json:"bvid"`
				Stat  struct {
					View int `json:"view"`
				} `json:"stat"`
			} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var items []HotItem
	for i, item := range result.Data.List {
		items = append(items, HotItem{
			Rank:       i + 1,
			Title:      item.Title,
			URL:        fmt.Sprintf("https://www.bilibili.com/video/%s", item.Bvid),
			Heat:       item.Stat.View,
			Platform:   "bilibili",
			PlatformCN: "B站热门",
		})
		if len(items) >= limit {
			break
		}
	}

	return items, nil
}

// fetchMarketIndices 获取大盘指数
func fetchMarketIndices() ([]MarketIndex, error) {
	codes := "s_sh000001,s_sz399001,s_sz399006"
	stocks, err := fetchStockRealTime(codes)
	if err != nil {
		return nil, err
	}

	nameMap := map[string]string{
		"s_sh000001": "上证指数",
		"s_sz399001": "深证成指",
		"s_sz399006": "创业板指",
	}

	var indices []MarketIndex
	for _, stock := range stocks {
		indices = append(indices, MarketIndex{
			Code:       stock.Code,
			Name:       nameMap[stock.Code],
			Price:      stock.Price,
			Change:     stock.Change,
			ChangeRate: stock.ChangeRate,
		})
	}

	return indices, nil
}

// getMarketStatus 获取市场状态
type MarketStatus struct {
	Status     string `json:"status"`
	StatusText string `json:"statusText"`
	IsTradeDay bool   `json:"isTradeDay"`
}

func getMarketStatus() MarketStatus {
	now := time.Now()
	hour := now.Hour()
	minute := now.Minute()
	timeVal := hour*100 + minute
	weekday := now.Weekday()

	// 判断是否是交易日（周末休市）
	isTradeDay := weekday != time.Saturday && weekday != time.Sunday

	var status, statusText string
	switch {
	case timeVal >= 930 && timeVal < 1130:
		status = "trading"
		statusText = "交易中（上午）"
	case timeVal >= 1130 && timeVal < 1300:
		status = "lunch_break"
		statusText = "午间休市"
	case timeVal >= 1300 && timeVal < 1500:
		status = "trading"
		statusText = "交易中（下午）"
	case timeVal >= 1500:
		status = "closed"
		statusText = "已收盘"
	default:
		status = "pre_market"
		statusText = "盘前"
	}

	if !isTradeDay {
		status = "closed"
		statusText = "周末休市"
	}

	return MarketStatus{
		Status:     status,
		StatusText: statusText,
		IsTradeDay: isTradeDay,
	}
}

// searchStocksFromEmbedded 从内置列表搜索股票
func searchStocksFromEmbedded(keyword string) []StockSearchResult {
	// 这里简化处理，实际应该从内置的 stock_basic.json 加载
	// 返回匹配的A股列表
	var results []StockSearchResult
	keyword = strings.ToLower(keyword)

	// 预定义的常见股票数据
	stockList := []struct {
		Code   string
		Name   string
		Pinyin string
	}{
		{"sh600519", "贵州茅台", "maotai"},
		{"sz000001", "平安银行", "pinganyinhang"},
		{"sh600036", "招商银行", "zhaoshangyinhang"},
		{"sz000002", "万科A", "wanke"},
		{"sh600276", "恒瑞医药", "hengruiyiyao"},
		{"sh600030", "中信证券", "zhongxinzhengquan"},
		{"sz000858", "五粮液", "wuliangye"},
		{"sh601318", "中国平安", "zhongguopingan"},
		{"sz002594", "比亚迪", "biyadi"},
		{"sh600887", "伊利股份", "yiligufen"},
		{"sh600844", "丹化科技", "danhuakeji"},
		{"sz300059", "东方财富", "dongfangcaifu"},
		{"sh600000", "浦发银行", "pufayinhang"},
	}

	for _, s := range stockList {
		if strings.Contains(strings.ToLower(s.Code), keyword) ||
			strings.Contains(strings.ToLower(s.Name), keyword) ||
			strings.Contains(s.Pinyin, keyword) {
			// 提取市场前缀
			market := "sh"
			if strings.HasPrefix(s.Code, "sz") {
				market = "sz"
			}
			results = append(results, StockSearchResult{
				Code:   s.Code,
				Name:   s.Name,
				Market: market,
			})
		}
	}

	return results
}

// getString 从 map 获取字符串
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getFloat 从 map 获取 float64
func getFloat(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

// newToolResultText 创建正确的 MCP 文本结果
func newToolResultText(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: text,
			},
		},
	}
}

// ========== 龙虎榜功能 ==========

// registerLongHuBangTools 注册龙虎榜相关 Tools
func registerLongHuBangTools(s *server.MCPServer) {
	lhbTool := mcp.NewTool("get_longhubang",
		mcp.WithDescription("获取龙虎榜数据（每日活跃营业部买卖情况）"),
		mcp.WithString("tradeDate",
			mcp.Description("交易日期，格式 YYYY-MM-DD，为空则获取最新"),
		),
		mcp.WithNumber("pageSize",
			mcp.Description("每页数量，默认50"),
			mcp.DefaultNumber(50),
		),
		mcp.WithNumber("pageNumber",
			mcp.Description("页码，默认1"),
			mcp.DefaultNumber(1),
		),
	)
	s.AddTool(lhbTool, handleGetLongHuBang)
}

// handleGetLongHuBang 处理获取龙虎榜
func handleGetLongHuBang(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tradeDate, _ := request.Params.Arguments["tradeDate"].(string)
	pageSize, _ := request.Params.Arguments["pageSize"].(float64)
	pageNumber, _ := request.Params.Arguments["pageNumber"].(float64)

	if pageSize == 0 {
		pageSize = 50
	}
	if pageNumber == 0 {
		pageNumber = 1
	}

	items, err := fetchLongHuBang(int(pageSize), int(pageNumber), tradeDate)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取龙虎榜失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// fetchLongHuBang 获取龙虎榜数据
func fetchLongHuBang(pageSize, pageNumber int, tradeDate string) ([]LongHuBangItem, error) {
	url := fmt.Sprintf(lhbListBaseURL, pageSize, pageNumber)
	if tradeDate != "" {
		url += fmt.Sprintf("&filter=(TRADE_DATE%%3D%%27%s%%27)", tradeDate)
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result struct {
			Data []struct {
				TradeDate         string  `json:"TRADE_DATE"`
				SecurityCode      string  `json:"SECURITY_CODE"`
				SecurityNameAbbr  string  `json:"SECURITY_NAME_ABBR"`
				ClosePrice        float64 `json:"CLOSE_PRICE"`
				ChangeRate        float64 `json:"CHANGE_RATE"`
				BillboardNetAmt   float64 `json:"BILLBOARD_NET_AMT"`
				BillboardBuyAmt   float64 `json:"BILLBOARD_BUY_AMT"`
				BillboardSellAmt  float64 `json:"BILLBOARD_SELL_AMT"`
				BillboardDealAmt  float64 `json:"BILLBOARD_DEAL_AMT"`
				DealNetRatio      float64 `json:"DEAL_NET_RATIO"`
				TurnoverRate      float64 `json:"TURNOVERRATE"`
				Explanation       string  `json:"EXPLANATION"`
				D1CloseAdjChgRate float64 `json:"D1_CLOSE_ADJCHRATE"`
				D5CloseAdjChgRate float64 `json:"D5_CLOSE_ADJCHRATE"`
			} `json:"data"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var items []LongHuBangItem
	for _, d := range result.Result.Data {
		items = append(items, LongHuBangItem{
			TradeDate:         d.TradeDate,
			SecurityCode:      d.SecurityCode,
			SecurityName:      d.SecurityNameAbbr,
			ClosePrice:        d.ClosePrice,
			ChangeRate:        d.ChangeRate,
			BillboardNetAmt:   d.BillboardNetAmt,
			BillboardBuyAmt:   d.BillboardBuyAmt,
			BillboardSellAmt:  d.BillboardSellAmt,
			BillboardDealAmt:  d.BillboardDealAmt,
			DealNetRatio:      d.DealNetRatio,
			TurnoverRate:      d.TurnoverRate,
			Explanation:       d.Explanation,
			D1CloseAdjChgRate: d.D1CloseAdjChgRate,
			D5CloseAdjChgRate: d.D5CloseAdjChgRate,
		})
	}

	return items, nil
}

// ========== 研报功能 ==========

// registerResearchReportTools 注册研报相关 Tools
func registerResearchReportTools(s *server.MCPServer) {
	reportTool := mcp.NewTool("get_research_reports",
		mcp.WithDescription("获取个股研报列表"),
		mcp.WithString("stockCode",
			mcp.Required(),
			mcp.Description("股票代码，如：600519 或 sh600519"),
		),
		mcp.WithNumber("pageSize",
			mcp.Description("每页数量，默认10"),
			mcp.DefaultNumber(10),
		),
		mcp.WithNumber("pageNo",
			mcp.Description("页码，默认1"),
			mcp.DefaultNumber(1),
		),
	)
	s.AddTool(reportTool, handleGetResearchReports)
}

// handleGetResearchReports 处理获取研报
func handleGetResearchReports(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stockCode, _ := request.Params.Arguments["stockCode"].(string)
	pageSize, _ := request.Params.Arguments["pageSize"].(float64)
	pageNo, _ := request.Params.Arguments["pageNo"].(float64)

	if stockCode == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: "股票代码不能为空"}},
			IsError: true,
		}, nil
	}

	// 去除前缀
	stockCode = strings.TrimPrefix(stockCode, "sz")
	stockCode = strings.TrimPrefix(stockCode, "sh")

	if pageSize == 0 {
		pageSize = 10
	}
	if pageNo == 0 {
		pageNo = 1
	}

	reports, err := fetchResearchReports(stockCode, int(pageSize), int(pageNo))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取研报失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// fetchResearchReports 获取研报数据
func fetchResearchReports(stockCode string, pageSize, pageNo int) ([]ResearchReport, error) {
	url := fmt.Sprintf("%s?industryCode=*&pageSize=%d&industry=*&rating=*&ratingChange=*&beginTime=2020-01-01&endTime=%d-01-01&pageNo=%d&fields=&qType=0&orgCode=&code=%s&rcode=",
		reportAPI, pageSize, time.Now().Year()+1, pageNo, stockCode)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com/")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			Title              string `json:"title"`
			StockName          string `json:"stockName"`
			StockCode          string `json:"stockCode"`
			OrgSName           string `json:"orgSName"`
			PublishDate        string `json:"publishDate"`
			PredictThisYearEps string `json:"predictThisYearEps"`
			PredictThisYearPe  string `json:"predictThisYearPe"`
			IndvInduName       string `json:"indvInduName"`
			EmRatingName       string `json:"emRatingName"`
			Researcher         string `json:"researcher"`
			InfoCode           string `json:"infoCode"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var reports []ResearchReport
	for _, d := range result.Data {
		reports = append(reports, ResearchReport{
			Title:              d.Title,
			StockName:          d.StockName,
			StockCode:          d.StockCode,
			OrgSName:           d.OrgSName,
			PublishDate:        d.PublishDate,
			PredictThisYearEps: d.PredictThisYearEps,
			PredictThisYearPe:  d.PredictThisYearPe,
			IndvInduName:       d.IndvInduName,
			EmRatingName:       d.EmRatingName,
			Researcher:         d.Researcher,
			InfoCode:           d.InfoCode,
		})
	}

	return reports, nil
}

// ========== 财联社快讯功能 ==========

// registerTelegraphTools 注册快讯相关 Tools
func registerTelegraphTools(s *server.MCPServer) {
	telegraphTool := mcp.NewTool("get_telegraphs",
		mcp.WithDescription("获取财联社最新快讯"),
		mcp.WithNumber("limit",
			mcp.Description("返回条数，默认20"),
			mcp.DefaultNumber(20),
		),
	)
	s.AddTool(telegraphTool, handleGetTelegraphs)
}

// handleGetTelegraphs 处理获取快讯
func handleGetTelegraphs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit, _ := request.Params.Arguments["limit"].(float64)
	if limit == 0 {
		limit = 20
	}

	tels, err := fetchTelegraphs(int(limit))
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("获取快讯失败: %v", err)}},
			IsError: true,
		}, nil
	}

	data, err := json.MarshalIndent(tels, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Text: fmt.Sprintf("序列化数据失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return newToolResultText(string(data)), nil
}

// fetchTelegraphs 获取财联社快讯
func fetchTelegraphs(limit int) ([]Telegraph, error) {
	req, _ := http.NewRequest("GET", clsTelegraphURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 简单的正则提取快讯内容
	telegraphs := extractTelegraphsFromHTML(string(body), limit)
	return telegraphs, nil
}

// extractTelegraphsFromHTML 从HTML中提取快讯
func extractTelegraphsFromHTML(html string, limit int) []Telegraph {
	var telegraphs []Telegraph

	// 尝试匹配 JSON 数据
	re := regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*({.+?});`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		// 尝试解析 JSON
		var state map[string]interface{}
		if err := json.Unmarshal([]byte(matches[1]), &state); err == nil {
			if telegraphList, ok := state["telegraphList"].([]interface{}); ok {
				for i, item := range telegraphList {
					if i >= limit {
						break
					}
					if m, ok := item.(map[string]interface{}); ok {
						tel := Telegraph{
							Time:    getString(m, "time"),
							Content: cleanContent(getString(m, "content")),
							URL:     getString(m, "url"),
						}
						if tel.Content != "" {
							telegraphs = append(telegraphs, tel)
						}
					}
				}
			}
		}
	}

	return telegraphs
}

// cleanContent 清理内容
func cleanContent(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}
