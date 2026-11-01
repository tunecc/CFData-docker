package main

import (
	"strings"
	"testing"
)

// 结果操作面板显示设置持久化（cfdata-result-panel）的关键链路断言。
// 回归点：恢复必须晚于 switchMode 初始化渲染，且不被默认值回写覆盖。

func TestResultPanelPersistenceInitOrder(t *testing.T) {
	html := readFrontendSource(t)

	start := strings.Index(html, "window.addEventListener('DOMContentLoaded'")
	if start < 0 {
		t.Fatal("未找到 DOMContentLoaded 初始化块")
	}
	end := strings.Index(html[start:], "});")
	if end < 0 {
		t.Fatal("未找到 DOMContentLoaded 初始化块结尾")
	}
	initBlock := html[start : start+end]

	switchIdx := strings.Index(initBlock, "switchMode('official')")
	restoreIdx := strings.Index(initBlock, "restoreResultPanelSettings()")
	if switchIdx < 0 || restoreIdx < 0 {
		t.Fatal("初始化块缺少 switchMode 或 restoreResultPanelSettings 调用")
	}
	if switchIdx > restoreIdx {
		t.Fatal("restoreResultPanelSettings 必须晚于 switchMode('official')，否则恢复值会被初始化渲染重置")
	}
}

func TestResultPanelPersistenceSaveGate(t *testing.T) {
	html := readFrontendSource(t)

	if !strings.Contains(html, "let resultPanelLoaded = false;") {
		t.Fatal("缺少 resultPanelLoaded 门控标记")
	}
	if !strings.Contains(html, "if (!resultPanelLoaded) return;") {
		t.Fatal("saveResultPanelSettings 必须在初始化读取完成前跳过回写，避免默认值覆盖已存设置")
	}
	if !strings.Contains(html, "resultPanelLoaded = true;") {
		t.Fatal("恢复读取完成后必须放开回写标记")
	}
}

func TestResultPanelRestoreKeepsFieldOrder(t *testing.T) {
	html := readFrontendSource(t)

	start := strings.Index(html, "function restoreResultPanelSettings()")
	end := strings.Index(html[start:], "\n        }")
	if start < 0 || end < 0 {
		t.Fatal("未找到 restoreResultPanelSettings 函数体")
	}
	body := html[start : start+end]
	if strings.Contains(body, "handleResultFormatChange()") {
		t.Fatal("恢复链路不得调用 handleResultFormatChange，它会把字段选择/输出顺序重置为默认值")
	}
	if !strings.Contains(body, "applyResultModeFields()") {
		t.Fatal("恢复链路应按恢复后的来源同步字段显隐")
	}
	if !strings.Contains(body, `setVal('resultIPTypeFilter', s.ipType)`) {
		t.Fatal("恢复链路必须在 applyResultModeFields 重置后回填已存的 IP 类型筛选")
	}
	if !strings.Contains(body, `setVal('resultQualifiedFilter', s.qualified)`) {
		t.Fatal("恢复链路必须在 applyResultModeFields 重置后回填已存的合格筛选")
	}
}
