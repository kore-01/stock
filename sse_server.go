// SSE Server - MCP Server with SSE transport
// 支持 HTTP SSE 传输模式，可被远程访问
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"
)

// runSSEServer 启动 SSE 模式服务器
func runSSEServer() {
	// 加载环境变量
	godotenv.Load()

	// 创建 MCP Server
	s := server.NewMCPServer(
		"JCP Stock MCP Server (SSE)",
		version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	// 注册所有 Tools
	registerStockTools(s)
	registerKLineTools(s)
	registerOrderBookTools(s)
	registerHotTrendTools(s)
	registerMarketTools(s)
	registerLongHuBangTools(s)
	registerResearchReportTools(s)
	registerTelegraphTools(s)

	// 创建 SSE 服务器
	sseServer := server.NewSSEServer(s,
		server.WithBaseURL(getEnv("BASE_URL", "http://localhost:8080")),
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
	)

	// 获取端口
	port := getEnv("PORT", "8080")
	addr := fmt.Sprintf(":%s", port)

	// 创建 HTTP 服务器
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"version": version,
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// MCP SSE 端点
	mux.HandleFunc("/sse", sseServer.ServeHTTP)
	mux.HandleFunc("/message", sseServer.ServeHTTP)

	// 根路径信息
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "JCP Stock MCP Server",
			"version":     version,
			"description": "股票数据服务 MCP Server",
			"endpoints": map[string]string{
				"/health":   "健康检查",
				"/sse":      "MCP SSE 端点",
				"/message":  "MCP 消息端点",
			},
			"tools": []string{
				"get_stock_realtime",
				"search_stocks",
				"get_kline_data",
				"get_order_book",
				"get_market_indices",
				"get_market_status",
				"get_baidu_hot",
				"get_douyin_hot",
				"get_bilibili_hot",
				"get_longhubang",
				"get_research_reports",
				"get_telegraphs",
			},
		})
	})

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("正在关闭服务器...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
	}()

	log.Printf("JCP MCP SSE Server v%s starting on %s", version, addr)
	log.Printf("SSE endpoint: http://localhost%s/sse", addr)
	log.Printf("Health check: http://localhost%s/health", addr)

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// getEnv 获取环境变量，带默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
