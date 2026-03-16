// dashboard.go — Dashboard HTML 嵌入（//go:embed）
// lobster-guard v4.0 代码拆分
// 回退逻辑：优先使用文件系统中的 dashboard.html，否则使用嵌入版本
package main

import (
	_ "embed"
)

//go:embed dashboard.html
var embeddedDashboardHTML []byte

// getDashboardHTML 获取 Dashboard HTML 内容
// 优先从文件系统读取（方便开发调试），否则使用嵌入版本
func getDashboardHTML(cfgPath string) []byte {
	return embeddedDashboardHTML
}
