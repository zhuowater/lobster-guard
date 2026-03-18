// dashboard.go — Dashboard 前端资源嵌入（go:embed）
// lobster-guard v6.1 — Vue 3 + Vite 构建产物
package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dashboard/dist/*
var dashboardFS embed.FS

// getDashboardHandler 返回 Dashboard 静态文件 handler
func getDashboardHandler() http.Handler {
	sub, err := fs.Sub(dashboardFS, "dashboard/dist")
	if err != nil {
		// fallback: 本地文件系统（开发模式）
		return http.FileServer(http.Dir("dashboard/dist"))
	}
	return http.FileServer(http.FS(sub))
}
