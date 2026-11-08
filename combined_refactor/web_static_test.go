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
	for _, f := range []string{"index.html", "web/common.js", "web/validators.js", "web/app.js", "web/enhance.js"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		b.WriteString(string(data))
		b.WriteString("\n")
	}
	return b.String()
}

// TestPWAAssetsAndWiring 固化 PWA 可安装（A8）：manifest/sw/图标随 embed 提供、
// index.html 引用 manifest、enhance.js 以根 scope 注册、main.go 声明 Service-Worker-Allowed。
func TestPWAAssetsAndWiring(t *testing.T) {
	handler := staticTestHandler()
	for _, file := range []string{"site.webmanifest", "sw.js", "icon-192.png", "icon-512.png"} {
		req := httptest.NewRequest(http.MethodGet, "/static/"+file, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /static/%s = %d，PWA 资源缺失", file, rec.Code)
		}
	}
	manifest, err := staticFiles.ReadFile("web/site.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"start_url": "/"`, `"display": "standalone"`, `/static/icon-192.png`, `/static/icon-512.png`} {
		if !strings.Contains(string(manifest), want) {
			t.Fatalf("site.webmanifest 缺少 %s", want)
		}
	}
	sw, err := staticFiles.ReadFile("web/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	// 零缓存透传：不允许出现任何缓存 API
	if strings.Contains(string(sw), "caches") || strings.Contains(string(sw), "cache.put") {
		t.Fatal("sw.js 应为零缓存透传，不得引入离线缓存")
	}
	htmlData, err := staticFiles.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(htmlData), `<link rel="manifest" href="/static/site.webmanifest">`) {
		t.Fatal("index.html 缺少 manifest 引用")
	}
	enhanceData, err := os.ReadFile("web/enhance.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(enhanceData), `navigator.serviceWorker.register('/static/sw.js', { scope: '/' })`) {
		t.Fatal("enhance.js 应以根作用域注册 Service Worker")
	}
	mainData, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainData), `Service-Worker-Allowed`) || !strings.Contains(string(mainData), `/static/sw.js`) {
		t.Fatal("main.go 应为 /static/sw.js 声明 Service-Worker-Allowed: / 根作用域")
	}
}

// TestUIInteractionMarkers 固化交互学习点落地的静态标记：
// A1 hover 分离、A2 16px、A3 缓冲、A4 活校验接线、A5 倒计时/对勾、A6 三态、
// A7 体检面板、A9 计数/截断、A10 广播、A11 aria 与 safe-area、A12 滚动记忆。
func TestUIInteractionMarkers(t *testing.T) {
	cssData, err := os.ReadFile("web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(cssData)
	for _, want := range []string{
		"@media (hover: hover)",       // A1
		"@media (hover: none)",        // A1
		"font-size: 16px",             // A2
		"toast-countdown",             // A5
		"var(--toast-ms, 2.6s)",       // A5 与 JS 同源
		".copied",                     // A5
		".cfg-inline-hint",            // A4 活校验行内提示（统一三档组件）
		".hero-num",                   // A6
		".health-panel",               // A7
		"@supports (padding-top: env(safe-area-inset-top))", // A11
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("styles.css 缺少标记: %s", want)
		}
	}
	htmlData, err := staticFiles.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlData)
	for _, want := range []string{
		`role="log" aria-live="polite"`, // A11
		`aria-pressed="false"`,          // A11
		`id="healthPanelWrap"`,          // A7
		`id="copyResultsBtn"`,           // A9
		`<script src="/static/validators.js"></script>`, // A4
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("index.html 缺少标记: %s", want)
		}
	}
	appData, err := os.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	app := string(appData)
	for _, want := range []string{
		"bindConfigValidators",       // A4
		"nzToggleHealthPanel",        // A7
		"nzRenderHealthPanel",        // A7
		"复制结果 (",                     // A9 计数
		"window.truncateMiddle",      // A9
		"parseTaskDoneBroadcast",     // A10
		"nzFetchLatestOnHistory",     // A10
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("app.js 缺少标记: %s", want)
		}
	}
	enhanceData, err := os.ReadFile("web/enhance.js")
	if err != nil {
		t.Fatal(err)
	}
	enhance := string(enhanceData)
	for _, want := range []string{
		"nzLogBuffer",             // A3
		"nzFlushLogBuffer",        // A3
		"nzCopyFeedback",          // A5
		"nzHeroNum",               // A6
		"TASK_DONE_BROADCAST_KEY", // A10
		"aria-pressed",            // A11
		"nzPanelScrollMemory",     // A12
	} {
		if !strings.Contains(enhance, want) {
			t.Fatalf("enhance.js 缺少标记: %s", want)
		}
	}
}
