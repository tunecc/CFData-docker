package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func staticTestHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "web")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}

func TestStaticAssetsServed(t *testing.T) {
	handler := staticTestHandler()
	cases := []struct {
		file   string
		ctype  string
		marker string
	}{
		{"styles.css", "text/css", ".history-list"},
		{"common.js", "javascript", "window.formatEtaText"},
		{"app.js", "javascript", "function updateProgress("},
		{"enhance.js", "javascript", "新界面增强层"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/static/"+tc.file, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /static/%s = %d", tc.file, rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), tc.ctype) {
			t.Fatalf("GET /static/%s Content-Type = %q", tc.file, rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), tc.marker) {
			t.Fatalf("GET /static/%s 缺少标记 %q", tc.file, tc.marker)
		}
	}
}

func TestIndexReferencesSplitAssets(t *testing.T) {
	data, err := staticFiles.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, ref := range []string{
		`<link rel="stylesheet" href="/static/styles.css">`,
		`<script src="/static/common.js"></script>`,
		`<script src="/static/app.js"></script>`,
		`<script src="/static/enhance.js"></script>`,
	} {
		if !strings.Contains(html, ref) {
			t.Fatalf("index.html 缺少引用: %s", ref)
		}
	}
	if !strings.Contains(html, versionLinePlaceholder) {
		t.Fatal("index.html 应保留版本占位（renderIndexHTML 注入依赖）")
	}
}

// TestConfigPanelSingleModeTab 固化右侧配置面板形态：任务模式只在任务控制卡切换，
// 面板仅保留一个跟随当前模式的标签；enhance.js 不得反向调用 switchMode（编辑页切换模式）。
func TestConfigPanelSingleModeTab(t *testing.T) {
	htmlData, err := staticFiles.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlData)
	if strings.Contains(html, `id="cfgTabOfficial"`) || strings.Contains(html, `id="cfgTabNSB"`) {
		t.Fatal("index.html 不应再包含双模式标签 cfgTabOfficial/cfgTabNSB")
	}
	for _, want := range []string{`id="cfgTabMode"`, `id="cfgTabModeLabel"`, `nzShowConfigPanel('mode')`} {
		if !strings.Contains(html, want) {
			t.Fatalf("index.html 缺少当前模式标签标记: %s", want)
		}
	}
	enhanceData, err := os.ReadFile("web/enhance.js")
	if err != nil {
		t.Fatal(err)
	}
	enhance := string(enhanceData)
	if strings.Contains(enhance, "switchMode(") {
		t.Fatal("enhance.js 不应调用 switchMode（配置面板不得切换任务模式）")
	}
	if strings.Contains(enhance, "nzModePanelSyncing") || strings.Contains(enhance, "nzNavMode") {
		t.Fatal("enhance.js 不应再包含面板反向同步 nzModePanelSyncing 或死代码 nzNavMode")
	}
	for _, want := range []string{"cfgTabMode", "cfgTabModeLabel", "官方优选", "非标优选"} {
		if !strings.Contains(enhance, want) {
			t.Fatalf("enhance.js 缺少当前模式标签同步标记: %s", want)
		}
	}
}

// TestIndexLogBoxCompactPlaceholder 固化日志框初始占位行的单行书写形态：
// #logBox 使用 white-space: pre-wrap，源码缩进空白会被渲染成「[系统] 等待操作...」上下的多余空行。
func TestIndexLogBoxCompactPlaceholder(t *testing.T) {
	data, err := staticFiles.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	const want = `<div id="logBox"><div>[系统] 等待操作...</div></div>`
	if !strings.Contains(string(data), want) {
		t.Fatalf("index.html #logBox 初始内容应为单行 %s", want)
	}
}

// readFrontendSource 返回合并后的前端源码（拆分后 JS/CSS 位于 web/ 下）。
func readFrontendSource(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, f := range []string{"index.html", "web/common.js", "web/app.js", "web/enhance.js"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		b.WriteString(string(data))
		b.WriteString("\n")
	}
	return b.String()
}
