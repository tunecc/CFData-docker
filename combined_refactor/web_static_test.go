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
